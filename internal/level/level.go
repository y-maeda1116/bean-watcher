// Package level は豆残量とメンテナンス時期のレベルを計算する純粋関数を提供する。
package level

import (
	"bean-watcher/internal/dateutil"
	"bean-watcher/internal/model"
)

// レベル文字列。
const (
	BeanOK       = "OK"
	BeanLOW      = "LOW"
	BeanCRITICAL = "CRITICAL"
	MaintOK      = "OK"
	MaintDUE     = "DUE"
)

// Beans は予測残り日数から豆のレベル(OK/LOW/CRITICAL)を返す。
func Beans(d model.Data, cfg model.Config, today string) string {
	avg := avgCupsPerDay(d.Usage.DailyRecords, cfg, today)
	gramsPerDay := float64(cfg.GramsPerCup) * avg
	if gramsPerDay <= 0 {
		gramsPerDay = float64(cfg.GramsPerCup) * cfg.FallbackCupsPerDay
	}
	if gramsPerDay < 0.001 {
		gramsPerDay = 0.001 // 0除算回避
	}
	remainingDays := d.Beans.RemainingGrams / gramsPerDay
	if remainingDays <= float64(cfg.CriticalDaysThreshold) {
		return BeanCRITICAL
	}
	if remainingDays <= float64(cfg.LowDaysThreshold) {
		return BeanLOW
	}
	return BeanOK
}

// avgCupsPerDay は今日を含む過去 windowDays 日の平均杯数を返す。
// 記録が1件もなければ FallbackCupsPerDay を返す。
func avgCupsPerDay(records []model.DailyRecord, cfg model.Config, today string) float64 {
	start, err := dateutil.WindowStart(today, cfg.AvgWindowDays)
	if err != nil {
		return cfg.FallbackCupsPerDay
	}
	total := 0
	hasData := false
	for _, r := range records {
		if r.Date >= start && r.Date <= today {
			total += r.Cups
			hasData = true
		}
	}
	if !hasData {
		return cfg.FallbackCupsPerDay
	}
	return float64(total) / float64(cfg.AvgWindowDays)
}

// Maintenance は前回掃除基準点としきい値からレベル(OK/DUE)を返す。
// last_date が空（未設定）の時は空文字を返し、判定をスキップする。
func Maintenance(state model.MaintenanceState, th model.Threshold, totalShots int, today string) string {
	if state.LastDate == "" {
		return ""
	}
	days, err := dateutil.DaysBetween(state.LastDate, today)
	if err == nil && days > th.ThresholdDays {
		return MaintDUE
	}
	if (totalShots - state.LastShots) > th.ThresholdShots {
		return MaintDUE
	}
	return MaintOK
}
