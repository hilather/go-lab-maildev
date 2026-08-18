package config

import (
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestNormalizeMaterializesDefaults(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.Listeners.SMTP.Address != DefaultSMTPAddress {
		t.Fatalf("smtp addr=%q", st.Spec.Listeners.SMTP.Address)
	}
	if st.Spec.Listeners.Management.Address != DefaultMgmtAddress {
		t.Fatalf("mgmt addr=%q", st.Spec.Listeners.Management.Address)
	}
	if st.Spec.SMTP.Hostname != DefaultSMTPHostname {
		t.Fatalf("hostname=%q", st.Spec.SMTP.Hostname)
	}
	if st.Spec.SMTP.MaxMessageBytes != DefaultMaxMessageBytes {
		t.Fatalf("maxMessageBytes=%d", st.Spec.SMTP.MaxMessageBytes)
	}
	if st.Spec.SMTP.TLS.Mode != model.TLSModeOff {
		t.Fatalf("tls.mode=%q", st.Spec.SMTP.TLS.Mode)
	}
	if st.Spec.Store.FullPolicy != model.FullPolicyReject {
		t.Fatalf("fullPolicy=%q", st.Spec.Store.FullPolicy)
	}
	if !st.Spec.UI.Enabled {
		t.Fatal("ui.enabled default")
	}
	if st.Spec.Management.Auth.Mode != model.MgmtAuthBearerAndBasic {
		t.Fatalf("auth.mode=%q", st.Spec.Management.Auth.Mode)
	}
	if st.Spec.SMTP.HideExtensions == nil {
		t.Fatal("hideExtensions must be empty slice, not nil")
	}
}

func TestNormalizeDoesNotMutateInput(t *testing.T) {
	st, err := Decode([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	n, err := Normalize(st)
	if err != nil {
		t.Fatal(err)
	}
	st.Spec.SMTP.Hostname = "mutated"
	if n.Spec.SMTP.Hostname == "mutated" {
		t.Fatal("Normalize mutated caller")
	}
}

func TestNormalizeNil(t *testing.T) {
	_, err := Normalize(nil)
	_ = requireValidation(t, err, violationRequired)
}
