package model

import "time"

// Message is one captured inbox item.
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
	// Data is decoded bytes (RAM or loaded from spill). Not rewritten into Raw.
	Data []byte
}

// InsertResult is the store acknowledgement for one accepted message.
type InsertResult struct {
	ID         string
	Generation uint64
}

// MessageFilter selects inbox rows for List and Wait.
type MessageFilter struct {
	To              string
	From            string
	Subject         string
	SubjectContains string
	Unread          *bool
	After           time.Time
	Before          time.Time
}

// ListQuery is a filtered, cursor-paginated inbox read.
type ListQuery struct {
	Filter MessageFilter
	Cursor string
	Limit  int
}

// ListResult is one page of inbox messages, newest first.
type ListResult struct {
	Items      []*Message
	NextCursor string
	Generation uint64
}

// StoreStats is a point-in-time occupancy snapshot.
type StoreStats struct {
	MessageCount int
	Bytes        int64
	UnreadCount  int
	Generation   uint64
	Epoch        uint64
	Evictions    uint64
}

// ResidentBytes is raw plus decoded text/html/attachment bodies.
func (m *Message) ResidentBytes() int64 {
	if m == nil {
		return 0
	}
	n := int64(len(m.Raw))
	if n == 0 {
		n = int64(m.Size)
	}
	n += int64(len(m.Text)) + int64(len(m.HTML))
	for i := range m.Attachments {
		a := &m.Attachments[i]
		if len(a.Data) > 0 {
			n += int64(len(a.Data))
		} else {
			n += int64(a.Size)
		}
	}
	return n
}
