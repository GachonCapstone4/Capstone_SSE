package sse

import (
	"encoding/json"
	"sync"
)

// Event is the message pushed to SSE clients.
type Event struct {
	Type string          `json:"event"`
	Data json.RawMessage `json:"data"`
}

// Hub manages SSE client channels keyed by user_id.
// One user may have multiple open connections (e.g. multiple browser tabs).
type Hub struct {
	mu      sync.RWMutex
	clients map[string][]chan Event
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string][]chan Event),
	}
}

// Register adds a channel for the given user.
func (h *Hub) Register(userID string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[userID] = append(h.clients[userID], ch)
}

// Unregister removes a specific channel for the given user.
func (h *Hub) Unregister(userID string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	channels := h.clients[userID]
	for i, c := range channels {
		if c == ch {
			h.clients[userID] = append(channels[:i], channels[i+1:]...)
			break
		}
	}
	if len(h.clients[userID]) == 0 {
		delete(h.clients, userID)
	}
}

// Broadcast sends an event to all connections owned by the given user.
func (h *Hub) Broadcast(userID string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients[userID] {
		select {
		case ch <- event:
		default:
			// skip slow consumer; channel buffer is full
		}
	}
}
