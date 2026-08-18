package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaxSMTPBehaviorDelay is the fail-closed cap on greeting/command delays.
const MaxSMTPBehaviorDelay = 30 * time.Second

// SMTPBehaviorVerbs are legal closeAfterVerb values.
var SMTPBehaviorVerbs = []string{
	"GREETING", "HELO", "EHLO", "MAIL", "RCPT", "DATA", "DATA-END",
	"RSET", "NOOP", "VRFY", "AUTH", "STARTTLS", "UNKNOWN",
}

// ParseSMTPReplyLine parses "CODE text" used by smtp.behavior.replies.*.
func ParseSMTPReplyLine(raw string) (code int, text string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "", fmt.Errorf("empty reply")
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return 0, "", fmt.Errorf("reply must be a single line without control characters")
		}
	}
	codePart, rest, _ := strings.Cut(raw, " ")
	if len(codePart) != 3 {
		return 0, "", fmt.Errorf("reply must start with a 3-digit SMTP code")
	}
	n, err := strconv.Atoi(codePart)
	if err != nil || n < 200 || n > 599 {
		return 0, "", fmt.Errorf("reply code must be 200-599")
	}
	return n, strings.TrimSpace(rest), nil
}

// KnownSMTPBehaviorVerb reports whether v is a closeAfterVerb token.
func KnownSMTPBehaviorVerb(v string) bool {
	v = strings.ToUpper(strings.TrimSpace(v))
	for _, k := range SMTPBehaviorVerbs {
		if v == k {
			return true
		}
	}
	return false
}

// ReplyOverride returns the configured line for verb, or "".
func (b SMTPBehaviorSpec) ReplyOverride(verb string) string {
	switch strings.ToUpper(strings.TrimSpace(verb)) {
	case "GREETING":
		return b.Replies.Greeting
	case "HELO":
		return b.Replies.Helo
	case "EHLO":
		return b.Replies.Ehlo
	case "MAIL":
		return b.Replies.Mail
	case "RCPT":
		return b.Replies.Rcpt
	case "DATA":
		return b.Replies.Data
	case "DATA-END":
		return b.Replies.DataEnd
	case "RSET":
		return b.Replies.Rset
	case "NOOP":
		return b.Replies.Noop
	case "VRFY":
		return b.Replies.Vrfy
	case "AUTH":
		return b.Replies.Auth
	case "STARTTLS":
		return b.Replies.StartTLS
	case "UNKNOWN":
		return b.Replies.Unknown
	default:
		return ""
	}
}
