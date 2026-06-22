package notify

import (
	"strings"
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

func TestCurrentLevels(t *testing.T) {
	d := model.Data{
		Beans: model.Beans{RemainingGrams: 48}, // 履歴なし->fallback2->48/24=2日 -> CRITICAL
		Maintenance: model.Maintenance{
			Descaling: model.MaintenanceState{LastDate: "2026-05-01", LastShots: 0}, // 日数超過 -> DUE
			Grinder:   model.MaintenanceState{LastDate: "2026-06-20", LastShots: 0}, // OK
		},
	}
	lv := CurrentLevels(d, cfg(), "2026-06-21")
	if lv.Beans != "CRITICAL" {
		t.Errorf("beans: got %q, want CRITICAL", lv.Beans)
	}
	if lv.Descaling != "DUE" {
		t.Errorf("descaling: got %q, want DUE", lv.Descaling)
	}
	if lv.Grinder != "OK" {
		t.Errorf("grinder: got %q, want OK", lv.Grinder)
	}
}

func TestDiffOnlyOnLevelUp(t *testing.T) {
	prev := model.NotifyState{Beans: "OK", Descaling: "OK", Grinder: "OK"}
	cur := Levels{Beans: "LOW", Descaling: "DUE", Grinder: "OK"}
	diff := ComputeDiff(prev, cur)
	if !diff.Beans || !diff.Descaling {
		t.Errorf("expected beans+descaling changes, got %+v", diff)
	}
	if diff.Grinder {
		t.Error("grinder should not change")
	}
}

func TestDiffNoChangeWhenSameLevel(t *testing.T) {
	prev := model.NotifyState{Beans: "LOW", Descaling: "DUE", Grinder: "OK"}
	cur := Levels{Beans: "CRITICAL", Descaling: "DUE", Grinder: "OK"}
	diff := ComputeDiff(prev, cur)
	if !diff.Beans {
		t.Error("beans LOW->CRITICAL should change")
	}
	if diff.Descaling {
		t.Error("descaling DUE->DUE should not change")
	}
}

func TestDiffSkipsPendingMaintenance(t *testing.T) {
	prev := model.NotifyState{Beans: "OK", Descaling: "OK", Grinder: "OK"}
	cur := Levels{Beans: "OK", Descaling: "", Grinder: "OK"} // grinder 未設定
	diff := ComputeDiff(prev, cur)
	if diff.Descaling {
		t.Error("pending (empty) maintenance should not change")
	}
}

func TestBuildMessageEmptyWhenNoChange(t *testing.T) {
	d := model.Data{Beans: model.Beans{RemainingGrams: 300}}
	cur := Levels{Beans: "OK", Descaling: "OK", Grinder: "OK"}
	diff := Diff{Beans: false, Descaling: false, Grinder: false}
	if msg := BuildMessage(d, cfg(), cur, diff, "2026-06-21"); msg != "" {
		t.Errorf("expected empty message, got %q", msg)
	}
}

func TestBuildMessageIncludesBeanWarning(t *testing.T) {
	d := model.Data{Beans: model.Beans{RemainingGrams: 48}}
	cur := Levels{Beans: "CRITICAL", Descaling: "OK", Grinder: "OK"}
	diff := Diff{Beans: true, Descaling: false, Grinder: false}
	msg := BuildMessage(d, cfg(), cur, diff, "2026-06-21")
	if !strings.Contains(msg, "豆") {
		t.Errorf("message should mention beans: %q", msg)
	}
	if !strings.Contains(msg, "2日") {
		t.Errorf("message should mention 2 days: %q", msg)
	}
	if !strings.Contains(msg, "袋") {
		t.Errorf("message should mention bags: %q", msg)
	}
}

func TestBuildMessageIncludesMaintenance(t *testing.T) {
	d := model.Data{Maintenance: model.Maintenance{Descaling: model.MaintenanceState{LastDate: "2026-05-01", LastShots: 0}}}
	cur := Levels{Beans: "OK", Descaling: "DUE", Grinder: "OK"}
	diff := Diff{Beans: false, Descaling: true, Grinder: false}
	msg := BuildMessage(d, cfg(), cur, diff, "2026-06-21")
	if !strings.Contains(msg, "クエン酸") {
		t.Errorf("message should mention descaling: %q", msg)
	}
}
