package clock

import (
	"testing"
	"time"
)

func TestFakeReturnsFixedTime(t *testing.T) {
	want := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	c := Fake{T: want}
	if got := c.Now(); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRealReturnsNonZero(t *testing.T) {
	c := Real{}
	if c.Now().IsZero() {
		t.Error("real clock returned zero time")
	}
}
