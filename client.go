package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const sendBufferSize = 16

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
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
		c.hub.broadcastCh <- outboundMessage{sender: c, data: data}
	}
}

func serveWs(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}

		client := &Client{hub: hub, conn: conn, send: make(chan []byte, sendBufferSize)}

		hub.register <- client

		go client.writePump()
		client.readPump()
	}
}
