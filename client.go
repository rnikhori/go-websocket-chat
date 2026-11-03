package main

import (
	"log"
	"net/http"
)

func serveWs(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		hub.add(conn)
		defer hub.remove(conn)

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			hub.broadcast(conn, messageType, data)
		}
	}
}
