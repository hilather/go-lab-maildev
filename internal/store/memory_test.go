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
	if got.Attachments != nil && len(got.Attachments) == 1 && got.Attachments[0].ID != res.ID+":0" {
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
	res, err := s.Insert(context.Background(), s.Epoch(), rawMsg("r", "x"))
	if err != nil {
		t.Fatal(err)
	}
	g := s.Generation()
	if _, err := s.Get(res.ID, true); err != nil {
		t.Fatal(err)
	}
	if s.Generation() != g {
		t.Fatalf("generation %d -> %d", g, s.Generation())
	}
	n, err := s.MarkAllRead()
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if s.Generation() != g {
		t.Fatal("mark-all bumped generation")
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
