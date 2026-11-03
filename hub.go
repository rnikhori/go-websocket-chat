package main

import (
	"log"
	"time"
)

const maxClientsPerRoom = 100

type outboundMessage struct {
	sender  *Client
	data    []byte
	persist bool
}

// Hub owns all room membership, per-room history, and the roster. All of
// its state is only ever touched from the single goroutine running run(),
// which receives events over the register/unregister/broadcastCh channels;
// nothing else should mutate a Hub's fields directly once run() has started.
type Hub struct {
	rooms       map[string]map[*Client]bool
	history     map[string]*historyBuffer
	register    chan *Client
	unregister  chan *Client
	broadcastCh chan outboundMessage
	roomLimit   int
}

func newHub() *Hub {
	return &Hub{
		rooms:       make(map[string]map[*Client]bool),
		history:     make(map[string]*historyBuffer),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcastCh: make(chan outboundMessage),
		roomLimit:   maxClientsPerRoom,
	}
}

func (h *Hub) historyFor(room string) *historyBuffer {
	b, ok := h.history[room]
	if !ok {
		b = &historyBuffer{}
		h.history[room] = b
	}
	return b
}

// run is the hub's actor loop. It must be started exactly once (as a
// goroutine) before any client registers, and never returns.
func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.registerClient(c)
		case c := <-h.unregister:
			h.removeClient(c)
		case m := <-h.broadcastCh:
			if m.persist {
				h.historyFor(m.sender.room).add(m.data)
			}
			h.broadcastToRoom(m.sender.room, m.sender, m.data)
		}
	}
}

// registerClient adds c to its room (creating the room if needed), replays
// the room's history directly to c, then announces the join and an updated
// roster to the whole room.
func (h *Hub) registerClient(c *Client) {
	room, ok := h.rooms[c.room]
	if !ok {
		room = make(map[*Client]bool)
		h.rooms[c.room] = room
	}

	if len(room) >= h.roomLimit {
		h.rejectClient(c)
		return
	}
	room[c] = true

	for _, data := range h.historyFor(c.room).messages {
		c.send <- data
	}

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
	h.broadcastRoster(c.room)
}

// rejectClient refuses a connection that arrived once its room was already
// at maxClientsPerRoom. The client was never added to the room, so it is
// not counted and removeClient will no-op for it later.
func (h *Hub) rejectClient(c *Client) {
	log.Printf("rejecting client %s: room %s is full", c.username, c.room)
	notice := Message{
		Type:      MessageTypeError,
		Room:      c.room,
		Body:      "room is full, try again later",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := notice.Encode()
	if err == nil {
		c.send <- encoded
	}
	close(c.send)
}

// removeClient takes c out of its room, closing its send channel so its
// writePump (and in turn its connection) shuts down, then announces the
// leave and an updated roster to whoever remains. It is safe to call for a
// client that was already removed or never added; it is then a no-op.
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
	h.broadcastRoster(c.room)
}

func (h *Hub) broadcastRoster(roomName string) {
	room, ok := h.rooms[roomName]
	if !ok {
		return
	}
	users := make([]string, 0, len(room))
	for c := range room {
		users = append(users, c.username)
	}

	roster := Message{
		Type:  MessageTypeRoster,
		Room:  roomName,
		Users: users,
	}
	encoded, err := roster.Encode()
	if err != nil {
		log.Printf("encode error: %v", err)
		return
	}
	h.broadcastToAll(roomName, encoded)
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
		h.send(room, roomName, c, data)
	}
}

func (h *Hub) broadcastToAll(roomName string, data []byte) {
	room, ok := h.rooms[roomName]
	if !ok {
		return
	}
	for c := range room {
		h.send(room, roomName, c, data)
	}
}

// send delivers data to c's buffer without blocking the hub loop. If the
// client's buffer is full it is too slow to keep up and gets dropped
// instead of stalling every other client in the room.
func (h *Hub) send(room map[*Client]bool, roomName string, c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
		log.Printf("dropping slow client %s in room %s", c.username, roomName)
		delete(room, c)
		close(c.send)
		if len(room) == 0 {
			delete(h.rooms, roomName)
		}
	}
}
