package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/mimeparse"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/observability"
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
	tls          bool
	authed       bool
	endResult    string
}

func (s *session) run() {
	defer s.finish()
	beh := s.spec().Behavior
	if beh.DropOnConnect {
		s.endResult = "behavior"
		return
	}
	if beh.GreetingDelay > 0 {
		time.Sleep(beh.GreetingDelay)
	}
	if handled, keep := s.applyErrorOverride("GREETING"); handled {
		if !keep {
			s.endResult = "behavior"
			return
		}
	} else if !s.replyKeyed("GREETING", 220, s.spec().Hostname+" LabMail ready") {
		s.endResult = "behavior"
		return
	}
	for {
		if err := s.setIdle(s.spec().Admission.CommandIdle); err != nil {
			s.endResult = "timeout"
			return
		}
		line, err := s.rd.ReadCommandLine()
		if err != nil {
			if errors.Is(err, codec.ErrLineTooLong) {
				_ = s.reply(500, "5.5.2 Line too long")
			} else if isTimeout(err) {
				s.endResult = "timeout"
			}
			return
		}
		s.applyCommandDelay()
		verb, arg := codec.SplitVerb(line)
		if !s.dispatch(verb, arg) {
			return
		}
	}
}

func (s *session) dispatch(verb, arg string) bool {
	switch verb {
	case "HELO":
		return s.cmdHello(arg, false)
	case "EHLO":
		return s.cmdHello(arg, true)
	case "MAIL":
		return s.cmdMail(arg)
	case "RCPT":
		return s.cmdRcpt(arg)
	case "DATA":
		return s.cmdData()
	case "RSET":
		return s.cmdRset()
	case "NOOP":
		return s.replyKeyed("NOOP", 250, "2.0.0 OK")
	case "QUIT":
		_ = s.reply(221, "2.0.0 Bye")
		return false
	case "HELP":
		_ = s.reply(214, "HELO EHLO MAIL RCPT DATA RSET NOOP QUIT HELP VRFY", "End of HELP")
		return true
	case "VRFY":
		return s.replyKeyed("VRFY", 252, "2.5.2 Cannot VRFY user")
	case "EXPN":
		_ = s.reply(502, "5.5.1 EXPN not implemented")
		return true
	case "AUTH":
		return s.cmdAuth(arg)
	case "STARTTLS":
		return s.cmdStartTLS()
	case "BDAT":
		_ = s.reply(502, "5.5.1 BDAT not implemented")
		return true
	case "ETRN", "ATRN", "TURN":
		_ = s.reply(502, "5.5.1 "+verb+" not implemented")
		return true
	case "":
		if handled, keep := s.applyErrorOverride("UNKNOWN"); handled {
			return keep
		}
		return s.replyKeyed("UNKNOWN", 500, "5.5.1 Command unrecognized")
	default:
		if handled, keep := s.applyErrorOverride("UNKNOWN"); handled {
			return keep
		}
		return s.replyKeyed("UNKNOWN", 500, "5.5.1 Command unrecognized")
	}
}

func (s *session) cmdHello(arg string, ehlo bool) bool {
	verb := "HELO"
	if ehlo {
		verb = "EHLO"
	}
	domain := strings.TrimSpace(arg)
	if domain == "" {
		_ = s.reply(501, "5.5.4 Missing domain")
		return true
	}
	if handled, keep := s.applyErrorOverride(verb); handled {
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	s.resetTxn()
	s.helo = domain
	s.state = stateHelloed
	if !ehlo {
		if !s.replyKeyed("HELO", 250, "2.0.0 "+s.spec().Hostname) {
			s.endResult = "behavior"
			return false
		}
		return true
	}
	if !s.replyKeyed("EHLO", 250, s.ehloLines()...) {
		s.endResult = "behavior"
		return false
	}
	return true
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
	if spec.TLS.Mode == model.TLSModeStartTLS && !s.tls && !hidden["STARTTLS"] {
		lines = append(lines, "STARTTLS")
	}
	if spec.Auth.Mode == model.SMTPAuthPlainLogin && !s.authed && !hidden["AUTH"] && !s.tlsRequiredCleartext() {
		lines = append(lines, "AUTH PLAIN LOGIN")
	}
	return lines
}

func (s *session) cmdMail(arg string) bool {
	spec := s.spec()
	if s.state == stateGreeting {
		_ = s.reply(503, "5.5.1 HELO/EHLO first")
		return true
	}
	if s.state != stateHelloed {
		_ = s.reply(503, "5.5.1 Nested MAIL")
		return true
	}
	if handled, keep := s.applyErrorOverride("MAIL"); handled {
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	if !s.policyOK() {
		return true
	}
	path, params, err := parsePathArg(arg, "FROM:")
	if err != nil {
		_ = s.reply(501, "5.5.4 Syntax error in parameters")
		return true
	}
	size, sizeSet, err := parseMailParams(params)
	if err != nil {
		_ = s.reply(501, "5.5.4 Syntax error in parameters")
		return true
	}
	if sizeSet && size > spec.MaxMessageBytes {
		_ = s.reply(552, "5.3.4 Message too large")
		return true
	}
	s.from = path
	s.declaredSize = size
	s.sizeSet = sizeSet
	s.rcpt = s.rcpt[:0]
	s.state = stateMail
	if !s.replyKeyed("MAIL", 250, "2.1.0 OK") {
		s.endResult = "behavior"
		return false
	}
	return true
}

func (s *session) cmdRcpt(arg string) bool {
	spec := s.spec()
	if s.state != stateMail && s.state != stateRcpt {
		_ = s.reply(503, "5.5.1 Need MAIL")
		return true
	}
	if handled, keep := s.applyErrorOverride("RCPT"); handled {
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	if !s.policyOK() {
		return true
	}
	path, _, err := parsePathArg(arg, "TO:")
	if err != nil {
		_ = s.reply(501, "5.5.4 Syntax error in parameters")
		return true
	}
	if len(s.rcpt) >= spec.MaxRecipients {
		_ = s.reply(452, "4.5.3 Too many recipients")
		return true
	}
	s.rcpt = append(s.rcpt, path)
	s.state = stateRcpt
	if !s.replyKeyed("RCPT", 250, "2.1.5 OK") {
		s.endResult = "behavior"
		return false
	}
	return true
}

func (s *session) cmdData() bool {
	if s.state != stateRcpt || len(s.rcpt) == 0 {
		_ = s.reply(503, "5.5.1 Need RCPT")
		return true
	}
	if handled, keep := s.applyErrorOverride("DATA"); handled {
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	if !s.policyOK() {
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
	if !s.replyKeyed("DATA", 354, "Start mail input; end with <CRLF>.<CRLF>") {
		s.endResult = "behavior"
		return false
	}

	raw, err := s.readData(spec.MaxMessageBytes)
	if err != nil {
		switch {
		case errors.Is(err, errMessageTooLarge):
			_ = s.reply(552, "5.3.4 Message too large")
			s.noteMessage("too_large", 552, "")
			s.resetToHelloed()
			return true
		case errors.Is(err, errDiscardBudget):
			_ = s.reply(552, "5.3.4 Message too large")
			s.noteMessage("too_large", 552, "")
			s.resetToHelloed()
			return false
		case errors.Is(err, codec.ErrLineTooLong):
			_ = s.reply(500, "5.5.2 Line too long")
			s.resetToHelloed()
			return true
		case isTimeout(err):
			_ = s.reply(451, "4.4.2 Timeout")
			s.endResult = "timeout"
			s.resetToHelloed()
			return false
		default:
			s.resetToHelloed()
			return false
		}
	}

	msg := mimeparse.Parse(raw)
	msg.ReceivedAt = time.Now().UTC()
	msg.Envelope = model.Envelope{
		From:       s.from,
		To:         append([]string(nil), s.rcpt...),
		HELO:       s.helo,
		RemoteAddr: s.conn.RemoteAddr().String(),
	}
	msg.Size = len(raw)
	if handled, keep := s.applyErrorOverride("DATA-END"); handled {
		s.resetToHelloed()
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	res, err := s.srv.store.Insert(context.Background(), epoch, msg)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrStaleEpoch):
			_ = s.reply(451, "4.3.2 Requested action aborted")
		case errors.Is(err, store.ErrFull):
			_ = s.reply(452, "4.3.1 Insufficient storage")
			s.noteMessage("store_full", 452, "")
		case errors.Is(err, store.ErrTooLarge):
			_ = s.reply(552, "5.3.4 Message too large")
			s.noteMessage("too_large", 552, "")
		default:
			_ = s.reply(451, "4.3.2 Requested action aborted")
		}
		s.resetToHelloed()
		return true
	}
	keep := s.replyKeyed("DATA-END", 250, "2.0.0 Queued as "+res.ID)
	s.noteMessage("accepted", 250, res.ID)
	if !keep {
		s.endResult = "behavior"
	}
	s.resetToHelloed()
	return keep
}

func (s *session) noteMessage(result string, code int, id string) {
	s.srv.incMessage(result)
	ev := observability.EventSMTPRejected
	if result == "accepted" {
		ev = observability.EventSMTPAccepted
	}
	s.srv.logSMTP(observability.Record{
		Event:     ev,
		Component: "smtp",
		MessageID: id,
		SMTPCode:  code,
		Result:    result,
		Remote:    s.conn.RemoteAddr().String(),
	})
}

func (s *session) finish() {
	result := s.endResult
	if result == "" {
		result = "ok"
	}
	s.srv.incSession(result)
	s.srv.observeSession(time.Since(s.started))
	s.srv.logSMTP(observability.Record{
		Event:      observability.EventSMTPSessionEnd,
		Component:  "smtp",
		Result:     result,
		DurationMS: float64(time.Since(s.started).Milliseconds()),
		Remote:     s.conn.RemoteAddr().String(),
	})
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
				return nil, errDiscardBudget
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

func (s *session) cmdRset() bool {
	if handled, keep := s.applyErrorOverride("RSET"); handled {
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	s.resetTxn()
	if s.state != stateGreeting {
		s.state = stateHelloed
	}
	if !s.replyKeyed("RSET", 250, "2.0.0 OK") {
		s.endResult = "behavior"
		return false
	}
	return true
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

func (s *session) resetAfterTLS() {
	s.resetTxn()
	s.helo = ""
	s.authed = false
	s.tls = true
	s.state = stateGreeting
}

func (s *session) policyOK() bool {
	spec := s.spec()
	if s.tlsRequiredCleartext() {
		_ = s.reply(530, "5.7.0 Must issue a STARTTLS command first")
		return false
	}
	if spec.Auth.Mode == model.SMTPAuthPlainLogin && !s.authed {
		_ = s.reply(530, "5.7.0 Authentication required")
		return false
	}
	return true
}

// tlsRequiredCleartext is true when STARTTLS is mandatory and this session
// has not completed the handshake. AUTH must not be advertised or accepted.
func (s *session) tlsRequiredCleartext() bool {
	spec := s.spec()
	return spec.TLS.Mode == model.TLSModeStartTLS && spec.TLS.Required && !s.tls
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
