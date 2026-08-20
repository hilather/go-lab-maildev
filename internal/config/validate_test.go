package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestValidateDefaults(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(st); err != nil {
		t.Fatal(err)
	}
}

func TestValidateImplicitTLS(t *testing.T) {
	_, err := Load([]byte(mustLoad(t, "invalid", "implicit-tls.yaml")))
	de := requireValidation(t, err, violationInvalidValue)
	found := false
	for _, v := range de.FieldViolations {
		if v.Path == "spec.smtp.tls.mode" && v.Message != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want implicit message in %+v", de.FieldViolations)
	}
}

func TestValidateStartTLSRequiresFiles(t *testing.T) {
	_, err := Load([]byte(mustLoad(t, "invalid", "starttls-missing-cert.yaml")))
	_ = requireValidation(t, err, violationRequired)
}

func TestValidateStartTLSResolvesFiles(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(cert, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: tls\nspec:\n  smtp:\n    tls:\n      mode: starttls\n      certFile: " + cert + "\n      keyFile: " + key + "\n"
	st, err := Load([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.SMTP.TLS.Mode != model.TLSModeStartTLS {
		t.Fatalf("mode=%q", st.Spec.SMTP.TLS.Mode)
	}
}

func TestValidateStartTLSMissingFilePath(t *testing.T) {
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: tls\nspec:\n  smtp:\n    tls:\n      mode: starttls\n      certFile: /no/such/cert.pem\n      keyFile: /no/such/key.pem\n"
	_, err := Load([]byte(doc))
	_ = requireValidation(t, err, violationUnresolved)
}

func TestValidateRequiredOnlyWithStartTLS(t *testing.T) {
	_, err := Load([]byte(mustLoad(t, "invalid", "required-without-starttls.yaml")))
	_ = requireValidation(t, err, violationInvalidValue)
}

func TestValidateOffWithUnusedCerts(t *testing.T) {
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: x\nspec:\n  smtp:\n    tls:\n      mode: off\n      certFile: /tmp/x.pem\n"
	_, err := Load([]byte(doc))
	_ = requireValidation(t, err, violationInvalidValue)
}

func TestValidateMaxMessageBytesZero(t *testing.T) {
	_, err := Load([]byte(mustLoad(t, "invalid", "max-message-bytes-zero.yaml")))
	_ = requireValidation(t, err, violationInvalidValue)
}

func TestValidateMissingAPIVersion(t *testing.T) {
	_, err := Load([]byte(mustLoad(t, "invalid", "missing-apiversion.yaml")))
	_ = requireValidation(t, err, violationRequired)
}

func TestValidateNil(t *testing.T) {
	_ = requireValidation(t, Validate(nil), violationRequired)
}

func TestValidateOriginAllowlistCap(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	st.Spec.Management.OriginAllowlist = make([]string, originAllowlistCap+1)
	for i := range st.Spec.Management.OriginAllowlist {
		st.Spec.Management.OriginAllowlist[i] = "*"
	}
	de := requireValidation(t, Validate(st), violationInvalidValue)
	found := false
	for _, v := range de.FieldViolations {
		if v.Path == "spec.management.originAllowlist" && v.Message == "originAllowlist cap 64" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want cap violation in %+v", de.FieldViolations)
	}
}

func TestValidateOriginAllowlistExactOK(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "origin-exact.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(st); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeDoesNotRewriteOriginAllowlist(t *testing.T) {
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    originAllowlist: [\"PRIVATE\", \"http://devbox:1080/\"]\n"
	st, err := Load([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	got := st.Spec.Management.OriginAllowlist
	if len(got) != 2 || got[0] != "PRIVATE" || got[1] != "http://devbox:1080/" {
		t.Fatalf("rewrote originAllowlist: %q", got)
	}
}

func TestOriginAllowlistRevisionStarDiffers(t *testing.T) {
	empty, err := Load([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	star, err := Load([]byte(mustLoad(t, "valid", "origin-star.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	revEmpty, err := Revision(empty)
	if err != nil {
		t.Fatal(err)
	}
	revStar, err := Revision(star)
	if err != nil {
		t.Fatal(err)
	}
	if revEmpty == revStar {
		t.Fatal("empty and [\"*\"] must have different revisions")
	}
}

func TestValidateShortTokenSecret(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte("tooshort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n"
	_, err := Load([]byte(doc))
	_ = requireValidation(t, err, violationInvalidValue)
}
