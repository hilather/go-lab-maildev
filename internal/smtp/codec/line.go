package codec

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

const (
	// MaxCommandLine is RFC 5321 §4.5.3.1.4 including CRLF.
	MaxCommandLine = 512
	// MaxDataLine is the liberal DATA-line cap (HTML-friendly).
	MaxDataLine = 8192
)

// ErrLineTooLong is returned when a line exceeds the configured cap
// before CRLF. The server replies 500 and closes or aborts DATA.
var ErrLineTooLong = errors.New("smtp: line too long")

// Reader reads SMTP command and DATA lines.
type Reader struct {
	br *bufio.Reader
}

// NewReader wraps r. r may already be a *bufio.Reader.
func NewReader(r io.Reader) *Reader {
	if br, ok := r.(*bufio.Reader); ok {
		return &Reader{br: br}
	}
	return &Reader{br: bufio.NewReaderSize(r, MaxDataLine)}
}

// ReadCommandLine reads one command line without the terminator.
func (r *Reader) ReadCommandLine() (string, error) {
	return ReadLine(r.br, MaxCommandLine)
}

// ReadDataLine reads one DATA line without the terminator.
func (r *Reader) ReadDataLine() (string, error) {
	return ReadLine(r.br, MaxDataLine)
}

// ReadLine reads until LF, strips a preceding CR, and enforces maxInclCRLF.
func ReadLine(r *bufio.Reader, maxInclCRLF int) (string, error) {
	if r == nil {
		return "", errors.New("smtp: nil reader")
	}
	if maxInclCRLF < 2 {
		return "", ErrLineTooLong
	}
	var buf []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && len(buf) > 0 {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}
		if len(buf)+1 > maxInclCRLF {
			return "", ErrLineTooLong
		}
		if b == '\n' {
			if n := len(buf); n > 0 && buf[n-1] == '\r' {
				buf = buf[:n-1]
			}
			return string(buf), nil
		}
		buf = append(buf, b)
	}
}

// Unstuff applies RFC 5321 dot-unstuffing. isEnd is the terminating "." line.
func Unstuff(line string) (body string, isEnd bool) {
	if line == "." {
		return "", true
	}
	if strings.HasPrefix(line, ".") {
		return line[1:], false
	}
	return line, false
}
