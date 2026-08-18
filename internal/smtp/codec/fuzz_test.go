package codec

import (
	"bufio"
	"bytes"
	"testing"
)

func FuzzReadLine(f *testing.F) {
	f.Add([]byte("EHLO x\r\n"))
	f.Add([]byte("MAIL FROM:<>\r\n"))
	f.Add([]byte(".\r\n"))
	f.Add([]byte(string(make([]byte, 600)) + "\r\n"))
	f.Add([]byte("NOOP\n"))
	f.Fuzz(func(t *testing.T, in []byte) {
		r := bufio.NewReader(bytes.NewReader(in))
		_, _ = ReadLine(r, MaxCommandLine)
		_, _ = Unstuff(string(in))
		_, _ = SplitVerb(string(in))
	})
}
