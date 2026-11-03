package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// sendBufferSize must comfortably hold a full history replay (historySize)
	// plus a few live messages so registerClient never blocks the hub loop.
	sendBufferSize  = historySize + 16
	defaultUsername = "anonymous"
	defaultRoom     = "general"

	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	maxMessageBytes    = 2000
	maxIdentifierBytes = 32
)

func sanitizeIdentifier(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if len(value) > maxIdentifierBytes {
		value = value[:maxIdentifierBytes]
	}
	return value
}

// Client wraps one WebSocket connection. Its send channel is the only way
// the hub (or Client itself) delivers outbound bytes; readPump and
// writePump each own one goroutine and never touch the connection
// concurrently with each other.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	username string
	room     string
	limiter  *TokenBucket
}

// writePump is the only goroutine that ever calls conn.WriteMessage. It
// relays buffered outbound frames and sends a heartbeat ping every
// pingPeriod, exiting (and closing the connection) on the first write
// error or once send is closed.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump is the only goroutine that ever calls conn.ReadMessage. It
// decodes, rate-limits, and dispatches each incoming frame, and always
// ends by unregistering c from the hub -- on a read error, a closed
// connection, or a missed pong deadline -- so no connection outlives its
// goroutines.
func (c *Client) readPump() {
	defer func() { c.hub.unregister <- c }()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		in, err := decodeMessage(data)
		if err != nil {
			log.Printf("discarding malformed message: %v", err)
			continue
		}

		if !c.limiter.Allow() {
			c.sendError("rate limit exceeded, slow down")
			continue
		}

		switch in.Type {
		case MessageTypeTyping:
			c.sendTyping()
		default:
			c.sendChatMessage(in.Body)
		}
	}
}

func (c *Client) sendChatMessage(body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	if len(body) > maxMessageBytes {
		c.sendError("message too long, max 2000 bytes")
		return
	}

	out := Message{
		Type:      MessageTypeMessage,
		Room:      c.room,
		Username:  c.username,
		Body:      body,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := out.Encode()
	if err != nil {
		log.Printf("encode error: %v", err)
		return
	}
	c.hub.broadcastCh <- outboundMessage{sender: c, data: encoded, persist: true}
}

func (c *Client) sendTyping() {
	out := Message{
		Type:      MessageTypeTyping,
		Room:      c.room,
		Username:  c.username,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := out.Encode()
	if err != nil {
		log.Printf("encode error: %v", err)
		return
	}
	c.hub.broadcastCh <- outboundMessage{sender: c, data: encoded, persist: false}
}

func (c *Client) sendError(body string) {
	out := Message{
		Type:      MessageTypeError,
		Room:      c.room,
		Body:      body,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := out.Encode()
	if err != nil {
		log.Printf("encode error: %v", err)
		return
	}
	c.send <- encoded
}

// serveWs upgrades an HTTP request to a WebSocket, builds a Client for it
// from the ?username=/?room= query params, and hands it to hub before
// starting its read/write goroutines.
func serveWs(hub *Hub, rateCapacity, ratePerSec float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}

		username := sanitizeIdentifier(r.URL.Query().Get("username"), defaultUsername)
		room := sanitizeIdentifier(r.URL.Query().Get("room"), defaultRoom)

		client := &Client{
			hub:      hub,
			conn:     conn,
			send:     make(chan []byte, sendBufferSize),
			username: username,
			room:     room,
			limiter:  NewTokenBucket(rateCapacity, ratePerSec),
		}

		log.Printf("client connecting: username=%q room=%q remote=%s", username, room, r.RemoteAddr)
		hub.register <- client

		go client.writePump()
		client.readPump()
		log.Printf("client disconnected: username=%q room=%q remote=%s", username, room, r.RemoteAddr)
	}
}
