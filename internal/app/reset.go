package app

import (
	"context"
	"os"

	"github.com/hilather/go-lab-maildev/internal/audit"
	"github.com/hilather/go-lab-maildev/internal/compiler"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/snapshot"
	"github.com/hilather/go-lab-maildev/internal/store"
)

// Reset rereads the bootstrap mount, compiles, wipes the inbox, and swaps
// only after success. It never writes the bootstrap file. A missing or
// invalid file leaves the active snapshot and inbox unchanged.
func (s *App) Reset(ctx context.Context, actor Actor, in ResetIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.snaps.Load()
	gen := model.Generation(0)
	if prev != nil {
		gen = prev.Generation + 1
	}

	next, err := s.loadBootstrapCandidate(ctx, gen)
	if err != nil {
		return nil, err
	}

	diff, _, err := diffStates(canonicalOf(prev), next.Canonical)
	if err != nil {
		return nil, err
	}

	if err := rejectUnimplementedSMTP(next.Canonical.Spec.SMTP); err != nil {
		return nil, err
	}

	// Validate new store options (including creatable spill dir) before
	// Wipe so a failed Reset cannot empty the inbox under the old snapshot.
	if s.inbox != nil {
		opts, err := store.CheckOptions(store.OptionsFromSpec(next.Canonical.Spec.Store))
		if err != nil {
			return nil, asDomain(err)
		}
		if err := s.inbox.ResetTo(opts); err != nil {
			return nil, asDomain(err)
		}
	}

	displaced := s.snaps.Swap(next)
	s.snaps.SetBootstrap(next)
	s.idemp.clear()

	res := &ApplyResult{
		Plan:            *s.planFrom(&candidate{prev: displaced, next: next, diff: diff}),
		Applied:         true,
		Generation:      next.Generation,
		RuntimeRevision: next.Revision,
	}
	res.AuditEventID = s.recordAudit(ctx, audit.Event{
		Time:            s.now(),
		ActorID:         actor.ID,
		ActorClass:      actor.Class,
		Transport:       actor.Transport,
		Capability:      "state.reset",
		Reason:          in.Reason,
		Revision:        next.Revision,
		Previous:        revisionOf(displaced),
		StoreGeneration: storeGeneration(s.inbox),
		Result:          audit.ResultOK,
		Diff:            toAuditDiff(diff),
	})
	return cloneApply(res), nil
}

func (s *App) loadBootstrapCandidate(ctx context.Context, gen model.Generation) (*snapshot.Snapshot, error) {
	if s.bootstrapPath != "" {
		if _, err := os.Stat(s.bootstrapPath); err != nil {
			if os.IsNotExist(err) {
				return nil, domainerr.ValidationFailed("bootstrap file unavailable",
					domainerr.FieldViolation{Path: "bootstrapPath", Code: "required", Message: "bootstrap file is missing; active snapshot unchanged"})
			}
			return nil, domainerr.Internal("stat bootstrap: " + err.Error())
		}
		st, err := config.LoadFile(s.bootstrapPath)
		if err != nil {
			return nil, asDomain(err)
		}
		snap, err := compiler.Compile(ctx, st, compiler.CompileOpts{
			Now:        s.now(),
			Generation: gen,
		})
		if err != nil {
			return nil, asDomain(err)
		}
		return snap, nil
	}
	boot := s.snaps.Bootstrap()
	if boot == nil || boot.Canonical == nil {
		return nil, domainerr.ValidationFailed("no bootstrap snapshot",
			domainerr.FieldViolation{Path: "bootstrap", Code: "required", Message: "no bootstrap path or snapshot to reset to"})
	}
	copied, err := cloneState(boot.Canonical)
	if err != nil {
		return nil, err
	}
	snap, err := compiler.Compile(ctx, copied, compiler.CompileOpts{
		Now:        s.now(),
		Generation: gen,
	})
	if err != nil {
		return nil, asDomain(err)
	}
	return snap, nil
}

func canonicalOf(s *snapshot.Snapshot) *model.State {
	if s == nil {
		return nil
	}
	return s.Canonical
}

func storeGeneration(inbox *store.Memory) uint64 {
	if inbox == nil {
		return 0
	}
	return inbox.Generation()
}
