package id

import (
	"testing"
	"time"
)

func TestNewIsSortableAndUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	var prev string
	for i := 0; i < 1000; i++ {
		got := New()
		if len(got) != Length {
			t.Fatalf("length = %d, want %d", len(got), Length)
		}
		if seen[got] {
			t.Fatalf("duplicate identifier %q", got)
		}
		seen[got] = true
		if prev != "" && got <= prev {
			t.Fatalf("identifier %q not greater than previous %q", got, prev)
		}
		prev = got
	}
}

func TestTimestampRoundTrip(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	got := New()
	after := time.Now().UTC().Add(time.Second)

	ts, err := Timestamp(got)
	if err != nil {
		t.Fatalf("Timestamp: %v", err)
	}
	if ts.Before(before) || ts.After(after) {
		t.Fatalf("timestamp %v outside [%v, %v]", ts, before, after)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "short", "0123456789012345678901234U", "012345678901234567890123456"} {
		if _, err := Timestamp(s); err == nil {
			t.Fatalf("Timestamp(%q) = nil error, want error", s)
		}
	}
}
