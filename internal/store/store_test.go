package store

import (
	"os"
	"path/filepath"
	"testing"

	"bean-watcher/internal/model"
)

func TestDataSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	src := model.Data{
		Beans:       model.Beans{RemainingGrams: 120.5},
		Usage:       model.Usage{TotalShots: 9, DailyRecords: []model.DailyRecord{{Date: "2026-06-20", Cups: 2}}},
		Maintenance: model.Maintenance{Descaling: model.MaintenanceState{LastDate: "2026-06-01", LastShots: 1}},
		NotifyState: model.NotifyState{Beans: "LOW", Descaling: "OK", Grinder: "OK"},
	}
	if err := SaveData(path, src); err != nil {
		t.Fatalf("SaveData: %v", err)
	}
	got, err := LoadData(path)
	if err != nil {
		t.Fatalf("LoadData: %v", err)
	}
	if got.Beans.RemainingGrams != 120.5 {
		t.Errorf("remaining: got %v, want 120.5", got.Beans.RemainingGrams)
	}
	if got.NotifyState.Beans != "LOW" {
		t.Errorf("beans level: got %q, want LOW", got.NotifyState.Beans)
	}
}

func TestLoadDataInvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadData(path); err == nil {
		t.Error("expected error for invalid json")
	}
}

func TestLoadConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	src := model.Config{
		GramsPerCup:           12,
		LowDaysThreshold:      5,
		CriticalDaysThreshold: 2,
		AvgWindowDays:         7,
		FallbackCupsPerDay:    2,
		Maintenance: model.MaintenanceConfig{
			Descaling: model.Threshold{ThresholdDays: 30, ThresholdShots: 200},
			Grinder:   model.Threshold{ThresholdDays: 7, ThresholdShots: 30},
		},
	}
	if err := SaveConfig(path, src); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got.GramsPerCup != 12 || got.Maintenance.Descaling.ThresholdShots != 200 {
		t.Errorf("config round trip mismatch: %+v", got)
	}
}
