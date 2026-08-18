package codec

import (
	"strings"
	"testing"
)

func TestFormatReplySingle(t *testing.T) {
	got := FormatReply(250, "2.0.0 OK")
	if got != "250 2.0.0 OK\r\n" {
		t.Fatalf("%q", got)
	}
}

func TestFormatReplyMulti(t *testing.T) {
	got := FormatReply(250, "labmail.lab", "SIZE 10485760", "8BITMIME")
	want := "250-labmail.lab\r\n250-SIZE 10485760\r\n250 8BITMIME\r\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWriteReply(t *testing.T) {
	var b strings.Builder
	if err := WriteReply(&b, 221, "2.0.0 Bye"); err != nil {
		t.Fatal(err)
	}
	if b.String() != "221 2.0.0 Bye\r\n" {
		t.Fatalf("%q", b.String())
	}
}

func TestWriteReplyNil(t *testing.T) {
	if err := WriteReply(nil, 250, "OK"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatReplyEmptyLines(t *testing.T) {
	got := FormatReply(500)
	if got != "500 \r\n" {
		t.Fatalf("%q", got)
	}
}
