package domainerr

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCodesMatchRESTTable(t *testing.T) {
	want := []Code{
		CodeValidationFailed,
		CodeUnauthenticated,
		CodeForbidden,
		CodeReceiveOnly,
		CodeNotFound,
		CodeMethodNotAllowed,
		CodeRevisionConflict,
		CodeIdempotencyConflict,
		CodeStoreFull,
		CodeStoreOverNewCap,
		CodeCursorStale,
		CodeRateLimited,
		CodeTimeout,
		CodeInternalError,
	}
	got := Codes()
	if len(got) != len(want) {
		t.Fatalf("catalog=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("catalog[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestConstructors(t *testing.T) {
	cases := []struct {
		code Code
		err  *Error
	}{
		{CodeValidationFailed, ValidationFailed("msg")},
		{CodeUnauthenticated, Unauthenticated("msg")},
		{CodeForbidden, Forbidden("msg")},
		{CodeReceiveOnly, ReceiveOnly("msg")},
		{CodeNotFound, NotFound("msg")},
		{CodeMethodNotAllowed, MethodNotAllowed("msg")},
		{CodeRevisionConflict, RevisionConflict("msg", "sha256:abc")},
		{CodeIdempotencyConflict, IdempotencyConflict("msg")},
		{CodeStoreFull, StoreFull("msg")},
		{CodeStoreOverNewCap, StoreOverNewCap("msg")},
		{CodeCursorStale, CursorStale("msg")},
		{CodeRateLimited, RateLimited("msg")},
		{CodeTimeout, Timeout("msg")},
		{CodeInternalError, Internal("msg")},
	}
	if len(cases) != len(catalog) {
		t.Fatalf("constructor cases=%d catalog=%d", len(cases), len(catalog))
	}
	for _, tc := range cases {
		if tc.err.Code != tc.code {
			t.Fatalf("%s constructor code=%q", tc.code, tc.err.Code)
		}
		if tc.err.Retryable != Retryable(tc.code) {
			t.Fatalf("%s retryable=%v", tc.code, tc.err.Retryable)
		}
	}
}

func TestErrorJSONShapeAndNoStack(t *testing.T) {
	err := ValidationFailed("Candidate state is invalid.", FieldViolation{
		Path:    "spec.smtp.tls.mode",
		Code:    "invalid_value",
		Message: "implicit is not supported until 1.1",
	}).WithRemediation("use starttls").WithRevision("sha256:deadbeef")

	raw, jerr := json.Marshal(err)
	if jerr != nil {
		t.Fatal(jerr)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["code"] != string(CodeValidationFailed) {
		t.Fatalf("code=%v", obj["code"])
	}
	if strings.Contains(err.Error(), "goroutine") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("Error() looks like a stack: %q", err.Error())
	}
}

func TestAsAndIs(t *testing.T) {
	base := ReceiveOnly("no relay")
	wrapped := errors.Join(base, errors.New("wrap"))
	got, ok := As(wrapped)
	if !ok || got.Code != CodeReceiveOnly {
		t.Fatalf("As = (%v, %v)", got, ok)
	}
	if !errors.Is(wrapped, &Error{Code: CodeReceiveOnly}) {
		t.Fatal("errors.Is did not match by code")
	}
}

func TestUnknownCodeNotRetryable(t *testing.T) {
	if Retryable(Code("not_a_real_code")) {
		t.Fatal("unknown code should not be retryable")
	}
}

func TestNilError(t *testing.T) {
	var err *Error
	if err.Error() != "" {
		t.Fatalf("nil Error()=%q", err.Error())
	}
	if err.WithRemediation("x") != nil || err.WithRevision("x") != nil || err.WithViolations(FieldViolation{}) != nil {
		t.Fatal("nil receiver With* should return nil")
	}
}

func TestWithMethodsCopyOnWrite(t *testing.T) {
	base := NotFound("message")
	a := base.WithRevision("sha256:a").WithRemediation("re-read")
	if base.CurrentRevision != "" || base.Remediation != "" {
		t.Fatalf("base mutated: %+v", base)
	}
	if a.CurrentRevision != "sha256:a" || a.Remediation != "re-read" {
		t.Fatalf("a=%+v", a)
	}
}
