// Package notify は現在レベルの計算、前回通知との変化検知、Discord メッセージの構築を行う。
package notify

import (
	"fmt"
	"strings"

	"bean-watcher/internal/level"
	"bean-watcher/internal/model"
)

// Levels は3項目の現在レベル。
type Levels struct {
	Beans     string
	Descaling string
	Grinder   string
}

// Diff は各項目が「前回より厳しい方向に変わった」か。
type Diff struct {
	Beans     bool
	Descaling bool
	Grinder   bool
}

var beanOrder = map[string]int{
	level.BeanOK: 0, level.BeanLOW: 1, level.BeanCRITICAL: 2,
}

var maintOrder = map[string]int{
	level.MaintOK: 0, level.MaintDUE: 1, "": -1, // 空は未設定で最も低い（変化させない）
}

// CurrentLevels は data と config から現在の3レベルを計算する。
func CurrentLevels(d model.Data, cfg model.Config, today string) Levels {
	return Levels{
		Beans:     level.Beans(d, cfg, today),
		Descaling: level.Maintenance(d.Maintenance.Descaling, cfg.Maintenance.Descaling, d.Usage.TotalShots, today),
		Grinder:   level.Maintenance(d.Maintenance.Grinder, cfg.Maintenance.Grinder, d.Usage.TotalShots, today),
	}
}

// ComputeDiff は前回通知レベルと現在レベルを比較し、厳しい方向への変化を検知する。
// メンテナンスの現在レベルが空（未設定）の場合は変化なし扱い。
//
// 注: プランでは関数名も Diff だったが、同名の Diff 型と衝突するため
// ComputeDiff にリネームしている。
func ComputeDiff(prev model.NotifyState, cur Levels) Diff {
	return Diff{
		Beans:     orderUp(beanOrder, prev.Beans, cur.Beans),
		Descaling: maintChange(prev.Descaling, cur.Descaling),
		Grinder:   maintChange(prev.Grinder, cur.Grinder),
	}
}

func orderUp(order map[string]int, prev, cur string) bool {
	return order[cur] > order[prev]
}

func maintChange(prev, cur string) bool {
	if cur == "" {
		return false
	}
	return maintOrder[cur] > maintOrder[prev]
}

// BuildMessage は変化した項目から Discord 送信用メッセージを構築する。
// 変化がない時は空文字を返す（呼び出し側は送信しない）。
func BuildMessage(d model.Data, cfg model.Config, cur Levels, diff Diff, today string) string {
	if !diff.Beans && !diff.Descaling && !diff.Grinder {
		return ""
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("🔔 **bean-watcher お知らせ** (%s)", today))
	if diff.Beans {
		lines = append(lines, beanLine(d, cfg, cur.Beans, today))
	}
	if diff.Descaling {
		lines = append(lines, "🧽 **掃除タイミング**: クエン酸洗浄の目安です")
	}
	if diff.Grinder {
		lines = append(lines, "🧽 **掃除タイミング**: ミル掃除の目安です")
	}
	return strings.Join(lines, "\n")
}

// beanLine は豆の警告行を生成する。
func beanLine(d model.Data, cfg model.Config, lvl, today string) string {
	avg := level.AvgCupsPerDay(d, cfg, today)
	gramsPerDay := float64(cfg.GramsPerCup) * avg
	if gramsPerDay < 0.001 {
		gramsPerDay = 0.001
	}
	days := d.Beans.RemainingGrams / gramsPerDay
	grams := int(d.Beans.RemainingGrams)
	bags := 0.0
	if cfg.BagGrams > 0 {
		bags = d.Beans.RemainingGrams / float64(cfg.BagGrams)
	}
	switch lvl {
	case level.BeanCRITICAL:
		return fmt.Sprintf("🚨 **豆がもうすぐ切れます**: あと約%.0f日（残り %dg / %.1f袋）。早めの補充をお願いします！", days, grams, bags)
	default: // LOW
		return fmt.Sprintf("☕ **豆の買い時**: あと約%.0f日でなくなりそうです（残り %dg / %.1f袋）。週末に焙煎・購入を！", days, grams, bags)
	}
}
