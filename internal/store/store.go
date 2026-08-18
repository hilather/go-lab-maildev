package store

import (
	"context"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// DefaultListLimit / MaxListLimit match the native REST list contract.
const (
	DefaultListLimit = 50
	MaxListLimit     = 200
)

// Store is the queryable inbox. SMTP uses Sink (epoch-aware Insert).
// Wipe is the only epoch bump.
type Store interface {
	Sink
	Get(id string, markRead bool) (*model.Message, error)
	List(model.ListQuery) (model.ListResult, error)
	Delete(id string) error
	DeleteAll() (deleted int, err error)
	MarkRead(id string) error
	MarkAllRead() (int, error)
	Wait(ctx context.Context, filter model.MessageFilter) (*model.Message, error)
	Generation() uint64
	Stats() model.StoreStats
	Wipe()
	// ReplaceCaps applies maxMessages/maxBytes/fullPolicy. Shrink + reject
	// fails with ErrOverNewCap unless force (or the new policy is evict_oldest).
	ReplaceCaps(opts Options, force bool) error
	// Configure replaces all store options. Occupancy is checked against
	// the new caps before mutation (reject overflow does not change caps).
	Configure(opts Options) error
}
