package mimeparse

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSimpleText(t *testing.T) {
	raw := testdataMIME(t, "simple-text.eml")
	msg := Parse(raw)
	if msg.ParseWarning != "" {
		t.Fatalf("warning=%q", msg.ParseWarning)
	}
	if msg.Subject != "hello lab" {
		t.Fatalf("subject=%q", msg.Subject)
	}
	if msg.MessageID != "1412535729.simple@fbi.gov" {
		t.Fatalf("message-id=%q", msg.MessageID)
	}
	if !strings.Contains(msg.Text, "plain body line") {
		t.Fatalf("text=%q", msg.Text)
	}
	if !bytes.Equal(msg.Raw, raw) {
		t.Fatal("raw rewritten")
	}
	if len(msg.From) != 1 || msg.From[0].Address != "alice@lab.test" || msg.From[0].Name != "Alice" {
		t.Fatalf("from=%v", msg.From)
	}
	if len(msg.To) != 1 || msg.To[0].Address != "bob@lab.test" {
		t.Fatalf("to=%v", msg.To)
	}
	if len(msg.Cc) != 1 || msg.Cc[0].Address != "carol@lab.test" {
		t.Fatalf("cc=%v", msg.Cc)
	}
	if len(msg.Headers) == 0 {
		t.Fatal("headers")
	}
	if msg.Date.IsZero() {
		t.Fatal("date")
	}
	if msg.Priority != "normal" {
		t.Fatalf("priority=%q", msg.Priority)
	}
}

func TestParseMultipartAlternative(t *testing.T) {
	msg := Parse(testdataMIME(t, "multipart-alternative.eml"))
	if msg.ParseWarning != "" {
		t.Fatalf("warning=%q", msg.ParseWarning)
	}
	if !strings.Contains(msg.Text, "plain alternative") {
		t.Fatalf("text=%q", msg.Text)
	}
	if !strings.Contains(msg.HTML, "html alternative") {
		t.Fatalf("html=%q", msg.HTML)
	}
}

func TestParseAttachmentBase64(t *testing.T) {
	msg := Parse(testdataMIME(t, "attachment-base64.eml"))
	if msg.ParseWarning != "" {
		t.Fatalf("warning=%q", msg.ParseWarning)
	}
	if !strings.Contains(msg.Text, "see attached") {
		t.Fatalf("text=%q", msg.Text)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("atts=%d", len(msg.Attachments))
	}
	a := msg.Attachments[0]
	if a.Filename != "notes.txt" {
		t.Fatalf("filename=%q", a.Filename)
	}
	if !bytes.Equal(a.Data, []byte("hello att\n")) && !bytes.Equal(a.Data, []byte("hello att\r\n")) {
		t.Fatalf("decoded=%q", a.Data)
	}
	if a.Size != len(a.Data) || a.Checksum == "" {
		t.Fatalf("size=%d checksum=%q", a.Size, a.Checksum)
	}
	if a.Disposition != "attachment" {
		t.Fatalf("disp=%q", a.Disposition)
	}
}

func TestParseQuotedPrintable(t *testing.T) {
	msg := Parse(testdataMIME(t, "quoted-printable.eml"))
	if msg.ParseWarning != "" {
		t.Fatalf("warning=%q", msg.ParseWarning)
	}
	if !strings.Contains(msg.Text, "café = coffee") {
		t.Fatalf("text=%q", msg.Text)
	}
}

func TestParseNoMessageID(t *testing.T) {
	msg := Parse(testdataMIME(t, "no-message-id.eml"))
	if msg.MessageID != "" {
		t.Fatalf("message-id=%q", msg.MessageID)
	}
	if msg.Subject != "missing id" {
		t.Fatalf("subject=%q", msg.Subject)
	}
}

func TestParseHTMLInlineCID(t *testing.T) {
	msg := Parse(testdataMIME(t, "html-inline-cid.eml"))
	if !strings.Contains(msg.HTML, "cid:logo@lab") {
		t.Fatalf("html=%q", msg.HTML)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("atts=%d", len(msg.Attachments))
	}
	a := msg.Attachments[0]
	if a.ContentID != "logo@lab" {
		t.Fatalf("cid=%q", a.ContentID)
	}
	if a.Disposition != "inline" {
		t.Fatalf("disp=%q", a.Disposition)
	}
	if a.Filename != "logo.png" {
		t.Fatalf("filename=%q", a.Filename)
	}
	if a.ContentType != "image/png" {
		t.Fatalf("ctype=%q", a.ContentType)
	}
}

func TestParseMalformedStillStored(t *testing.T) {
	raw := testdataMIME(t, "malformed.eml")
	msg := Parse(raw)
	if !bytes.Equal(msg.Raw, raw) {
		t.Fatal("raw rewritten")
	}
	if msg.Size != len(raw) {
		t.Fatalf("size=%d", msg.Size)
	}
	if msg.ParseWarning == "" {
		t.Fatal("expected parseWarning for malformed MIME")
	}
}

func TestParseBrokenFromSetsWarning(t *testing.T) {
	raw := []byte("From: <not-an-address\r\nSubject: only-from\r\n\r\nbody\r\n")
	msg := Parse(raw)
	if !bytes.Equal(msg.Raw, raw) {
		t.Fatal("raw rewritten")
	}
	if msg.ParseWarning == "" {
		t.Fatal("expected parseWarning for broken From")
	}
}

func TestParsePathTraversalFilename(t *testing.T) {
	msg := Parse(testdataMIME(t, "path-traversal-name.eml"))
	if len(msg.Attachments) != 1 {
		t.Fatalf("atts=%d warn=%q", len(msg.Attachments), msg.ParseWarning)
	}
	if msg.Attachments[0].Filename != "passwd" {
		t.Fatalf("filename=%q", msg.Attachments[0].Filename)
	}
}

func TestParseEmpty(t *testing.T) {
	msg := Parse(nil)
	if msg.ParseWarning == "" {
		t.Fatal("expected warning")
	}
	if msg.Raw == nil {
		msg.Raw = []byte{}
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"":                       "attachment",
		".":                      "attachment",
		"..":                     "attachment",
		"../../etc/passwd":       "passwd",
		`C:\temp\x.txt`:          "x.txt",
		"ok name.txt":            "ok name.txt",
		"a:b*c?.txt":             "a_b_c_.txt",
		strings.Repeat("z", 250): strings.Repeat("z", 200),
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q)=%q want %q", in, got, want)
		}
	}
}

func testdataMIME(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(mimeDir(t), name))
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
}

func mimeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, "testdata", "mime")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("testdata/mime not found")
		}
		dir = parent
	}
}
