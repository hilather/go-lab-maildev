package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/compiler"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/smtp/server"
	"github.com/hilather/go-lab-maildev/internal/store"
)

type serveFlags struct {
	Config           string
	SMTPListen       string
	ManagementListen string
	ShutdownTimeout  time.Duration
	PIDFile          string
}

func parseServeFlags(args []string, stderr io.Writer) (serveFlags, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	smtpListen := fs.String("smtp-listen", "", "override SMTP listen address (empty uses YAML)")
	mgmtListen := fs.String("management-listen", "", "override management listen address; off/none/- leaves it unbound")
	shutdown := fs.Duration("shutdown-timeout", server.DefaultShutdownWait, "graceful shutdown deadline")
	pidFile := fs.String("pid-file", "", "write process id after the SMTP listener binds")
	if err := fs.Parse(args); err != nil {
		return serveFlags{}, err
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labmail serve: --config is required")
		return serveFlags{}, fmt.Errorf("missing --config")
	}
	return serveFlags{
		Config:           *path,
		SMTPListen:       *smtpListen,
		ManagementListen: *mgmtListen,
		ShutdownTimeout:  *shutdown,
		PIDFile:          *pidFile,
	}, nil
}

func serveCmd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, err := parseServeFlags(args, stderr)
	if err != nil {
		return 2
	}
	rt, err := serveFromConfig(ctx, flags)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labmail serve: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "labmail smtp listen=%s\n", rt.smtp.Addr().String())
	if managementUnbound(flags.ManagementListen) {
		_, _ = fmt.Fprintln(stdout, "labmail management: not bound")
	} else {
		_, _ = fmt.Fprintln(stdout, "labmail management: not implemented")
	}
	<-ctx.Done()
	deadline := flags.ShutdownTimeout
	if deadline <= 0 {
		deadline = server.DefaultShutdownWait
	}
	shctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	_ = rt.shutdown(shctx)
	_, _ = fmt.Fprintln(stdout, "labmail: shutting down")
	return 0
}

type serveRuntime struct {
	smtp    *server.Server
	inbox   *store.Memory
	pidPath string
}

func serveFromConfig(ctx context.Context, flags serveFlags) (*serveRuntime, error) {
	st, err := config.LoadFile(flags.Config)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", flags.Config, err)
	}
	res, err := compiler.Compile(ctx, st, compiler.CompileOpts{})
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	addr := res.Canonical.Spec.Listeners.SMTP.Address
	if flags.SMTPListen != "" {
		addr = flags.SMTPListen
	}
	inbox, err := store.New(store.OptionsFromSpec(res.Canonical.Spec.Store))
	if err != nil {
		return nil, err
	}
	srv, err := server.New(server.Options{
		Address: addr,
		Spec:    res.Canonical.Spec.SMTP,
		Store:   inbox,
	})
	if err != nil {
		inbox.Wipe()
		return nil, err
	}
	if err := srv.Start(); err != nil {
		inbox.Wipe()
		return nil, err
	}
	rt := &serveRuntime{smtp: srv, inbox: inbox, pidPath: flags.PIDFile}
	if err := writePIDFile(flags.PIDFile); err != nil {
		_ = srv.Shutdown(context.Background())
		inbox.Wipe()
		return nil, fmt.Errorf("pid-file: %w", err)
	}
	return rt, nil
}

func (r *serveRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var first error
	if r.smtp != nil {
		if err := r.smtp.Shutdown(ctx); err != nil {
			first = err
		}
	}
	if r.inbox != nil {
		r.inbox.Wipe()
	}
	if r.pidPath != "" {
		_ = os.Remove(r.pidPath)
	}
	return first
}

func managementUnbound(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "off", "none", "-":
		return true
	default:
		return false
	}
}

func writePIDFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
