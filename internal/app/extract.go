package app

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Frozen RE2 extractors from docs/06-rest-api.md. Do not invent ML.
var (
	reURLPlain = regexp.MustCompile(`(?i)\bhttps?://[^\s"'<>]+`)
	reURLAttr  = regexp.MustCompile(`(?i)(?:href|src)\s*=\s*["'](https?://[^"']+)["']`)
	reOTPNear  = regexp.MustCompile(`(?i)(?:code|otp|pin|verify|token)[^\n]{0,40}\b(\d{4,8})\b`)
	reOTPQuery = regexp.MustCompile(`(?i)(?:[?&](?:token|code)=)([A-Za-z0-9_-]{4,64})`)
)

const (
	extractURLCap   = 32
	extractTokenCap = 16
	extractCtxRunes = 120
)

const (
	tokenKindOTPDigits = "otp_digits"
	tokenKindOTPQuery  = "otp_query"
)

// Extract runs the frozen URL/OTP regexes on a stored message.
func (s *App) Extract(ctx context.Context, actor Actor, id string) (*ExtractResult, error) {
	msg, err := s.GetMessage(ctx, actor, id, false)
	if err != nil {
		return nil, err
	}
	return extractMessage(msg.Text, msg.HTML), nil
}

func extractMessage(text, html string) *ExtractResult {
	urls := make([]string, 0, 8)
	seenURL := map[string]bool{}
	addURL := func(u string) {
		if u == "" || seenURL[u] || len(urls) >= extractURLCap {
			return
		}
		seenURL[u] = true
		urls = append(urls, u)
	}
	for _, u := range reURLPlain.FindAllString(text, -1) {
		addURL(u)
	}
	for _, m := range reURLAttr.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			addURL(m[1])
		}
	}

	tokens := make([]ExtractedToken, 0, 8)
	seenTok := map[string]bool{}
	addTok := func(kind, value, body string, idx int) {
		if value == "" || len(tokens) >= extractTokenCap {
			return
		}
		key := kind + "\x00" + value
		if seenTok[key] {
			return
		}
		seenTok[key] = true
		tokens = append(tokens, ExtractedToken{
			Kind:    kind,
			Value:   value,
			Context: truncateRunes(lineAt(body, idx), extractCtxRunes),
		})
	}
	for _, body := range []string{text, html} {
		for _, m := range reOTPNear.FindAllStringSubmatchIndex(body, -1) {
			if len(m) >= 4 {
				addTok(tokenKindOTPDigits, body[m[2]:m[3]], body, m[0])
			}
		}
		for _, m := range reOTPQuery.FindAllStringSubmatchIndex(body, -1) {
			if len(m) >= 4 {
				addTok(tokenKindOTPQuery, body[m[2]:m[3]], body, m[0])
			}
		}
	}
	if urls == nil {
		urls = []string{}
	}
	if tokens == nil {
		tokens = []ExtractedToken{}
	}
	return &ExtractResult{URLs: urls, Tokens: tokens}
}

func lineAt(s string, idx int) string {
	if idx < 0 {
		idx = 0
	}
	if idx > len(s) {
		idx = len(s)
	}
	start := strings.LastIndex(s[:idx], "\n") + 1
	end := strings.Index(s[idx:], "\n")
	if end < 0 {
		end = len(s)
	} else {
		end = idx + end
	}
	return strings.TrimRight(s[start:end], "\r")
}

func truncateRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}
