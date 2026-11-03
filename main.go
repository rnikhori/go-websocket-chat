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
		#log div.system { color: #888; font-style: italic; }
		form { display: flex; gap: 0.5rem; }
		#msg { flex: 1; }
		#chat { display: none; }
	</style>
</head>
<body>
	<h1>Go WebSocket Chat</h1>
	<form id="setup">
		<input id="username" type="text" autocomplete="off" placeholder="Your name">
		<input id="room" type="text" autocomplete="off" placeholder="Room (default: general)">
		<button type="submit">Join</button>
	</form>
	<div id="chat">
		<h2 id="roomTitle"></h2>
		<div id="log"></div>
		<form id="form">
			<input id="msg" type="text" autocomplete="off" placeholder="Type a message...">
			<button type="submit">Send</button>
		</form>
	</div>
	<script>
		const log = document.getElementById("log");
		const chat = document.getElementById("chat");
		const roomTitle = document.getElementById("roomTitle");
		const setup = document.getElementById("setup");
		const usernameInput = document.getElementById("username");
		const roomInput = document.getElementById("room");
		const form = document.getElementById("form");
		const input = document.getElementById("msg");

		function append(text, className) {
			const div = document.createElement("div");
			if (className) div.className = className;
			div.textContent = text;
			log.appendChild(div);
			log.scrollTop = log.scrollHeight;
		}

		function renderMessage(msg) {
			switch (msg.type) {
				case "message":
					append("[" + msg.timestamp + "] " + msg.username + ": " + msg.body);
					break;
				case "join":
					append("[" + msg.timestamp + "] * " + msg.username + " joined " + msg.room, "system");
					break;
				case "leave":
					append("[" + msg.timestamp + "] * " + msg.username + " left " + msg.room, "system");
					break;
				default:
					append(JSON.stringify(msg), "system");
			}
		}

		let ws;

		function connect(username, room) {
			const params = "?username=" + encodeURIComponent(username) + "&room=" + encodeURIComponent(room);
			ws = new WebSocket("ws://" + location.host + "/ws" + params);
			ws.onopen = () => append("[connected as " + username + " in " + room + "]");
			ws.onclose = () => append("[disconnected]");
			ws.onmessage = (event) => renderMessage(JSON.parse(event.data));
		}

		setup.addEventListener("submit", (e) => {
			e.preventDefault();
			if (!usernameInput.value) return;
			const room = roomInput.value || "general";
			roomTitle.textContent = "Room: " + room;
			setup.style.display = "none";
			chat.style.display = "block";
			connect(usernameInput.value, room);
		});

		form.addEventListener("submit", (e) => {
			e.preventDefault();
			if (!input.value) return;
			ws.send(JSON.stringify({ type: "message", body: input.value }));
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
