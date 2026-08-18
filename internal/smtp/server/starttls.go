package server

import (
	"crypto/tls"
	"io"
	"net"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtp/codec"
)

func (s *session) cmdStartTLS() bool {
	spec := s.spec()
	if spec.TLS.Mode != model.TLSModeStartTLS {
		_ = s.reply(502, "5.5.1 STARTTLS not implemented")
		return true
	}
	if s.tls {
		_ = s.reply(503, "5.5.1 TLS already active")
		return true
	}
	if s.state == stateGreeting {
		_ = s.reply(503, "5.5.1 HELO/EHLO first")
		return true
	}
	if s.state != stateHelloed {
		_ = s.reply(503, "5.5.1 Bad sequence")
		return true
	}
	if handled, keep := s.applyErrorOverride("STARTTLS"); handled {
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	cfg, err := loadTLSConfig(spec)
	if err != nil {
		_ = s.reply(454, "4.7.0 TLS not available")
		return true
	}
	if !s.replyKeyed("STARTTLS", 220, "2.0.0 Ready to start TLS") {
		s.endResult = "behavior"
		return false
	}
	if err := s.setIdle(spec.Admission.CommandIdle); err != nil {
		return false
	}
	tc := tls.Server(starttlsConn{Conn: s.conn, r: s.rd}, cfg)
	if err := tc.Handshake(); err != nil {
		return false
	}
	s.conn = tc
	s.rd = codec.NewReader(tc)
	s.resetAfterTLS()
	return true
}

func loadTLSConfig(spec model.SMTPSpec) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(spec.TLS.CertFile, spec.TLS.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// starttlsConn prefers leftover command-reader bytes so a pipelined
// ClientHello is not dropped when wrapping the TCP conn.
type starttlsConn struct {
	net.Conn
	r io.Reader
}

func (c starttlsConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}
