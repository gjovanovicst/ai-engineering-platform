package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	l := New(2, time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("first request denied")
	}
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("second request denied")
	}
	ok, retry := l.Allow("a")
	if ok {
		t.Fatal("third request allowed, want denied")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("retry = %v, want within the window", retry)
	}
}

func TestWindowResets(t *testing.T) {
	l := New(1, time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("first denied")
	}
	now = now.Add(time.Minute)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("request after the window denied")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, time.Minute)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("a denied")
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("b denied, keys must be independent")
	}
}
