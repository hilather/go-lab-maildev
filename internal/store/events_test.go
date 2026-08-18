package store

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestSubscribeInsertDeleteWipe(t *testing.T) {
	s := newTestStore(t, Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	ch, cancel := s.Subscribe(8)
	defer cancel()
	res, err := s.Insert(context.Background(), s.Epoch(), &model.Message{Raw: []byte("Subject: ev\r\n\r\nbody\r\n")})
	if err != nil {
		t.Fatal(err)
	}
	got := <-ch
	if got.Type != EventMailReceived || got.ID != res.ID || got.Subject != "ev" {
		t.Fatalf("insert event %+v", got)
	}
	if err := s.Delete(res.ID); err != nil {
		t.Fatal(err)
	}
	got = <-ch
	if got.Type != EventMailDeleted || got.ID != res.ID {
		t.Fatalf("delete event %+v", got)
	}
	s.Wipe()
	got = <-ch
	if got.Type != EventStoreWiped || got.Generation == 0 {
		t.Fatalf("wipe event %+v", got)
	}
}
