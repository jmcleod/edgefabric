package cdnserver

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(10)

	// Should allow requests up to the limit.
	for i := 0; i < 10; i++ {
		if !rl.Allow() {
			t.Errorf("expected Allow() to return true at request %d", i)
		}
	}
}

func TestRateLimiterExhaust(t *testing.T) {
	rl := NewRateLimiter(5)

	// Exhaust all tokens.
	for i := 0; i < 5; i++ {
		rl.Allow()
	}

	// Next request should be denied.
	if rl.Allow() {
		t.Error("expected Allow() to return false when tokens exhausted")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(100)

	// Exhaust all tokens.
	for i := 0; i < 100; i++ {
		rl.Allow()
	}

	if rl.Allow() {
		t.Error("expected no tokens remaining")
	}

	// Wait for refill (100 tokens/sec means after 50ms we should have ~5 tokens).
	time.Sleep(60 * time.Millisecond)

	if !rl.Allow() {
		t.Error("expected Allow() to return true after refill")
	}
}

func TestRateLimiterMaxTokensCapped(t *testing.T) {
	rl := NewRateLimiter(5)

	// Wait to accumulate tokens beyond max.
	time.Sleep(200 * time.Millisecond)

	// Should be capped at maxTokens (5).
	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.Allow() {
			allowed++
		}
	}

	if allowed > 5 {
		t.Errorf("expected at most 5 tokens (burst), got %d", allowed)
	}
}
