// Package model は bean-watcher のデータ構造を定義する。
package model

// Config は data/config.json の設定。手動編集を想定。
type Config struct {
	GramsPerCup           int              `json:"grams_per_cup"`
	LowDaysThreshold      int              `json:"low_days_threshold"`
	CriticalDaysThreshold int              `json:"critical_days_threshold"`
	AvgWindowDays         int              `json:"avg_window_days"`
	FallbackCupsPerDay    float64          `json:"fallback_cups_per_day"`
	Maintenance           MaintenanceConfig `json:"maintenance"`
}

// MaintenanceConfig は各掃除種別のしきい値。
type MaintenanceConfig struct {
	Descaling Threshold `json:"descaling"`
	Grinder   Threshold `json:"grinder"`
}

// Threshold は日数か抽出回数のどちらか超過で DUE。
type Threshold struct {
	ThresholdDays  int `json:"threshold_days"`
	ThresholdShots int `json:"threshold_shots"`
}

// Data は data/data.json の状態。自動更新される。
type Data struct {
	Beans       Beans       `json:"beans"`
	Usage       Usage       `json:"usage"`
	Maintenance Maintenance `json:"maintenance"`
	NotifyState NotifyState `json:"notify_state"`
}

// Beans は豆の残量。
type Beans struct {
	RemainingGrams float64 `json:"remaining_grams"`
}

// Usage は抽出の利用履歴。
type Usage struct {
	TotalShots   int           `json:"total_shots"`
	DailyRecords []DailyRecord `json:"daily_records"`
}

// DailyRecord は1日あたりの杯数。Date は YYYY-MM-DD。
type DailyRecord struct {
	Date string `json:"date"`
	Cups int    `json:"cups"`
}

// Maintenance は各掃除種別の最終実施状態。
type Maintenance struct {
	Descaling MaintenanceState `json:"descaling"`
	Grinder   MaintenanceState `json:"grinder"`
}

// MaintenanceState は前回掃除の基準点。LastDate が空は未設定。
type MaintenanceState struct {
	LastDate  string `json:"last_date"`
	LastShots int    `json:"last_shots"`
}

// NotifyState は前回通知したレベル。変化検知に使用。
type NotifyState struct {
	Beans     string `json:"beans"`
	Descaling string `json:"descaling"`
	Grinder   string `json:"grinder"`
}
