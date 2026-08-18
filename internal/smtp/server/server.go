package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtp/codec"
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
)

// Options construct a Server.
type Options struct {
	Address string
	Spec    model.SMTPSpec
	Store   store.Sink
}

// Server is a plain SMTP receive listener.
type Server struct {
	addr  string
	spec  atomic.Pointer[model.SMTPSpec]
	store store.Sink
	gate  *gate

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
	sink := opts.Store
	if sink == nil {
		sink = store.NewNull()
	}
	spec := withSpecDefaults(opts.Spec)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		addr:   opts.Address,
		store:  sink,
		gate:   newGate(),
		ctx:    ctx,
		cancel: cancel,
		conns:  make(map[net.Conn]struct{}),
	}
	s.spec.Store(&spec)
	return s, nil
}

// SwapSpec replaces the snapshot used by the next MAIL, RCPT, and DATA.
func (s *Server) SwapSpec(spec model.SMTPSpec) {
	spec = withSpecDefaults(spec)
	s.spec.Store(&spec)
}

func (s *Server) specNow() model.SMTPSpec {
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

// Addr is the bound address, or nil before Start.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
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
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
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
		_ = codec.WriteReply(conn, 421, spec.Hostname+" Too many connections")
		return
	}
	defer s.gate.release(ip)

	sess := &session{
		srv:     s,
		conn:    conn,
		rd:      codec.NewReader(conn),
		started: time.Now(),
		state:   stateGreeting,
	}
	sess.run()
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
