package main

import (
	"log"
	"net/http"

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

		function renderMessage(msg) {
			switch (msg.type) {
				case "message":
					append("[" + msg.timestamp + "] " + msg.body);
					break;
				default:
					append(JSON.stringify(msg));
			}
		}

		const ws = new WebSocket("ws://" + location.host + "/ws");
		ws.onopen = () => append("[connected]");
		ws.onclose = () => append("[disconnected]");
		ws.onmessage = (event) => renderMessage(JSON.parse(event.data));

		form.addEventListener("submit", (e) => {
			e.preventDefault();
			if (!input.value) return;
			ws.send(JSON.stringify({ type: "message", body: input.value }));
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

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func main() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/ws", serveWs(hub))

	addr := ":8081"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
