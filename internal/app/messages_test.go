package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestDeleteAndClearAudit(t *testing.T) {
	svc, _ := mustBoot(t)
	ctx := context.Background()
	id := insertRaw(t, svc, "gone")
	insertRaw(t, svc, "also")
	if err := svc.DeleteMessage(ctx, actor(), id, DeleteIn{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetMessage(ctx, actor(), id, false); err == nil {
		t.Fatal("deleted")
	} else {
		requireCode(t, err, domainerr.CodeNotFound)
	}
	n, err := svc.ClearMessages(ctx, actor(), DeleteIn{})
	if err != nil || n != 1 {
		t.Fatalf("clear n=%d err=%v", n, err)
	}
	if svc.Inbox().Stats().MessageCount != 0 {
		t.Fatal("clear left mail")
	}
	listed, err := svc.QueryAudit(ctx, actor(), AuditQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var sawDelete, sawClear bool
	for _, ev := range listed.Events {
		if ev.Capability == "messages.delete" && ev.MessageID == id {
			sawDelete = true
		}
		if ev.Capability == "messages.clear" {
			sawClear = true
		}
	}
	if !sawDelete || !sawClear {
		t.Fatalf("audit delete=%v clear=%v events=%+v", sawDelete, sawClear, listed.Events)
	}
}

func TestClearExpectedGeneration(t *testing.T) {
	svc, _ := mustBoot(t)
	insertRaw(t, svc, "x")
	wrong := uint64(999)
	_, err := svc.ClearMessages(context.Background(), actor(), DeleteIn{ExpectedStoreGeneration: &wrong})
	requireCode(t, err, domainerr.CodeRevisionConflict)
	if svc.Inbox().Stats().MessageCount != 1 {
		t.Fatal("conflict cleared")
	}
	cur := svc.Inbox().Generation()
	n, err := svc.ClearMessages(context.Background(), actor(), DeleteIn{ExpectedStoreGeneration: &cur})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestMarkReadDoesNotBumpGeneration(t *testing.T) {
	svc, _ := mustBoot(t)
	id := insertRaw(t, svc, "read-me")
	gen := svc.Inbox().Generation()
	if err := svc.MarkRead(context.Background(), actor(), id); err != nil {
		t.Fatal(err)
	}
	if svc.Inbox().Generation() != gen {
		t.Fatal("mark-read bumped generation")
	}
	msg, err := svc.GetMessage(context.Background(), actor(), id, false)
	if err != nil || !msg.Read {
		t.Fatalf("read=%v err=%v", msg, err)
	}
}

func TestWaitTimeout(t *testing.T) {
	svc, _ := mustBoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_, err := svc.Wait(ctx, actor(), model.MessageFilter{Subject: "never"})
	requireCode(t, err, domainerr.CodeTimeout)
}

func TestGetAudit(t *testing.T) {
	svc, _ := mustBoot(t)
	id := insertRaw(t, svc, "aud")
	if err := svc.DeleteMessage(context.Background(), actor(), id, DeleteIn{}); err != nil {
		t.Fatal(err)
	}
	listed, err := svc.QueryAudit(context.Background(), actor(), AuditQuery{Limit: 1})
	if err != nil || len(listed.Events) != 1 {
		t.Fatalf("list=%+v err=%v", listed, err)
	}
	got, err := svc.GetAudit(context.Background(), actor(), listed.Events[0].ID)
	if err != nil || got.Capability != "messages.delete" {
		t.Fatalf("get=%+v err=%v", got, err)
	}
	_, err = svc.GetAudit(context.Background(), actor(), "aud-missing")
	requireCode(t, err, domainerr.CodeNotFound)
}
