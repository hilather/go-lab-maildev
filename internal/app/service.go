package app

import (
	"context"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// Service is the HTTP-less capability surface. REST and MCP must call these
// methods rather than implementing mutation or query logic. SMTP insert is
// not on this interface.
type Service interface {
	GetState(ctx context.Context, actor Actor) (*StateView, error)
	Validate(ctx context.Context, actor Actor, in ValidateIn) (*Plan, error)
	Plan(ctx context.Context, actor Actor, in ChangeIn) (*Plan, error)
	Apply(ctx context.Context, actor Actor, in ChangeIn) (*ApplyResult, error)
	Export(ctx context.Context, actor Actor, format ExportFormat) (*Export, error)
	Reset(ctx context.Context, actor Actor, in ResetIn) (*ApplyResult, error)
	Status(ctx context.Context, actor Actor) (*Status, error)

	ListMessages(ctx context.Context, actor Actor, q model.ListQuery) (model.ListResult, error)
	GetMessage(ctx context.Context, actor Actor, id string, markRead bool) (*model.Message, error)
	DeleteMessage(ctx context.Context, actor Actor, id string, in DeleteIn) error
	ClearMessages(ctx context.Context, actor Actor, in DeleteIn) (int, error)
	MarkRead(ctx context.Context, actor Actor, id string) error
	MarkAllRead(ctx context.Context, actor Actor) (int, error)
	Wait(ctx context.Context, actor Actor, in WaitIn) (*model.Message, error)
	Extract(ctx context.Context, actor Actor, id string) (*ExtractResult, error)
	Subscribe(ctx context.Context, actor Actor, buffer int) (<-chan InboxEvent, func())
	OnReset(fn func())

	QueryAudit(ctx context.Context, actor Actor, in AuditQuery) (*AuditList, error)
	GetAudit(ctx context.Context, actor Actor, id string) (*AuditEvent, error)
}
