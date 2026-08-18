package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestRaceInsertDeleteWaitWipe(t *testing.T) {
	s := newTestStore(t, Options{
		MaxMessages: 200,
		MaxBytes:    8 << 20,
		FullPolicy:  model.FullPolicyEvictOldest,
		MaxWait:     50 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				res, err := s.Insert(context.Background(), s.Epoch(), rawMsg("race", "x"))
				if err != nil {
					continue
				}
				_, _ = s.Get(res.ID, j%2 == 0)
				_, _ = s.List(model.ListQuery{Limit: 10})
				_ = s.Delete(res.ID)
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Millisecond)
				_, _ = s.Wait(wctx, model.MessageFilter{SubjectContains: "race"})
				wcancel()
				if j%11 == 0 {
					_, _ = s.DeleteAll()
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8; i++ {
			time.Sleep(2 * time.Millisecond)
			s.Wipe()
		}
	}()
	wg.Wait()
}
