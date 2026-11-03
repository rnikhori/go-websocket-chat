# go-websocket-chat

A WebSocket chat server in Go with rooms, usernames, join/leave and typing
notifications, per-room message history, a ping/pong heartbeat, and
per-client rate limiting. The server and browser client speak a small JSON
message protocol over `/ws`.

## Run

```sh
go run .
```

Or build and run the binary:

```sh
go build -o go-websocket-chat .
./go-websocket-chat
```

Then open http://localhost:8081 in two (or more) browser tabs, pick a
username and room, and start chatting. Messages are only broadcast to other
clients in the same room.

## Configuration

The server accepts a few flags:

| Flag             | Default | Description                                   |
|------------------|---------|------------------------------------------------|
| `-addr`          | `:8081` | HTTP listen address                             |
| `-rate-burst`    | `5`     | Per-client rate limit burst capacity (messages) |
| `-rate-per-sec`  | `1`     | Per-client sustained rate limit (messages/sec)  |

Example:

```sh
go run . -addr=:9090 -rate-burst=10 -rate-per-sec=2
```

## Connecting

Open a WebSocket to `/ws`, optionally passing a username and room as query
parameters:

```
ws://localhost:8081/ws?username=alice&room=general
```

Both default (`anonymous` and `general`) when omitted, and are trimmed and
capped at 32 bytes.

## Protocol

Every WebSocket frame, in both directions, is a single JSON object with a
`type` field:

```json
{"type": "message", "room": "general", "username": "alice", "body": "hi", "timestamp": "2024-01-01T00:00:00Z"}
```

| Type      | Direction       | Fields used                              | Meaning                                          |
|-----------|-----------------|-------------------------------------------|---------------------------------------------------|
| `message` | client ↔ server | `body` (client); `room`, `username`, `body`, `timestamp` (server) | A chat message. Client only needs to send `body`; the server stamps room/username/timestamp. |
| `join`    | server → client | `room`, `username`, `timestamp`          | Sent to the whole room when a client connects.    |
| `leave`   | server → client | `room`, `username`, `timestamp`          | Sent to the remaining room members when a client disconnects. |
| `typing`  | client ↔ server | `room`, `username`, `timestamp`          | Client sends `{"type":"typing"}`; server fans it out to the rest of the room. Never stored in history. |
| `roster`  | server → client | `room`, `users`                          | Full list of usernames currently in the room; sent after every join/leave. |
| `error`   | server → client | `room`, `body`, `timestamp`              | Sent only to the offending client, e.g. for rate limiting or an oversized message. |

Unrecognized incoming `type` values are treated as a chat message body.

## Features

- **Rooms** — clients join a named room via `?room=`; broadcasts, history,
  and the roster are all scoped to that room.
- **Usernames** — passed via `?username=`, stamped onto every message the
  client sends.
- **Join/leave notifications** — the whole room is told when someone
  connects or disconnects.
- **Online roster** — a `roster` message listing current usernames is sent
  after every join/leave.
- **Per-room history** — the last 50 messages in a room are replayed to a
  client immediately after it joins, before any live traffic.
- **Typing indicators** — throttled client-side, fanned out to the rest of
  the room, never persisted to history.
- **Ping/pong heartbeat** — the server pings every ~54s and expects a pong
  within 60s; a connection that stops responding is closed and cleaned up
  from the hub, so dead clients don't linger.
- **Backpressure handling** — broadcasting never blocks on a slow client;
  if a client's outbound buffer is full it is dropped instead of stalling
  delivery to everyone else in the room.
- **Rate limiting** — each client gets its own in-process token bucket
  (see `-rate-burst` / `-rate-per-sec`); exceeding it returns an `error`
  message instead of broadcasting.
- **Graceful shutdown** — SIGINT/SIGTERM trigger an `http.Server.Shutdown`
  with a 5s timeout rather than killing connections abruptly.

## Architecture

- `hub.go` — the `Hub` is a single goroutine ("actor") that owns all room
  membership, history buffers, and the roster, communicated with over
  channels (`register`, `unregister`, `broadcastCh`). This avoids locking
  and keeps all mutation of shared state on one goroutine.
- `client.go` — `Client` wraps one WebSocket connection with a buffered
  send channel, a `readPump` (reads + validates + rate-limits incoming
  frames) and a `writePump` (writes outgoing frames and ping frames) each
  running in their own goroutine.
- `protocol.go` — the `Message` JSON schema and type constants.
- `history.go` — a small capped-size buffer per room.
- `ratelimit.go` — the local `TokenBucket` rate limiter.
- `static/index.html` — the browser client, embedded into the binary via
  `go:embed`.
