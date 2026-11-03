package main

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func newHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) add(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = true
}

func (h *Hub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
}

func (h *Hub) broadcast(sender *websocket.Conn, messageType int, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		if conn == sender {
			continue
		}
		if err := conn.WriteMessage(messageType, data); err != nil {
			log.Printf("write error: %v", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}
