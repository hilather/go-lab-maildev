package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// TestIdempotencyReplayDoesNotClaimStaleRevision: a later apply moves the
// runtime revision. Replaying the original key must not report the old
// revision as the applied result; without the cache that call is a
// revision conflict.
func TestIdempotencyReplayDoesNotClaimStaleRevision(t *testing.T) {
	svc, snap := mustBoot(t)
	ctx := context.Background()
	firstIn := ChangeIn{
		ExpectedRevision: snap.Revision,
		IdempotencyKey:   "hide-size",
		Reason:           "hide",
		Operations:       []model.Operation{hideSIZE()},
	}
	first, err := svc.Apply(ctx, actor(), firstIn)
	if err != nil {
		t.Fatal(err)
	}
	live := svc.Active()
	_, err = svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: live.Revision,
		IdempotencyKey:   "clear-hide",
		Reason:           "clear",
		Operations: []model.Operation{{
			Op:             model.OpReplaceHideExtensions,
			HideExtensions: []string{},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	current := svc.Active().Revision
	if current == first.RuntimeRevision {
		t.Fatal("second apply did not move the revision")
	}
	replay, err := svc.Apply(ctx, actor(), firstIn)
	if err == nil {
		t.Fatalf("stale idempotency replay claimed revision %s (applied=%v); live revision is %s", replay.RuntimeRevision, replay.Applied, current)
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeIdempotencyConflict {
		t.Fatalf("stale replay err=%v want %s", err, domainerr.CodeIdempotencyConflict)
	}
	_, err = svc.Apply(ctx, actor(), firstIn)
	requireCode(t, err, domainerr.CodeIdempotencyConflict)
}
