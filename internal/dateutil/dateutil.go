// Package dateutil は YYYY-MM-DD 形式の日付演算ヘルパを提供する。
package dateutil

import "time"

// Layout は日付文字列の書式。
const Layout = "2006-01-02"

// MinusDays は date から n 日前の日付文字列を返す。
func MinusDays(date string, n int) (string, error) {
	t, err := time.Parse(Layout, date)
	if err != nil {
		return "", err
	}
	return t.AddDate(0, 0, -n).Format(Layout), nil
}

// DaysBetween は from と to の日数差（to - from）を返す。
func DaysBetween(from, to string) (int, error) {
	tf, err := time.Parse(Layout, from)
	if err != nil {
		return 0, err
	}
	tt, err := time.Parse(Layout, to)
	if err != nil {
		return 0, err
	}
	return int(tt.Sub(tf).Hours() / 24), nil
}

// WindowStart は今日を含む過去 windowDays 日の開始日（最も古い日）を返す。
// 例: today=2026-06-21, windowDays=7 -> 2026-06-15
func WindowStart(today string, windowDays int) (string, error) {
	if windowDays < 1 {
		windowDays = 1
	}
	return MinusDays(today, windowDays-1)
}
