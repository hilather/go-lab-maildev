package server

import (
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func (s *session) applyCommandDelay() {
	d := s.spec().Behavior.CommandDelay
	if d > 0 {
		time.Sleep(d)
	}
}

func (s *session) closeAfter(verb string) bool {
	want := strings.ToUpper(strings.TrimSpace(s.spec().Behavior.CloseAfterVerb))
	return want != "" && want == strings.ToUpper(strings.TrimSpace(verb))
}

func (s *session) replyKeyed(verb string, code int, lines ...string) bool {
	ov := s.spec().Behavior.ReplyOverride(verb)
	if ov != "" {
		c, text, err := model.ParseSMTPReplyLine(ov)
		if err == nil {
			code = c
			if len(lines) == 0 {
				lines = []string{text}
			} else {
				lines[0] = text
			}
		}
	}
	if err := s.reply(code, lines...); err != nil {
		return false
	}
	return !s.closeAfter(verb)
}

// applyErrorOverride sends a configured 4xx/5xx reply. handled means the
// caller must skip the default success path.
func (s *session) applyErrorOverride(verb string) (handled, keepOpen bool) {
	ov := s.spec().Behavior.ReplyOverride(verb)
	if ov == "" {
		return false, true
	}
	code, text, err := model.ParseSMTPReplyLine(ov)
	if err != nil || code < 400 {
		return false, true
	}
	if err := s.reply(code, text); err != nil {
		return true, false
	}
	return true, !s.closeAfter(verb)
}
