package model

import "testing"

func TestParseSMTPReplyLine(t *testing.T) {
	code, text, err := ParseSMTPReplyLine("421 4.3.2 try later")
	if err != nil || code != 421 || text != "4.3.2 try later" {
		t.Fatalf("got %d %q %v", code, text, err)
	}
	if _, _, err := ParseSMTPReplyLine(""); err == nil {
		t.Fatal("empty")
	}
	if _, _, err := ParseSMTPReplyLine("not-a-code"); err == nil {
		t.Fatal("missing code")
	}
	if _, _, err := ParseSMTPReplyLine("199 too low"); err == nil {
		t.Fatal("199")
	}
	if _, _, err := ParseSMTPReplyLine("600 too high"); err == nil {
		t.Fatal("600")
	}
	if _, _, err := ParseSMTPReplyLine("421 later\r\n250-injected"); err == nil {
		t.Fatal("crlf")
	}
}

func TestKnownSMTPBehaviorVerb(t *testing.T) {
	if !KnownSMTPBehaviorVerb("ehlo") || !KnownSMTPBehaviorVerb("DATA-END") {
		t.Fatal("known verbs")
	}
	if KnownSMTPBehaviorVerb("BANANA") || KnownSMTPBehaviorVerb("") {
		t.Fatal("unknown verbs")
	}
}

func TestReplyOverride(t *testing.T) {
	b := SMTPBehaviorSpec{Replies: SMTPReplyOverrides{Mail: "421 later"}}
	if b.ReplyOverride("mail") != "421 later" {
		t.Fatal("mail override")
	}
	if b.ReplyOverride("RCPT") != "" {
		t.Fatal("empty override")
	}
}

func TestKnownOpIncludesReplaceSMTPBehavior(t *testing.T) {
	if !KnownOp(OpReplaceSMTPBehavior) {
		t.Fatal("replaceSMTPBehavior must be a known op")
	}
}
