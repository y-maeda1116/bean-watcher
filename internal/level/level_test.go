package level

import (
	"testing"

	"bean-watcher/internal/model"
)

func baseConfig() model.Config {
	return model.Config{
		GramsPerCup:           12,
		LowDaysThreshold:      5,
		CriticalDaysThreshold: 2,
		AvgWindowDays:         7,
		FallbackCupsPerDay:    2,
	}
}

func TestBeansLevel(t *testing.T) {
	cfg := baseConfig()
	// 過去7日で計14杯 -> 平均2杯/日 -> 1日24g消費
	records := []model.DailyRecord{
		{Date: "2026-06-15", Cups: 2}, {Date: "2026-06-16", Cups: 2},
		{Date: "2026-06-17", Cups: 2}, {Date: "2026-06-18", Cups: 2},
		{Date: "2026-06-19", Cups: 2}, {Date: "2026-06-20", Cups: 2},
		{Date: "2026-06-21", Cups: 2},
	}
	cases := []struct {
		name     string
		grams    float64
		records  []model.DailyRecord
		want     string
	}{
		{"OK_残り十分", 300, records, "OK"},     // 300/24 = 12.5日
		{"LOW_境界5日", 120, records, "LOW"},    // 120/24 = 5.0日
		{"CRITICAL_境界2日", 48, records, "CRITICAL"}, // 48/24 = 2.0日
		{"CRITICAL_残量0", 0, records, "CRITICAL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := model.Data{Beans: model.Beans{RemainingGrams: c.grams}, Usage: model.Usage{DailyRecords: c.records}}
			got := Beans(d, cfg, "2026-06-21")
			if got != c.want {
				t.Errorf("Beans(grams=%v) = %s, want %s", c.grams, got, c.want)
			}
		})
	}
}

func TestBeansLevelFallback(t *testing.T) {
	cfg := baseConfig()
	// 履歴なし -> fallback 2杯/日 -> 1日24g
	d := model.Data{Beans: model.Beans{RemainingGrams: 120}}
	if got := Beans(d, cfg, "2026-06-21"); got != "LOW" {
		t.Errorf("fallback: got %s, want LOW", got)
	}
}

func TestBeansLevelAboveFiveDaysIsOK(t *testing.T) {
	cfg := baseConfig()
	records := []model.DailyRecord{{Date: "2026-06-21", Cups: 2}} // 1件 -> 2/7 cup/day
	d := model.Data{Beans: model.Beans{RemainingGrams: 300}, Usage: model.Usage{DailyRecords: records}}
	// 1日平均 = 2/7 杯 -> 1日 g = 12 * 2/7 = 3.428g -> 300/3.428 = 87.5日 -> OK
	if got := Beans(d, cfg, "2026-06-21"); got != "OK" {
		t.Errorf("got %s, want OK", got)
	}
}

func TestMaintenanceLevel(t *testing.T) {
	th := model.Threshold{ThresholdDays: 30, ThresholdShots: 200}
	cases := []struct {
		name   string
		state  model.MaintenanceState
		shots  int
		today  string
		want   string
	}{
		{"未設定は空", model.MaintenanceState{LastDate: "", LastShots: 0}, 100, "2026-06-21", ""},
		{"日数超過でDUE", model.MaintenanceState{LastDate: "2026-05-01", LastShots: 0}, 10, "2026-06-01", "DUE"}, // 31日
		{"回数超過でDUE", model.MaintenanceState{LastDate: "2026-06-20", LastShots: 0}, 250, "2026-06-21", "DUE"},
		{"どちらも未超過でOK", model.MaintenanceState{LastDate: "2026-06-20", LastShots: 0}, 10, "2026-06-21", "OK"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Maintenance(c.state, th, c.shots, c.today)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
