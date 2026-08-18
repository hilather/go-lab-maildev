package rest

import (
	"encoding/json"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/audit"
	"github.com/hilather/go-lab-maildev/internal/buildinfo"
	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/model"
)

type healthResponse struct {
	Status string `json:"status"`
}

type versionResponse struct {
	Version   string           `json:"version"`
	Commit    string           `json:"commit"`
	BuildTime string           `json:"buildTime"`
	Protocols versionProtocols `json:"protocols"`
}

type versionProtocols struct {
	ConfigAPI string `json:"configAPI"`
	REST      string `json:"rest"`
	MCP       string `json:"mcp"`
	Compat    string `json:"compat"`
}

type capabilityViewResponse struct {
	Capabilities []capabilityInfo `json:"capabilities"`
}

type capabilityInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating"`
	Idempotent  bool   `json:"idempotent"`
}

type statusResponse struct {
	Ready     bool            `json:"ready"`
	Revisions json.RawMessage `json:"revisions"`
	Listeners []listenerJSON  `json:"listeners"`
	Store     storeStatsJSON  `json:"store"`
}

type listenerJSON struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type storeStatsJSON struct {
	MessageCount int    `json:"messageCount"`
	Bytes        int64  `json:"storeBytes"`
	UnreadCount  int    `json:"unreadCount"`
	Generation   uint64 `json:"storeGeneration"`
	Epoch        uint64 `json:"epoch"`
}

type stateViewJSON struct {
	BootstrapRevision string          `json:"bootstrapRevision"`
	RuntimeRevision   string          `json:"runtimeRevision"`
	Generation        uint64          `json:"generation"`
	StoreGeneration   uint64          `json:"storeGeneration"`
	Drifted           bool            `json:"drifted"`
	LoadedAt          string          `json:"loadedAt,omitempty"`
	MessageCount      int             `json:"messageCount"`
	StoreBytes        int64           `json:"storeBytes"`
	Canonical         json.RawMessage `json:"canonical"`
}

type changeRequest struct {
	ExpectedRevision string            `json:"expectedRevision"`
	IdempotencyKey   string            `json:"idempotencyKey"`
	Reason           string            `json:"reason"`
	Force            bool              `json:"force"`
	Operations       []model.Operation `json:"operations"`
	State            json.RawMessage   `json:"state"`
}

type resetRequest struct {
	Reason string `json:"reason"`
}

type sessionCreateJSON struct {
	CSRF      string `json:"csrf"`
	ExpiresAt string `json:"expiresAt"`
}

type sessionViewJSON struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expiresAt,omitempty"`
}

type deleteRequest struct {
	ExpectedStoreGeneration *uint64 `json:"expectedStoreGeneration"`
}

type waitRequest struct {
	Filter  waitFilter `json:"filter"`
	Timeout string     `json:"timeout"`
}

type waitFilter struct {
	To              string `json:"to"`
	From            string `json:"from"`
	Subject         string `json:"subject"`
	SubjectContains string `json:"subjectContains"`
	Unread          *bool  `json:"unread"`
	After           string `json:"after"`
	Before          string `json:"before"`
}

type messageListJSON struct {
	Revision        string        `json:"revision"`
	StoreGeneration uint64        `json:"storeGeneration"`
	Items           []messageJSON `json:"items"`
	NextCursor      *string       `json:"nextCursor"`
}

type messageJSON struct {
	ID           string           `json:"id"`
	ReceivedAt   string           `json:"receivedAt"`
	Subject      string           `json:"subject"`
	From         []addressJSON    `json:"from"`
	To           []addressJSON    `json:"to"`
	Cc           []addressJSON    `json:"cc"`
	Bcc          []addressJSON    `json:"bcc"`
	ReplyTo      []addressJSON    `json:"replyTo,omitempty"`
	MessageID    string           `json:"messageId"`
	InReplyTo    string           `json:"inReplyTo,omitempty"`
	Date         string           `json:"date,omitempty"`
	Read         bool             `json:"read"`
	Size         int              `json:"size"`
	Priority     string           `json:"priority,omitempty"`
	ParseWarning string           `json:"parseWarning,omitempty"`
	HasHTML      bool             `json:"hasHTML"`
	Envelope     envelopeJSON     `json:"envelope"`
	Headers      []headerJSON     `json:"headers,omitempty"`
	Attachments  []attachmentJSON `json:"attachments"`
	Text         string           `json:"text,omitempty"`
	HTML         string           `json:"html,omitempty"`
}

type addressJSON struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type envelopeJSON struct {
	From       string   `json:"from"`
	To         []string `json:"to"`
	HELO       string   `json:"helo"`
	RemoteAddr string   `json:"remoteAddress"`
	TLS        bool     `json:"tls"`
	AuthUser   string   `json:"authUser,omitempty"`
}

type headerJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type attachmentJSON struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	ContentID   string `json:"contentId,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	Size        int    `json:"size"`
	Checksum    string `json:"checksum"`
}

type planJSON struct {
	PreviousRevision  string            `json:"previousRevision"`
	CandidateRevision string            `json:"candidateRevision"`
	RuntimeRevision   string            `json:"runtimeRevision,omitempty"`
	Generation        uint64            `json:"generation,omitempty"`
	Applied           bool              `json:"applied,omitempty"`
	Drifted           bool              `json:"drifted"`
	Diff              []app.DiffEntry   `json:"diff"`
	Warnings          []app.Warning     `json:"warnings,omitempty"`
	Operations        []model.Operation `json:"operations,omitempty"`
}

type exportJSON struct {
	Format            string          `json:"format"`
	Revision          string          `json:"revision"`
	BootstrapRevision string          `json:"bootstrapRevision"`
	Drifted           bool            `json:"drifted"`
	Body              json.RawMessage `json:"body"`
	HumanDiff         string          `json:"humanDiff,omitempty"`
}

type sseEventJSON struct {
	ID              string `json:"id,omitempty"`
	Subject         string `json:"subject,omitempty"`
	StoreGeneration uint64 `json:"storeGeneration"`
}

func fromVersion(info buildinfo.Info) versionResponse {
	return versionResponse{
		Version:   info.Version,
		Commit:    info.Commit,
		BuildTime: info.BuildTime,
		Protocols: versionProtocols{
			ConfigAPI: info.Protocols.ConfigAPI,
			REST:      info.Protocols.REST,
			MCP:       info.Protocols.MCP,
			Compat:    info.Protocols.Compat,
		},
	}
}

func fromCapabilities() capabilityViewResponse {
	src := capabilities.DiscoveryList()
	out := make([]capabilityInfo, 0, len(src))
	for _, d := range src {
		out = append(out, capabilityInfo{
			Name: d.Name, Version: d.Version, Description: d.Description,
			Mutating: d.Mutating, Idempotent: d.Idempotent,
		})
	}
	return capabilityViewResponse{Capabilities: out}
}

func fromPlan(p *app.Plan) planJSON {
	if p == nil {
		return planJSON{Diff: []app.DiffEntry{}}
	}
	return planJSON{
		PreviousRevision:  string(p.PreviousRevision),
		CandidateRevision: string(p.CandidateRevision),
		Drifted:           p.Drifted,
		Diff:              p.Diff,
		Warnings:          p.Warnings,
		Operations:        p.Operations,
	}
}

func fromApply(r *app.ApplyResult) planJSON {
	if r == nil {
		return planJSON{Diff: []app.DiffEntry{}}
	}
	out := fromPlan(&r.Plan)
	out.Applied = r.Applied
	out.Generation = uint64(r.Generation)
	out.RuntimeRevision = string(r.RuntimeRevision)
	return out
}

func fromMessage(m *model.Message, listItem bool) messageJSON {
	if m == nil {
		return messageJSON{}
	}
	out := messageJSON{
		ID:           m.ID,
		ReceivedAt:   rfc3339(m.ReceivedAt),
		Subject:      m.Subject,
		From:         fromAddrs(m.From),
		To:           fromAddrs(m.To),
		Cc:           fromAddrs(m.Cc),
		Bcc:          fromAddrs(m.Bcc),
		ReplyTo:      fromAddrs(m.ReplyTo),
		MessageID:    m.MessageID,
		InReplyTo:    m.InReplyTo,
		Date:         rfc3339(m.Date),
		Read:         m.Read,
		Size:         m.Size,
		Priority:     m.Priority,
		ParseWarning: m.ParseWarning,
		HasHTML:      m.HTML != "",
		Envelope: envelopeJSON{
			From: m.Envelope.From, To: m.Envelope.To, HELO: m.Envelope.HELO,
			RemoteAddr: m.Envelope.RemoteAddr, TLS: m.Envelope.TLS, AuthUser: m.Envelope.AuthUser,
		},
		Attachments: fromAttachments(m.Attachments),
	}
	if !listItem {
		out.Text = m.Text
		out.HTML = m.HTML
		out.Headers = fromHeaders(m.Headers)
	}
	return out
}

func fromAddrs(in []model.Address) []addressJSON {
	out := make([]addressJSON, 0, len(in))
	for _, a := range in {
		out = append(out, addressJSON{Name: a.Name, Address: a.Address})
	}
	return out
}

func fromHeaders(in []model.Header) []headerJSON {
	out := make([]headerJSON, 0, len(in))
	for _, h := range in {
		out = append(out, headerJSON{Name: h.Name, Value: h.Value})
	}
	return out
}

func fromAttachments(in []model.Attachment) []attachmentJSON {
	out := make([]attachmentJSON, 0, len(in))
	for _, a := range in {
		out = append(out, attachmentJSON{
			ID: a.ID, Filename: a.Filename, ContentType: a.ContentType,
			ContentID: a.ContentID, Disposition: a.Disposition, Size: a.Size, Checksum: a.Checksum,
		})
	}
	return out
}

func fromAudit(ev audit.Event) audit.Event {
	return ev
}
