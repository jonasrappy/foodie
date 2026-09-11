package web

import (
	"encoding/json"
	"sync"

	"github.com/jonasrappy/foodie/internal/store"
)

type event struct {
	revision int64
	data     []byte
}

// The hub never writes to sockets. Each subscriber keeps only the latest snapshot,
// so a slow tablet cannot block writers or grow an unbounded message queue.
type hub struct {
	mu      sync.Mutex
	clients map[chan event]struct{}
	last    int64
}

func newHub() *hub { return &hub{clients: make(map[chan event]struct{}), last: -1} }
func (h *hub) subscribe() (chan event, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.clients) >= 100 {
		return nil, false
	}
	ch := make(chan event, 1)
	h.clients[ch] = struct{}{}
	return ch, true
}
func (h *hub) unsubscribe(ch chan event) { h.mu.Lock(); defer h.mu.Unlock(); delete(h.clients, ch) }
func (h *hub) publish(state store.State) {
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	// A goroutine may resume late after commit. Never publish an older revision.
	if state.Revision <= h.last {
		return
	}
	h.last = state.Revision
	for ch := range h.clients {
		select {
		case ch <- event{state.Revision, data}:
		default:
			select {
			case <-ch:
			default:
			}
			ch <- event{state.Revision, data}
		}
	}
}
