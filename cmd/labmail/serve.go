package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/control/compat"
	"github.com/hilather/go-lab-maildev/internal/control/mcp"
	"github.com/hilather/go-lab-maildev/internal/control/rest"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtp/server"
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
	if rt.http == nil {
		_, _ = fmt.Fprintln(stdout, "labmail management: not bound")
	} else {
		_, _ = fmt.Fprintf(stdout, "labmail management listen=%s\n", rt.http.Addr())
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
	http    *rest.Server
	svc     *app.App
	pidPath string
}

func serveFromConfig(ctx context.Context, flags serveFlags) (*serveRuntime, error) {
	svc, err := app.Boot(ctx, app.Options{BootstrapPath: flags.Config})
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", flags.Config, err)
	}
	snap := svc.Active()
	if snap == nil || snap.Canonical == nil {
		svc.Close()
		return nil, fmt.Errorf("compile: no snapshot")
	}
	addr := snap.Canonical.Spec.Listeners.SMTP.Address
	if flags.SMTPListen != "" {
		addr = flags.SMTPListen
	}
	srv, err := server.New(server.Options{
		Address:   addr,
		Spec:      snap.Canonical.Spec.SMTP,
		Store:     svc.Inbox(),
		Snapshots: svc.Snapshots(),
	})
	if err != nil {
		svc.Close()
		return nil, err
	}
	if err := srv.Start(); err != nil {
		svc.Close()
		return nil, err
	}
	rt := &serveRuntime{smtp: srv, svc: svc, pidPath: flags.PIDFile}
	mgmt, unbound := managementListen(flags.ManagementListen, snap.Canonical.Spec.Listeners.Management.Address)
	if !unbound {
		hs, err := startManagement(svc, srv, mgmt, snap.Canonical.Spec)
		if err != nil {
			_ = srv.Shutdown(context.Background())
			svc.Close()
			return nil, err
		}
		rt.http = hs
	}
	if err := writePIDFile(flags.PIDFile); err != nil {
		_ = rt.shutdown(context.Background())
		return nil, fmt.Errorf("pid-file: %w", err)
	}
	return rt, nil
}

func startManagement(svc *app.App, smtp *server.Server, addr string, spec model.Spec) (*rest.Server, error) {
	if addr == "" {
		addr = rest.DefaultAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	ready := func() bool { return smtp.Addr() != nil }
	mcpPath := spec.Listeners.Management.MCPPath
	if mcpPath == "" {
		mcpPath = mcp.DefaultPath
	}
	mcpSrv, err := mcp.New(mcp.Config{
		Service:            svc,
		AllowedOrigins:     spec.Management.OriginAllowlist,
		AllowLegacyClients: spec.Management.MCP.AllowLegacyClients,
		MaxBodyBytes:       spec.Management.BodyLimit,
		MaxConcurrent:      spec.Management.MaxConcurrent,
		RatePerSec:         float64(spec.Management.RequestsPerSecond),
		RateBurst:          float64(spec.Management.Burst),
	})
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	mounts, err := compatMounts(svc, spec, ready)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	if mounts == nil {
		mounts = map[string]http.Handler{}
	}
	mounts[mcpPath] = mcpSrv.Handler()
	hs, err := rest.New(rest.Config{
		Addr:           addr,
		Service:        svc,
		AllowedOrigins: spec.Management.OriginAllowlist,
		MaxBodyBytes:   spec.Management.BodyLimit,
		MaxConcurrent:  spec.Management.MaxConcurrent,
		RatePerSec:     float64(spec.Management.RequestsPerSecond),
		RateBurst:      float64(spec.Management.Burst),
		PublicMetrics:  spec.Observability.Metrics.PublicPath,
		Mounts:         mounts,
		Ready:          ready,
	})
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	hs.Attach(ln)
	go func() { _ = hs.Serve(ln) }()
	return hs, nil
}

func compatMounts(svc *app.App, spec model.Spec, ready func() bool) (map[string]http.Handler, error) {
	if !spec.Listeners.Management.CompatEnabled {
		return nil, nil
	}
	h, err := compat.New(compat.Config{
		Service:        svc,
		AllowedOrigins: spec.Management.OriginAllowlist,
		Ready:          ready,
	})
	if err != nil {
		return nil, err
	}
	return h.Mounts(), nil
}

func managementListen(flagAddr, yamlAddr string) (addr string, unbound bool) {
	switch strings.ToLower(strings.TrimSpace(flagAddr)) {
	case "off", "none", "-":
		return "", true
	case "":
		if yamlAddr == "" {
			yamlAddr = rest.DefaultAddr
		}
		return yamlAddr, false
	default:
		return strings.TrimSpace(flagAddr), false
	}
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
	if r.http != nil {
		if err := r.http.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	if r.svc != nil {
		r.svc.Close()
	}
	if r.pidPath != "" {
		_ = os.Remove(r.pidPath)
	}
	return first
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
