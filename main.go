package main

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed static/index.html
var indexHTML string

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
