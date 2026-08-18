package app

import (
	"context"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestExtractFrozenRegexes(t *testing.T) {
	svc, _ := mustBoot(t)
	raw := []byte("Subject: verify\r\nContent-Type: text/plain\r\n\r\n" +
		"Visit https://app.lab.test/verify?token=abc_123 and https://app.lab.test/verify?token=abc_123\r\n" +
		"Your code is 482193\r\n")
	res, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), &model.Message{Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Extract(context.Background(), actor(), res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.URLs) != 1 || out.URLs[0] != "https://app.lab.test/verify?token=abc_123" {
		t.Fatalf("urls=%v", out.URLs)
	}
	var digits, query bool
	for _, tok := range out.Tokens {
		switch tok.Kind {
		case "otp_digits":
			if tok.Value != "482193" {
				t.Fatalf("digits=%+v", tok)
			}
			if !strings.Contains(tok.Context, "482193") || len([]rune(tok.Context)) > 120 {
				t.Fatalf("context=%q", tok.Context)
			}
			digits = true
		case "otp_query":
			if tok.Value != "abc_123" {
				t.Fatalf("query=%+v", tok)
			}
			query = true
		default:
			t.Fatalf("unexpected kind %+v", tok)
		}
	}
	if !digits || !query {
		t.Fatalf("tokens=%+v", out.Tokens)
	}
}

func TestExtractHTMLAttrs(t *testing.T) {
	got := extractMessage("plain", `<a href="https://app.lab.test/ok">x</a><img src="https://cdn.lab.test/i.png">`)
	if len(got.URLs) != 2 || got.URLs[0] != "https://app.lab.test/ok" || got.URLs[1] != "https://cdn.lab.test/i.png" {
		t.Fatalf("urls=%v", got.URLs)
	}
}

func TestExtractCaps(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		b.WriteString("https://x.test/")
		b.WriteByte('a' + byte(i%26))
		b.WriteByte('0' + byte(i%10))
		b.WriteByte(' ')
	}
	got := extractMessage(b.String(), "")
	if len(got.URLs) != extractURLCap {
		t.Fatalf("url cap %d", len(got.URLs))
	}
}

func TestExtractMissing(t *testing.T) {
	svc, _ := mustBoot(t)
	_, err := svc.Extract(context.Background(), actor(), "01AAAAAAAAAAAAAAAAAAAAAAAA")
	requireCode(t, err, domainerr.CodeNotFound)
}
