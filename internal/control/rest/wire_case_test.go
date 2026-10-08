package rest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
)

func applyBody(rev, op string) string {
	return fmt.Sprintf(`{"expectedRevision":%q,"reason":"repro","operations":[%s]}`, rev, op)
}

// TestRESTRejectsCaseVariantBareDuration: canonical greetingDelay: 30 is
// rejected. The same bare number under GreetingDelay must be rejected too.
// encoding/json otherwise installs it as 30ns.
func TestRESTRejectsCaseVariantBareDuration(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	h := s.Handler()

	canonical := applyBody(rev, `{"op":"replaceSMTPBehavior","behavior":{"greetingDelay":30}}`)
	rec := doReq(t, h, http.MethodPost, "/v1/changes:apply", canonical)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
	if got := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay; got != 0 {
		t.Fatalf("canonical bare number changed greetingDelay to %s", got)
	}

	cased := applyBody(rev, `{"op":"replaceSMTPBehavior","behavior":{"GreetingDelay":30}}`)
	rec = doReq(t, h, http.MethodPost, "/v1/changes:apply", cased)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
	if got := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay; got != 0 {
		t.Fatalf("GreetingDelay bare number applied as %s", got)
	}
}

// TestRESTRejectsCaseVariantBareByteSize: maxBytes must be a unit string.
// MaxBytes: 1024 must not install a 1024-byte inbox cap.
func TestRESTRejectsCaseVariantBareByteSize(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	before := svc.Active().Canonical.Spec.Store.MaxBytes
	h := s.Handler()

	canonical := applyBody(rev, `{"op":"replaceStoreCaps","store":{"maxMessages":1000,"maxBytes":1024,"fullPolicy":"reject"}}`)
	rec := doReq(t, h, http.MethodPost, "/v1/changes:apply", canonical)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")

	cased := applyBody(rev, `{"op":"replaceStoreCaps","store":{"maxMessages":1000,"MaxBytes":1024,"fullPolicy":"reject"}}`)
	rec = doReq(t, h, http.MethodPost, "/v1/changes:apply", cased)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
	if got := svc.Active().Canonical.Spec.Store.MaxBytes; got != before {
		t.Fatalf("MaxBytes bare number applied as %d (was %d)", got, before)
	}
}

// TestRESTRejectsUnknownChangeFields: DisallowUnknownFields is set on the
// request decoder, so an unknown member must be validation_failed.
func TestRESTRejectsUnknownChangeFields(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	body := applyBody(rev, `{"op":"replaceHideExtensions","hideExtensions":["SIZE"]}`)
	body = body[:len(body)-1] + `,"notAField":true}`
	rec := doReq(t, s.Handler(), http.MethodPost, "/v1/changes:apply", body)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
	if len(svc.Active().Canonical.Spec.SMTP.HideExtensions) > 0 {
		t.Fatal("unknown field was ignored and the change applied")
	}
	if !strings.Contains(rec.Body.String(), "notAField") {
		t.Fatalf("problem did not name the field: %s", rec.Body.String())
	}
}

func TestRESTAppliesCaseVariantDurationString(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	body := applyBody(rev, `{"op":"replaceSMTPBehavior","behavior":{"GreetingDelay":"30s"}}`)
	rec := doReq(t, s.Handler(), http.MethodPost, "/v1/changes:apply", body)
	requireStatus(t, rec, http.StatusOK)
	if got := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay; got != 30*time.Second {
		t.Fatalf("GreetingDelay string applied as %s body=%s", got, rec.Body.String())
	}
}

func TestRESTAppliesCaseVariantByteSizeString(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	body := applyBody(rev, `{"op":"replaceStoreCaps","store":{"maxMessages":1000,"MaxBytes":"10MiB","fullPolicy":"reject"}}`)
	rec := doReq(t, s.Handler(), http.MethodPost, "/v1/changes:apply", body)
	requireStatus(t, rec, http.StatusOK)
	const want = int64(10 << 20)
	if got := svc.Active().Canonical.Spec.Store.MaxBytes; got != want {
		t.Fatalf("MaxBytes string applied as %d body=%s", got, rec.Body.String())
	}
}

func TestRESTValidateStateCaseVariantKeyStillUnknown(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"state":{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x"},"spec":{"smtp":{"behavior":{"GreetingDelay":"0s"}}}}}`
	rec := doReq(t, s.Handler(), http.MethodPost, "/v1/state:validate", body)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
	if !strings.Contains(rec.Body.String(), "GreetingDelay") && !strings.Contains(rec.Body.String(), "unknown") {
		t.Fatalf("state case-variant key was not an unknown field: %s", rec.Body.String())
	}
}

func TestRESTApplyIgnoresStateMemberCaseVariant(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	state := `{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x"},"spec":{"smtp":{"behavior":{"GreetingDelay":"0s"}}}}`
	body := fmt.Sprintf(`{"expectedRevision":%q,"reason":"repro","operations":[{"op":"replaceHideExtensions","hideExtensions":["SIZE"]}],"state":%s}`, rev, state)
	rec := doReq(t, s.Handler(), http.MethodPost, "/v1/changes:apply", body)
	requireStatus(t, rec, http.StatusOK)
	got := svc.Active().Canonical.Spec.SMTP.HideExtensions
	if len(got) != 1 || got[0] != "SIZE" {
		t.Fatalf("apply with state member did not apply: %v body=%s", got, rec.Body.String())
	}
}

// caseVariantBareUnits is GreetingDelay: 30 and MaxBytes: 1024 under one
// operations array. encoding/json would store 30ns and 1024 bytes.
const caseVariantBareUnits = `{"op":"replaceSMTPBehavior","behavior":{"GreetingDelay":30}},{"op":"replaceStoreCaps","store":{"maxMessages":1000,"MaxBytes":1024,"fullPolicy":"reject"}}`

func changeBodyKeyed(rev, key, ops string) string {
	return fmt.Sprintf(`{"expectedRevision":%q,"reason":"repro",%q:[%s]}`, rev, key, ops)
}

func assertRuntimeUntouched(t *testing.T, svc *app.App, rev string, delay time.Duration, maxBytes int64) {
	t.Helper()
	if got := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay; got != delay {
		t.Fatalf("greetingDelay=%s want %s", got, delay)
	}
	if got := svc.Active().Canonical.Spec.Store.MaxBytes; got != maxBytes {
		t.Fatalf("maxBytes=%d want %d", got, maxBytes)
	}
	if got := string(svc.Active().Revision); got != rev {
		t.Fatalf("revision changed to %s", got)
	}
}

// TestRESTRejectsCaseVariantOperationsKey: the operations member is matched
// case-insensitively, same as encoding/json. GreetingDelay: 30 and
// MaxBytes: 1024 under Operations must be validation_failed on plan and apply.
func TestRESTRejectsCaseVariantOperationsKey(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	delay := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay
	maxBytes := svc.Active().Canonical.Spec.Store.MaxBytes
	h := s.Handler()
	for _, key := range []string{"Operations", "OPERATIONS"} {
		body := changeBodyKeyed(rev, key, caseVariantBareUnits)
		for _, path := range []string{"/v1/changes:plan", "/v1/changes:apply"} {
			rec := doReq(t, h, http.MethodPost, path, body)
			requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
			if !strings.Contains(rec.Body.String(), "bare number") {
				t.Fatalf("%s %s did not reject bare units: %s", path, key, rec.Body.String())
			}
			assertRuntimeUntouched(t, svc, rev, delay, maxBytes)
		}
	}
}

// TestRESTAppliesCaseVariantOperationsKey: one Operations member is folded,
// so a duration string applies. Rejecting the parent spelling would be wrong.
func TestRESTAppliesCaseVariantOperationsKey(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	body := changeBodyKeyed(rev, "Operations", `{"op":"replaceSMTPBehavior","behavior":{"GreetingDelay":"30s"}}`)
	rec := doReq(t, s.Handler(), http.MethodPost, "/v1/changes:plan", body)
	requireStatus(t, rec, http.StatusOK)
	rec = doReq(t, s.Handler(), http.MethodPost, "/v1/changes:apply", body)
	requireStatus(t, rec, http.StatusOK)
	if got := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay; got != 30*time.Second {
		t.Fatalf("Operations GreetingDelay string applied as %s body=%s", got, rec.Body.String())
	}
}

// TestRESTRejectsDuplicateOperationsKeys: operations and Operations together
// are a duplicate key. Neither member is applied.
func TestRESTRejectsDuplicateOperationsKeys(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	delay := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay
	maxBytes := svc.Active().Canonical.Spec.Store.MaxBytes
	body := fmt.Sprintf(`{"expectedRevision":%q,"reason":"repro","operations":[{"op":"replaceHideExtensions","hideExtensions":["SIZE"]}],"Operations":[%s]}`, rev, caseVariantBareUnits)
	h := s.Handler()
	for _, path := range []string{"/v1/changes:plan", "/v1/changes:apply"} {
		rec := doReq(t, h, http.MethodPost, path, body)
		requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
		if !strings.Contains(rec.Body.String(), "duplicate") {
			t.Fatalf("%s did not report a duplicate operations key: %s", path, rec.Body.String())
		}
		assertRuntimeUntouched(t, svc, rev, delay, maxBytes)
		if len(svc.Active().Canonical.Spec.SMTP.HideExtensions) != 0 {
			t.Fatal("duplicate operations keys applied hideExtensions")
		}
	}
}
