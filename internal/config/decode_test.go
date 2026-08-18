package config

import (
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestDecodeDefaultsYAML(t *testing.T) {
	st, err := Decode([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if st.APIVersion != model.APIVersionV1Alpha1 || st.Kind != model.KindLabMail {
		t.Fatalf("api=%q kind=%q", st.APIVersion, st.Kind)
	}
	if st.Metadata.Name != "lab-sink" {
		t.Fatalf("name=%q", st.Metadata.Name)
	}
	if !st.Spec.UI.Enabled {
		t.Fatal("omitted ui.enabled must materialize true at decode")
	}
	if !st.Spec.Listeners.Management.CompatEnabled {
		t.Fatal("omitted compatEnabled must materialize true at decode")
	}
	if st.Spec.Observability.Metrics.Listen != DefaultMetricsListen {
		t.Fatalf("listen=%q", st.Spec.Observability.Metrics.Listen)
	}
}

func TestDecodeJSONUnknownField(t *testing.T) {
	_, err := DecodeJSON([]byte(`{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x"},"spec":{"nope":1}}`))
	_ = requireValidation(t, err, violationUnknownField)
}

func TestDecodeUnknownFieldEveryLevel(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		path string
	}{
		{"root", `{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x"},"extra":true,"spec":{}}`, "extra"},
		{"metadata", `{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x","nope":1},"spec":{}}`, "metadata.nope"},
		{"spec", `{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x"},"spec":{"nope":1}}`, "spec.nope"},
		{"smtp", `{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x"},"spec":{"smtp":{"zzz":1}}}`, "spec.smtp.zzz"},
		{"store", `{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"x"},"spec":{"store":{"foo":true}}}`, "spec.store.foo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tc.doc))
			de := requireValidation(t, err, violationUnknownField)
			found := false
			for _, v := range de.FieldViolations {
				if v.Path == tc.path {
					found = true
				}
			}
			if !found {
				t.Fatalf("want path %q in %+v", tc.path, de.FieldViolations)
			}
		})
	}
}

func TestDecodeYAMLCommentsDropped(t *testing.T) {
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\n# keep me\nmetadata:\n  name: x\nspec: {}\n"
	st, err := DecodeYAML([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	y, err := CanonicalYAML(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(y), "keep me") {
		t.Fatalf("comment preserved:\n%s", y)
	}
}

func TestDecodeUIEnabledExplicitFalse(t *testing.T) {
	st, err := Decode([]byte(mustLoad(t, "valid", "ui-disabled.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.UI.Enabled {
		t.Fatal("explicit ui.enabled: false was overwritten")
	}
	if st.Spec.Listeners.Management.CompatEnabled {
		t.Fatal("explicit compatEnabled: false was overwritten")
	}
	if st.Spec.Observability.Metrics.Listen != "" {
		t.Fatalf("explicit empty metrics.listen was overwritten: %q", st.Spec.Observability.Metrics.Listen)
	}
}

func TestDecodeRejectsTrailingDocuments(t *testing.T) {
	yamlDoc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: a\nspec: {}\n---\nkind: LabMail\n"
	_, err := DecodeYAML([]byte(yamlDoc))
	_ = requireValidation(t, err, violationInvalidValue)

	jsonDoc := `{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"a"},"spec":{}}{"x":1}`
	_, err = DecodeJSON([]byte(jsonDoc))
	_ = requireValidation(t, err, violationInvalidValue)
}

func TestDecodeRejectsUnitlessDurationAndSize(t *testing.T) {
	_, err := Decode([]byte(mustLoad(t, "invalid", "bare-duration.yaml")))
	_ = requireValidation(t, err, violationInvalidValue)
	_, err = Decode([]byte(mustLoad(t, "invalid", "bare-bytes.yaml")))
	_ = requireValidation(t, err, violationInvalidValue)
}

func TestDecodeRejectsEmptyAndTooLarge(t *testing.T) {
	if _, err := Decode(nil); err == nil {
		t.Fatal("empty")
	}
	big := make([]byte, MaxDocumentBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	if _, err := Decode(big); err == nil {
		t.Fatal("too large")
	}
}

func TestDecodeJSONRoundTripSample(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CanonicalJSON(st)
	if err != nil {
		t.Fatal(err)
	}
	st2, err := Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	r1, err := Revision(st)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Revision(st2)
	if err != nil {
		t.Fatal(err)
	}
	if r1 != r2 {
		t.Fatalf("export/reimport revision %s != %s", r1, r2)
	}
}
