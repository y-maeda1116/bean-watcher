// Package record は「淹れた/掃除した/購入した」の記録と履歴の整理を行う純粋関数を提供する。
// いずれも引数の Data を破壊せず、新しい Data を返す（イミュータブル）。
package record

import (
	"fmt"

	"bean-watcher/internal/dateutil"
	"bean-watcher/internal/level"
	"bean-watcher/internal/model"
)

// PourCoffee は cups 杯の抽出を記録する。残量を減算（0未満クリップ）し、
// 総抽出回数と当日の daily_records を更新する。
func PourCoffee(d model.Data, cfg model.Config, date string, cups int) (model.Data, error) {
	if cups <= 0 {
		return model.Data{}, fmt.Errorf("cups must be positive, got %d", cups)
	}
	consume := float64(cups * cfg.GramsPerCup)
	remaining := d.Beans.RemainingGrams - consume
	if remaining < 0 {
		remaining = 0
	}
	updated := updateOrAppend(d.Usage.DailyRecords, date, cups)
	return model.Data{
		Beans: model.Beans{RemainingGrams: remaining},
		Usage: model.Usage{
			TotalShots:   d.Usage.TotalShots + cups,
			DailyRecords: updated,
		},
		Maintenance: d.Maintenance,
		NotifyState: d.NotifyState,
	}, nil
}

// Clean は target(descaling/grinder) の掃除を記録し、基準点と notify_state をリセットする。
func Clean(d model.Data, date, target string) (model.Data, error) {
	ms := d.Maintenance
	ns := d.NotifyState
	switch target {
	case "descaling":
		ms.Descaling = model.MaintenanceState{LastDate: date, LastShots: d.Usage.TotalShots}
		ns.Descaling = level.MaintOK
	case "grinder":
		ms.Grinder = model.MaintenanceState{LastDate: date, LastShots: d.Usage.TotalShots}
		ns.Grinder = level.MaintOK
	default:
		return model.Data{}, fmt.Errorf("unknown maintenance target: %s", target)
	}
	return model.Data{
		Beans:       d.Beans,
		Usage:       d.Usage,
		Maintenance: ms,
		NotifyState: ns,
	}, nil
}

// AddBeans は grams の豆購入を記録し、残量を加算して豆レベルを再計算する。
func AddBeans(d model.Data, cfg model.Config, date string, grams int) (model.Data, error) {
	if grams <= 0 {
		return model.Data{}, fmt.Errorf("grams must be positive, got %d", grams)
	}
	ns := d.NotifyState
	next := model.Data{
		Beans:       model.Beans{RemainingGrams: d.Beans.RemainingGrams + float64(grams)},
		Usage:       d.Usage,
		Maintenance: d.Maintenance,
		NotifyState: ns,
	}
	ns.Beans = level.Beans(next, cfg, date)
	next.NotifyState = ns
	return next, nil
}

// Prune は今日を含む過去 windowDays 日より古い daily_records を取り除く。
func Prune(d model.Data, windowDays int, today string) model.Data {
	start, err := dateutil.WindowStart(today, windowDays)
	if err != nil {
		return d
	}
	kept := make([]model.DailyRecord, 0, len(d.Usage.DailyRecords))
	for _, r := range d.Usage.DailyRecords {
		if r.Date >= start {
			kept = append(kept, r)
		}
	}
	return model.Data{
		Beans:       d.Beans,
		Usage:       model.Usage{TotalShots: d.Usage.TotalShots, DailyRecords: kept},
		Maintenance: d.Maintenance,
		NotifyState: d.NotifyState,
	}
}

// updateOrAppend は同日レコードがあれば cups を合算、なければ追記する。
// 元のスライスは破壊しない（新しいスライスを返す）。
func updateOrAppend(records []model.DailyRecord, date string, cups int) []model.DailyRecord {
	out := make([]model.DailyRecord, len(records))
	copy(out, records)
	for i, r := range out {
		if r.Date == date {
			out[i].Cups = r.Cups + cups
			return out
		}
	}
	return append(out, model.DailyRecord{Date: date, Cups: cups})
}
