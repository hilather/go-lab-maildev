package store

import "sync"

// Inbox event types for REST SSE / MCP notify.
const (
	EventMailReceived = "mail.received"
	EventMailDeleted  = "mail.deleted"
	EventStoreWiped   = "store.wiped"
)

// Event is one inbox membership change.
type Event struct {
	Type       string
	ID         string
	Subject    string
	Generation uint64
}

type subscriber struct {
	ch chan Event
}

// Subscribe receives membership events. The buffer drops events if the
// consumer is slower than inserts. cancel must be called.
func (m *Memory) Subscribe(buffer int) (<-chan Event, func()) {
	if m == nil {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}
	if buffer <= 0 {
		buffer = 16
	}
	sub := &subscriber{ch: make(chan Event, buffer)}
	m.mu.Lock()
	m.subs = append(m.subs, sub)
	m.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			m.mu.Lock()
			for i, s := range m.subs {
				if s == sub {
					m.subs = append(m.subs[:i], m.subs[i+1:]...)
					break
				}
			}
			m.mu.Unlock()
			close(sub.ch)
		})
	}
	return sub.ch, cancel
}

func (m *Memory) emitLocked(ev Event) {
	for _, s := range m.subs {
		select {
		case s.ch <- ev:
		default:
		}
	}
}
