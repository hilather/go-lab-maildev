package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestPlanApplyHideExtensions(t *testing.T) {
	svc, boot := mustBoot(t)
	ctx := context.Background()
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "hide-1",
		Reason:           "hide SIZE",
		Operations:       []model.Operation{hideSIZE()},
	}
	plan, err := svc.Plan(ctx, actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PreviousRevision != boot.Revision || plan.CandidateRevision == boot.Revision {
		t.Fatalf("plan revs prev=%s cand=%s", plan.PreviousRevision, plan.CandidateRevision)
	}
	if svc.Active().Revision != boot.Revision {
		t.Fatal("plan swapped")
	}

	res, err := svc.Apply(ctx, actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied || !res.Drifted {
		t.Fatalf("applied=%v drifted=%v", res.Applied, res.Drifted)
	}
	if res.RuntimeRevision != plan.CandidateRevision {
		t.Fatalf("apply rev=%s plan=%s", res.RuntimeRevision, plan.CandidateRevision)
	}
	got := svc.Active().Canonical.Spec.SMTP.HideExtensions
	if len(got) != 1 || got[0] != "SIZE" {
		t.Fatalf("hide=%v", got)
	}

	again, err := svc.Apply(ctx, actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if again.Generation != res.Generation {
		t.Fatal("idempotent apply must not swap again")
	}

	in.Reason = "different"
	_, err = svc.Apply(ctx, actor(), in)
	requireCode(t, err, domainerr.CodeIdempotencyConflict)
}

func TestApplyRequiresExpectedRevision(t *testing.T) {
	svc, boot := mustBoot(t)
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		Operations: []model.Operation{hideSIZE()},
	})
	requireCode(t, err, domainerr.CodeValidationFailed)
	_, err = svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: "sha256:dead",
		Operations:       []model.Operation{hideSIZE()},
	})
	requireCode(t, err, domainerr.CodeRevisionConflict)
	if svc.Active().Revision != boot.Revision {
		t.Fatal("conflict swapped")
	}
}

func TestReplaceStoreCapsRejectWithoutForce(t *testing.T) {
	svc, boot := mustBoot(t)
	ctx := context.Background()
	insertRaw(t, svc, "one")
	insertRaw(t, svc, "two")
	_, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{shrinkStore(1, model.FullPolicyReject)},
	})
	requireCode(t, err, domainerr.CodeStoreOverNewCap)
	if svc.Inbox().Stats().MessageCount != 2 {
		t.Fatal("reject shrink evicted")
	}
	if svc.Active().Revision != boot.Revision {
		t.Fatal("failed apply swapped")
	}
}

func TestReplaceStoreCapsForceEvicts(t *testing.T) {
	svc, boot := mustBoot(t)
	ctx := context.Background()
	first := insertRaw(t, svc, "one")
	second := insertRaw(t, svc, "two")
	res, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Force:            true,
		Operations:       []model.Operation{shrinkStore(1, model.FullPolicyReject)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatal("force apply")
	}
	if svc.Inbox().Stats().MessageCount != 1 {
		t.Fatalf("count=%d", svc.Inbox().Stats().MessageCount)
	}
	if _, err := svc.GetMessage(ctx, actor(), first, false); err == nil {
		t.Fatal("oldest should be evicted")
	} else {
		requireCode(t, err, domainerr.CodeNotFound)
	}
	if _, err := svc.GetMessage(ctx, actor(), second, false); err != nil {
		t.Fatalf("newest should remain: %v", err)
	}
	if svc.Active().Canonical.Spec.Store.MaxMessages != 1 {
		t.Fatal("caps not applied")
	}
}

func TestReplaceStoreCapsEvictOldest(t *testing.T) {
	svc, boot := mustBoot(t)
	first := insertRaw(t, svc, "one")
	second := insertRaw(t, svc, "two")
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{shrinkStore(1, model.FullPolicyEvictOldest)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Inbox().Stats().MessageCount != 1 {
		t.Fatalf("count=%d", svc.Inbox().Stats().MessageCount)
	}
	if _, err := svc.GetMessage(context.Background(), actor(), first, false); err == nil {
		t.Fatal("oldest should be evicted")
	} else {
		requireCode(t, err, domainerr.CodeNotFound)
	}
	if _, err := svc.GetMessage(context.Background(), actor(), second, false); err != nil {
		t.Fatalf("newest should remain: %v", err)
	}
}

func TestPlanReplaceStoreCapsRejectWithoutForce(t *testing.T) {
	svc, boot := mustBoot(t)
	insertRaw(t, svc, "one")
	insertRaw(t, svc, "two")
	_, err := svc.Plan(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{shrinkStore(1, model.FullPolicyReject)},
	})
	requireCode(t, err, domainerr.CodeStoreOverNewCap)
	if svc.Inbox().Stats().MessageCount != 2 {
		t.Fatal("plan must not evict")
	}
	if svc.Active().Revision != boot.Revision {
		t.Fatal("plan swapped")
	}
}

func TestPlanReplaceStoreCapsDoesNotEvict(t *testing.T) {
	svc, boot := mustBoot(t)
	insertRaw(t, svc, "one")
	insertRaw(t, svc, "two")
	plan, err := svc.Plan(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{shrinkStore(1, model.FullPolicyEvictOldest)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Inbox().Stats().MessageCount != 2 {
		t.Fatal("plan must not evict")
	}
	saw := false
	for _, w := range plan.Warnings {
		if w.Code == "store_evict" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("missing store_evict warning: %+v", plan.Warnings)
	}
}

func TestReplaceStoreCapsLastOpWins(t *testing.T) {
	svc, boot := mustBoot(t)
	first := insertRaw(t, svc, "one")
	second := insertRaw(t, svc, "two")
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{
			shrinkStore(1, model.FullPolicyReject),
			shrinkStore(1000, model.FullPolicyReject),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Inbox().Stats().MessageCount != 2 {
		t.Fatalf("count=%d", svc.Inbox().Stats().MessageCount)
	}
	if _, err := svc.GetMessage(context.Background(), actor(), first, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetMessage(context.Background(), actor(), second, false); err != nil {
		t.Fatal(err)
	}
	if svc.Active().Canonical.Spec.Store.MaxMessages != 1000 {
		t.Fatalf("maxMessages=%d", svc.Active().Canonical.Spec.Store.MaxMessages)
	}
}

func TestApplyReplaceSMTPAuthRejectedUntil001b(t *testing.T) {
	svc, boot := mustBoot(t)
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op: model.OpReplaceSMTPAuth,
			Auth: &model.SMTPAuthSpec{
				Mode:         model.SMTPAuthPlainLogin,
				Username:     "lab",
				PasswordFile: "/tmp/does-not-need-to-exist",
			},
		}},
	})
	requireCode(t, err, domainerr.CodeValidationFailed)
	if svc.Active().Revision != boot.Revision {
		t.Fatal("AUTH apply swapped")
	}
	if svc.Active().Canonical.Spec.SMTP.Auth.Mode != model.SMTPAuthNone && svc.Active().Canonical.Spec.SMTP.Auth.Mode != "" {
		t.Fatalf("auth mode=%q", svc.Active().Canonical.Spec.SMTP.Auth.Mode)
	}
}

func TestApplyReplaceSMTPBehavior(t *testing.T) {
	svc, boot := mustBoot(t)
	ctx := context.Background()
	res, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Reason:           "qa 421 MAIL",
		Operations: []model.Operation{{
			Op: model.OpReplaceSMTPBehavior,
			Behavior: &model.SMTPBehaviorSpec{
				Replies: model.SMTPReplyOverrides{Mail: "421 4.3.2 try later"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatal("expected apply")
	}
	got := svc.Active().Canonical.Spec.SMTP.Behavior.Replies.Mail
	if got != "421 4.3.2 try later" {
		t.Fatalf("mail override=%q", got)
	}

	_, err = svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: res.RuntimeRevision,
		Operations:       []model.Operation{{Op: model.OpReplaceSMTPBehavior}},
	})
	requireCode(t, err, domainerr.CodeValidationFailed)

	_, err = svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: res.RuntimeRevision,
		Operations: []model.Operation{{
			Op:       model.OpReplaceSMTPBehavior,
			Behavior: &model.SMTPBehaviorSpec{GreetingDelay: 31 * time.Second},
		}},
	})
	requireCode(t, err, domainerr.CodeValidationFailed)
}

func TestValidateUnknownOp(t *testing.T) {
	svc, _ := mustBoot(t)
	_, err := svc.Validate(context.Background(), actor(), ValidateIn{
		Operations: []model.Operation{{Op: "replaceRelay"}},
	})
	requireCode(t, err, domainerr.CodeValidationFailed)
}

func TestExportJSON(t *testing.T) {
	svc, boot := mustBoot(t)
	out, err := svc.Export(context.Background(), actor(), ExportJSON)
	if err != nil {
		t.Fatal(err)
	}
	if out.Revision != boot.Revision || out.Drifted {
		t.Fatalf("export %+v", out)
	}
	if !strings.Contains(string(out.Body), `"apiVersion"`) {
		t.Fatalf("body=%s", out.Body)
	}
}

func TestGetStateCopyIsSafe(t *testing.T) {
	svc, _ := mustBoot(t)
	view, err := svc.GetState(context.Background(), actor())
	if err != nil {
		t.Fatal(err)
	}
	view.Canonical.Spec.SMTP.Hostname = "mutated"
	if svc.Active().Canonical.Spec.SMTP.Hostname == "mutated" {
		t.Fatal("GetState leaked live pointer")
	}
}
