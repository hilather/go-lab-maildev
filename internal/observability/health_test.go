package observability

import "testing"

func TestEvaluateReadyRequiresSMTPAndStore(t *testing.T) {
	ready := Evaluate(Facts{SMTPBound: true, StoreUp: true, MgmtBound: true})
	if !ready.Live || !ready.Ready {
		t.Fatalf("healthy probe=%+v", ready)
	}

	down := Evaluate(Facts{ProcessDown: true, SMTPBound: true, StoreUp: true, MgmtBound: true})
	if down.Live || down.Ready {
		t.Fatalf("process down still live/ready: %+v", down)
	}

	noSMTP := Evaluate(Facts{StoreUp: true, MgmtBound: true})
	if noSMTP.Ready {
		t.Fatal("missing SMTP must be unready")
	}
	if !hasCode(noSMTP.Warnings, WarnSMTPUnbound) {
		t.Fatalf("warnings=%v", noSMTP.Warnings)
	}

	noStore := Evaluate(Facts{SMTPBound: true, MgmtBound: true})
	if noStore.Ready || !hasCode(noStore.Warnings, WarnStoreDown) {
		t.Fatalf("missing store=%+v", noStore)
	}

	mgmtOff := Evaluate(Facts{SMTPBound: true, StoreUp: true, MgmtOff: true})
	if !mgmtOff.Ready {
		t.Fatalf("explicitly-off management must still be ready: %+v", mgmtOff)
	}

	mgmtMissing := Evaluate(Facts{SMTPBound: true, StoreUp: true})
	if mgmtMissing.Ready {
		t.Fatal("management neither bound nor off must be unready")
	}
}

func TestWarningBound(t *testing.T) {
	p := Evaluate(Facts{})
	if len(p.Warnings) > MaxWarnings {
		t.Fatalf("warnings=%d", len(p.Warnings))
	}
}

func hasCode(ws []Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}
