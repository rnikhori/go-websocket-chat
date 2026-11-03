package main

import (
	"testing"
	"time"
)

func TestTokenBucketAllowsBurstUpToCapacity(t *testing.T) {
	b := NewTokenBucket(3, 1)
	frozen := b.last
	b.now = func() time.Time { return frozen }

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("expected Allow() to succeed for token %d", i)
		}
	}
	if b.Allow() {
		t.Fatal("expected Allow() to fail once capacity is exhausted")
	}
}

func TestTokenBucketRefillsOverTime(t *testing.T) {
	b := NewTokenBucket(2, 1) // 1 token/sec
	current := b.last
	b.now = func() time.Time { return current }

	if !b.Allow() || !b.Allow() {
		t.Fatal("expected initial burst of 2 to succeed")
	}
	if b.Allow() {
		t.Fatal("expected bucket to be empty")
	}

	current = current.Add(1500 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("expected a token to have refilled after 1.5s at 1 token/sec")
	}
	if b.Allow() {
		t.Fatal("expected only one token to have refilled")
	}
}

func TestTokenBucketDoesNotExceedCapacity(t *testing.T) {
	b := NewTokenBucket(2, 10)
	current := b.last
	b.now = func() time.Time { return current }

	current = current.Add(10 * time.Second)
	for i := 0; i < 2; i++ {
		if !b.Allow() {
			t.Fatalf("expected Allow() to succeed for token %d after long idle", i)
		}
	}
	if b.Allow() {
		t.Fatal("tokens should be capped at capacity, not unbounded")
	}
}
