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
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	username string
	room     string
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
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

		out := Message{
			Type:      MessageTypeMessage,
			Room:      c.room,
			Username:  c.username,
			Body:      in.Body,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		encoded, err := out.Encode()
		if err != nil {
			log.Printf("encode error: %v", err)
			continue
		}
		c.hub.broadcastCh <- outboundMessage{sender: c, data: encoded}
	}
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
