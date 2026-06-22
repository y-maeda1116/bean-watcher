// Package summary はウェブダッシュボード表示用のサマリを計算する純粋関数を提供する。
// Go 側で計算して data.json に保存し、JavaScript は表示のみ行う（ロジック重複を避ける）。
package summary

import (
	"bean-watcher/internal/dateutil"
	"bean-watcher/internal/level"
	"bean-watcher/internal/model"
)

// Compute は data と config から表示用サマリを計算する。
func Compute(d model.Data, cfg model.Config, today string) model.Summary {
	avg := level.AvgCupsPerDay(d, cfg, today)
	gramsPerDay := float64(cfg.GramsPerCup) * avg
	if gramsPerDay <= 0 {
		gramsPerDay = float64(cfg.GramsPerCup) * cfg.FallbackCupsPerDay
	}
	if gramsPerDay < 0.001 {
		gramsPerDay = 0.001 // 0除算回避
	}
	predictedDays := d.Beans.RemainingGrams / gramsPerDay

	bags := 0.0
	if cfg.BagGrams > 0 {
		bags = d.Beans.RemainingGrams / float64(cfg.BagGrams)
	}

	return model.Summary{
		RemainingGrams:        d.Beans.RemainingGrams,
		RemainingBags:         bags,
		BeansLevel:            level.Beans(d, cfg, today),
		PredictedDays:         predictedDays,
		AvgCupsPerDay:         avg,
		DescalingLevel:        level.Maintenance(d.Maintenance.Descaling, cfg.Maintenance.Descaling, d.Usage.TotalShots, today),
		DescalingDaysElapsed:  daysElapsed(d.Maintenance.Descaling.LastDate, today),
		DescalingShotsElapsed: d.Usage.TotalShots - d.Maintenance.Descaling.LastShots,
		GrinderLevel:          level.Maintenance(d.Maintenance.Grinder, cfg.Maintenance.Grinder, d.Usage.TotalShots, today),
		GrinderDaysElapsed:    daysElapsed(d.Maintenance.Grinder.LastDate, today),
		GrinderShotsElapsed:   d.Usage.TotalShots - d.Maintenance.Grinder.LastShots,
		UpdatedAt:             today,
	}
}

// daysElapsed は lastDate から today までの日数を返す。
// lastDate が空（未設定）や不正な場合は 0 を返す。
func daysElapsed(lastDate, today string) int {
	if lastDate == "" {
		return 0
	}
	days, err := dateutil.DaysBetween(lastDate, today)
	if err != nil || days < 0 {
		return 0
	}
	return days
}
