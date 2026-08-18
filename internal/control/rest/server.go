package rest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/store"
)

const (
	// DefaultAddr is the first-GA management listen address.
	DefaultAddr = config.DefaultMgmtAddress

	// DefaultMaxBodyBytes matches the config document bound (1 MiB).
	DefaultMaxBodyBytes = 1 << 20

	// DefaultRequestTimeout is the per-request handler deadline.
	DefaultRequestTimeout = 30 * time.Second

	// DefaultReadHeaderTimeout bounds slowloris-style header stalls.
	DefaultReadHeaderTimeout = 5 * time.Second

	// DefaultReadTimeout bounds the whole request read.
	DefaultReadTimeout = 30 * time.Second

	// DefaultWriteTimeout bounds the response write (SSE can be long).
	DefaultWriteTimeout = 0

	// DefaultMaxConcurrent is the in-process overlapping-request cap.
	DefaultMaxConcurrent = 256

	headerRequestID   = "X-Request-ID"
	headerIdempotency = "Idempotency-Key"
	headerIfMatch     = "If-Match"
	headerExpected    = "X-LabMail-Expected-Revision"
	headerRevision    = "X-LabMail-Revision"
	headerAllow       = "Allow"

	requestURNPrefix = "urn:labmail:request:"
)

// Config constructs a management HTTP server.
type Config struct {
	// Addr is the listen address. Empty becomes DefaultAddr (:1080).
	Addr string
	// Service is required. Handlers call it and do not mutate snapshots.
	Service app.Service
	// AllowedOrigins are extra Origins accepted besides loopback. Empty denies
	// every non-loopback Origin (CORS/DNS-rebinding default-deny).
	AllowedOrigins []string
	// Live overrides liveness. Nil is always live while the process serves.
	Live func() bool
	// Ready overrides readiness. Nil is app.Status.Ready.
	Ready func() bool
	// MaxBodyBytes caps decoded request bodies. Non-positive uses DefaultMaxBodyBytes.
	MaxBodyBytes int64
	// RequestTimeout is the handler context deadline. Non-positive uses DefaultRequestTimeout.
	RequestTimeout time.Duration
	// ReadHeaderTimeout, ReadTimeout, WriteTimeout apply when ListenAndServe runs.
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	// MaxConcurrent admits at most this many overlapping requests. Non-positive uses DefaultMaxConcurrent.
	MaxConcurrent int
	// RatePerSec is a coarse per-source QPS. Zero uses config default. Negative disables.
	RatePerSec float64
	// RateBurst is the per-source burst. Zero uses config default.
	RateBurst float64
	// PublicMetrics serves GET /v1/metrics. False returns not_found.
	PublicMetrics bool
	// Mounts are additional handlers served ahead of REST routing.
	Mounts map[string]http.Handler
}

// Server is the stdlib net/http management listener.
type Server struct {
	cfg      Config
	svc      app.Service
	routes   []compiledRoute
	handler  http.Handler
	maxBody  int64
	timeout  time.Duration
	inflight chan struct{}
	rate     *limiter

	cursorMu  sync.Mutex
	cursorKey []byte

	mu     sync.Mutex
	http   *http.Server
	ln     net.Listener
	closed atomic.Bool
}

// New builds a Server. Routes come from the frozen capability registry.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("rest: Service is required")
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = DefaultMaxBodyBytes
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	n := cfg.MaxConcurrent
	if n <= 0 {
		n = DefaultMaxConcurrent
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	s := &Server{
		cfg:       cfg,
		svc:       cfg.Service,
		routes:    compileRoutes(capabilities.All()),
		maxBody:   maxBody,
		timeout:   timeout,
		inflight:  make(chan struct{}, n),
		rate:      newLimiter(cfg.RatePerSec, cfg.RateBurst),
		cursorKey: key,
	}
	s.handler = http.HandlerFunc(s.serveHTTP)
	if len(cfg.Mounts) > 0 {
		mux := http.NewServeMux()
		for path, h := range cfg.Mounts {
			mux.Handle(path, h)
		}
		mux.Handle("/", http.HandlerFunc(s.serveHTTP))
		s.handler = mux
	}
	return s, nil
}

// Handler returns the management mux. Safe for httptest.NewServer / ServeHTTP.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// ListenAndServe binds Addr (default :1080) and serves until Shutdown.
func (s *Server) ListenAndServe() error {
	addr := s.cfg.Addr
	if addr == "" {
		addr = DefaultAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Attach records ln so Addr() is correct before Serve returns.
func (s *Server) Attach(ln net.Listener) {
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
}

// Serve serves on ln until Shutdown. ln is closed by Shutdown or on return.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		_ = ln.Close()
		return nil
	}
	if s.http != nil {
		s.mu.Unlock()
		_ = ln.Close()
		return errors.New("rest: server already started")
	}
	rh := s.cfg.ReadHeaderTimeout
	if rh <= 0 {
		rh = DefaultReadHeaderTimeout
	}
	rt := s.cfg.ReadTimeout
	if rt <= 0 {
		rt = DefaultReadTimeout
	}
	hs := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: rh,
		ReadTimeout:       rt,
		WriteTimeout:      s.cfg.WriteTimeout,
		MaxHeaderBytes:    1 << 16,
	}
	s.http = hs
	s.ln = ln
	alreadyClosed := s.closed.Load()
	s.mu.Unlock()
	if alreadyClosed {
		_ = ln.Close()
		return nil
	}
	err := hs.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown closes the listener and waits for in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	s.closed.Store(true)
	s.mu.Lock()
	hs := s.http
	ln := s.ln
	s.mu.Unlock()
	if hs != nil {
		return hs.Shutdown(ctx)
	}
	if ln != nil {
		return ln.Close()
	}
	return nil
}

// Addr returns the bound address after Serve, or the configured listen address.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	if s.cfg.Addr != "" {
		return s.cfg.Addr
	}
	return DefaultAddr
}

// RotateCursors issues a new HMAC key. Reset/restart invalidate list cursors.
func (s *Server) RotateCursors() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return
	}
	s.cursorMu.Lock()
	s.cursorKey = key
	s.cursorMu.Unlock()
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)
	instance := requestURNPrefix + reqID

	if err := checkOrigin(r.Header.Get("Origin"), s.cfg.AllowedOrigins); err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	if r.Method == http.MethodOptions {
		s.writeProblem(w, r, instance, domainerr.Forbidden("CORS is disabled"))
		return
	}

	if strings.Contains(strings.ToLower(r.URL.Path), "/relay") {
		s.writeProblem(w, r, instance, domainerr.ReceiveOnly("LabMail is receive-only; relay is not implemented"))
		return
	}

	select {
	case s.inflight <- struct{}{}:
		defer func() { <-s.inflight }()
	default:
		s.writeProblem(w, r, instance, domainerr.RateLimited("too many concurrent management requests"))
		return
	}

	ctx := r.Context()
	var cancel context.CancelFunc
	if s.timeout > 0 && !isWaitPath(r.URL.Path) && !isSSEPath(r.URL.Path) {
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	r = r.WithContext(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			s.writeProblem(w, r, instance, domainerr.Internal("internal error"))
		}
	}()

	rt, params, pathOK, methodOK := matchRoute(s.routes, r.Method, r.URL.Path)
	if !pathOK {
		s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
		return
	}
	if !methodOK {
		w.Header().Set(headerAllow, allowedMethods(s.routes, r.URL.Path))
		s.writeProblem(w, r, instance, domainerr.MethodNotAllowed("method not allowed"))
		return
	}

	if !isHealthCap(rt.cap) {
		if err := s.rate.allow(r.RemoteAddr); err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
	}

	actor := s.authenticate(r)
	s.dispatch(w, r, instance, actor, rt, params)
}

func isHealthCap(cap capabilities.Capability) bool {
	return cap.ID == capabilities.HealthLive || cap.ID == capabilities.HealthReady
}

func isWaitPath(path string) bool {
	return strings.HasSuffix(path, ":wait")
}

func isSSEPath(path string) bool {
	return strings.HasSuffix(path, "/events/stream")
}

func requestID(r *http.Request) string {
	if id := r.Header.Get(headerRequestID); id != "" {
		return id
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}

func (s *Server) isLive() bool {
	if s.cfg.Live != nil {
		return s.cfg.Live()
	}
	return !s.closed.Load()
}

func (s *Server) isReady(ctx context.Context) bool {
	if s.cfg.Ready != nil {
		return s.cfg.Ready()
	}
	st, err := s.svc.Status(ctx, app.Actor{ID: "ready", Class: "startup", Transport: "rest"})
	return err == nil && st != nil && st.Ready
}

func (s *Server) inbox() *store.Memory {
	type hasInbox interface {
		Inbox() *store.Memory
	}
	if h, ok := s.svc.(hasInbox); ok {
		return h.Inbox()
	}
	return nil
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
