package main

import "log"

type outboundMessage struct {
	sender      *Client
	messageType int
	data        []byte
}

type Hub struct {
	clients     map[*Client]bool
	register    chan *Client
	unregister  chan *Client
	broadcastCh chan outboundMessage
}

func newHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcastCh: make(chan outboundMessage),
	}
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
			}
		case m := <-h.broadcastCh:
			for c := range h.clients {
				if c == m.sender {
					continue
				}
				if err := c.conn.WriteMessage(m.messageType, m.data); err != nil {
					log.Printf("write error: %v", err)
					c.conn.Close()
					delete(h.clients, c)
				}
			}
		}
	}
}
