package main

import "encoding/json"

const (
	MessageTypeMessage = "message"
	MessageTypeJoin    = "join"
	MessageTypeLeave   = "leave"
	MessageTypeTyping  = "typing"
	MessageTypeError   = "error"
	MessageTypeRoster  = "roster"
)

// Message is the single JSON schema used for every WebSocket frame in both
// directions; see the README for which fields each Type populates.
type Message struct {
	Type      string   `json:"type"`
	Room      string   `json:"room,omitempty"`
	Username  string   `json:"username,omitempty"`
	Body      string   `json:"body,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
	Users     []string `json:"users,omitempty"`
}

// Encode serializes m to the JSON bytes sent over the wire.
func (m Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// decodeMessage parses a raw WebSocket frame into a Message.
func decodeMessage(data []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return m, err
}
