package compiler

import (
	"context"
	"time"

	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/snapshot"
)

// CompileOpts controls revision metadata and the compile clock.
// RequireAuthFiles opts state:validate into config.ValidateRuntime, which
// fails an absent or unreadable token file and opens basic password files.
// Boot, plan, apply, and reset leave it false. Those compiles still
// length-check a present token file and do not open basic password files.
type CompileOpts struct {
	Now               time.Time
	BootstrapRevision model.Revision
	Generation        model.Generation
	RequireAuthFiles  bool
}

// Compile normalizes and validates st (copy-on-write) and hashes canonical JSON.
// The returned Snapshot is immutable; callers must not mutate Canonical.
func Compile(ctx context.Context, st *model.State, opts CompileOpts) (*snapshot.Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n, err := config.Normalize(st)
	if err != nil {
		return nil, err
	}
	if opts.RequireAuthFiles {
		err = config.ValidateRuntime(n)
	} else {
		err = config.Validate(n)
	}
	if err != nil {
		return nil, err
	}
	rev, err := config.Revision(n)
	if err != nil {
		return nil, err
	}
	bootRev := opts.BootstrapRevision
	if bootRev == "" {
		bootRev = rev
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &snapshot.Snapshot{
		Canonical:         n,
		Revision:          rev,
		BootstrapRevision: bootRev,
		Generation:        opts.Generation,
		CompiledAt:        now,
	}, nil
}
