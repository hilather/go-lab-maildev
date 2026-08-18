package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/observability"
)

func TestStatusReadyUsesHealthFacts(t *testing.T) {
	svc, _ := mustBoot(t)
	st, err := svc.Status(context.Background(), actor())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Ready {
		t.Fatal("store-up app is ready without a listener probe")
	}
	svc.SetHealth(func() observability.Facts {
		return observability.Facts{SMTPBound: false, StoreUp: true, MgmtBound: true}
	})
	st, err = svc.Status(context.Background(), actor())
	if err != nil {
		t.Fatal(err)
	}
	if st.Ready {
		t.Fatal("Status.Ready must follow Evaluate (SMTP unbound)")
	}
}
