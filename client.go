package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub  *Hub
	conn *websocket.Conn
}

func serveWs(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		client := &Client{hub: hub, conn: conn}

		hub.register <- client
		defer func() { hub.unregister <- client }()

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			hub.broadcastCh <- outboundMessage{sender: client, messageType: messageType, data: data}
		}
	}
}
