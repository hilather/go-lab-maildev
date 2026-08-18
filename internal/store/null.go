package store

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// Null acknowledges mail and retains nothing.
type Null struct {
	mu    sync.Mutex
	epoch uint64
	seq   uint64
}

// NewNull returns a Sink that discards every insert.
func NewNull() *Null {
	return &Null{epoch: 1}
}

// Insert assigns a discard id and drops the message.
func (n *Null) Insert(ctx context.Context, epoch uint64, msg *model.Message) (model.InsertResult, error) {
	if n == nil {
		return model.InsertResult{}, errors.New("store: nil Null")
	}
	if err := ctx.Err(); err != nil {
		return model.InsertResult{}, err
	}
	if msg == nil {
		return model.InsertResult{}, errors.New("store: nil message")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if epoch != n.epoch {
		return model.InsertResult{}, ErrStaleEpoch
	}
	n.seq++
	id := fmt.Sprintf("null-%d", n.seq)
	return model.InsertResult{ID: id}, nil
}

// Epoch is bumped by Wipe so in-flight DATA can be aborted.
func (n *Null) Epoch() uint64 {
	if n == nil {
		return 0
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.epoch
}

// Wipe increments the epoch. Nothing is retained to clear.
func (n *Null) Wipe() {
	if n == nil {
		return
	}
	n.mu.Lock()
	n.epoch++
	n.mu.Unlock()
}

var _ Sink = (*Null)(nil)
