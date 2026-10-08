package rest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
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
