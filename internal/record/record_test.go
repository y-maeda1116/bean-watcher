package record

import (
	"testing"

	"bean-watcher/internal/model"
)

func cfg() model.Config {
	return model.Config{GramsPerCup: 12, AvgWindowDays: 7, FallbackCupsPerDay: 2, LowDaysThreshold: 5, BagGrams: 200}
}

func baseData() model.Data {
	return model.Data{
		Beans:       model.Beans{RemainingGrams: 300},
		Usage:       model.Usage{TotalShots: 10, DailyRecords: []model.DailyRecord{{Date: "2026-06-20", Cups: 1}}},
		Maintenance: model.Maintenance{Descaling: model.MaintenanceState{LastDate: "2026-06-01", LastShots: 0}},
		NotifyState: model.NotifyState{Beans: "OK", Descaling: "OK", Grinder: "OK"},
	}
}

func TestPourCoffeeReducesBeansAndShots(t *testing.T) {
	d := baseData()
	got, err := PourCoffee(d, cfg(), "2026-06-21", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Beans.RemainingGrams != 276 { // 300 - 2*12
		t.Errorf("grams: got %v, want 276", got.Beans.RemainingGrams)
	}
	if got.Usage.TotalShots != 12 {
		t.Errorf("shots: got %v, want 12", got.Usage.TotalShots)
	}
	// 当日レコード追加
	if len(got.Usage.DailyRecords) != 2 {
		t.Errorf("records len: got %v, want 2", len(got.Usage.DailyRecords))
	}
	// 元データ不変（イミュータブル）
	if d.Beans.RemainingGrams != 300 {
		t.Error("source data was mutated")
	}
}

func TestPourCoffeeSameDayAccumulates(t *testing.T) {
	d := baseData()
	got, err := PourCoffee(d, cfg(), "2026-06-20", 3) // 6/20 は既存1杯
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Usage.DailyRecords) != 1 { // 同日なので1レコードに合算
		t.Fatalf("records len: got %v, want 1", len(got.Usage.DailyRecords))
	}
	if got.Usage.DailyRecords[0].Cups != 4 { // 1 + 3
		t.Errorf("cups: got %v, want 4", got.Usage.DailyRecords[0].Cups)
	}
}

func TestPourCoffeeClampsToZero(t *testing.T) {
	d := baseData()
	d.Beans.RemainingGrams = 10
	got, err := PourCoffee(d, cfg(), "2026-06-21", 5) // 消費60g
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Beans.RemainingGrams != 0 {
		t.Errorf("grams: got %v, want 0 (clamped)", got.Beans.RemainingGrams)
	}
}

func TestPourCoffeeRejectsNonPositive(t *testing.T) {
	if _, err := PourCoffee(baseData(), cfg(), "2026-06-21", 0); err == nil {
		t.Error("expected error for cups=0")
	}
}

func TestCleanResetsDescaling(t *testing.T) {
	d := baseData()
	d.NotifyState.Descaling = "DUE"
	got, err := Clean(d, "2026-06-21", "descaling")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Maintenance.Descaling.LastDate != "2026-06-21" {
		t.Errorf("last_date: got %q", got.Maintenance.Descaling.LastDate)
	}
	if got.Maintenance.Descaling.LastShots != 10 {
		t.Errorf("last_shots: got %v, want 10", got.Maintenance.Descaling.LastShots)
	}
	if got.NotifyState.Descaling != "OK" {
		t.Errorf("notify descaling: got %q, want OK", got.NotifyState.Descaling)
	}
}

func TestCleanRejectsUnknownTarget(t *testing.T) {
	if _, err := Clean(baseData(), "2026-06-21", "unknown"); err == nil {
		t.Error("expected error for unknown target")
	}
}

func TestAddBagsIncreasesAndAppendsPurchase(t *testing.T) {
	d := baseData()
	d.Beans.RemainingGrams = 24
	got, err := AddBags(d, cfg(), "2026-06-21", 2) // 2袋 x 200g = 400g
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Beans.RemainingGrams != 424 { // 24 + 400
		t.Errorf("grams: got %v, want 424", got.Beans.RemainingGrams)
	}
	if got.NotifyState.Beans != "OK" { // 424g は十分
		t.Errorf("beans level: got %q, want OK", got.NotifyState.Beans)
	}
	if len(got.Purchases) != 1 || got.Purchases[0].Bags != 2 || got.Purchases[0].Grams != 400 {
		t.Errorf("purchase: got %+v", got.Purchases)
	}
	// 元データ不変（イミュータブル）
	if len(d.Purchases) != 0 {
		t.Error("source purchases was mutated")
	}
}

func TestAddBagsRejectsNonPositive(t *testing.T) {
	if _, err := AddBags(baseData(), cfg(), "2026-06-21", 0); err == nil {
		t.Error("expected error for bags=0")
	}
}

func TestPruneRemovesOldRecords(t *testing.T) {
	d := model.Data{Usage: model.Usage{DailyRecords: []model.DailyRecord{
		{Date: "2026-06-01", Cups: 1}, // 古い
		{Date: "2026-06-20", Cups: 1}, // ウィンドウ内（7日: 6/15〜6/21）
	}}}
	got := Prune(d, 7, "2026-06-21")
	if len(got.Usage.DailyRecords) != 1 {
		t.Errorf("records len: got %v, want 1", len(got.Usage.DailyRecords))
	}
	if got.Usage.DailyRecords[0].Date != "2026-06-20" {
		t.Errorf("kept date: got %q", got.Usage.DailyRecords[0].Date)
	}
}
