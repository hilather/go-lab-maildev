package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestMemoryInsertParseAndULID(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	raw := []byte("From: alice@lab.test\r\nTo: bob@lab.test\r\nSubject: hello lab\r\n\r\nbody\r\n")
	res, err := s.Insert(context.Background(), s.Epoch(), &model.Message{
		Raw: raw,
		Envelope: model.Envelope{
			From: "alice@lab.test",
			To:   []string{"bob@lab.test"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ulid.Parse(res.ID); err != nil || len(res.ID) != 26 {
		t.Fatalf("id=%q err=%v", res.ID, err)
	}
	got, err := s.Get(res.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "hello lab" || !strings.Contains(got.Text, "body") {
		t.Fatalf("parsed %+v", got)
	}
	if got.MessageID != res.ID+"@labmail.lab" {
		t.Fatalf("messageId=%q", got.MessageID)
	}
	if !bytes.Equal(got.Raw, raw) {
		t.Fatal("raw rewritten")
	}
	if len(got.Attachments) == 1 && got.Attachments[0].ID != res.ID+":0" {
		t.Fatalf("att id=%q", got.Attachments[0].ID)
	}
	st := s.Stats()
	if st.MessageCount != 1 || st.Bytes != got.ResidentBytes() || st.Generation != res.Generation {
		t.Fatalf("stats=%+v resident=%d", st, got.ResidentBytes())
	}
	if st.Bytes <= int64(len(raw)) {
		t.Fatalf("resident %d should include decoded text", st.Bytes)
	}
}

func TestMemoryRejectFull(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 1, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("a", "one")); err != nil {
		t.Fatal(err)
	}
	_, err := s.Insert(context.Background(), s.Epoch(), rawMsg("b", "two"))
	if !errors.Is(err, ErrFull) {
		t.Fatalf("err=%v", err)
	}
	if s.Stats().MessageCount != 1 {
		t.Fatalf("count=%d", s.Stats().MessageCount)
	}
}

func TestMemoryTooLarge(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 32, FullPolicy: model.FullPolicyEvictOldest})
	_, err := s.Insert(context.Background(), s.Epoch(), rawMsg("big", strings.Repeat("x", 200)))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err=%v", err)
	}
	if s.Stats().MessageCount != 0 {
		t.Fatal("evicted inbox on oversized message")
	}
}

func TestMemoryEvictOldest(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 2, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyEvictOldest})
	a, err := s.Insert(context.Background(), s.Epoch(), rawMsg("a", "1"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	b, err := s.Insert(context.Background(), s.Epoch(), rawMsg("b", "2"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Insert(context.Background(), s.Epoch(), rawMsg("c", "3"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(a.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("oldest still present: %v", err)
	}
	if _, err := s.Get(b.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(c.ID, false); err != nil {
		t.Fatal(err)
	}
	if s.Stats().Evictions != 1 {
		t.Fatalf("evictions=%d", s.Stats().Evictions)
	}
}

func TestMemoryWipeStaleInsert(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	old := s.Epoch()
	if _, err := s.Insert(context.Background(), old, rawMsg("keep", "x")); err != nil {
		t.Fatal(err)
	}
	s.Wipe()
	if s.Epoch() == old {
		t.Fatal("epoch not bumped")
	}
	if s.Stats().MessageCount != 0 {
		t.Fatal("wipe left mail")
	}
	_, err := s.Insert(context.Background(), old, rawMsg("stale", "y"))
	if !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("err=%v", err)
	}
	if s.Stats().MessageCount != 0 {
		t.Fatal("stale insert stored")
	}
}

func TestMemoryWaitTimeout(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject, MaxWait: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := s.Wait(ctx, model.MessageFilter{Subject: "nope"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestMemoryWaitWakes(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject, MaxWait: 2 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		got, err := s.Wait(ctx, model.MessageFilter{SubjectContains: "wake"})
		if err != nil {
			errc <- err
			return
		}
		if got.Subject != "please wake" {
			errc <- errors.New(got.Subject)
			return
		}
		errc <- nil
	}()
	time.Sleep(15 * time.Millisecond)
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("please wake", "n")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not wake")
	}
}

func TestMemoryWaitDoesNotReturnDeleted(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject, MaxWait: 2 * time.Second})
	res, err := s.Insert(context.Background(), s.Epoch(), rawMsg("gone", "x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(res.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	got, err := s.Wait(ctx, model.MessageFilter{Subject: "gone"})
	if got != nil {
		t.Fatalf("returned deleted %s", got.ID)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestMemoryMarkReadDoesNotBumpGeneration(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	a, err := s.Insert(context.Background(), s.Epoch(), rawMsg("r1", "x"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("r2", "y")); err != nil {
		t.Fatal(err)
	}
	g := s.Generation()
	if err := s.MarkRead(a.ID); err != nil {
		t.Fatal(err)
	}
	if s.Generation() != g {
		t.Fatalf("MarkRead generation %d -> %d", g, s.Generation())
	}
	if s.Stats().UnreadCount != 1 {
		t.Fatalf("unread=%d", s.Stats().UnreadCount)
	}
	n, err := s.MarkAllRead()
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if s.Generation() != g {
		t.Fatal("MarkAllRead bumped generation")
	}
	if s.Stats().UnreadCount != 0 {
		t.Fatalf("unread after all=%d", s.Stats().UnreadCount)
	}
}

func TestMemoryDeleteAllDoesNotBumpEpoch(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	ep := s.Epoch()
	if _, err := s.Insert(context.Background(), ep, rawMsg("x", "y")); err != nil {
		t.Fatal(err)
	}
	g := s.Generation()
	n, err := s.DeleteAll()
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if s.Epoch() != ep {
		t.Fatalf("epoch %d -> %d", ep, s.Epoch())
	}
	if s.Generation() <= g {
		t.Fatalf("generation did not move: %d -> %d", g, s.Generation())
	}
}

func TestMemoryListAndDelete(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	_, _ = s.Insert(context.Background(), s.Epoch(), rawMsg("one", "a"))
	time.Sleep(2 * time.Millisecond)
	b, _ := s.Insert(context.Background(), s.Epoch(), rawMsg("two", "b"))
	list, err := s.List(model.ListQuery{Limit: 1})
	if err != nil || len(list.Items) != 1 || list.Items[0].Subject != "two" || list.NextCursor == "" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	page2, err := s.List(model.ListQuery{Limit: 1, Cursor: list.NextCursor})
	if err != nil || len(page2.Items) != 1 || page2.Items[0].Subject != "one" {
		t.Fatalf("page2=%+v err=%v", page2, err)
	}
	if err := s.Delete("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete %v", err)
	}
	if err := s.Delete(b.ID); err != nil {
		t.Fatal(err)
	}
	n, err := s.DeleteAll()
	if err != nil || n != 1 {
		t.Fatalf("deleteAll n=%d err=%v", n, err)
	}
	if s.Stats().MessageCount != 0 {
		t.Fatal("not empty")
	}
}

func TestMemorySpillRoundTripAndWipe(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t, Options{
		MaxMessages:    10,
		MaxBytes:       1 << 20,
		FullPolicy:     model.FullPolicyReject,
		SpillDirectory: dir,
		SpillThreshold: 8,
	})
	raw := []byte("From: a@b\r\nSubject: spill\r\n\r\n" + strings.Repeat("z", 40) + "\r\n")
	res, err := s.Insert(context.Background(), s.Epoch(), &model.Message{Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) == 0 {
		t.Fatal("expected spill file")
	}
	got, err := s.Get(res.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Raw, raw) {
		t.Fatalf("raw after spill len=%d want %d", len(got.Raw), len(raw))
	}
	s.Wipe()
	ents, _ = os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".raw") || strings.HasSuffix(e.Name(), ".att") {
			t.Fatalf("spill remained %s", e.Name())
		}
	}
}

func TestMemoryStartupWipesSpill(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "01HZYXWV7TSRQPJMKN76543210.raw")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = newTestStore(t, Options{
		MaxMessages:    10,
		MaxBytes:       1024,
		SpillDirectory: dir,
		SpillThreshold: 1,
	})
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale spill remained: %v", err)
	}
}

func TestMemoryMalformedMIMEStored(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	raw := []byte("From: <not-an-address\r\nContent-Type: multipart/mixed; boundary=abc\r\n\r\n--nope\r\n")
	res, err := s.Insert(context.Background(), s.Epoch(), &model.Message{Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(res.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Raw, raw) {
		t.Fatal("raw dropped")
	}
	if got.ParseWarning == "" {
		t.Fatal("expected parseWarning")
	}
}

func TestMemoryEvictOldestSpillFailureKeepsOld(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t, Options{
		MaxMessages:    1,
		MaxBytes:       1 << 20,
		FullPolicy:     model.FullPolicyEvictOldest,
		SpillDirectory: dir,
		SpillThreshold: 8,
	})
	firstRaw := []byte("Subject: keep\r\n\r\n" + strings.Repeat("a", 40) + "\r\n")
	first, err := s.Insert(context.Background(), s.Epoch(), &model.Message{Raw: firstRaw})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	_, err = s.Insert(context.Background(), s.Epoch(), &model.Message{
		Raw: []byte("Subject: new\r\n\r\n" + strings.Repeat("b", 40) + "\r\n"),
	})
	if err == nil {
		t.Fatal("expected spill failure")
	}
	if !errors.Is(err, ErrSpill) {
		t.Fatalf("err=%v", err)
	}
	if s.Stats().Evictions != 0 {
		t.Fatalf("evictions=%d", s.Stats().Evictions)
	}
	got, err := s.Get(first.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "keep" {
		t.Fatalf("subject=%q", got.Subject)
	}
}

func TestMemoryListDeletedCursorUsesReceivedAt(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	now := time.Now().UTC()
	newer := rawMsg("newer", "n")
	newer.ReceivedAt = now
	older := rawMsg("older", "o")
	older.ReceivedAt = now.Add(-time.Hour)
	// Newer first so its ULID is smaller than the older row's ULID.
	a, err := s.Insert(context.Background(), s.Epoch(), newer)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Insert(context.Background(), s.Epoch(), older)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	page, err := s.List(model.ListQuery{Cursor: a.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != b.ID {
		t.Fatalf("items=%v", idsOf(page))
	}
}

func TestMemoryListAfterWipeOldCursorIsEmpty(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	old, err := s.Insert(context.Background(), s.Epoch(), rawMsg("old", "a"))
	if err != nil {
		t.Fatal(err)
	}
	s.Wipe()
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("new", "b")); err != nil {
		t.Fatal(err)
	}
	page, err := s.List(model.ListQuery{Cursor: old.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("wipe+old cursor replayed %v", idsOf(page))
	}
}

func TestMemoryListDeletedOldestCursorIsEmpty(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	for _, subj := range []string{"a", "b", "c"} {
		time.Sleep(2 * time.Millisecond)
		if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg(subj, subj)); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.List(model.ListQuery{Limit: 10})
	if err != nil || len(page.Items) != 3 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	oldest := page.Items[len(page.Items)-1].ID
	if err := s.Delete(oldest); err != nil {
		t.Fatal(err)
	}
	next, err := s.List(model.ListQuery{Cursor: oldest, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 0 {
		t.Fatalf("deleted oldest cursor replayed %v", idsOf(next))
	}
}

func TestMemoryGetUnreadableSpill(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t, Options{
		MaxMessages:    10,
		MaxBytes:       1 << 20,
		FullPolicy:     model.FullPolicyReject,
		SpillDirectory: dir,
		SpillThreshold: 8,
	})
	raw := []byte("Subject: spill\r\n\r\n" + strings.Repeat("z", 40) + "\r\n")
	res, err := s.Insert(context.Background(), s.Epoch(), &model.Message{Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) == 0 {
		t.Fatalf("spill files: %v %v", ents, err)
	}
	for _, e := range ents {
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	if s.Stats().UnreadCount != 1 {
		t.Fatalf("unread before get=%d", s.Stats().UnreadCount)
	}
	_, err = s.Get(res.ID, true)
	if !errors.Is(err, ErrSpill) {
		t.Fatalf("get err=%v", err)
	}
	if s.Stats().UnreadCount != 1 {
		t.Fatalf("spill error marked read: unread=%d", s.Stats().UnreadCount)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = s.Wait(ctx, model.MessageFilter{Subject: "spill"})
	if !errors.Is(err, ErrSpill) {
		t.Fatalf("wait err=%v", err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("wait spun until timeout: %s", time.Since(start))
	}
	_, err = s.List(model.ListQuery{Limit: 10})
	if !errors.Is(err, ErrSpill) {
		t.Fatalf("list err=%v", err)
	}
	if s.Stats().MessageCount != 1 {
		t.Fatalf("list spill must not drop the index: count=%d", s.Stats().MessageCount)
	}
}

func TestNewFailsIfSpillCannotClear(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "01HZYXWV7TSRQPJMKN76543210.raw")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()
	_, err := New(Options{
		MaxMessages:    10,
		MaxBytes:       1024,
		SpillDirectory: dir,
		SpillThreshold: 1,
	})
	if !errors.Is(err, ErrSpill) {
		t.Fatalf("err=%v", err)
	}
}

func idsOf(r model.ListResult) []string {
	out := make([]string, len(r.Items))
	for i, m := range r.Items {
		out[i] = m.ID
	}
	return out
}

func TestMemoryInsertCanceled(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1024, FullPolicy: model.FullPolicyReject})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Insert(ctx, s.Epoch(), rawMsg("x", "y"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestReplaceCapsRejectOverWithoutForce(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("a", "one")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("b", "two")); err != nil {
		t.Fatal(err)
	}
	err := s.ReplaceCaps(Options{MaxMessages: 1, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject}, false)
	if !errors.Is(err, ErrOverNewCap) {
		t.Fatalf("err=%v", err)
	}
	if s.Stats().MessageCount != 2 {
		t.Fatal("reject shrink must not evict")
	}
}

func TestReplaceCapsForceEvictsOldest(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	first, err := s.Insert(context.Background(), s.Epoch(), rawMsg("a", "one"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("b", "two")); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceCaps(Options{MaxMessages: 1, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject}, true); err != nil {
		t.Fatal(err)
	}
	st := s.Stats()
	if st.MessageCount != 1 {
		t.Fatalf("count=%d", st.MessageCount)
	}
	if _, err := s.Get(first.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("oldest should be evicted: %v", err)
	}
}

func TestReplaceCapsEvictOldestPolicy(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("a", "one")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("b", "two")); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceCaps(Options{MaxMessages: 1, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyEvictOldest}, false); err != nil {
		t.Fatal(err)
	}
	if s.Stats().MessageCount != 1 {
		t.Fatalf("count=%d", s.Stats().MessageCount)
	}
}

func TestConfigureRejectOverDoesNotMutate(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("a", "one")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("b", "two")); err != nil {
		t.Fatal(err)
	}
	err := s.Configure(Options{MaxMessages: 1, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if !errors.Is(err, ErrOverNewCap) {
		t.Fatalf("err=%v", err)
	}
	if s.Stats().MessageCount != 2 {
		t.Fatal("configure reject evicted")
	}
	if _, err := s.Insert(context.Background(), s.Epoch(), rawMsg("c", "three")); err != nil {
		t.Fatalf("old caps should still accept: %v", err)
	}
}

func TestCheckOptionsBadSpill(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := CheckOptions(Options{
		MaxMessages:    1,
		MaxBytes:       1024,
		FullPolicy:     model.FullPolicyReject,
		SpillDirectory: filepath.Join(blocker, "spill"),
	})
	if err == nil {
		t.Fatal("expected spill mkdir failure")
	}
}

func TestNewRejectsBadCaps(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("empty options")
	}
	if _, err := New(Options{MaxMessages: 1, MaxBytes: 10, FullPolicy: "drop"}); err == nil {
		t.Fatal("bad policy")
	}
}

func TestSentinelTooLargeAndNotFound(t *testing.T) {
	if errors.Is(ErrTooLarge, ErrFull) || errors.Is(ErrNotFound, ErrStaleEpoch) {
		t.Fatal("sentinels must differ")
	}
}

func newTestStore(t *testing.T, opts Options) *Memory {
	t.Helper()
	s, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Wipe)
	return s
}

func rawMsg(subject, body string) *model.Message {
	raw := []byte("Subject: " + subject + "\r\n\r\n" + body + "\r\n")
	return &model.Message{Raw: raw}
}
