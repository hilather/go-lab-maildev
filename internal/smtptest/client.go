package smtptest

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Client is a line-oriented SMTP test client.
type Client struct {
	conn net.Conn
	br   *bufio.Reader
}

// Connect dials addr and does not consume the greeting.
func Connect(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	return &Client{conn: conn, br: bufio.NewReader(conn)}, nil
}

// Dial connects and reads the 220 greeting.
func Dial(addr string) (*Client, error) {
	c, err := Connect(addr)
	if err != nil {
		return nil, err
	}
	code, _, err := c.ReadReply()
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	if code != 220 {
		_ = c.Close()
		return nil, fmt.Errorf("smtptest: greeting %d", code)
	}
	return c, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// WriteLine writes s plus CRLF.
func (c *Client) WriteLine(s string) error {
	_ = c.conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, err := fmt.Fprintf(c.conn, "%s\r\n", s)
	return err
}

// ReadReply reads a single- or multi-line SMTP reply.
func (c *Client) ReadReply() (code int, lines []string, err error) {
	return c.ReadReplyTimeout(10 * time.Second)
}

// ReadReplyTimeout is ReadReply with a caller-chosen deadline.
func (c *Client) ReadReplyTimeout(d time.Duration) (code int, lines []string, err error) {
	_ = c.conn.SetDeadline(time.Now().Add(d))
	for {
		raw, e := c.br.ReadString('\n')
		if e != nil {
			return 0, lines, e
		}
		raw = strings.TrimRight(raw, "\r\n")
		if len(raw) < 3 {
			return 0, lines, fmt.Errorf("smtptest: short reply %q", raw)
		}
		n, e := strconv.Atoi(raw[:3])
		if e != nil {
			return 0, lines, fmt.Errorf("smtptest: reply %q", raw)
		}
		lines = append(lines, raw)
		if len(raw) == 3 || raw[3] == ' ' {
			return n, lines, nil
		}
		if raw[3] != '-' {
			return 0, lines, fmt.Errorf("smtptest: reply %q", raw)
		}
	}
}

// Cmd writes a command line and reads one reply.
func (c *Client) Cmd(line string) (code int, lines []string, err error) {
	if err := c.WriteLine(line); err != nil {
		return 0, nil, err
	}
	return c.ReadReply()
}

// StartTLS upgrades the connection after a 220 STARTTLS reply.
func (c *Client) StartTLS(cfg *tls.Config) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("smtptest: nil conn")
	}
	if cfg == nil {
		cfg = &tls.Config{InsecureSkipVerify: true} // test helper; production smtp never dials
	}
	tc := tls.Client(c.conn, cfg)
	if err := tc.Handshake(); err != nil {
		return err
	}
	c.conn = tc
	c.br = bufio.NewReader(tc)
	return nil
}

// ReplyText joins reply lines without the status code prefix.
func ReplyText(lines []string) string {
	var parts []string
	for _, ln := range lines {
		if len(ln) >= 4 {
			parts = append(parts, ln[4:])
		} else {
			parts = append(parts, ln)
		}
	}
	return strings.Join(parts, "\n")
}
