package model

import (
	"encoding/json"
	"testing"
)

func TestDataRoundTrip(t *testing.T) {
	src := Data{
		Beans: Beans{RemainingGrams: 300},
		Usage: Usage{
			TotalShots: 5,
			DailyRecords: []DailyRecord{
				{Date: "2026-06-20", Cups: 2},
			},
		},
		Maintenance: Maintenance{
			Descaling: MaintenanceState{LastDate: "2026-06-01", LastShots: 0},
			Grinder:   MaintenanceState{LastDate: "", LastShots: 0},
		},
		NotifyState: NotifyState{Beans: "OK", Descaling: "OK", Grinder: "OK"},
	}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Data
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Beans.RemainingGrams != 300 {
		t.Errorf("remaining: got %v, want 300", got.Beans.RemainingGrams)
	}
	if got.Usage.TotalShots != 5 {
		t.Errorf("shots: got %v, want 5", got.Usage.TotalShots)
	}
	if got.Maintenance.Grinder.LastDate != "" {
		t.Errorf("grinder last_date: got %q, want empty", got.Maintenance.Grinder.LastDate)
	}
	// JSON キー名の確認
	if want := `"remaining_grams"`; !contains(string(raw), want) {
		t.Errorf("json key %s missing in %s", want, string(raw))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
