package snapshot

import (
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// Snapshot is immutable after Compile returns. Inbox contents are not a field.
type Snapshot struct {
	Canonical         *model.State
	Revision          model.Revision
	BootstrapRevision model.Revision
	Generation        model.Generation
	CompiledAt        time.Time
}

// Drifted reports runtimeRevision != bootstrapRevision.
func (s *Snapshot) Drifted() bool {
	if s == nil {
		return false
	}
	return s.Revision != s.BootstrapRevision
}
