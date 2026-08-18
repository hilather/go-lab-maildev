package store

import (
	"context"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// Sink accepts captured messages from the SMTP data plane.
type Sink interface {
	// Insert records msg if epoch still matches the value captured at DATA start.
	Insert(ctx context.Context, epoch uint64, msg *model.Message) (model.InsertResult, error)
	Epoch() uint64
}
