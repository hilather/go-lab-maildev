#!/usr/bin/env bash
# Container contract for DEP-001. Requires Docker. Fail closed if the daemon
# is missing so this is a real check, not an unimplemented stub.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${LABMAIL_TEST_IMAGE:-ghcr.io/hilather/labmail:test}"
NAME="labmail-container-test-$$"
CFG="${ROOT}/testdata/container/config.yaml"
COMPOSE="${ROOT}/examples/compose.smoke.yaml"

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required for make test-container" >&2
	exit 1
fi
if ! docker info >/dev/null 2>&1; then
	echo "docker daemon is not available for make test-container" >&2
	exit 1
fi

cleanup() {
	docker rm -f "${NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "building ${IMAGE}"
docker build -t "${IMAGE}" "${ROOT}"

inspect_user="$(docker image inspect --format '{{.Config.User}}' "${IMAGE}")"
if [ "${inspect_user}" != "65532:65532" ]; then
	echo "image User=${inspect_user}, want 65532:65532" >&2
	exit 1
fi

licenses="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.licenses"}}' "${IMAGE}")"
if [ "${licenses}" != "Apache-2.0" ]; then
	echo "image license label=${licenses}, want Apache-2.0" >&2
	exit 1
fi

hc="$(docker image inspect --format '{{json .Config.Healthcheck.Test}}' "${IMAGE}")"
case "${hc}" in
*CMD-SHELL*)
	echo "image HEALTHCHECK=${hc} must be exec form, not shell" >&2
	exit 1
	;;
esac
case "${hc}" in
'["CMD",'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want JSON array starting with CMD" >&2
	exit 1
	;;
esac
case "${hc}" in
*'/v1/health/ready'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want /v1/health/ready" >&2
	exit 1
	;;
esac
case "${hc}" in
*healthcheck*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want exec-form labmail healthcheck" >&2
	exit 1
	;;
esac
case "${hc}" in
*node*|*1025*)
	echo "image HEALTHCHECK=${hc} must be HTTP ready, not SMTP/node" >&2
	exit 1
	;;
esac

if docker compose version >/dev/null 2>&1; then
	docker compose -f "${COMPOSE}" config >/dev/null
else
	echo "docker compose plugin not available; compose file parse skipped" >&2
fi

docker run -d --name "${NAME}" \
	--read-only \
	--cap-drop=ALL \
	--security-opt=no-new-privileges:true \
	--tmpfs /tmp:rw,noexec,nosuid,size=16m \
	-v "${CFG}:/etc/labmail/config.yaml:ro" \
	-p 127.0.0.1::1025/tcp \
	-p 127.0.0.1::1080/tcp \
	"${IMAGE}"

readonly_root="$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "${NAME}")"
if [ "${readonly_root}" != "true" ]; then
	echo "HostConfig.ReadonlyRootfs=${readonly_root}, want true" >&2
	exit 1
fi

assert_identity() {
	local uid capeef
	# Prefer in-container /proc/self/status (works when the client cannot
	# see the container PID, e.g. Docker Desktop / remote DOCKER_HOST).
	if status="$(docker exec "${NAME}" /labmail debug-status 2>/dev/null)"; then
		uid="$(printf '%s\n' "${status}" | awk '/^Uid:/{print $2; exit}')"
		capeef="$(printf '%s\n' "${status}" | awk '/^CapEff:/{print $2; exit}')"
		if [ "${uid}" = "65532" ] && [ "${capeef}" = "0000000000000000" ]; then
			return 0
		fi
		echo "in-container Uid=${uid} CapEff=${capeef}, want 65532 / 0000000000000000" >&2
		return 1
	fi
	local pid
	pid="$(docker inspect --format '{{.State.Pid}}' "${NAME}")"
	if [ -r "/proc/${pid}/status" ]; then
		uid="$(awk '/^Uid:/{print $2}' "/proc/${pid}/status")"
		capeef="$(awk '/^CapEff:/{print $2}' "/proc/${pid}/status")"
		if [ "${uid}" = "65532" ] && [ "${capeef}" = "0000000000000000" ]; then
			return 0
		fi
		echo "host /proc Uid=${uid} CapEff=${capeef}, want 65532 / 0000000000000000" >&2
		return 1
	fi
	local capdrop privileged
	capdrop="$(docker inspect --format '{{json .HostConfig.CapDrop}}' "${NAME}")"
	privileged="$(docker inspect --format '{{.HostConfig.Privileged}}' "${NAME}")"
	case "${capdrop}" in
	*ALL*)
		;;
	*)
		echo "cannot read CapEff (need Linux Docker); CapDrop=${capdrop}, want ALL" >&2
		return 1
		;;
	esac
	if [ "${privileged}" != "false" ]; then
		echo "cannot read CapEff (need Linux Docker); Privileged=${privileged}, want false" >&2
		return 1
	fi
	echo "CapEff not readable from this client; accepted CapDrop=${capdrop} Privileged=${privileged}" >&2
	return 0
}
assert_identity

mgmt_port="$(docker port "${NAME}" 1080/tcp | head -n1 | awk -F: '{print $NF}')"
smtp_port="$(docker port "${NAME}" 1025/tcp | head -n1 | awk -F: '{print $NF}')"

ok=0
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:${mgmt_port}/v1/health/ready" >/dev/null 2>&1; then
		ok=1
		break
	fi
	sleep 0.25
done
if [ "${ok}" -ne 1 ]; then
	echo "management ready check failed on 127.0.0.1:${mgmt_port}" >&2
	docker logs "${NAME}" >&2 || true
	exit 1
fi

# The image has no shell; docker exec still runs the copied binary.
if ! docker exec "${NAME}" /labmail version >/dev/null; then
	echo "non-root exec of /labmail version failed" >&2
	exit 1
fi
if ! docker exec "${NAME}" /labmail healthcheck --url=http://127.0.0.1:1080/v1/health/ready >/dev/null; then
	echo "in-container HTTP ready healthcheck failed" >&2
	exit 1
fi
if docker exec "${NAME}" /bin/sh -c true >/dev/null 2>&1; then
	echo "image has a shell at /bin/sh" >&2
	exit 1
fi
if docker exec "${NAME}" /bin/busybox true >/dev/null 2>&1; then
	echo "image has busybox" >&2
	exit 1
fi
if ! docker exec "${NAME}" /labmail debug-status --check-readonly >/dev/null; then
	echo "read-only root check failed (could write /probe-ro)" >&2
	exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
	echo "python3 is required to deliver SMTP in make test-container" >&2
	exit 1
fi
python3 - "${smtp_port}" <<'PY'
import smtplib
import sys
from email.mime.text import MIMEText

port = int(sys.argv[1])
msg = MIMEText("container contract\n")
msg["Subject"] = "container-smoke"
msg["From"] = "alice@lab.test"
msg["To"] = "bob@lab.test"
with smtplib.SMTP("127.0.0.1", port, timeout=5) as s:
    s.sendmail("alice@lab.test", ["bob@lab.test"], msg.as_string())
PY

listed="$(curl -fsS "http://127.0.0.1:${mgmt_port}/v1/messages")"
if ! printf '%s\n' "${listed}" | grep -q 'container-smoke'; then
	echo "GET /v1/messages missing container-smoke subject: ${listed}" >&2
	exit 1
fi

echo "container contract ok image=${IMAGE}"
