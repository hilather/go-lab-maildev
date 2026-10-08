package config

import (
	"strings"
	"testing"
)

func TestCoerceWireTreeDuplicateFoldedKey(t *testing.T) {
	tree := map[string]any{
		"operations": []any{
			map[string]any{
				"op": "replaceSMTPBehavior",
				"behavior": map[string]any{
					"greetingDelay": "1s",
					"GreetingDelay": "2s",
				},
			},
		},
	}
	vs := CoerceWireChange(tree)
	if len(vs) != 1 {
		t.Fatalf("violations=%+v", vs)
	}
	if !strings.Contains(vs[0].Message, "duplicate") {
		t.Fatalf("message=%q", vs[0].Message)
	}
	behavior := tree["operations"].([]any)[0].(map[string]any)["behavior"].(map[string]any)
	if behavior["greetingDelay"] != "1s" || behavior["GreetingDelay"] != "2s" {
		t.Fatalf("duplicate keys were coerced or dropped: %#v", behavior)
	}
}

func TestDecodeYAMLStillRejectsCaseVariantUnitKey(t *testing.T) {
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  smtp:\n    behavior:\n      GreetingDelay: 30s\n"
	_, err := Decode([]byte(doc))
	de := requireValidation(t, err, violationUnknownField)
	found := false
	for _, v := range de.FieldViolations {
		if strings.Contains(v.Path, "GreetingDelay") || strings.Contains(v.Message, "GreetingDelay") {
			found = true
		}
	}
	if !found {
		t.Fatalf("violations=%+v", de.FieldViolations)
	}
}
