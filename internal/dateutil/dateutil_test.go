package dateutil

import "testing"

func TestMinusDays(t *testing.T) {
	cases := []struct {
		date string
		n    int
		want string
	}{
		{"2026-06-21", 6, "2026-06-15"},
		{"2026-06-21", 1, "2026-06-20"},
		{"2026-03-01", 1, "2026-02-28"}, // 月跨ぎ・非閏年
	}
	for _, c := range cases {
		got, err := MinusDays(c.date, c.n)
		if err != nil {
			t.Fatalf("MinusDays(%s,%d) err: %v", c.date, c.n, err)
		}
		if got != c.want {
			t.Errorf("MinusDays(%s,%d) = %s, want %s", c.date, c.n, got, c.want)
		}
	}
}

func TestMinusDaysInvalid(t *testing.T) {
	if _, err := MinusDays("not-a-date", 1); err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestDaysBetween(t *testing.T) {
	got, err := DaysBetween("2026-05-01", "2026-06-01")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 31 {
		t.Errorf("got %d, want 31", got)
	}
}
