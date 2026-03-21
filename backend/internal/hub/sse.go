package hub

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/devutility/webhookplatform/internal/domain"
)

// SSEHub manages per-endpoint Server-Sent Event client channels.
// When a webhook arrives, Broadcast pushes it to every connected browser tab
// watching that endpoint — no database polling needed.
type SSEHub struct {
	mu      sync.RWMutex
	clients map[string]map[chan []byte]struct{} // endpointID → set of channels
	log     *slog.Logger
}

func New(log *slog.Logger) *SSEHub {
	return &SSEHub{
		clients: make(map[string]map[chan []byte]struct{}),
		log:     log,
	}
}

// Subscribe registers a new SSE client for the given endpoint.
// Returns a channel that receives JSON-encoded WebhookRequest messages.
func (h *SSEHub) Subscribe(endpointID string) chan []byte {
	ch := make(chan []byte, 16) // buffer so slow clients don't block ingestion
	h.mu.Lock()
	if h.clients[endpointID] == nil {
		h.clients[endpointID] = make(map[chan []byte]struct{})
	}
	h.clients[endpointID][ch] = struct{}{}
	h.mu.Unlock()
	h.log.Info("sse client connected", "endpoint_id", endpointID, "total", h.countLocked(endpointID))
	return ch
}

// Unsubscribe removes a client channel when the browser disconnects.
func (h *SSEHub) Unsubscribe(endpointID string, ch chan []byte) {
	h.mu.Lock()
	delete(h.clients[endpointID], ch)
	if len(h.clients[endpointID]) == 0 {
		delete(h.clients, endpointID)
	}
	h.mu.Unlock()
	h.log.Info("sse client disconnected", "endpoint_id", endpointID)
}

// Broadcast sends a new webhook request to all clients watching its endpoint.
// Non-blocking: slow clients are skipped (their buffer is full).
func (h *SSEHub) Broadcast(req *domain.WebhookRequest) {
	data, err := json.Marshal(req)
	if err != nil {
		h.log.Error("sse: marshal broadcast", "error", err)
		return
	}

	h.mu.RLock()
	clients := h.clients[req.EndpointID]
	h.mu.RUnlock()

	for ch := range clients {
		select {
		case ch <- data:
		default:
			h.log.Warn("sse: client buffer full, skipping", "endpoint_id", req.EndpointID)
		}
	}
}

func (h *SSEHub) countLocked(endpointID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[endpointID])
}
