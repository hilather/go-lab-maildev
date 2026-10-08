package config

import (
	"encoding/json"
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

func TestCoerceWireChangeFoldsCaseVariantOperationsKey(t *testing.T) {
	behavior := map[string]any{"GreetingDelay": json.Number("30")}
	store := map[string]any{"MaxBytes": json.Number("1024")}
	tree := map[string]any{
		"Operations": []any{
			map[string]any{"op": "replaceSMTPBehavior", "behavior": behavior},
			map[string]any{"op": "replaceStoreCaps", "store": store},
		},
	}
	vs := CoerceWireChange(tree)
	if len(vs) < 2 {
		t.Fatalf("violations=%+v", vs)
	}
	var sawDuration, sawSize bool
	for _, v := range vs {
		if strings.Contains(v.Message, "duration") && strings.Contains(v.Message, "bare number") {
			sawDuration = true
		}
		if strings.Contains(v.Message, "byte size") && strings.Contains(v.Message, "bare number") {
			sawSize = true
		}
	}
	if !sawDuration || !sawSize {
		t.Fatalf("violations=%+v", vs)
	}
	if !stillBareNumber(behavior["GreetingDelay"]) && !stillBareNumber(behavior["greetingDelay"]) {
		t.Fatalf("bare GreetingDelay was coerced: %#v", behavior)
	}
	if !stillBareNumber(store["MaxBytes"]) && !stillBareNumber(store["maxBytes"]) {
		t.Fatalf("bare MaxBytes was coerced: %#v", store)
	}
}

func stillBareNumber(v any) bool {
	_, ok := v.(json.Number)
	return ok
}

func TestCoerceWireChangeDuplicateOperationsKey(t *testing.T) {
	lower := map[string]any{"greetingDelay": "1s"}
	upper := map[string]any{"GreetingDelay": json.Number("30")}
	tree := map[string]any{
		"operations": []any{map[string]any{"op": "replaceSMTPBehavior", "behavior": lower}},
		"Operations": []any{map[string]any{"op": "replaceSMTPBehavior", "behavior": upper}},
	}
	vs := CoerceWireChange(tree)
	if len(vs) != 1 || !strings.Contains(vs[0].Message, "duplicate") {
		t.Fatalf("violations=%+v", vs)
	}
	if lower["greetingDelay"] != "1s" {
		t.Fatalf("operations member was coerced: %#v", lower)
	}
	if _, ok := upper["greetingDelay"]; ok || upper["GreetingDelay"] != json.Number("30") {
		t.Fatalf("Operations member was folded: %#v", upper)
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
