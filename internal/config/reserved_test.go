package config

import (
	"fmt"
	"testing"
)

func TestReservedKeysTable(t *testing.T) {
	keys := []string{
		"outgoing", "outgoingHost", "outgoing-host", "outgoingPort", "outgoingUser",
		"outgoingPass", "outgoingSecure", "autoRelay", "auto-relay", "autoRelayRules",
		"auto-relay-rules", "relay", "smarthost", "smartHost", "forwardTo", "mx", "deliver",
		"--auto-relay", "auto_relay", "OUTGOING_HOST", "outgoingSomething",
	}
	for _, k := range keys {
		doc := fmt.Sprintf("apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: x\nspec:\n  %s: true\n", k)
		t.Run(k, func(t *testing.T) {
			_, err := Decode([]byte(doc))
			_ = requireValidation(t, err, violationReservedName)
		})
	}
}

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"--auto-relay":  "autorelay",
		"auto_relay":    "autorelay",
		"autoRelay":     "autorelay",
		"OUTGOING_HOST": "outgoinghost",
		"outgoing-host": "outgoinghost",
		"smartHost":     "smarthost",
		"forwardTo":     "forwardto",
	}
	for in, want := range cases {
		if got := normalizeKey(in); got != want {
			t.Fatalf("normalizeKey(%q)=%q want %q", in, got, want)
		}
	}
}

func TestReservedDoesNotMatchLegitFields(t *testing.T) {
	if why := reservedReason(normalizeKey("maxMessages")); why != "" {
		t.Fatalf("maxMessages reserved: %s", why)
	}
	if why := reservedReason(normalizeKey("maxMessageBytes")); why != "" {
		t.Fatalf("maxMessageBytes reserved: %s", why)
	}
}
