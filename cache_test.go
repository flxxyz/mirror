package main

import (
	"testing"
	"time"
)

func TestCacheTTL(t *testing.T) {
	if got := cacheTTL("NOPE", time.Minute); got != time.Minute {
		t.Fatalf("unset: got %v", got)
	}
	t.Setenv("CACHE_GIST", "5s")
	if got := cacheTTL("GIST", time.Minute); got != 5*time.Second {
		t.Fatalf("set: got %v", got)
	}
	t.Setenv("CACHE_GIST", "garbage")
	if got := cacheTTL("GIST", time.Minute); got != time.Minute {
		t.Fatalf("invalid: got %v", got)
	}
}
