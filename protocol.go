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

type Message struct {
	Type      string   `json:"type"`
	Room      string   `json:"room,omitempty"`
	Username  string   `json:"username,omitempty"`
	Body      string   `json:"body,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
	Users     []string `json:"users,omitempty"`
}

func (m Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func decodeMessage(data []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return m, err
}
