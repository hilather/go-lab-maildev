package compatcheck

import "testing"

func TestSwapGateOKRequires401AndArraySubject(t *testing.T) {
	r := Report{
		Name:           "labmail",
		UnauthStatus:   401,
		HealthzStatus:  200,
		ListStatus:     200,
		ListIsArray:    true,
		ListSubjectHit: true,
		RelayStatus:    403,
		Subject:        "s",
	}
	if errs := SwapGateOK(r, true); len(errs) != 0 {
		t.Fatalf("ok report: %v", errs)
	}
	r.UnauthStatus = 200
	if errs := SwapGateOK(r, true); len(errs) == 0 {
		t.Fatal("200 unauth must fail swap-gate")
	}
}

func TestDocumentedLabMailDeltasRejectsListBodies(t *testing.T) {
	r := Report{
		Name:     "labmail",
		ListItem: map[string]any{"id": "01JEXAMPLENOTAREALULID0001", "text": "leaked", "html": ""},
	}
	if errs := DocumentedLabMailDeltas(r); len(errs) == 0 {
		t.Fatal("list body leak must fail")
	}
}

func TestSharedShapeDiffCatchesFromMismatch(t *testing.T) {
	md := Report{ListItem: map[string]any{
		"subject": "s", "priority": "normal",
		"from":    []any{map[string]any{"address": "a@x", "name": "A"}},
		"to":      []any{map[string]any{"address": "b@x", "name": "B"}},
		"headers": map[string]any{"from": "A <a@x>", "to": "B <b@x>", "subject": "s"},
	}}
	lm := Report{ListItem: map[string]any{
		"subject": "s", "priority": "normal",
		"from":    []any{map[string]any{"address": "wrong@x", "name": "A"}},
		"to":      []any{map[string]any{"address": "b@x", "name": "B"}},
		"headers": map[string]any{"from": "A <a@x>", "to": "B <b@x>", "subject": "s"},
	}}
	if errs := SharedShapeDiff(md, lm); len(errs) == 0 {
		t.Fatal("from address mismatch must fail")
	}
}
