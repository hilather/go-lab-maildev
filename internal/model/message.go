package model

import "time"

// Message is one captured inbox item. STORE-001 fills parse and spill details.
type Message struct {
	ID           string
	ReceivedAt   time.Time
	Envelope     Envelope
	Headers      []Header
	Subject      string
	From         []Address
	To           []Address
	Cc           []Address
	Bcc          []Address
	ReplyTo      []Address
	MessageID    string
	InReplyTo    string
	Date         time.Time
	Text         string
	HTML         string
	Raw          []byte
	Size         int
	Read         bool
	Priority     string
	ParseWarning string
	Attachments  []Attachment
}

// Envelope is the SMTP transaction that produced a Message.
type Envelope struct {
	From       string
	To         []string
	HELO       string
	RemoteAddr string
	TLS        bool
	AuthUser   string
}

// Header is one ordered, case-preserving header field.
type Header struct {
	Name  string
	Value string
}

// Address is a parsed mailbox.
type Address struct {
	Name    string
	Address string
}

// Attachment is one MIME part offered for download.
type Attachment struct {
	ID          string
	Filename    string
	ContentType string
	ContentID   string
	Disposition string
	Size        int
	Checksum    string
}
