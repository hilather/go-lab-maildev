package codec

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadLineCRLF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("EHLO client.example\r\nQUIT\r\n"))
	got, err := ReadLine(r, MaxCommandLine)
	if err != nil || got != "EHLO client.example" {
		t.Fatalf("got %q err=%v", got, err)
	}
	got, err = ReadLine(r, MaxCommandLine)
	if err != nil || got != "QUIT" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestReadLineBareLF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("NOOP\n"))
	got, err := ReadLine(r, MaxCommandLine)
	if err != nil || got != "NOOP" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestReadLineTooLong(t *testing.T) {
	payload := strings.Repeat("A", MaxCommandLine) + "\r\nNOOP\r\n"
	r := bufio.NewReader(strings.NewReader(payload))
	_, err := ReadLine(r, MaxCommandLine)
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("err=%v", err)
	}
	got, err := ReadLine(r, MaxCommandLine)
	if err != nil || got != "NOOP" {
		t.Fatalf("after too-long: got %q err=%v", got, err)
	}
}

func TestReadLineExactlyMax(t *testing.T) {
	// 510 octets + CRLF = 512.
	payload := strings.Repeat("B", MaxCommandLine-2) + "\r\n"
	r := bufio.NewReader(strings.NewReader(payload))
	got, err := ReadLine(r, MaxCommandLine)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxCommandLine-2 {
		t.Fatalf("len=%d", len(got))
	}
}

func TestReadLineEOFMidLine(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("HELO"))
	_, err := ReadLine(r, MaxCommandLine)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err=%v", err)
	}
}

func TestReadLineEOFEmpty(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(""))
	_, err := ReadLine(r, MaxCommandLine)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err=%v", err)
	}
}

func TestReaderCommandAndData(t *testing.T) {
	raw := "MAIL FROM:<>\r\nSubject: hi\r\n.\r\n"
	rd := NewReader(strings.NewReader(raw))
	cmd, err := rd.ReadCommandLine()
	if err != nil || cmd != "MAIL FROM:<>" {
		t.Fatalf("cmd=%q err=%v", cmd, err)
	}
	line, err := rd.ReadDataLine()
	if err != nil || line != "Subject: hi" {
		t.Fatalf("data=%q err=%v", line, err)
	}
	end, err := rd.ReadDataLine()
	if err != nil || end != "." {
		t.Fatalf("end=%q err=%v", end, err)
	}
}

func TestUnstuff(t *testing.T) {
	body, end := Unstuff(".")
	if !end || body != "" {
		t.Fatalf("end marker")
	}
	body, end = Unstuff("..dot")
	if end || body != ".dot" {
		t.Fatalf("unstuff=%q", body)
	}
	body, end = Unstuff("hello")
	if end || body != "hello" {
		t.Fatalf("plain=%q", body)
	}
}

func TestSplitVerb(t *testing.T) {
	v, a := SplitVerb("mail from:<>")
	if v != "MAIL" || a != "from:<>" {
		t.Fatalf("%q %q", v, a)
	}
	v, a = SplitVerb("QUIT")
	if v != "QUIT" || a != "" {
		t.Fatalf("%q %q", v, a)
	}
	v, a = SplitVerb("  ")
	if v != "" || a != "" {
		t.Fatalf("blank %q %q", v, a)
	}
}

func TestNewReaderReuseBufio(t *testing.T) {
	br := bufio.NewReader(bytes.NewReader([]byte("NOOP\r\n")))
	rd := NewReader(br)
	got, err := rd.ReadCommandLine()
	if err != nil || got != "NOOP" {
		t.Fatalf("%q %v", got, err)
	}
}
