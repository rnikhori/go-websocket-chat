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

func TestHubBroadcastRoomIsolation(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	bob := newTestClient(hub, "bob", "general")
	carol := newTestClient(hub, "carol", "other")

	hub.register <- alice
	recvMessage(t, alice)

	hub.register <- bob
	recvMessage(t, bob)
	recvMessage(t, alice) // alice sees bob join

	hub.register <- carol
	recvMessage(t, carol)

	out := Message{Type: MessageTypeMessage, Room: "general", Username: "alice", Body: "hello"}
	data, err := out.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	hub.broadcastCh <- outboundMessage{sender: alice, data: data, persist: true}

	msg := recvMessage(t, bob)
	if msg.Type != MessageTypeMessage || msg.Body != "hello" {
		t.Fatalf("bob should have received the message, got: %+v", msg)
	}

	expectNoMessage(t, alice)
	expectNoMessage(t, carol)
}

func TestHubHistoryReplayOnJoin(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	hub.register <- alice
	recvMessage(t, alice)

	for _, body := range []string{"first", "second", "third"} {
		out := Message{Type: MessageTypeMessage, Room: "general", Username: "alice", Body: body}
		data, err := out.Encode()
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		hub.broadcastCh <- outboundMessage{sender: alice, data: data, persist: true}
	}

	bob := newTestClient(hub, "bob", "general")
	hub.register <- bob

	for _, want := range []string{"first", "second", "third"} {
		msg := recvMessage(t, bob)
		if msg.Body != want {
			t.Fatalf("expected history message %q, got: %+v", want, msg)
		}
	}

	joinNotice := recvMessage(t, bob)
	if joinNotice.Type != MessageTypeJoin {
		t.Fatalf("expected join notice after history, got: %+v", joinNotice)
	}
}

func TestHubTypingIsNotPersistedAndExcludesSender(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	bob := newTestClient(hub, "bob", "general")

	hub.register <- alice
	recvMessage(t, alice)
	hub.register <- bob
	recvMessage(t, bob)
	recvMessage(t, alice) // alice sees bob join

	typing := Message{Type: MessageTypeTyping, Room: "general", Username: "alice"}
	data, err := typing.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	hub.broadcastCh <- outboundMessage{sender: alice, data: data, persist: false}

	msg := recvMessage(t, bob)
	if msg.Type != MessageTypeTyping || msg.Username != "alice" {
		t.Fatalf("expected typing notice for alice, got: %+v", msg)
	}
	expectNoMessage(t, alice)

	if len(hub.historyFor("general").messages) != 0 {
		t.Fatalf("typing events must not be persisted to history")
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
