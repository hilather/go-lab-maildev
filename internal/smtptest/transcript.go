package smtptest

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

type scriptLine struct {
	kind byte // 'C' or 'S'
	text string
}

// PlayTranscript runs a testdata/smtp script against addr.
func PlayTranscript(t *testing.T, addr, path string) {
	t.Helper()
	script, err := loadTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	i := 0
	for i < len(script) {
		switch script[i].kind {
		case 'S':
			var want []string
			for i < len(script) && script[i].kind == 'S' {
				want = append(want, script[i].text)
				i++
			}
			_, got, err := c.ReadReply()
			if err != nil {
				t.Fatalf("%s: read reply: %v (want %q)", path, err, want)
			}
			if err := matchReply(got, want); err != nil {
				t.Fatalf("%s: %v\ngot:  %#v\nwant: %#v", path, err, got, want)
			}
		case 'C':
			for i < len(script) && script[i].kind == 'C' {
				if err := c.WriteLine(script[i].text); err != nil {
					t.Fatalf("%s: write %q: %v", path, script[i].text, err)
				}
				i++
			}
		default:
			t.Fatalf("%s: bad script kind", path)
		}
	}
}

func loadTranscript(path string) ([]scriptLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []scriptLine
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if len(trim) < 2 || (trim[0] != 'C' && trim[0] != 'S') || trim[1] != ':' {
			return nil, fmt.Errorf("%s:%d: expected C: or S prefix", path, lineNo)
		}
		text := ""
		if len(trim) > 2 {
			text = strings.TrimSpace(trim[2:])
		}
		out = append(out, scriptLine{kind: trim[0], text: text})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: empty transcript", path)
	}
	return out, nil
}

func matchReply(got, want []string) error {
	if len(got) != len(want) {
		return fmt.Errorf("reply lines %d != %d", len(got), len(want))
	}
	for i := range want {
		if !matchLine(got[i], want[i]) {
			return fmt.Errorf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
	return nil
}

func matchLine(got, want string) bool {
	if strings.HasSuffix(want, "*") {
		return strings.HasPrefix(got, strings.TrimSuffix(want, "*"))
	}
	return got == want
}
