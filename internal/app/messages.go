package app

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/hilather/go-lab-maildev/internal/audit"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/store"
)

func (s *App) requireInbox() error {
	if s == nil || s.inbox == nil {
		return domainerr.Internal("no inbox")
	}
	return nil
}

func (s *App) matchStoreGeneration(in DeleteIn) error {
	if in.ExpectedStoreGeneration == nil || s.inbox == nil {
		return nil
	}
	cur := s.inbox.Generation()
	if cur != *in.ExpectedStoreGeneration {
		return domainerr.RevisionConflict("store generation does not match", strconv.FormatUint(cur, 10))
	}
	return nil
}

// ListMessages is a filtered inbox page. SMTP insert is not on this path.
func (s *App) ListMessages(ctx context.Context, actor Actor, q model.ListQuery) (model.ListResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return model.ListResult{}, err
	}
	_ = actor
	if err := s.requireInbox(); err != nil {
		return model.ListResult{}, err
	}
	return s.inbox.List(q)
}

// GetMessage loads one message. markRead does not bump storeGeneration.
func (s *App) GetMessage(ctx context.Context, actor Actor, id string, markRead bool) (*model.Message, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	if err := s.requireInbox(); err != nil {
		return nil, err
	}
	msg, err := s.inbox.Get(id, markRead)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return msg, nil
}

// DeleteMessage removes one message and audits.
func (s *App) DeleteMessage(ctx context.Context, actor Actor, id string, in DeleteIn) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	if err := s.requireInbox(); err != nil {
		return err
	}
	if err := s.matchStoreGeneration(in); err != nil {
		return err
	}
	if err := s.inbox.Delete(id); err != nil {
		return mapStoreErr(err)
	}
	s.recordAudit(ctx, audit.Event{
		Time:            s.now(),
		ActorID:         actor.ID,
		ActorClass:      actor.Class,
		Transport:       actor.Transport,
		Capability:      "messages.delete",
		MessageID:       id,
		StoreGeneration: s.inbox.Generation(),
		Result:          audit.ResultOK,
	})
	return nil
}

// ClearMessages empties the inbox without bumping epoch.
func (s *App) ClearMessages(ctx context.Context, actor Actor, in DeleteIn) (int, error) {
	if err := s.requireCtx(ctx); err != nil {
		return 0, err
	}
	if err := s.requireInbox(); err != nil {
		return 0, err
	}
	if err := s.matchStoreGeneration(in); err != nil {
		return 0, err
	}
	n, err := s.inbox.DeleteAll()
	if err != nil {
		return 0, mapStoreErr(err)
	}
	s.recordAudit(ctx, audit.Event{
		Time:            s.now(),
		ActorID:         actor.ID,
		ActorClass:      actor.Class,
		Transport:       actor.Transport,
		Capability:      "messages.clear",
		StoreGeneration: s.inbox.Generation(),
		Result:          audit.ResultOK,
	})
	return n, nil
}

// MarkRead sets the read bit. Generation is unchanged.
func (s *App) MarkRead(ctx context.Context, actor Actor, id string) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	_ = actor
	if err := s.requireInbox(); err != nil {
		return err
	}
	return mapStoreErr(s.inbox.MarkRead(id))
}

// MarkAllRead sets every read bit. Generation is unchanged.
func (s *App) MarkAllRead(ctx context.Context, actor Actor) (int, error) {
	if err := s.requireCtx(ctx); err != nil {
		return 0, err
	}
	_ = actor
	if err := s.requireInbox(); err != nil {
		return 0, err
	}
	n, err := s.inbox.MarkAllRead()
	return n, mapStoreErr(err)
}

// Wait returns the newest matching message or a timeout domain error.
// Timeout 0 uses DefaultWaitTimeout (10s). The wait is capped by store.maxWait.
func (s *App) Wait(ctx context.Context, actor Actor, in WaitIn) (*model.Message, error) {
	if err := s.requireCtx(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, domainerr.Timeout("wait timed out")
		}
		return nil, err
	}
	_ = actor
	if err := s.requireInbox(); err != nil {
		return nil, err
	}
	timeout := in.Timeout
	if timeout <= 0 {
		timeout = DefaultWaitTimeout
	}
	maxWait := config.DefaultStoreMaxWait
	if snap := s.Active(); snap != nil && snap.Canonical != nil && snap.Canonical.Spec.Store.MaxWait > 0 {
		maxWait = snap.Canonical.Spec.Store.MaxWait
	}
	if timeout > maxWait {
		timeout = maxWait
	}
	if dl, ok := ctx.Deadline(); ok {
		remain := time.Until(dl)
		if remain <= 0 {
			return nil, domainerr.Timeout("wait timed out")
		}
		if remain < timeout {
			timeout = remain
		}
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	msg, err := s.inbox.Wait(wctx, in.Filter)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, domainerr.Timeout("wait timed out")
		}
		return nil, mapStoreErr(err)
	}
	return msg, nil
}

// Subscribe fans out inbox membership events. cancel must be called.
func (s *App) Subscribe(ctx context.Context, actor Actor, buffer int) (<-chan InboxEvent, func()) {
	_ = ctx
	_ = actor
	if s == nil || s.inbox == nil {
		ch := make(chan InboxEvent)
		close(ch)
		return ch, func() {}
	}
	src, cancelSrc := s.inbox.Subscribe(buffer)
	if buffer <= 0 {
		buffer = 16
	}
	out := make(chan InboxEvent, buffer)
	done := make(chan struct{})
	go func() {
		defer close(out)
		for {
			select {
			case <-done:
				return
			case ev, ok := <-src:
				if !ok {
					return
				}
				next := InboxEvent{Type: ev.Type, ID: ev.ID, Subject: ev.Subject, Generation: ev.Generation}
				select {
				case out <- next:
				case <-done:
					return
				}
			}
		}
	}()
	var once sync.Once
	return out, func() {
		once.Do(func() {
			close(done)
			cancelSrc()
		})
	}
}

// OnReset registers a hook invoked after a successful Reset (cursor rotation).
func (s *App) OnReset(fn func()) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.resetHooks = append(s.resetHooks, fn)
	s.mu.Unlock()
}

func mapStoreErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return domainerr.NotFound("message not found")
	case errors.Is(err, store.ErrFull):
		return domainerr.StoreFull("inbox is full")
	case errors.Is(err, store.ErrOverNewCap):
		return domainerr.StoreOverNewCap("inbox occupancy exceeds the new store caps")
	default:
		return asDomain(err)
	}
}
