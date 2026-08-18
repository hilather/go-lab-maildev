package store

import (
	"context"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// Sink accepts captured messages from the SMTP data plane.
type Sink interface {
	Insert(context.Context, *model.Message) (model.InsertResult, error)
	Epoch() uint64
}
