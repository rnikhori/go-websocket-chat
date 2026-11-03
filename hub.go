package main

type outboundMessage struct {
	sender *Client
	data   []byte
}

type Hub struct {
	clients     map[*Client]bool
	register    chan *Client
	unregister  chan *Client
	broadcastCh chan outboundMessage
}

func newHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcastCh: make(chan outboundMessage),
	}
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case m := <-h.broadcastCh:
			for c := range h.clients {
				if c == m.sender {
					continue
				}
				c.send <- m.data
			}
		}
	}
}
