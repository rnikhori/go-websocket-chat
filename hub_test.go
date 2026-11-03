package main

import (
	"encoding/json"
	"testing"
	"time"
)

func newTestClient(hub *Hub, username, room string) *Client {
	return &Client{
		hub:      hub,
		send:     make(chan []byte, sendBufferSize),
		username: username,
		room:     room,
	}
}

func recvMessage(t *testing.T, c *Client) Message {
	t.Helper()
	select {
	case data := <-c.send:
		var m Message
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return m
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
		return Message{}
	}
}

func expectNoMessage(t *testing.T, c *Client) {
	t.Helper()
	select {
	case data := <-c.send:
		t.Fatalf("expected no message, got: %s", data)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHubRegisterSendsJoinNotice(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	hub.register <- alice

	msg := recvMessage(t, alice)
	if msg.Type != MessageTypeJoin || msg.Username != "alice" || msg.Room != "general" {
		t.Fatalf("unexpected join notice: %+v", msg)
	}
}

func TestHubUnregisterSendsLeaveNoticeToOthers(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	bob := newTestClient(hub, "bob", "general")

	hub.register <- alice
	recvMessage(t, alice) // alice's own join notice

	hub.register <- bob
	recvMessage(t, bob)   // bob's own join notice
	recvMessage(t, alice) // alice sees bob join

	hub.unregister <- bob

	msg := recvMessage(t, alice)
	if msg.Type != MessageTypeLeave || msg.Username != "bob" {
		t.Fatalf("expected leave notice for bob, got: %+v", msg)
	}
}
