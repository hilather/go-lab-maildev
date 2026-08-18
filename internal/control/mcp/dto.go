package mcp

import (
	"encoding/json"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/audit"
	"github.com/hilather/go-lab-maildev/internal/buildinfo"
	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

type emptyIn struct{}

type idIn struct {
	ID string `json:"id"`
}

type messageIDIn struct {
	ID string `json:"id"`
}

type exportIn struct {
	Format string `json:"format,omitempty"`
}

type changeIn struct {
	ExpectedRevision string            `json:"expectedRevision,omitempty"`
	IdempotencyKey   string            `json:"idempotencyKey,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	Force            bool              `json:"force,omitempty"`
	Operations       []model.Operation `json:"operations,omitempty"`
}

type validateIn struct {
	State      json.RawMessage   `json:"state,omitempty"`
	Operations []model.Operation `json:"operations,omitempty"`
}

type resetIn struct {
	Reason string `json:"reason,omitempty"`
}

type deleteIn struct {
	ID                      string  `json:"id,omitempty"`
	ExpectedStoreGeneration *uint64 `json:"expectedStoreGeneration,omitempty"`
}

type clearIn struct {
	ExpectedStoreGeneration *uint64 `json:"expectedStoreGeneration,omitempty"`
}

type listIn struct {
	To              string `json:"to,omitempty"`
	From            string `json:"from,omitempty"`
	Subject         string `json:"subject,omitempty"`
	SubjectContains string `json:"subjectContains,omitempty"`
	Unread          *bool  `json:"unread,omitempty"`
	After           string `json:"after,omitempty"`
	Before          string `json:"before,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	Limit           int    `json:"limit,omitempty"`
}

type getIn struct {
	ID       string `json:"id"`
	MarkRead bool   `json:"markRead,omitempty"`
}

type waitIn struct {
	Filter  waitFilter `json:"filter"`
	Timeout string     `json:"timeout,omitempty"`
}

type waitFilter struct {
	To              string `json:"to,omitempty"`
	From            string `json:"from,omitempty"`
	Subject         string `json:"subject,omitempty"`
	SubjectContains string `json:"subjectContains,omitempty"`
	Unread          *bool  `json:"unread,omitempty"`
	After           string `json:"after,omitempty"`
	Before          string `json:"before,omitempty"`
}

type attachmentIn struct {
	ID    string `json:"id"`
	AttID string `json:"attId"`
}

type auditQueryIn struct {
	Limit int `json:"limit,omitempty"`
}

func (in changeIn) toChange() app.ChangeIn {
	return app.ChangeIn{
		ExpectedRevision: model.Revision(in.ExpectedRevision),
		IdempotencyKey:   in.IdempotencyKey,
		Reason:           in.Reason,
		Force:            in.Force,
		Operations:       in.Operations,
	}
}

func (in validateIn) toValidate() (app.ValidateIn, error) {
	st, err := decodeCandidateState(in.State)
	if err != nil {
		return app.ValidateIn{}, err
	}
	return app.ValidateIn{State: st, Operations: in.Operations}, nil
}

func decodeCandidateState(raw json.RawMessage) (*model.State, error) {
	raw = json.RawMessage(trimSpaceJSON(raw))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return config.DecodeJSON(raw)
}

func trimSpaceJSON(raw json.RawMessage) []byte {
	i, j := 0, len(raw)
	for i < j && (raw[i] == ' ' || raw[i] == '\n' || raw[i] == '\r' || raw[i] == '\t') {
		i++
	}
	for j > i && (raw[j-1] == ' ' || raw[j-1] == '\n' || raw[j-1] == '\r' || raw[j-1] == '\t') {
		j--
	}
	return raw[i:j]
}

func parseOptionalTime(raw, field string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, raw)
	}
	if err != nil {
		return time.Time{}, domainerr.ValidationFailed("invalid timestamp",
			domainerr.FieldViolation{Path: field, Code: "invalid_value", Message: field + " must be RFC3339"})
	}
	return t.UTC(), nil
}

func (in listIn) filter() (model.MessageFilter, error) {
	f := model.MessageFilter{
		To: in.To, From: in.From, Subject: in.Subject,
		SubjectContains: in.SubjectContains, Unread: in.Unread,
	}
	after, err := parseOptionalTime(in.After, "after")
	if err != nil {
		return f, err
	}
	f.After = after
	before, err := parseOptionalTime(in.Before, "before")
	if err != nil {
		return f, err
	}
	f.Before = before
	return f, nil
}

func (in waitIn) toWait() (app.WaitIn, error) {
	f := model.MessageFilter{
		To: in.Filter.To, From: in.Filter.From, Subject: in.Filter.Subject,
		SubjectContains: in.Filter.SubjectContains, Unread: in.Filter.Unread,
	}
	after, err := parseOptionalTime(in.Filter.After, "filter.after")
	if err != nil {
		return app.WaitIn{}, err
	}
	f.After = after
	before, err := parseOptionalTime(in.Filter.Before, "filter.before")
	if err != nil {
		return app.WaitIn{}, err
	}
	f.Before = before
	var timeout time.Duration
	if in.Timeout != "" {
		d, err := time.ParseDuration(in.Timeout)
		if err != nil || d < 0 {
			return app.WaitIn{}, domainerr.ValidationFailed("invalid timeout",
				domainerr.FieldViolation{Path: "timeout", Code: "invalid_value", Message: "timeout must use Go duration syntax"})
		}
		timeout = d
	}
	return app.WaitIn{Filter: f, Timeout: timeout}, nil
}

type versionJSON struct {
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

type capabilityViewJSON struct {
	Capabilities []capabilityInfoJSON `json:"capabilities"`
}

type capabilityInfoJSON struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating"`
	Idempotent  bool   `json:"idempotent"`
}

type statusJSON struct {
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
	Format            string `json:"format"`
	Revision          string `json:"revision"`
	BootstrapRevision string `json:"bootstrapRevision"`
	Drifted           bool   `json:"drifted"`
	Body              string `json:"body"`
	HumanDiff         string `json:"humanDiff,omitempty"`
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

type rawBodyJSON struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Body        string `json:"body"`
}

type htmlBodyJSON struct {
	ID   string `json:"id"`
	HTML string `json:"html"`
}

type attachmentGetJSON struct {
	attachmentJSON
	Data string `json:"data"`
}

type countJSON struct {
	Deleted int `json:"deleted,omitempty"`
	Updated int `json:"updated,omitempty"`
}

type auditListJSON struct {
	Events []audit.Event `json:"events"`
}

func fromVersion(info buildinfo.Info) versionJSON {
	return versionJSON{
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

func fromCapabilities() capabilityViewJSON {
	src := capabilities.DiscoveryList()
	out := make([]capabilityInfoJSON, 0, len(src))
	for _, d := range src {
		out = append(out, capabilityInfoJSON{
			Name: d.Name, Version: d.Version, Description: d.Description,
			Mutating: d.Mutating, Idempotent: d.Idempotent,
		})
	}
	return capabilityViewJSON{Capabilities: out}
}

func fromStatus(st *app.Status, view *app.StateView) (statusJSON, error) {
	if st == nil {
		return statusJSON{Listeners: []listenerJSON{}, Store: storeStatsJSON{}}, nil
	}
	rev, err := marshalAPI(st.Revisions)
	if err != nil {
		return statusJSON{}, err
	}
	out := statusJSON{
		Ready:     st.Ready,
		Revisions: rev,
		Listeners: []listenerJSON{},
		Store: storeStatsJSON{
			MessageCount: st.Revisions.MessageCount,
			Bytes:        st.Revisions.StoreBytes,
			Generation:   st.Revisions.StoreGeneration,
			UnreadCount:  st.UnreadCount,
			Epoch:        st.Epoch,
		},
	}
	if view != nil && view.Canonical != nil {
		out.Listeners = []listenerJSON{
			{Name: "smtp", Address: view.Canonical.Spec.Listeners.SMTP.Address},
			{Name: "management", Address: view.Canonical.Spec.Listeners.Management.Address},
		}
	}
	return out, nil
}

func fromStateView(v *app.StateView) (stateViewJSON, error) {
	if v == nil {
		return stateViewJSON{}, nil
	}
	canon, err := marshalAPI(v.Canonical)
	if err != nil {
		return stateViewJSON{}, err
	}
	return stateViewJSON{
		BootstrapRevision: string(v.BootstrapRevision),
		RuntimeRevision:   string(v.RuntimeRevision),
		Generation:        uint64(v.Generation),
		StoreGeneration:   v.StoreGeneration,
		Drifted:           v.Drifted,
		LoadedAt:          rfc3339(v.LoadedAt),
		MessageCount:      v.MessageCount,
		StoreBytes:        v.StoreBytes,
		Canonical:         canon,
	}, nil
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

func fromExport(exp *app.Export) exportJSON {
	if exp == nil {
		return exportJSON{}
	}
	return exportJSON{
		Format:            string(exp.Format),
		Revision:          string(exp.Revision),
		BootstrapRevision: string(exp.BootstrapRevision),
		Drifted:           exp.Drifted,
		Body:              string(exp.Body),
		HumanDiff:         exp.HumanDiff,
	}
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

func fromAuditList(list *app.AuditList) auditListJSON {
	events := []audit.Event{}
	if list != nil && list.Events != nil {
		events = list.Events
	}
	return auditListJSON{Events: events}
}
