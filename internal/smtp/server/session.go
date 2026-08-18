package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtp/codec"
	"github.com/hilather/go-lab-maildev/internal/store"
)

type sessionState int

const (
	stateGreeting sessionState = iota
	stateHelloed
	stateMail
	stateRcpt
)

type session struct {
	srv     *Server
	conn    net.Conn
	rd      *codec.Reader
	started time.Time
	state   sessionState

	helo         string
	from         string
	rcpt         []string
	declaredSize int64
	sizeSet      bool
}

func (s *session) run() {
	if err := s.reply(220, s.spec().Hostname+" LabMail ready"); err != nil {
		return
	}
	for {
		if err := s.setIdle(s.spec().Admission.CommandIdle); err != nil {
			return
		}
		line, err := s.rd.ReadCommandLine()
		if err != nil {
			if errors.Is(err, codec.ErrLineTooLong) {
				_ = s.reply(500, "5.5.2 Line too long")
			}
			return
		}
		verb, arg := codec.SplitVerb(line)
		if !s.dispatch(verb, arg) {
			return
		}
	}
}

func (s *session) dispatch(verb, arg string) bool {
	switch verb {
	case "HELO":
		s.cmdHello(arg, false)
	case "EHLO":
		s.cmdHello(arg, true)
	case "MAIL":
		s.cmdMail(arg)
	case "RCPT":
		s.cmdRcpt(arg)
	case "DATA":
		return s.cmdData()
	case "RSET":
		s.cmdRset()
	case "NOOP":
		_ = s.reply(250, "2.0.0 OK")
	case "QUIT":
		_ = s.reply(221, "2.0.0 Bye")
		return false
	case "HELP":
		_ = s.reply(214, "HELO EHLO MAIL RCPT DATA RSET NOOP QUIT HELP VRFY", "End of HELP")
	case "VRFY":
		_ = s.reply(252, "2.5.2 Cannot VRFY user")
	case "EXPN":
		_ = s.reply(502, "5.5.1 EXPN not implemented")
	case "AUTH":
		_ = s.reply(502, "5.5.1 AUTH not implemented")
	case "STARTTLS":
		_ = s.reply(502, "5.5.1 STARTTLS not implemented")
	case "BDAT":
		_ = s.reply(502, "5.5.1 BDAT not implemented")
	case "ETRN", "ATRN", "TURN":
		_ = s.reply(502, "5.5.1 "+verb+" not implemented")
	case "":
		_ = s.reply(500, "5.5.1 Command unrecognized")
	default:
		_ = s.reply(500, "5.5.1 Command unrecognized")
	}
	return true
}

func (s *session) cmdHello(arg string, ehlo bool) {
	domain := strings.TrimSpace(arg)
	if domain == "" {
		_ = s.reply(501, "5.5.4 Missing domain")
		return
	}
	s.resetTxn()
	s.helo = domain
	s.state = stateHelloed
	if !ehlo {
		_ = s.reply(250, "2.0.0 "+s.spec().Hostname)
		return
	}
	_ = s.reply(250, s.ehloLines()...)
}

func (s *session) ehloLines() []string {
	spec := s.spec()
	hidden := hiddenSet(spec.HideExtensions)
	lines := []string{spec.Hostname}
	if !hidden["SIZE"] {
		lines = append(lines, fmt.Sprintf("SIZE %d", spec.MaxMessageBytes))
	}
	if !hidden["8BITMIME"] {
		lines = append(lines, "8BITMIME")
	}
	if !hidden["SMTPUTF8"] {
		lines = append(lines, "SMTPUTF8")
	}
	if !hidden["ENHANCEDSTATUSCODES"] {
		lines = append(lines, "ENHANCEDSTATUSCODES")
	}
	return lines
}

func (s *session) cmdMail(arg string) {
	spec := s.spec()
	if s.state == stateGreeting {
		_ = s.reply(503, "5.5.1 HELO/EHLO first")
		return
	}
	if s.state != stateHelloed {
		_ = s.reply(503, "5.5.1 Nested MAIL")
		return
	}
	path, params, err := parsePathArg(arg, "FROM:")
	if err != nil {
		_ = s.reply(501, "5.5.4 Syntax error in parameters")
		return
	}
	size, sizeSet, err := parseMailParams(params)
	if err != nil {
		_ = s.reply(501, "5.5.4 Syntax error in parameters")
		return
	}
	if sizeSet && size > spec.MaxMessageBytes {
		_ = s.reply(552, "5.3.4 Message too large")
		return
	}
	s.from = path
	s.declaredSize = size
	s.sizeSet = sizeSet
	s.rcpt = s.rcpt[:0]
	s.state = stateMail
	_ = s.reply(250, "2.1.0 OK")
}

func (s *session) cmdRcpt(arg string) {
	spec := s.spec()
	if s.state != stateMail && s.state != stateRcpt {
		_ = s.reply(503, "5.5.1 Need MAIL")
		return
	}
	path, _, err := parsePathArg(arg, "TO:")
	if err != nil {
		_ = s.reply(501, "5.5.4 Syntax error in parameters")
		return
	}
	if len(s.rcpt) >= spec.MaxRecipients {
		_ = s.reply(452, "4.5.3 Too many recipients")
		return
	}
	s.rcpt = append(s.rcpt, path)
	s.state = stateRcpt
	_ = s.reply(250, "2.1.5 OK")
}

func (s *session) cmdData() bool {
	if s.state != stateRcpt || len(s.rcpt) == 0 {
		_ = s.reply(503, "5.5.1 Need RCPT")
		return true
	}
	spec := s.spec()
	reserve := reserveBytes(s.declaredSize, s.sizeSet, spec.MaxMessageBytes)
	if err := s.srv.gate.reserveData(reserve, spec.Admission); err != nil {
		if errors.Is(err, errTooManyInFlight) {
			_ = s.reply(421, "4.3.2 Too many concurrent messages")
			return true
		}
		_ = s.reply(452, "4.3.1 Insufficient storage")
		return true
	}
	defer s.srv.gate.releaseData(reserve)

	epoch := s.srv.store.Epoch()
	if err := s.reply(354, "Start mail input; end with <CRLF>.<CRLF>"); err != nil {
		return false
	}

	raw, err := s.readData(spec.MaxMessageBytes)
	if err != nil {
		switch {
		case errors.Is(err, errMessageTooLarge):
			_ = s.reply(552, "5.3.4 Message too large")
			s.resetToHelloed()
			return true
		case errors.Is(err, codec.ErrLineTooLong):
			_ = s.reply(500, "5.5.2 Line too long")
			s.resetToHelloed()
			return false
		case isTimeout(err):
			_ = s.reply(451, "4.4.2 Timeout")
			s.resetToHelloed()
			return false
		default:
			s.resetToHelloed()
			return false
		}
	}
	if s.srv.store.Epoch() != epoch {
		_ = s.reply(451, "4.3.2 Requested action aborted")
		s.resetToHelloed()
		return true
	}

	msg := &model.Message{
		ReceivedAt: time.Now().UTC(),
		Envelope: model.Envelope{
			From:       s.from,
			To:         append([]string(nil), s.rcpt...),
			HELO:       s.helo,
			RemoteAddr: s.conn.RemoteAddr().String(),
		},
		Raw:  raw,
		Size: len(raw),
	}
	res, err := s.srv.store.Insert(context.Background(), msg)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrStaleEpoch):
			_ = s.reply(451, "4.3.2 Requested action aborted")
		case errors.Is(err, store.ErrFull):
			_ = s.reply(452, "4.3.1 Insufficient storage")
		default:
			_ = s.reply(451, "4.3.2 Requested action aborted")
		}
		s.resetToHelloed()
		return true
	}
	_ = s.reply(250, "2.0.0 Queued as "+res.ID)
	s.resetToHelloed()
	return true
}

func (s *session) readData(max int64) ([]byte, error) {
	if err := s.setIdle(s.spec().Admission.DataIdle); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	var discarded int64
	over := false
	for {
		if err := s.setIdle(s.spec().Admission.DataIdle); err != nil {
			return nil, err
		}
		line, err := s.rd.ReadDataLine()
		if err != nil {
			return nil, err
		}
		body, end := codec.Unstuff(line)
		if end {
			break
		}
		need := int64(len(body) + 2)
		if over {
			discarded += need
			if discarded > dataDiscardSlack {
				return nil, codec.ErrLineTooLong
			}
			continue
		}
		if int64(buf.Len())+need > max {
			over = true
			discarded += need
			continue
		}
		buf.WriteString(body)
		buf.WriteString("\r\n")
	}
	if over {
		return nil, errMessageTooLarge
	}
	return buf.Bytes(), nil
}

func (s *session) cmdRset() {
	s.resetTxn()
	if s.state != stateGreeting {
		s.state = stateHelloed
	}
	_ = s.reply(250, "2.0.0 OK")
}

func (s *session) resetTxn() {
	s.from = ""
	s.rcpt = s.rcpt[:0]
	s.declaredSize = 0
	s.sizeSet = false
}

func (s *session) resetToHelloed() {
	s.resetTxn()
	if s.helo != "" {
		s.state = stateHelloed
	} else {
		s.state = stateGreeting
	}
}

func (s *session) spec() model.SMTPSpec {
	return s.srv.specNow()
}

func (s *session) reply(code int, lines ...string) error {
	_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return codec.WriteReply(s.conn, code, lines...)
}

func (s *session) setIdle(d time.Duration) error {
	spec := s.spec()
	deadline := s.started.Add(spec.Admission.SessionTimeout)
	idle := time.Now().Add(d)
	if idle.After(deadline) {
		idle = deadline
	}
	if !idle.After(time.Now()) {
		return osTimeout()
	}
	return s.conn.SetDeadline(idle)
}

func osTimeout() error {
	return timeoutErr{}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "smtp: session timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
