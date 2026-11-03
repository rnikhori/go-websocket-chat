package main

import (
	"strings"
	"testing"
)

func TestSanitizeIdentifier(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		fallback string
		want     string
	}{
		{"empty uses fallback", "", "general", "general"},
		{"whitespace only uses fallback", "   ", "general", "general"},
		{"trims surrounding whitespace", "  alice  ", "anonymous", "alice"},
		{"passes through short value", "bob", "anonymous", "bob"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeIdentifier(tc.value, tc.fallback)
			if got != tc.want {
				t.Fatalf("sanitizeIdentifier(%q, %q) = %q, want %q", tc.value, tc.fallback, got, tc.want)
			}
		})
	}

	long := strings.Repeat("x", maxIdentifierBytes+10)
	got := sanitizeIdentifier(long, "fallback")
	if len(got) != maxIdentifierBytes {
		t.Fatalf("expected identifier truncated to %d bytes, got %d", maxIdentifierBytes, len(got))
	}
}

func TestClientSendChatMessageDropsEmptyBody(t *testing.T) {
	hub := newHub()
	go hub.run()

	c := newTestClient(hub, "alice", "general")
	hub.register <- c
	drainJoinAndRoster(t, c) // own join notice + roster

	c.sendChatMessage("   ")
	expectNoMessage(t, c)
}

func TestClientSendChatMessageRejectsOversizedBody(t *testing.T) {
	hub := newHub()
	go hub.run()

	c := newTestClient(hub, "alice", "general")
	hub.register <- c
	drainJoinAndRoster(t, c)

	c.sendChatMessage(strings.Repeat("x", maxMessageBytes+1))

	msg := recvMessage(t, c)
	if msg.Type != MessageTypeError {
		t.Fatalf("expected an error message for an oversized body, got: %+v", msg)
	}
}
