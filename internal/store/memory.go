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

var spillName = regexp.MustCompile(`(?i)^[0-7][0-9A-HJKMNP-TV-Z]{25}(-[0-9]+)?\.(raw|att)(\.tmp)?$`)

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

var _ Store = (*Memory)(nil)

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

func normalizeOptions(opts Options) (Options, error) {
	if opts.MaxMessages <= 0 {
		return opts, errors.New("store: maxMessages must be > 0")
	}
	if opts.MaxBytes <= 0 {
		return opts, errors.New("store: maxBytes must be > 0")
	}
	switch opts.FullPolicy {
	case "", model.FullPolicyReject:
		opts.FullPolicy = model.FullPolicyReject
	case model.FullPolicyEvictOldest:
	default:
		return opts, fmt.Errorf("store: unknown fullPolicy %q", opts.FullPolicy)
	}
	if opts.SpillDirectory != "" && opts.SpillThreshold <= 0 {
		opts.SpillThreshold = 256 << 10
	}
	return opts, nil
}

// New builds an empty inbox and wipes any leftover spill files.
func New(opts Options) (*Memory, error) {
	opts, err := normalizeOptions(opts)
	if err != nil {
		return nil, err
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
	if err := m.unlinkAllSpill(); err != nil {
		return err
	}
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
	if candidate > m.maxBytes {
		return model.InsertResult{}, ErrTooLarge
	}

	// Spill to temp names before taking the index lock so a write failure
	// cannot evict existing mail. Rename + evict + index happen atomically.
	job, err := m.writeSpillTemps(prepared)
	if err != nil {
		return model.InsertResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			unlinkSpillJob(job)
		}
	}()

	m.mu.Lock()
	defer m.mu.Unlock()
	if epoch != m.epoch {
		return model.InsertResult{}, ErrStaleEpoch
	}
	if err := m.canAcceptLocked(candidate); err != nil {
		return model.InsertResult{}, err
	}
	rec := &record{msg: prepared, resident: candidate}
	if err := commitSpill(rec, job); err != nil {
		return model.InsertResult{}, err
	}
	if err := m.evictUntilFitsLocked(candidate); err != nil {
		unlinkRecord(rec)
		return model.InsertResult{}, err
	}
	m.byID[id] = rec
	m.insertOrderLocked(id, prepared.ReceivedAt)
	m.bytes += candidate
	m.generation++
	m.cond.Broadcast()
	committed = true
	return model.InsertResult{ID: id, Generation: m.generation}, nil
}

func (m *Memory) fitsLocked(candidate int64) bool {
	return len(m.byID) < m.maxMessages && m.bytes+candidate <= m.maxBytes
}

func (m *Memory) canAcceptLocked(candidate int64) error {
	if m.fitsLocked(candidate) {
		return nil
	}
	if m.fullPolicy != model.FullPolicyEvictOldest {
		return ErrFull
	}
	bytes := m.bytes
	count := len(m.byID)
	for _, id := range m.order {
		if count < m.maxMessages && bytes+candidate <= m.maxBytes {
			return nil
		}
		rec := m.byID[id]
		if rec == nil {
			continue
		}
		bytes -= rec.resident
		if bytes < 0 {
			bytes = 0
		}
		count--
	}
	if count < m.maxMessages && bytes+candidate <= m.maxBytes {
		return nil
	}
	return ErrFull
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

func (m *Memory) occupancyOKLocked() bool {
	return len(m.byID) <= m.maxMessages && m.bytes <= m.maxBytes
}

func (m *Memory) evictUntilUnderCapsLocked() error {
	for len(m.order) > 0 && !m.occupancyOKLocked() {
		m.removeLocked(m.order[0], true)
	}
	if !m.occupancyOKLocked() {
		return ErrOverNewCap
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
	rec, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return nil, ErrNotFound
	}
	snap := snapshotRecord(rec)
	m.mu.Unlock()
	if err := loadSpill(snap); err != nil {
		if os.IsNotExist(err) {
			m.mu.Lock()
			_, still := m.byID[id]
			m.mu.Unlock()
			if !still {
				return nil, ErrNotFound
			}
		}
		return nil, fmt.Errorf("%w: %v", ErrSpill, err)
	}
	if markRead {
		m.mu.Lock()
		if rec, ok := m.byID[id]; ok {
			rec.msg.Read = true
			snap.msg.Read = true
		}
		m.mu.Unlock()
	}
	return snap.msg, nil
}

// List returns newest-first pages. Cursor is the last id from the previous page.
func (m *Memory) List(q model.ListQuery) (model.ListResult, error) {
	m.mu.Lock()
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	cur, fresh := m.resolveCursorLocked(q.Cursor)
	passed := q.Cursor == "" || fresh
	var snaps []recSnap
	var lastID string
	var next string
	for i := len(m.order) - 1; i >= 0; i-- {
		id := m.order[i]
		rec := m.byID[id]
		if rec == nil {
			continue
		}
		if !passed {
			if cur.found && id == cur.id {
				passed = true
				continue
			}
			if !cur.found && !newerThanCursor(rec.msg, cur) {
				passed = true
			} else {
				continue
			}
		}
		if !matchFilter(rec.msg, q.Filter) {
			continue
		}
		if len(snaps) == limit {
			next = lastID
			break
		}
		snaps = append(snaps, snapshotRecord(rec))
		lastID = id
	}
	gen := m.generation
	m.mu.Unlock()

	items := make([]*model.Message, 0, len(snaps))
	for _, snap := range snaps {
		if err := loadSpill(snap); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return model.ListResult{}, fmt.Errorf("%w: %v", ErrSpill, err)
		}
		items = append(items, snap.msg)
	}
	return model.ListResult{Items: items, NextCursor: next, Generation: gen}, nil
}

type cursorPos struct {
	id    string
	at    time.Time
	found bool
}

func (m *Memory) resolveCursorLocked(id string) (cursorPos, bool) {
	if id == "" {
		return cursorPos{}, true
	}
	if rec, ok := m.byID[id]; ok {
		return cursorPos{id: id, at: rec.msg.ReceivedAt, found: true}, false
	}
	u, err := ulid.Parse(id)
	if err != nil {
		// Not a store cursor; do not restart from newest (would replay the inbox).
		return cursorPos{id: id}, false
	}
	// Missing id: resume after this ReceivedAt/ULID time. If the cursor was
	// the oldest current row and was deleted, every remaining row is newer
	// and the next page is empty. Wipe/DeleteAll invalidation is generation.
	return cursorPos{id: id, at: ulid.Time(u.Time())}, false
}

func newerThanCursor(msg *model.Message, cur cursorPos) bool {
	if msg.ReceivedAt.After(cur.at) {
		return true
	}
	if msg.ReceivedAt.Equal(cur.at) && msg.ID > cur.id {
		return true
	}
	return false
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
	for {
		if rec := m.newestMatchLocked(filter); rec != nil {
			snap := snapshotRecord(rec)
			m.mu.Unlock()
			if err := loadSpill(snap); err != nil {
				if os.IsNotExist(err) {
					m.mu.Lock()
					_, still := m.byID[snap.msg.ID]
					if still {
						m.mu.Unlock()
						return nil, fmt.Errorf("%w: %v", ErrSpill, err)
					}
					continue
				}
				return nil, fmt.Errorf("%w: %v", ErrSpill, err)
			}
			return snap.msg, nil
		}
		if err := ctx.Err(); err != nil {
			m.mu.Unlock()
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

// ReplaceCaps applies the three replaceStoreCaps fields.
func (m *Memory) ReplaceCaps(opts Options, force bool) error {
	if m == nil {
		return errors.New("store: nil Memory")
	}
	opts, err := normalizeOptions(opts)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	over := len(m.byID) > opts.MaxMessages || m.bytes > opts.MaxBytes
	if over && opts.FullPolicy != model.FullPolicyEvictOldest && !force {
		return ErrOverNewCap
	}
	m.maxMessages = opts.MaxMessages
	m.maxBytes = opts.MaxBytes
	m.fullPolicy = opts.FullPolicy
	if over {
		if err := m.evictUntilUnderCapsLocked(); err != nil {
			return ErrOverNewCap
		}
	}
	return nil
}

// Configure replaces all store options. Call after Wipe on reset.
func (m *Memory) Configure(opts Options) error {
	if m == nil {
		return errors.New("store: nil Memory")
	}
	opts, err := normalizeOptions(opts)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxMessages = opts.MaxMessages
	m.maxBytes = opts.MaxBytes
	m.fullPolicy = opts.FullPolicy
	m.maxWait = opts.MaxWait
	spillChanged := m.spillDir != opts.SpillDirectory || m.spillThreshold != opts.SpillThreshold
	m.spillDir = opts.SpillDirectory
	m.spillThreshold = opts.SpillThreshold
	if spillChanged && m.spillDir != "" {
		if err := m.prepareSpill(); err != nil {
			return err
		}
	}
	if len(m.byID) > m.maxMessages || m.bytes > m.maxBytes {
		if m.fullPolicy != model.FullPolicyEvictOldest {
			return ErrOverNewCap
		}
		if err := m.evictUntilUnderCapsLocked(); err != nil {
			return ErrOverNewCap
		}
	}
	return nil
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
	_ = m.unlinkAllSpill()
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

type recSnap struct {
	msg      *model.Message
	rawSpill string
	attSpill []string
}

func snapshotRecord(rec *record) recSnap {
	return recSnap{
		msg:      cloneMessage(rec.msg),
		rawSpill: rec.rawSpill,
		attSpill: append([]string(nil), rec.attSpill...),
	}
}

func loadSpill(s recSnap) error {
	if s.rawSpill != "" && len(s.msg.Raw) == 0 {
		b, err := os.ReadFile(s.rawSpill)
		if err != nil {
			return err
		}
		s.msg.Raw = b
	}
	for i := range s.msg.Attachments {
		if len(s.msg.Attachments[i].Data) > 0 {
			continue
		}
		if i < len(s.attSpill) && s.attSpill[i] != "" {
			b, err := os.ReadFile(s.attSpill[i])
			if err != nil {
				return err
			}
			s.msg.Attachments[i].Data = b
		}
	}
	return nil
}

type spillJob struct {
	rawTmp   string
	rawFinal string
	attTmp   []string
	attFinal []string
}

func (m *Memory) writeSpillTemps(msg *model.Message) (*spillJob, error) {
	if m.spillDir == "" || m.spillThreshold <= 0 {
		return nil, nil
	}
	job := &spillJob{}
	if int64(len(msg.Raw)) >= m.spillThreshold {
		tmp := filepath.Join(m.spillDir, msg.ID+".raw.tmp")
		if err := os.WriteFile(tmp, msg.Raw, 0o600); err != nil {
			return nil, fmt.Errorf("%w: raw: %v", ErrSpill, err)
		}
		job.rawTmp = tmp
		job.rawFinal = filepath.Join(m.spillDir, msg.ID+".raw")
	}
	if len(msg.Attachments) == 0 {
		return job, nil
	}
	job.attTmp = make([]string, len(msg.Attachments))
	job.attFinal = make([]string, len(msg.Attachments))
	for i := range msg.Attachments {
		a := &msg.Attachments[i]
		if int64(len(a.Data)) < m.spillThreshold {
			continue
		}
		tmp := filepath.Join(m.spillDir, fmt.Sprintf("%s-%d.att.tmp", msg.ID, i))
		if err := os.WriteFile(tmp, a.Data, 0o600); err != nil {
			unlinkSpillJob(job)
			return nil, fmt.Errorf("%w: attachment: %v", ErrSpill, err)
		}
		job.attTmp[i] = tmp
		job.attFinal[i] = filepath.Join(m.spillDir, fmt.Sprintf("%s-%d.att", msg.ID, i))
	}
	return job, nil
}

func commitSpill(rec *record, job *spillJob) error {
	if job == nil {
		return nil
	}
	if job.rawTmp != "" {
		if err := os.Rename(job.rawTmp, job.rawFinal); err != nil {
			return fmt.Errorf("%w: rename raw: %v", ErrSpill, err)
		}
		job.rawTmp = ""
		rec.rawSpill = job.rawFinal
		rec.msg.Raw = nil
	}
	if len(job.attTmp) == 0 {
		return nil
	}
	rec.attSpill = make([]string, len(job.attTmp))
	for i, tmp := range job.attTmp {
		if tmp == "" {
			continue
		}
		if err := os.Rename(tmp, job.attFinal[i]); err != nil {
			unlinkRecord(rec)
			return fmt.Errorf("%w: rename attachment: %v", ErrSpill, err)
		}
		job.attTmp[i] = ""
		rec.attSpill[i] = job.attFinal[i]
		rec.msg.Attachments[i].Data = nil
	}
	return nil
}

func unlinkSpillJob(job *spillJob) {
	if job == nil {
		return
	}
	if job.rawTmp != "" {
		_ = os.Remove(job.rawTmp)
		job.rawTmp = ""
	}
	for i, p := range job.attTmp {
		if p != "" {
			_ = os.Remove(p)
			job.attTmp[i] = ""
		}
	}
}

func (m *Memory) unlinkAllSpill() error {
	if m.spillDir == "" {
		return nil
	}
	ents, err := os.ReadDir(m.spillDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%w: read dir: %v", ErrSpill, err)
	}
	var first error
	for _, e := range ents {
		if e.IsDir() || !spillName.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(m.spillDir, e.Name())
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) && first == nil {
			first = fmt.Errorf("%w: unlink %s: %v", ErrSpill, e.Name(), err)
		}
	}
	return first
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
