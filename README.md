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
