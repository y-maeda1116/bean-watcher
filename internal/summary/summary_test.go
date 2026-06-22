package summary

import (
	"testing"

	"bean-watcher/internal/model"
)

func cfg() model.Config {
	return model.Config{
		GramsPerCup: 12, LowDaysThreshold: 5, CriticalDaysThreshold: 2,
		AvgWindowDays: 7, FallbackCupsPerDay: 2, BagGrams: 200,
		Maintenance: model.MaintenanceConfig{
			Descaling: model.Threshold{ThresholdDays: 30, ThresholdShots: 200},
			Grinder:   model.Threshold{ThresholdDays: 7, ThresholdShots: 30},
		},
	}
}

func sevenDaysTwoCups() []model.DailyRecord {
	return []model.DailyRecord{
		{Date: "2026-06-15", Cups: 2}, {Date: "2026-06-16", Cups: 2},
		{Date: "2026-06-17", Cups: 2}, {Date: "2026-06-18", Cups: 2},
		{Date: "2026-06-19", Cups: 2}, {Date: "2026-06-20", Cups: 2},
		{Date: "2026-06-21", Cups: 2},
	}
}

func TestComputeBeansOKAndBags(t *testing.T) {
	// 過去7日で14杯 -> 平均2杯/日 -> 1日24g -> 300g で12.5日 -> OK
	d := model.Data{
		Beans: model.Beans{RemainingGrams: 300},
		Usage: model.Usage{DailyRecords: sevenDaysTwoCups(), TotalShots: 14},
	}
	s := Compute(d, cfg(), "2026-06-21")
	if s.BeansLevel != "OK" {
		t.Errorf("beans_level: got %q, want OK", s.BeansLevel)
	}
	if s.RemainingBags != 1.5 { // 300/200
		t.Errorf("remaining_bags: got %v, want 1.5", s.RemainingBags)
	}
	if s.PredictedDays < 12 || s.PredictedDays > 13 { // ~12.5
		t.Errorf("predicted_days: got %v, want ~12.5", s.PredictedDays)
	}
	if s.AvgCupsPerDay != 2 {
		t.Errorf("avg_cups: got %v, want 2", s.AvgCupsPerDay)
	}
	if s.UpdatedAt != "2026-06-21" {
		t.Errorf("updated_at: got %q", s.UpdatedAt)
	}
}

func TestComputeBeansCritical(t *testing.T) {
	d := model.Data{Beans: model.Beans{RemainingGrams: 48}} // 履歴なし -> fallback2 -> 48/24=2日
	s := Compute(d, cfg(), "2026-06-21")
	if s.BeansLevel != "CRITICAL" {
		t.Errorf("beans_level: got %q, want CRITICAL", s.BeansLevel)
	}
}

func TestComputeMaintenanceElapsed(t *testing.T) {
	d := model.Data{
		Maintenance: model.Maintenance{
			Descaling: model.MaintenanceState{LastDate: "2026-05-01", LastShots: 5},
			Grinder:   model.MaintenanceState{LastDate: "", LastShots: 0},
		},
		Usage: model.Usage{TotalShots: 50},
	}
	s := Compute(d, cfg(), "2026-06-01") // 31日経過
	if s.DescalingLevel != "DUE" {       // 31日 > 30
		t.Errorf("descaling_level: got %q, want DUE", s.DescalingLevel)
	}
	if s.DescalingDaysElapsed != 31 {
		t.Errorf("descaling_days: got %v, want 31", s.DescalingDaysElapsed)
	}
	if s.DescalingShotsElapsed != 45 { // 50-5
		t.Errorf("descaling_shots: got %v, want 45", s.DescalingShotsElapsed)
	}
	if s.GrinderLevel != "" { // 未設定
		t.Errorf("grinder_level: got %q, want empty", s.GrinderLevel)
	}
	if s.GrinderDaysElapsed != 0 {
		t.Errorf("grinder_days: got %v, want 0", s.GrinderDaysElapsed)
	}
	if s.GrinderShotsElapsed != 50 { // 50-0
		t.Errorf("grinder_shots: got %v, want 50", s.GrinderShotsElapsed)
	}
}

func TestComputeBagGramsZeroAvoidsDivByZero(t *testing.T) {
	c := cfg()
	c.BagGrams = 0
	d := model.Data{Beans: model.Beans{RemainingGrams: 300}}
	s := Compute(d, c, "2026-06-21")
	if s.RemainingBags != 0 { // 0除算回避で0
		t.Errorf("remaining_bags: got %v, want 0", s.RemainingBags)
	}
}
