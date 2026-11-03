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
