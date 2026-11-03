# go-websocket-chat

A minimal WebSocket broadcast chat server in Go.

## Run

```sh
go run .
```

Or build and run the binary:

```sh
go build -o go-websocket-chat .
./go-websocket-chat
```

Then open http://localhost:8081 in two (or more) browser tabs. Messages typed
in one tab are broadcast to all other connected tabs.
