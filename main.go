package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const indexHTML = `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Go WebSocket Chat</title>
	<style>
		body { font-family: sans-serif; max-width: 760px; margin: 2rem auto; }
		#log { border: 1px solid #ccc; height: 300px; overflow-y: auto; padding: 0.5rem; margin-bottom: 0.5rem; }
		#log div { margin-bottom: 0.25rem; }
		#log div.system { color: #888; font-style: italic; }
		form { display: flex; gap: 0.5rem; }
		#msg { flex: 1; }
		#chat { display: none; }
		#typing { height: 1.25rem; font-size: 0.9rem; color: #888; font-style: italic; }
		#chatArea { display: flex; gap: 1rem; }
		#chatArea > div:first-child { flex: 1; min-width: 0; }
		#roster { width: 150px; border: 1px solid #ccc; padding: 0.5rem; height: 300px; overflow-y: auto; }
		#roster h3 { margin: 0 0 0.5rem 0; font-size: 0.9rem; }
		#roster ul { list-style: none; margin: 0; padding: 0; font-size: 0.9rem; }
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
		<div id="chatArea">
			<div>
				<div id="log"></div>
				<div id="typing"></div>
				<form id="form">
					<input id="msg" type="text" autocomplete="off" placeholder="Type a message...">
					<button type="submit">Send</button>
				</form>
			</div>
			<div id="roster">
				<h3>Online</h3>
				<ul id="rosterList"></ul>
			</div>
		</div>
	</div>
	<script>
		const log = document.getElementById("log");
		const chat = document.getElementById("chat");
		const roomTitle = document.getElementById("roomTitle");
		const typingEl = document.getElementById("typing");
		const rosterList = document.getElementById("rosterList");
		const setup = document.getElementById("setup");
		const usernameInput = document.getElementById("username");
		const roomInput = document.getElementById("room");
		const form = document.getElementById("form");
		const input = document.getElementById("msg");

		function renderRoster(users) {
			rosterList.innerHTML = "";
			for (const u of users) {
				const li = document.createElement("li");
				li.textContent = u;
				rosterList.appendChild(li);
			}
		}

		let typingTimer;

		function showTyping(username) {
			typingEl.textContent = username + " is typing...";
			clearTimeout(typingTimer);
			typingTimer = setTimeout(() => { typingEl.textContent = ""; }, 3000);
		}

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
				case "typing":
					showTyping(msg.username);
					break;
				case "error":
					append("[" + msg.timestamp + "] ! " + msg.body, "system");
					break;
				case "roster":
					renderRoster(msg.users || []);
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

		let lastTypingSent = 0;
		input.addEventListener("input", () => {
			const now = Date.now();
			if (now - lastTypingSent < 1500) return;
			lastTypingSent = now;
			ws.send(JSON.stringify({ type: "typing" }));
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
	addr := flag.String("addr", ":8081", "HTTP listen address")
	rateCapacity := flag.Float64("rate-burst", 5, "per-client rate limit burst capacity (messages)")
	ratePerSec := flag.Float64("rate-per-sec", 1, "per-client sustained rate limit (messages/sec)")
	flag.Parse()

	hub := newHub()
	go hub.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/ws", serveWs(hub, *rateCapacity, *ratePerSec))

	server := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		log.Printf("listening on %s", *addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
