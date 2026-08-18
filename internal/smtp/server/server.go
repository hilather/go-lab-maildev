package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/observability"
	"github.com/hilather/go-lab-maildev/internal/smtp/codec"
	"github.com/hilather/go-lab-maildev/internal/snapshot"
	"github.com/hilather/go-lab-maildev/internal/store"
)

// DefaultShutdownWait is the serve drain deadline when none is configured.
const DefaultShutdownWait = 5 * time.Second

const (
	defaultHostname      = "labmail.lab"
	defaultMaxMessage    = int64(10 << 20)
	defaultMaxRecipients = 100
	defaultMaxSessions   = 256
	defaultMaxPerIP      = 32
	defaultMaxInFlight   = 8
	defaultMaxInFlightB  = int64(64 << 20)
	defaultSessionTO     = 10 * time.Minute
	defaultCommandIdle   = 120 * time.Second
	defaultDataIdle      = 180 * time.Second
	writeWait            = 30 * time.Second
	dataDiscardSlack     = int64(1 << 20)
	acceptBackoffMin     = 5 * time.Millisecond
	acceptBackoffMax     = 200 * time.Millisecond
)

// Options construct a Server.
type Options struct {
	Address string
	Spec    model.SMTPSpec
	Store   store.Sink
	// Snapshots, when set, is re-read on every command (and the greeting)
	// via specNow, so live apply including smtp.behavior takes effect
	// without restarting the session.
	Snapshots *snapshot.Store
	// Metrics and Logger are optional. Nil is a no-op.
	Metrics *observability.Registry
	Logger  *observability.Logger
}

// Server is a plain SMTP receive listener.
type Server struct {
	addr    string
	spec    atomic.Pointer[model.SMTPSpec]
	snaps   *snapshot.Store
	store   store.Sink
	gate    *gate
	metrics *observability.Registry
	logger  *observability.Logger

	ctx    context.Context
	cancel context.CancelFunc

	ln net.Listener

	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	started bool
	stopped bool

	wg sync.WaitGroup
}

// New validates opts. Start binds and accepts.
func New(opts Options) (*Server, error) {
	if opts.Address == "" {
		return nil, errors.New("smtp/server: Address is required")
	}
	if err := rejectUnimplemented(opts.Spec); err != nil {
		return nil, err
	}
	sink := opts.Store
	if sink == nil {
		sink = store.NewNull()
	}
	spec := withSpecDefaults(opts.Spec)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		addr:    opts.Address,
		snaps:   opts.Snapshots,
		store:   sink,
		gate:    newGate(),
		metrics: opts.Metrics,
		logger:  opts.Logger,
		ctx:     ctx,
		cancel:  cancel,
		conns:   make(map[net.Conn]struct{}),
	}
	s.spec.Store(&spec)
	return s, nil
}

// SwapSpec replaces the fallback spec used when no snapshot store is attached.
// Live apply prefers Snapshots.Load on the next command (or greeting).
func (s *Server) SwapSpec(spec model.SMTPSpec) error {
	if err := rejectUnimplemented(spec); err != nil {
		return err
	}
	spec = withSpecDefaults(spec)
	s.spec.Store(&spec)
	return nil
}

func (s *Server) specNow() model.SMTPSpec {
	if s.snaps != nil {
		if snap := s.snaps.Load(); snap != nil && snap.Canonical != nil {
			return withSpecDefaults(snap.Canonical.Spec.SMTP)
		}
	}
	p := s.spec.Load()
	if p == nil {
		return withSpecDefaults(model.SMTPSpec{})
	}
	return *p
}

// Start binds the listener and serves in the background.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("smtp/server: already started")
	}
	if s.stopped {
		return errors.New("smtp/server: start after shutdown")
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("smtp/server: listen: %w", err)
	}
	s.ln = ln
	s.started = true
	s.wg.Add(1)
	go s.accept()
	return nil
}

// Addr is the last bound address, or nil before Start. It stays set after
// Shutdown so callers can log the former listen address. Use Accepting to
// decide whether new sessions are still admitted.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// Accepting is true after Start and before Shutdown begins (listener still
// admits connections). Ready probes must use this, not Addr() != nil.
func (s *Server) Accepting() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started && !s.stopped && s.ln != nil
}

// Shutdown stops accepts, closes sessions, and waits up to ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.cancel()
	ln := s.ln
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range conns {
		_ = c.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) accept() {
	defer s.wg.Done()
	backoff := acceptBackoffMin
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			if acceptShouldStop(err) {
				return
			}
			time.Sleep(backoff)
			if backoff < acceptBackoffMax {
				backoff *= 2
				if backoff > acceptBackoffMax {
					backoff = acceptBackoffMax
				}
			}
			continue
		}
		backoff = acceptBackoffMin
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns[conn] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serveConn(conn)
	}
}

func acceptShouldStop(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}

func (s *Server) serveConn(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
	}()

	spec := s.specNow()
	ip := remoteIP(conn.RemoteAddr())
	if err := s.gate.acquire(ip, spec.Admission); err != nil {
		_ = conn.SetDeadline(time.Now().Add(writeWait))
		_ = codec.WriteReply(conn, 421, "4.3.2 Too many connections")
		s.incSession("rejected")
		s.logSMTP(observability.Record{
			Event:     observability.EventSMTPRejected,
			Component: "smtp",
			SMTPCode:  421,
			Result:    "rejected",
			Remote:    conn.RemoteAddr().String(),
		})
		return
	}
	defer func() {
		s.gate.release(ip)
		s.setActive()
	}()
	s.setActive()

	sess := &session{
		srv:     s,
		conn:    conn,
		rd:      codec.NewReader(conn),
		started: time.Now(),
		state:   stateGreeting,
	}
	sess.run()
}

func (s *Server) incSession(result string) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.Inc(observability.MetricSMTPSessionsTotal, map[string]string{
		"result": observability.SMTPSessionResult(result),
	}, 1)
}

func (s *Server) incMessage(result string) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.Inc(observability.MetricSMTPMessagesTotal, map[string]string{
		"result": observability.SMTPMessageResult(result),
	}, 1)
}

func (s *Server) setActive() {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.Set(observability.MetricSMTPSessionsActive, nil, float64(s.gate.Sessions()))
}

func (s *Server) observeSession(d time.Duration) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.Observe(observability.MetricSMTPSessionDuration, nil, d.Seconds())
}

func (s *Server) logSMTP(rec observability.Record) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Log(rec)
}

func withSpecDefaults(s model.SMTPSpec) model.SMTPSpec {
	if s.Hostname == "" {
		s.Hostname = defaultHostname
	}
	if s.MaxMessageBytes <= 0 {
		s.MaxMessageBytes = defaultMaxMessage
	}
	if s.MaxRecipients <= 0 {
		s.MaxRecipients = defaultMaxRecipients
	}
	if s.HideExtensions == nil {
		s.HideExtensions = []string{}
	}
	ad := s.Admission
	if ad.MaxSessions <= 0 {
		ad.MaxSessions = defaultMaxSessions
	}
	if ad.MaxSessionsPerIP <= 0 {
		ad.MaxSessionsPerIP = defaultMaxPerIP
	}
	if ad.MaxInFlightData <= 0 {
		ad.MaxInFlightData = defaultMaxInFlight
	}
	if ad.MaxInFlightDataBytes <= 0 {
		ad.MaxInFlightDataBytes = defaultMaxInFlightB
	}
	if ad.SessionTimeout <= 0 {
		ad.SessionTimeout = defaultSessionTO
	}
	if ad.CommandIdle <= 0 {
		ad.CommandIdle = defaultCommandIdle
	}
	if ad.DataIdle <= 0 {
		ad.DataIdle = defaultDataIdle
	}
	s.Admission = ad
	return s
}

func rejectUnimplemented(spec model.SMTPSpec) error {
	switch spec.Auth.Mode {
	case "", model.SMTPAuthNone, model.SMTPAuthPlainLogin:
	default:
		return fmt.Errorf("smtp/server: smtp.auth.mode %q is not supported", spec.Auth.Mode)
	}
	switch spec.TLS.Mode {
	case "", model.TLSModeOff, model.TLSModeStartTLS:
	case model.TLSModeImplicit:
		return fmt.Errorf("smtp/server: smtp.tls.mode implicit is not supported until 1.1; use starttls or a future listeners.smtpImplicit bind")
	default:
		return fmt.Errorf("smtp/server: smtp.tls.mode %q is not supported", spec.TLS.Mode)
	}
	if spec.TLS.Required {
		mode := spec.TLS.Mode
		if mode == "" {
			mode = model.TLSModeOff
		}
		if mode != model.TLSModeStartTLS {
			return fmt.Errorf("smtp/server: smtp.tls.required is legal only when mode is starttls")
		}
	}
	return nil
}
