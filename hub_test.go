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

// drainJoinAndRoster reads the join/leave notice a registration or
// disconnect always emits, immediately followed by the roster update that
// accompanies it, and returns the first (join/leave) message.
func drainJoinAndRoster(t *testing.T, c *Client) Message {
	t.Helper()
	notice := recvMessage(t, c)
	roster := recvMessage(t, c)
	if roster.Type != MessageTypeRoster {
		t.Fatalf("expected a roster update right after %s, got: %+v", notice.Type, roster)
	}
	return notice
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

	msg := drainJoinAndRoster(t, alice)
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
	drainJoinAndRoster(t, alice)

	hub.register <- bob
	drainJoinAndRoster(t, bob)
	drainJoinAndRoster(t, alice) // alice sees bob join

	hub.register <- carol
	drainJoinAndRoster(t, carol)

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
	drainJoinAndRoster(t, alice)

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

func TestHubRosterUpdatesOnJoinAndLeave(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	hub.register <- alice
	recvMessage(t, alice) // join

	roster := recvMessage(t, alice)
	if roster.Type != MessageTypeRoster || len(roster.Users) != 1 || roster.Users[0] != "alice" {
		t.Fatalf("expected roster of just alice, got: %+v", roster)
	}

	bob := newTestClient(hub, "bob", "general")
	hub.register <- bob
	recvMessage(t, bob)   // bob's own join
	recvMessage(t, bob)   // bob's own roster
	recvMessage(t, alice) // alice sees bob join

	roster = recvMessage(t, alice)
	if roster.Type != MessageTypeRoster || len(roster.Users) != 2 {
		t.Fatalf("expected roster of 2 users after bob joins, got: %+v", roster)
	}

	hub.unregister <- bob
	recvMessage(t, alice) // alice sees bob leave

	roster = recvMessage(t, alice)
	if roster.Type != MessageTypeRoster || len(roster.Users) != 1 || roster.Users[0] != "alice" {
		t.Fatalf("expected roster of just alice again after bob leaves, got: %+v", roster)
	}
}

func TestHubTypingIsNotPersistedAndExcludesSender(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	bob := newTestClient(hub, "bob", "general")

	hub.register <- alice
	drainJoinAndRoster(t, alice)
	hub.register <- bob
	drainJoinAndRoster(t, bob)
	drainJoinAndRoster(t, alice) // alice sees bob join

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

func TestHubDropsSlowClientWithoutBlockingOthers(t *testing.T) {
	hub := newHub()
	go hub.run()

	sender := newTestClient(hub, "sender", "general")
	hub.register <- sender
	drainJoinAndRoster(t, sender) // sender's own join

	// victim's buffer holds exactly the 4 setup messages it will receive
	// from its own join/roster and observer's join/roster below; it is
	// never drained, so the very first flood message overflows it.
	victim := &Client{hub: hub, send: make(chan []byte, 4), username: "victim", room: "general"}
	hub.register <- victim
	drainJoinAndRoster(t, sender) // sender sees victim join

	observer := newTestClient(hub, "observer", "general")
	hub.register <- observer
	drainJoinAndRoster(t, sender)   // sender sees observer join
	drainJoinAndRoster(t, observer) // observer's own join

	for i := 0; i < 5; i++ {
		out := Message{Type: MessageTypeMessage, Room: "general", Username: "sender", Body: "flood"}
		data, err := out.Encode()
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		hub.broadcastCh <- outboundMessage{sender: sender, data: data, persist: true}
		msg := recvMessage(t, observer)
		if msg.Body != "flood" {
			t.Fatalf("observer should keep receiving broadcasts, got: %+v", msg)
		}
	}
}

func TestHubRejectsClientWhenRoomIsFull(t *testing.T) {
	hub := newHub()
	hub.roomLimit = 2 // call registerClient directly: no run() goroutine needed

	a := newTestClient(hub, "a", "general")
	b := newTestClient(hub, "b", "general")
	c := newTestClient(hub, "c", "general")

	hub.registerClient(a)
	hub.registerClient(b)
	hub.registerClient(c) // room already has 2 members, should be rejected

	room := hub.rooms["general"]
	if len(room) != 2 {
		t.Fatalf("expected room to stay at 2 members, got %d", len(room))
	}
	if _, ok := room[c]; ok {
		t.Fatal("rejected client should not have been added to the room")
	}

	msg := recvMessage(t, c)
	if msg.Type != MessageTypeError {
		t.Fatalf("expected an error message for the rejected client, got: %+v", msg)
	}

	if _, ok := <-c.send; ok {
		t.Fatal("expected c's send channel to be closed after rejection")
	}
}

func TestHubUnregisterSendsLeaveNoticeToOthers(t *testing.T) {
	hub := newHub()
	go hub.run()

	alice := newTestClient(hub, "alice", "general")
	bob := newTestClient(hub, "bob", "general")

	hub.register <- alice
	drainJoinAndRoster(t, alice) // alice's own join notice

	hub.register <- bob
	drainJoinAndRoster(t, bob)   // bob's own join notice
	drainJoinAndRoster(t, alice) // alice sees bob join

	hub.unregister <- bob

	msg := recvMessage(t, alice)
	if msg.Type != MessageTypeLeave || msg.Username != "bob" {
		t.Fatalf("expected leave notice for bob, got: %+v", msg)
	}
}
