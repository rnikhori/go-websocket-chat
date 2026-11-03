package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

const indexHTML = `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Go WebSocket Chat</title>
	<style>
		body { font-family: sans-serif; max-width: 600px; margin: 2rem auto; }
		#log { border: 1px solid #ccc; height: 300px; overflow-y: auto; padding: 0.5rem; margin-bottom: 0.5rem; }
		#log div { margin-bottom: 0.25rem; }
		form { display: flex; gap: 0.5rem; }
		#msg { flex: 1; }
	</style>
</head>
<body>
	<h1>Go WebSocket Chat</h1>
	<div id="log"></div>
	<form id="form">
		<input id="msg" type="text" autocomplete="off" placeholder="Type a message...">
		<button type="submit">Send</button>
	</form>
	<script>
		const log = document.getElementById("log");
		const form = document.getElementById("form");
		const input = document.getElementById("msg");

		function append(text) {
			const div = document.createElement("div");
			div.textContent = text;
			log.appendChild(div);
			log.scrollTop = log.scrollHeight;
		}

		const ws = new WebSocket("ws://" + location.host + "/ws");
		ws.onopen = () => append("[connected]");
		ws.onclose = () => append("[disconnected]");
		ws.onmessage = (event) => append(event.data);

		form.addEventListener("submit", (e) => {
			e.preventDefault();
			if (!input.value) return;
			ws.send(input.value);
			append("me: " + input.value);
			input.value = "";
		});
	</script>
</body>
</html>
`

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

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

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

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

func main() {
	hub := newHub()

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/ws", serveWs(hub))

	addr := ":8081"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
