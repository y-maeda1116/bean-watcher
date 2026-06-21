package main

import (
	"testing"
	"time"

	"bean-watcher/internal/clock"
)

func TestJSTTodayFormat(t *testing.T) {
	// UTC 2026-06-20 23:00 == JST 2026-06-21 08:00
	c := clock.Fake{T: time.Date(2026, 6, 20, 23, 0, 0, 0, time.UTC)}
	got := jstToday(c)
	if got != "2026-06-21" {
		t.Errorf("got %q, want 2026-06-21", got)
	}
}
