package main

import (
	"log"
	"time"
)

type outboundMessage struct {
	sender *Client
	data   []byte
}

type Hub struct {
	rooms       map[string]map[*Client]bool
	register    chan *Client
	unregister  chan *Client
	broadcastCh chan outboundMessage
}

func newHub() *Hub {
	return &Hub{
		rooms:       make(map[string]map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcastCh: make(chan outboundMessage),
	}
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.registerClient(c)
		case c := <-h.unregister:
			h.removeClient(c)
		case m := <-h.broadcastCh:
			h.broadcastToRoom(m.sender.room, m.sender, m.data)
		}
	}
}

func (h *Hub) registerClient(c *Client) {
	room, ok := h.rooms[c.room]
	if !ok {
		room = make(map[*Client]bool)
		h.rooms[c.room] = room
	}
	room[c] = true

	notice := Message{
		Type:      MessageTypeJoin,
		Room:      c.room,
		Username:  c.username,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := notice.Encode()
	if err != nil {
		log.Printf("encode error: %v", err)
		return
	}
	h.broadcastToAll(c.room, encoded)
}

func (h *Hub) removeClient(c *Client) {
	room, ok := h.rooms[c.room]
	if !ok {
		return
	}
	if _, ok := room[c]; !ok {
		return
	}
	delete(room, c)
	close(c.send)

	if len(room) == 0 {
		delete(h.rooms, c.room)
		return
	}

	notice := Message{
		Type:      MessageTypeLeave,
		Room:      c.room,
		Username:  c.username,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := notice.Encode()
	if err != nil {
		log.Printf("encode error: %v", err)
		return
	}
	h.broadcastToAll(c.room, encoded)
}

func (h *Hub) broadcastToRoom(roomName string, sender *Client, data []byte) {
	room, ok := h.rooms[roomName]
	if !ok {
		return
	}
	for c := range room {
		if c == sender {
			continue
		}
		c.send <- data
	}
}

func (h *Hub) broadcastToAll(roomName string, data []byte) {
	room, ok := h.rooms[roomName]
	if !ok {
		return
	}
	for c := range room {
		c.send <- data
	}
}
