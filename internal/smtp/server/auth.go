package server

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"os"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtp/codec"
)

const (
	loginUserB64 = "VXNlcm5hbWU6" // base64("Username:")
	loginPassB64 = "UGFzc3dvcmQ6" // base64("Password:")
	authOKText   = "2.7.0 Authentication successful"
	authFailText = "5.7.8 Authentication failed"
)

func (s *session) cmdAuth(arg string) bool {
	spec := s.spec()
	if spec.Auth.Mode != model.SMTPAuthPlainLogin {
		_ = s.reply(502, "5.5.1 AUTH not implemented")
		return true
	}
	if s.tlsRequiredCleartext() {
		_ = s.reply(530, "5.7.0 Must issue a STARTTLS command first")
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
	if s.authed {
		_ = s.reply(503, "5.5.1 Already authenticated")
		return true
	}
	if handled, keep := s.applyErrorOverride("AUTH"); handled {
		if !keep {
			s.endResult = "behavior"
		}
		return keep
	}
	mech, init, hasInit := splitAuthArg(arg)
	switch strings.ToUpper(mech) {
	case "PLAIN":
		return s.authPLAIN(init, hasInit)
	case "LOGIN":
		return s.authLOGIN(init, hasInit)
	case "":
		_ = s.reply(501, "5.5.4 Missing authentication mechanism")
		return true
	default:
		_ = s.reply(504, "5.5.4 Unrecognized authentication type")
		return true
	}
}

func (s *session) authPLAIN(init string, hasInit bool) bool {
	payload := init
	if !hasInit {
		if err := s.reply(334, ""); err != nil {
			return false
		}
		line, ok := s.readAuthLine()
		if !ok {
			return false
		}
		if line == "*" {
			_ = s.reply(501, "5.5.4")
			return true
		}
		payload = line
	}
	raw, err := decodeAuthB64(payload)
	if err != nil {
		_ = s.reply(535, authFailText)
		return true
	}
	user, pass, ok := parsePLAIN(raw)
	if !ok || user == "" || pass == "" {
		_ = s.reply(535, authFailText)
		return true
	}
	return s.finishAuth(user, pass)
}

func (s *session) authLOGIN(init string, hasInit bool) bool {
	userB64 := init
	if !hasInit {
		if err := s.reply(334, loginUserB64); err != nil {
			return false
		}
		line, ok := s.readAuthLine()
		if !ok {
			return false
		}
		if line == "*" {
			_ = s.reply(501, "5.5.4")
			return true
		}
		userB64 = line
	}
	user, err := decodeAuthB64(userB64)
	if err != nil || user == "" {
		_ = s.reply(535, authFailText)
		return true
	}
	if err := s.reply(334, loginPassB64); err != nil {
		return false
	}
	line, ok := s.readAuthLine()
	if !ok {
		return false
	}
	if line == "*" {
		_ = s.reply(501, "5.5.4")
		return true
	}
	pass, err := decodeAuthB64(line)
	if err != nil || pass == "" {
		_ = s.reply(535, authFailText)
		return true
	}
	return s.finishAuth(user, pass)
}

func (s *session) finishAuth(user, pass string) bool {
	wantUser, wantPass, err := s.credentials()
	if err != nil {
		_ = s.reply(454, "4.7.0 Temporary authentication failure")
		return true
	}
	userOK := constEq(user, wantUser)
	passOK := constEq(pass, wantPass)
	if userOK && passOK {
		s.authed = true
		if !s.replyKeyed("AUTH", 235, authOKText) {
			s.endResult = "behavior"
			return false
		}
		return true
	}
	_ = s.reply(535, authFailText)
	return true
}

func (s *session) credentials() (string, string, error) {
	spec := s.spec()
	if spec.Auth.PasswordFile == "" {
		return spec.Auth.Username, "", errors.New("smtp: missing password file")
	}
	raw, err := os.ReadFile(spec.Auth.PasswordFile)
	if err != nil {
		return "", "", err
	}
	return spec.Auth.Username, strings.TrimRight(string(raw), "\r\n"), nil
}

func (s *session) readAuthLine() (string, bool) {
	if err := s.setIdle(s.spec().Admission.CommandIdle); err != nil {
		return "", false
	}
	line, err := s.rd.ReadCommandLine()
	if err != nil {
		if errors.Is(err, codec.ErrLineTooLong) {
			_ = s.reply(500, "5.5.2 Line too long")
		}
		return "", false
	}
	return line, true
}

func splitAuthArg(arg string) (mech, init string, hasInit bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", false
	}
	mech, rest, found := strings.Cut(arg, " ")
	if !found {
		return mech, "", false
	}
	rest = strings.TrimSpace(rest)
	if rest == "=" {
		return mech, "", true
	}
	return mech, rest, true
}

func decodeAuthB64(s string) (string, error) {
	s = strings.TrimSpace(s)
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(s)
		if err != nil {
			return "", err
		}
	}
	return string(raw), nil
}

func parsePLAIN(raw string) (user, pass string, ok bool) {
	parts := strings.SplitN(raw, "\x00", 3)
	if len(parts) != 3 {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func constEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
