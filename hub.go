package main

import (
	"log"
	"sync"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*Client]bool
}

func newHub() *Hub {
	return &Hub{clients: make(map[*Client]bool)}
}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

func (h *Hub) broadcast(sender *Client, messageType int, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if c == sender {
			continue
		}
		if err := c.conn.WriteMessage(messageType, data); err != nil {
			log.Printf("write error: %v", err)
			c.conn.Close()
			delete(h.clients, c)
		}
	}
}
