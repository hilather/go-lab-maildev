package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/hilather/go-lab-maildev/internal/mimeparse"
	"github.com/hilather/go-lab-maildev/internal/model"
)

var spillName = regexp.MustCompile(`(?i)^[0-7][0-9A-HJKMNP-TV-Z]{25}(-[0-9]+)?\.(raw|att)$`)

// Options construct a Memory inbox.
type Options struct {
	MaxMessages    int
	MaxBytes       int64
	FullPolicy     string
	MaxWait        time.Duration
	SpillDirectory string
	SpillThreshold int64
}

// OptionsFromSpec copies compiled store caps.
func OptionsFromSpec(spec model.StoreSpec) Options {
	return Options{
		MaxMessages:    spec.MaxMessages,
		MaxBytes:       spec.MaxBytes,
		FullPolicy:     spec.FullPolicy,
		MaxWait:        spec.MaxWait,
		SpillDirectory: spec.SpillDirectory,
		SpillThreshold: spec.SpillThreshold,
	}
}

// Memory is a process-local, mutex-protected inbox.
type Memory struct {
	maxMessages    int
	maxBytes       int64
	fullPolicy     string
	maxWait        time.Duration
	spillDir       string
	spillThreshold int64

	mu         sync.Mutex
	cond       *sync.Cond
	epoch      uint64
	generation uint64
	evictions  uint64
	bytes      int64
	byID       map[string]*record
	order      []string
}

type record struct {
	msg      *model.Message
	resident int64
	rawSpill string
	attSpill []string
}

// New builds an empty inbox and wipes any leftover spill files.
func New(opts Options) (*Memory, error) {
	if opts.MaxMessages <= 0 {
		return nil, errors.New("store: maxMessages must be > 0")
	}
	if opts.MaxBytes <= 0 {
		return nil, errors.New("store: maxBytes must be > 0")
	}
	switch opts.FullPolicy {
	case "", model.FullPolicyReject:
		opts.FullPolicy = model.FullPolicyReject
	case model.FullPolicyEvictOldest:
	default:
		return nil, fmt.Errorf("store: unknown fullPolicy %q", opts.FullPolicy)
	}
	if opts.SpillDirectory != "" && opts.SpillThreshold <= 0 {
		opts.SpillThreshold = 256 << 10
	}
	m := &Memory{
		maxMessages:    opts.MaxMessages,
		maxBytes:       opts.MaxBytes,
		fullPolicy:     opts.FullPolicy,
		maxWait:        opts.MaxWait,
		spillDir:       opts.SpillDirectory,
		spillThreshold: opts.SpillThreshold,
		epoch:          1,
		byID:           make(map[string]*record),
	}
	m.cond = sync.NewCond(&m.mu)
	if err := m.prepareSpill(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Memory) prepareSpill() error {
	if m.spillDir == "" {
		return nil
	}
	if err := os.MkdirAll(m.spillDir, 0o700); err != nil {
		return fmt.Errorf("store: spill directory: %w", err)
	}
	m.unlinkAllSpill()
	return nil
}

// Insert parses MIME if needed, assigns a ULID, and charges resident bytes.
func (m *Memory) Insert(ctx context.Context, epoch uint64, msg *model.Message) (model.InsertResult, error) {
	if m == nil {
		return model.InsertResult{}, errors.New("store: nil Memory")
	}
	if err := ctx.Err(); err != nil {
		return model.InsertResult{}, err
	}
	if msg == nil {
		return model.InsertResult{}, errors.New("store: nil message")
	}

	prepared := cloneMessage(msg)
	applyParse(prepared)
	if prepared.ReceivedAt.IsZero() {
		prepared.ReceivedAt = time.Now().UTC()
	}
	id := ulid.Make().String()
	prepared.ID = id
	if prepared.MessageID == "" {
		prepared.MessageID = id + "@labmail.lab"
	}
	for i := range prepared.Attachments {
		prepared.Attachments[i].ID = fmt.Sprintf("%s:%d", id, i)
		if prepared.Attachments[i].Size == 0 {
			prepared.Attachments[i].Size = len(prepared.Attachments[i].Data)
		}
	}
	prepared.Size = len(prepared.Raw)
	candidate := prepared.ResidentBytes()

	m.mu.Lock()
	defer m.mu.Unlock()
	if epoch != m.epoch {
		return model.InsertResult{}, ErrStaleEpoch
	}
	if candidate > m.maxBytes {
		return model.InsertResult{}, ErrTooLarge
	}
	if !m.fitsLocked(candidate) {
		if m.fullPolicy != model.FullPolicyEvictOldest {
			return model.InsertResult{}, ErrFull
		}
		if err := m.evictUntilFitsLocked(candidate); err != nil {
			return model.InsertResult{}, err
		}
	}

	rec := &record{msg: prepared, resident: candidate}
	if err := m.spillLocked(rec); err != nil {
		return model.InsertResult{}, err
	}
	m.byID[id] = rec
	m.insertOrderLocked(id, prepared.ReceivedAt)
	m.bytes += candidate
	m.generation++
	m.cond.Broadcast()
	return model.InsertResult{ID: id, Generation: m.generation}, nil
}

func (m *Memory) fitsLocked(candidate int64) bool {
	return len(m.byID) < m.maxMessages && m.bytes+candidate <= m.maxBytes
}

func (m *Memory) evictUntilFitsLocked(candidate int64) error {
	for len(m.order) > 0 && !m.fitsLocked(candidate) {
		m.removeLocked(m.order[0], true)
	}
	if !m.fitsLocked(candidate) {
		return ErrFull
	}
	return nil
}

func (m *Memory) insertOrderLocked(id string, at time.Time) {
	pos := len(m.order)
	for i, oid := range m.order {
		other := m.byID[oid]
		if other == nil {
			continue
		}
		ot := other.msg.ReceivedAt
		if at.Before(ot) || (at.Equal(ot) && id < oid) {
			pos = i
			break
		}
	}
	m.order = append(m.order, "")
	copy(m.order[pos+1:], m.order[pos:])
	m.order[pos] = id
}

// Get returns a clone. markRead flips the stored bit without bumping generation.
func (m *Memory) Get(id string, markRead bool) (*model.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	if markRead {
		rec.msg.Read = true
	}
	return m.materializeLocked(rec), nil
}

// List returns newest-first pages. Cursor is the last id from the previous page.
func (m *Memory) List(q model.ListQuery) (model.ListResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	var items []*model.Message
	passed := q.Cursor == ""
	var lastID string
	for i := len(m.order) - 1; i >= 0; i-- {
		id := m.order[i]
		if !passed {
			if id == q.Cursor {
				passed = true
				continue
			}
			// Cursor id was deleted: ULIDs sort by time, newest first.
			if id < q.Cursor {
				passed = true
			} else {
				continue
			}
		}
		rec := m.byID[id]
		if rec == nil || !matchFilter(rec.msg, q.Filter) {
			continue
		}
		if len(items) == limit {
			return model.ListResult{Items: items, NextCursor: lastID, Generation: m.generation}, nil
		}
		items = append(items, m.materializeLocked(rec))
		lastID = id
	}
	return model.ListResult{Items: items, Generation: m.generation}, nil
}

// Delete removes one message.
func (m *Memory) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; !ok {
		return ErrNotFound
	}
	m.removeLocked(id, false)
	return nil
}

// DeleteAll empties the inbox without bumping epoch.
func (m *Memory) DeleteAll() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.byID)
	if n == 0 {
		return 0, nil
	}
	ids := append([]string(nil), m.order...)
	for _, id := range ids {
		m.removeLocked(id, false)
	}
	return n, nil
}

// MarkRead sets the read bit. Generation is unchanged.
func (m *Memory) MarkRead(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	rec.msg.Read = true
	return nil
}

// MarkAllRead sets every read bit. Generation is unchanged.
func (m *Memory) MarkAllRead() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, rec := range m.byID {
		if !rec.msg.Read {
			rec.msg.Read = true
			n++
		}
	}
	return n, nil
}

// Wait returns the newest matching message or ctx/maxWait timeout.
func (m *Memory) Wait(ctx context.Context, filter model.MessageFilter) (*model.Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.maxWait > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.maxWait)
		defer cancel()
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.cond.Broadcast()
			m.mu.Unlock()
		case <-stop:
		}
	}()

	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		if rec := m.newestMatchLocked(filter); rec != nil {
			return m.materializeLocked(rec), nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m.cond.Wait()
	}
}

func (m *Memory) newestMatchLocked(filter model.MessageFilter) *record {
	for i := len(m.order) - 1; i >= 0; i-- {
		rec := m.byID[m.order[i]]
		if rec != nil && matchFilter(rec.msg, filter) {
			return rec
		}
	}
	return nil
}

// Generation increments on insert, delete, wipe, and evict.
func (m *Memory) Generation() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation
}

// Epoch is captured at DATA start; Wipe is the only bump.
func (m *Memory) Epoch() uint64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.epoch
}

// Stats is occupancy plus counters.
func (m *Memory) Stats() model.StoreStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	unread := 0
	for _, rec := range m.byID {
		if !rec.msg.Read {
			unread++
		}
	}
	return model.StoreStats{
		MessageCount: len(m.byID),
		Bytes:        m.bytes,
		UnreadCount:  unread,
		Generation:   m.generation,
		Epoch:        m.epoch,
		Evictions:    m.evictions,
	}
}

// Wipe increments epoch, empties the index, and unlinks spill.
func (m *Memory) Wipe() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.epoch++
	m.generation++
	m.byID = make(map[string]*record)
	m.order = nil
	m.bytes = 0
	m.unlinkAllSpill()
	m.cond.Broadcast()
}

func (m *Memory) removeLocked(id string, eviction bool) {
	rec, ok := m.byID[id]
	if !ok {
		return
	}
	delete(m.byID, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.bytes -= rec.resident
	if m.bytes < 0 {
		m.bytes = 0
	}
	unlinkRecord(rec)
	m.generation++
	if eviction {
		m.evictions++
	}
	m.cond.Broadcast()
}

func (m *Memory) materializeLocked(rec *record) *model.Message {
	out := cloneMessage(rec.msg)
	if rec.rawSpill != "" && len(out.Raw) == 0 {
		if b, err := os.ReadFile(rec.rawSpill); err == nil {
			out.Raw = b
		}
	}
	for i := range out.Attachments {
		if len(out.Attachments[i].Data) > 0 {
			continue
		}
		if i < len(rec.attSpill) && rec.attSpill[i] != "" {
			if b, err := os.ReadFile(rec.attSpill[i]); err == nil {
				out.Attachments[i].Data = b
			}
		}
	}
	return out
}

func (m *Memory) spillLocked(rec *record) error {
	if m.spillDir == "" || m.spillThreshold <= 0 {
		return nil
	}
	msg := rec.msg
	if int64(len(msg.Raw)) >= m.spillThreshold {
		path := filepath.Join(m.spillDir, msg.ID+".raw")
		if err := os.WriteFile(path, msg.Raw, 0o600); err != nil {
			return fmt.Errorf("store: spill raw: %w", err)
		}
		rec.rawSpill = path
		msg.Raw = nil
	}
	if len(msg.Attachments) == 0 {
		return nil
	}
	rec.attSpill = make([]string, len(msg.Attachments))
	for i := range msg.Attachments {
		a := &msg.Attachments[i]
		if int64(len(a.Data)) < m.spillThreshold {
			continue
		}
		path := filepath.Join(m.spillDir, fmt.Sprintf("%s-%d.att", msg.ID, i))
		if err := os.WriteFile(path, a.Data, 0o600); err != nil {
			unlinkRecord(rec)
			return fmt.Errorf("store: spill attachment: %w", err)
		}
		rec.attSpill[i] = path
		a.Data = nil
	}
	return nil
}

func (m *Memory) unlinkAllSpill() {
	if m.spillDir == "" {
		return
	}
	ents, err := os.ReadDir(m.spillDir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if e.IsDir() || !spillName.MatchString(e.Name()) {
			continue
		}
		_ = os.Remove(filepath.Join(m.spillDir, e.Name()))
	}
}

func unlinkRecord(rec *record) {
	if rec == nil {
		return
	}
	if rec.rawSpill != "" {
		_ = os.Remove(rec.rawSpill)
		rec.rawSpill = ""
	}
	for i, p := range rec.attSpill {
		if p != "" {
			_ = os.Remove(p)
			rec.attSpill[i] = ""
		}
	}
}

func applyParse(msg *model.Message) {
	if msg == nil || !needsParse(msg) {
		return
	}
	parsed := mimeparse.Parse(msg.Raw)
	msg.Headers = parsed.Headers
	msg.Subject = parsed.Subject
	msg.From = parsed.From
	msg.To = parsed.To
	msg.Cc = parsed.Cc
	msg.Bcc = parsed.Bcc
	msg.ReplyTo = parsed.ReplyTo
	msg.MessageID = parsed.MessageID
	msg.InReplyTo = parsed.InReplyTo
	msg.Date = parsed.Date
	msg.Text = parsed.Text
	msg.HTML = parsed.HTML
	msg.ParseWarning = parsed.ParseWarning
	msg.Priority = parsed.Priority
	msg.Attachments = parsed.Attachments
	if msg.Priority == "" {
		msg.Priority = "normal"
	}
}

func needsParse(msg *model.Message) bool {
	if len(msg.Raw) == 0 {
		return false
	}
	if msg.ParseWarning != "" {
		return false
	}
	if len(msg.Headers) > 0 || msg.Text != "" || msg.HTML != "" || len(msg.Attachments) > 0 {
		return false
	}
	return true
}

func matchFilter(msg *model.Message, f model.MessageFilter) bool {
	if msg == nil {
		return false
	}
	if f.Subject != "" && msg.Subject != f.Subject {
		return false
	}
	if f.SubjectContains != "" && !containsFold(msg.Subject, f.SubjectContains) {
		return false
	}
	if f.To != "" && !hasAddress(msg.Envelope.To, msg.To, f.To) {
		return false
	}
	if f.From != "" {
		env := []string(nil)
		if msg.Envelope.From != "" {
			env = []string{msg.Envelope.From}
		}
		if !hasAddress(env, msg.From, f.From) {
			return false
		}
	}
	if f.Unread != nil && msg.Read == *f.Unread {
		return false
	}
	if !f.After.IsZero() && !msg.ReceivedAt.After(f.After) {
		return false
	}
	if !f.Before.IsZero() && !msg.ReceivedAt.Before(f.Before) {
		return false
	}
	return true
}

func hasAddress(envelope []string, headers []model.Address, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return true
	}
	for _, a := range envelope {
		if strings.ToLower(strings.TrimSpace(a)) == want {
			return true
		}
	}
	for _, a := range headers {
		if strings.ToLower(a.Address) == want {
			return true
		}
	}
	return false
}

func containsFold(hay, needle string) bool {
	return strings.Contains(strings.ToLower(hay), strings.ToLower(needle))
}

func cloneMessage(in *model.Message) *model.Message {
	if in == nil {
		return nil
	}
	out := *in
	out.Raw = append([]byte(nil), in.Raw...)
	out.Envelope.To = append([]string(nil), in.Envelope.To...)
	out.Headers = append([]model.Header(nil), in.Headers...)
	out.From = append([]model.Address(nil), in.From...)
	out.To = append([]model.Address(nil), in.To...)
	out.Cc = append([]model.Address(nil), in.Cc...)
	out.Bcc = append([]model.Address(nil), in.Bcc...)
	out.ReplyTo = append([]model.Address(nil), in.ReplyTo...)
	if in.Attachments != nil {
		out.Attachments = make([]model.Attachment, len(in.Attachments))
		copy(out.Attachments, in.Attachments)
		for i := range out.Attachments {
			out.Attachments[i].Data = append([]byte(nil), in.Attachments[i].Data...)
		}
	}
	return &out
}

var _ Store = (*Memory)(nil)
