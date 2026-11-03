package main

import (
	"log"
	"net/http"
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
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	username string
	room     string
}

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

func (c *Client) readPump() {
	defer func() { c.hub.unregister <- c }()
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

		switch in.Type {
		case MessageTypeTyping:
			c.sendTyping()
		default:
			c.sendChatMessage(in.Body)
		}
	}
}

func (c *Client) sendChatMessage(body string) {
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

func serveWs(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}

		username := r.URL.Query().Get("username")
		if username == "" {
			username = defaultUsername
		}
		room := r.URL.Query().Get("room")
		if room == "" {
			room = defaultRoom
		}

		client := &Client{
			hub:      hub,
			conn:     conn,
			send:     make(chan []byte, sendBufferSize),
			username: username,
			room:     room,
		}

		hub.register <- client

		go client.writePump()
		client.readPump()
	}
}
