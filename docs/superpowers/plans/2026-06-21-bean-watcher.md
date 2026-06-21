# bean-watcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Go 製のバッチツールで、GitHub Actions の定期実行 + リポジトリ内JSON により、コーヒー豆の残量予測とメンテナンス時期を Discord 通知する完全無料運用ツールを構築する。

**Architecture:** 1つの Go バイナリが `record`/`notify` サブコマンドを持つ。`record` は手動実行（workflow_dispatch）で `data/data.json` を更新・コミット。`notify` は毎朝 cron でレベルを計算し、変化があれば Discord Webhook に通知して `notify_state` をコミット。ドメインロジックはイミュータブルな純粋関数として `internal/` 配下の小さなパッケージに分割し、時刻は `clock` で抽象化してテスト容易性を確保する。

**Tech Stack:** Go 1.21+（標準ライブラリのみ、外部依存ゼロ）、GitHub Actions（`setup-go`）、Discord Webhook、JSON ファイルストア。

---

## File Structure

| ファイル | 責務 |
|---|---|
| `go.mod` | モジュール定義（module `bean-watcher`） |
| `.gitignore` | `data/data.json` の誤コミット防止用設定（※ただし本プロジェクトでは `data.json` はgit管理するため、対象外。`.serena/` 等のみ無視） |
| `internal/model/types.go` | Config/Data 各構造体と JSON タグ |
| `internal/clock/clock.go` | `Clock` インターフェース（Real/Fake） |
| `internal/dateutil/dateutil.go` | 日付文字列(YYYY-MM-DD)の演算ヘルパ |
| `internal/store/store.go` | config/data の JSON 読み書き |
| `internal/level/level.go` | 豆・メンテナンスのレベル計算（純粋関数） |
| `internal/record/record.go` | pour/clean/buy/prune（純粋関数、新しいDataを返す） |
| `internal/notify/notify.go` | 現在レベル計算・変化検知・メッセージ構築 |
| `internal/discord/discord.go` | Webhook 送信（net/http） |
| `main.go` | サブコマンド・引数解析・依存結合 |
| `data/config.json` | 設定（手動編集） |
| `data/data.json` | 状態（自動更新・git管理） |
| `.github/workflows/record.yml` | 手動記録ワークフロー |
| `.github/workflows/notify.yml` | 定期通知ワークフロー |
| `.github/workflows/ci.yml` | テストCI |
| `README.md` | セットアップ・運用手順 |

依存グラフ: `main` → {store, record, notify, discord, clock, model}; `record` → {level, dateutil, model}; `notify` → {level, model}; `level` → {dateutil, model}; 循環なし。

---

## Task 1: プロジェクト初期化

**Files:**
- Create: `go.mod`
- Create: `.gitignore`

- [ ] **Step 1: Go モジュール初期化**

Run:
```bash
go mod init bean-watcher
```
Expected: `go: creating new go.mod: module bean-watcher`

- [ ] **Step 2: .gitignore 作成**

Create `.gitignore`:
```
# Serena tooling (local)
.serena/

# Go build artifacts
/bean-watcher
*.exe
```

- [ ] **Step 3: ビルドできる空の main.go を作成（後のタスクで拡張）**

Create `main.go`:
```go
package main

func main() {
}
```

Run:
```bash
go build ./...
```
Expected: エラーなし（出力なし）

- [ ] **Step 4: コミット**

```bash
git add go.mod .gitignore main.go
git commit -m "chore: Go プロジェクト初期化"
```

---

## Task 2: model パッケージ（データ型定義）

**Files:**
- Create: `internal/model/types.go`
- Test: `internal/model/types_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/model/types_test.go`:
```go
package model

import (
	"encoding/json"
	"testing"
)

func TestDataRoundTrip(t *testing.T) {
	src := Data{
		Beans: Beans{RemainingGrams: 300},
		Usage: Usage{
			TotalShots: 5,
			DailyRecords: []DailyRecord{
				{Date: "2026-06-20", Cups: 2},
			},
		},
		Maintenance: Maintenance{
			Descaling: MaintenanceState{LastDate: "2026-06-01", LastShots: 0},
			Grinder:   MaintenanceState{LastDate: "", LastShots: 0},
		},
		NotifyState: NotifyState{Beans: "OK", Descaling: "OK", Grinder: "OK"},
	}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Data
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Beans.RemainingGrams != 300 {
		t.Errorf("remaining: got %v, want 300", got.Beans.RemainingGrams)
	}
	if got.Usage.TotalShots != 5 {
		t.Errorf("shots: got %v, want 5", got.Usage.TotalShots)
	}
	if got.Maintenance.Grinder.LastDate != "" {
		t.Errorf("grinder last_date: got %q, want empty", got.Maintenance.Grinder.LastDate)
	}
	// JSON キー名の確認
	if want := `"remaining_grams"`; !contains(string(raw), want) {
		t.Errorf("json key %s missing in %s", want, string(raw))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/model/...
```
Expected: FAIL（型が未定義のためコンパイルエラー）

- [ ] **Step 3: 実装**

Create `internal/model/types.go`:
```go
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
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/model/...
```
Expected: `ok bean-watcher/internal/model`

- [ ] **Step 5: コミット**

```bash
git add internal/model/
git commit -m "feat: model パッケージのデータ型を追加"
```

---

## Task 3: clock パッケージ（時刻抽象化）

**Files:**
- Create: `internal/clock/clock.go`
- Test: `internal/clock/clock_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/clock/clock_test.go`:
```go
package clock

import (
	"testing"
	"time"
)

func TestFakeReturnsFixedTime(t *testing.T) {
	want := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	c := Fake{T: want}
	if got := c.Now(); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRealReturnsNonZero(t *testing.T) {
	c := Real{}
	if c.Now().IsZero() {
		t.Error("real clock returned zero time")
	}
}
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/clock/...
```
Expected: FAIL（型未定義）

- [ ] **Step 3: 実装**

Create `internal/clock/clock.go`:
```go
// Package clock は現在時刻の取得を抽象化し、テストで固定時刻を注入可能にする。
package clock

import "time"

// Clock は現在時刻を返す。
type Clock interface {
	Now() time.Time
}

// Real は実際の現在時刻を返す。
type Real struct{}

// Now は time.Now() を返す。
func (Real) Now() time.Time { return time.Now() }

// Fake は固定時刻を返す（テスト用）。
type Fake struct {
	T time.Time
}

// Now は設定された時刻を返す。
func (f Fake) Now() time.Time { return f.T }
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/clock/...
```
Expected: `ok bean-watcher/internal/clock`

- [ ] **Step 5: コミット**

```bash
git add internal/clock/
git commit -m "feat: clock パッケージで時刻を抽象化"
```

---

## Task 4: dateutil パッケージ（日付演算）

**Files:**
- Create: `internal/dateutil/dateutil.go`
- Test: `internal/dateutil/dateutil_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/dateutil/dateutil_test.go`:
```go
package dateutil

import "testing"

func TestMinusDays(t *testing.T) {
	cases := []struct {
		date string
		n    int
		want string
	}{
		{"2026-06-21", 6, "2026-06-15"},
		{"2026-06-21", 1, "2026-06-20"},
		{"2026-03-01", 1, "2026-02-28"}, // 月跨ぎ・非閏年
	}
	for _, c := range cases {
		got, err := MinusDays(c.date, c.n)
		if err != nil {
			t.Fatalf("MinusDays(%s,%d) err: %v", c.date, c.n, err)
		}
		if got != c.want {
			t.Errorf("MinusDays(%s,%d) = %s, want %s", c.date, c.n, got, c.want)
		}
	}
}

func TestMinusDaysInvalid(t *testing.T) {
	if _, err := MinusDays("not-a-date", 1); err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestDaysBetween(t *testing.T) {
	got, err := DaysBetween("2026-05-01", "2026-06-01")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 31 {
		t.Errorf("got %d, want 31", got)
	}
}
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/dateutil/...
```
Expected: FAIL（関数未定義）

- [ ] **Step 3: 実装**

Create `internal/dateutil/dateutil.go`:
```go
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
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/dateutil/...
```
Expected: `ok bean-watcher/internal/dateutil`

- [ ] **Step 5: コミット**

```bash
git add internal/dateutil/
git commit -m "feat: dateutil パッケージで日付演算を追加"
```

---

## Task 5: store パッケージ（JSON 読み書き）

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/store/store_test.go`:
```go
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
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/store/...
```
Expected: FAIL（関数未定義）

- [ ] **Step 3: 実装**

Create `internal/store/store.go`:
```go
// Package store は config/data の JSON ファイル読み書きを行う。
package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"bean-watcher/internal/model"
)

// LoadConfig は config.json を読み込む。
func LoadConfig(path string) (model.Config, error) {
	var c model.Config
	if err := readJSON(path, &c); err != nil {
		return model.Config{}, err
	}
	return c, nil
}

// SaveConfig は config.json を書き込む（テスト・初期生成用）。
func SaveConfig(path string, c model.Config) error {
	return writeJSON(path, c)
}

// LoadData は data.json を読み込む。
func LoadData(path string) (model.Data, error) {
	var d model.Data
	if err := readJSON(path, &d); err != nil {
		return model.Data{}, err
	}
	return d, nil
}

// SaveData は data.json を書き込む（インデント付き）。
func SaveData(path string, d model.Data) error {
	return writeJSON(path, d)
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/store/...
```
Expected: `ok bean-watcher/internal/store`

- [ ] **Step 5: コミット**

```bash
git add internal/store/
git commit -m "feat: store パッケージでJSON読み書きを追加"
```

---

## Task 6: level パッケージ（レベル計算）

**Files:**
- Create: `internal/level/level.go`
- Test: `internal/level/level_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/level/level_test.go`:
```go
package level

import (
	"testing"

	"bean-watcher/internal/model"
)

func baseConfig() model.Config {
	return model.Config{
		GramsPerCup:           12,
		LowDaysThreshold:      5,
		CriticalDaysThreshold: 2,
		AvgWindowDays:         7,
		FallbackCupsPerDay:    2,
	}
}

func TestBeansLevel(t *testing.T) {
	cfg := baseConfig()
	// 過去7日で計14杯 -> 平均2杯/日 -> 1日24g消費
	records := []model.DailyRecord{
		{Date: "2026-06-15", Cups: 2}, {Date: "2026-06-16", Cups: 2},
		{Date: "2026-06-17", Cups: 2}, {Date: "2026-06-18", Cups: 2},
		{Date: "2026-06-19", Cups: 2}, {Date: "2026-06-20", Cups: 2},
		{Date: "2026-06-21", Cups: 2},
	}
	cases := []struct {
		name     string
		grams    float64
		records  []model.DailyRecord
		want     string
	}{
		{"OK_残り十分", 300, records, "OK"},     // 300/24 = 12.5日
		{"LOW_境界5日", 120, records, "LOW"},    // 120/24 = 5.0日
		{"CRITICAL_境界2日", 48, records, "CRITICAL"}, // 48/24 = 2.0日
		{"CRITICAL_残量0", 0, records, "CRITICAL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := model.Data{Beans: model.Beans{RemainingGrams: c.grams}, Usage: model.Usage{DailyRecords: c.records}}
			got := Beans(d, cfg, "2026-06-21")
			if got != c.want {
				t.Errorf("Beans(grams=%v) = %s, want %s", c.grams, got, c.want)
			}
		})
	}
}

func TestBeansLevelFallback(t *testing.T) {
	cfg := baseConfig()
	// 履歴なし -> fallback 2杯/日 -> 1日24g
	d := model.Data{Beans: model.Beans{RemainingGrams: 120}}
	if got := Beans(d, cfg, "2026-06-21"); got != "LOW" {
		t.Errorf("fallback: got %s, want LOW", got)
	}
}

func TestBeansLevelAboveFiveDaysIsOK(t *testing.T) {
	cfg := baseConfig()
	records := []model.DailyRecord{{Date: "2026-06-21", Cups: 2}} // 1件 -> 2/7 cup/day
	d := model.Data{Beans: model.Beans{RemainingGrams: 300}}
	// 1日平均 = 2/7 杯 -> 1日 g = 12 * 2/7 = 3.428g -> 300/3.428 = 87.5日 -> OK
	if got := Beans(d, cfg, "2026-06-21"); got != "OK" {
		t.Errorf("got %s, want OK", got)
	}
}

func TestMaintenanceLevel(t *testing.T) {
	th := model.Threshold{ThresholdDays: 30, ThresholdShots: 200}
	cases := []struct {
		name   string
		state  model.MaintenanceState
		shots  int
		today  string
		want   string
	}{
		{"未設定は空", model.MaintenanceState{LastDate: "", LastShots: 0}, 100, "2026-06-21", ""},
		{"日数超過でDUE", model.MaintenanceState{LastDate: "2026-05-01", LastShots: 0}, 10, "2026-06-01", "DUE"}, // 31日
		{"回数超過でDUE", model.MaintenanceState{LastDate: "2026-06-20", LastShots: 0}, 250, "2026-06-21", "DUE"},
		{"どちらも未超過でOK", model.MaintenanceState{LastDate: "2026-06-20", LastShots: 0}, 10, "2026-06-21", "OK"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Maintenance(c.state, th, c.shots, c.today)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/level/...
```
Expected: FAIL（関数未定義）

- [ ] **Step 3: 実装**

Create `internal/level/level.go`:
```go
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
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/level/...
```
Expected: `ok bean-watcher/internal/level`

- [ ] **Step 5: コミット**

```bash
git add internal/level/
git commit -m "feat: level パッケージでレベル計算を追加"
```

---

## Task 7: record パッケージ（記録ロジック）

**Files:**
- Create: `internal/record/record.go`
- Test: `internal/record/record_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/record/record_test.go`:
```go
package record

import (
	"testing"

	"bean-watcher/internal/model"
)

func cfg() model.Config {
	return model.Config{GramsPerCup: 12, AvgWindowDays: 7, FallbackCupsPerDay: 2, LowDaysThreshold: 5}
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

func TestAddBeansIncreasesRemainingAndRecomputesLevel(t *testing.T) {
	d := baseData()
	d.Beans.RemainingGrams = 24 // 少量 -> LOW 相当を想定して増やす
	got, err := AddBeans(d, cfg(), "2026-06-21", 300)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Beans.RemainingGrams != 324 {
		t.Errorf("grams: got %v, want 324", got.Beans.RemainingGrams)
	}
	if got.NotifyState.Beans != "OK" { // 324g は十分
		t.Errorf("beans level: got %q, want OK", got.NotifyState.Beans)
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
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/record/...
```
Expected: FAIL（関数未定義）

- [ ] **Step 3: 実装**

Create `internal/record/record.go`:
```go
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
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/record/...
```
Expected: `ok bean-watcher/internal/record`

- [ ] **Step 5: コミット**

```bash
git add internal/record/
git commit -m "feat: record パッケージで記録ロジックを追加"
```

---

## Task 8: notify パッケージ（変化検知・メッセージ）

**Files:**
- Create: `internal/notify/notify.go`
- Test: `internal/notify/notify_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/notify/notify_test.go`:
```go
package notify

import (
	"strings"
	"testing"

	"bean-watcher/internal/model"
)

func cfg() model.Config {
	return model.Config{
		GramsPerCup: 12, LowDaysThreshold: 5, CriticalDaysThreshold: 2,
		AvgWindowDays: 7, FallbackCupsPerDay: 2,
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
	diff := Diff(prev, cur)
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
	diff := Diff(prev, cur)
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
	diff := Diff(prev, cur)
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
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/notify/...
```
Expected: FAIL（関数未定義）

- [ ] **Step 3: 実装**

Create `internal/notify/notify.go`:
```go
// Package notify は現在レベルの計算、前回通知との変化検知、Discord メッセージの構築を行う。
package notify

import (
	"fmt"
	"strings"

	"bean-watcher/internal/dateutil"
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

// Diff は前回通知レベルと現在レベルを比較し、厳しい方向への変化を検知する。
// メンテナンスの現在レベルが空（未設定）の場合は変化なし扱い。
func Diff(prev model.NotifyState, cur Levels) Diff {
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
	avg := avgCups(d, cfg, today)
	gramsPerDay := float64(cfg.GramsPerCup) * avg
	if gramsPerDay < 0.001 {
		gramsPerDay = 0.001
	}
	days := d.Beans.RemainingGrams / gramsPerDay
	grams := int(d.Beans.RemainingGrams)
	switch lvl {
	case level.BeanCRITICAL:
		return fmt.Sprintf("🚨 **豆がもうすぐ切れます**: あと約%.0f日（残り %dg）。早めの補充をお願いします！", days, grams)
	default: // LOW
		return fmt.Sprintf("☕ **豆の買い時**: あと約%.0f日でなくなりそうです（残り %dg）。週末に焙煎・購入を！", days, grams)
	}
}

func avgCups(d model.Data, cfg model.Config, today string) float64 {
	// level パッケージと同等の計算（dateutil を再利用）
	start, err := dateutil.WindowStart(today, cfg.AvgWindowDays)
	if err != nil {
		return cfg.FallbackCupsPerDay
	}
	total := 0
	hasData := false
	for _, r := range d.Usage.DailyRecords {
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
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/notify/...
```
Expected: `ok bean-watcher/internal/notify`

- [ ] **Step 5: コミット**

```bash
git add internal/notify/
git commit -m "feat: notify パッケージで変化検知とメッセージ構築を追加"
```

---

## Task 9: discord パッケージ（Webhook 送信）

**Files:**
- Create: `internal/discord/discord.go`
- Test: `internal/discord/discord_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `internal/discord/discord_test.go`:
```go
package discord

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendSuccess(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type: got %s", ct)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := Send(context.Background(), srv.URL, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(gotBody, `"content":"hello"`) {
		t.Errorf("body: got %s", gotBody)
	}
}

func TestSendErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if err := Send(context.Background(), srv.URL, "x"); err == nil {
		t.Error("expected error on 400")
	}
}

func TestSendMasksURLInError(t *testing.T) {
	secret := "https://discord.com/api/webhooks/SECRET_TOKEN_123"
	// 不正なホストで接続エラーを発生させる
	err := Send(context.Background(), "http://127.0.0.1:0/webhook", "x")
	if err == nil {
		t.Skip("connection did not fail")
	}
	if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), secret) {
		t.Errorf("error leaks URL: %v", err)
	}
}

func TestSendEmptyURL(t *testing.T) {
	if err := Send(context.Background(), "", "x"); err == nil {
		t.Error("expected error for empty URL")
	}
}
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./internal/discord/...
```
Expected: FAIL（関数未定義）

- [ ] **Step 3: 実装**

Create `internal/discord/discord.go`:
```go
// Package discord は Discord Webhook へのメッセージ送信を行う。
// Webhook URL はログに漏洩しないよう、エラーメッセージから除外する。
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Message は Discord Webhook のペイロード。
type Message struct {
	Content string `json:"content"`
}

// Send は content を Discord Webhook に送信する。
func Send(ctx context.Context, webhookURL, content string) error {
	if webhookURL == "" {
		return errors.New("discord webhook URL is empty")
	}
	body, err := json.Marshal(Message{Content: content})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return maskURL(webhookURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return maskURL(webhookURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// maskURL はエラー文言に webhook URL が含まれないよう取り除く。
func maskURL(url string, err error) error {
	msg := err.Error()
	msg = strings.ReplaceAll(msg, url, "***")
	return errors.New("discord send failed: " + msg)
}
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./internal/discord/...
```
Expected: `ok bean-watcher/internal/discord`

- [ ] **Step 5: コミット**

```bash
git add internal/discord/
git commit -m "feat: discord パッケージでWebhook送信を追加"
```

---

## Task 10: main.go（サブコマンド結合）

**Files:**
- Modify: `main.go`
- Test: `main_test.go`

- [ ] **Step 1: 失敗テストを書く**

Create `main_test.go`:
```go
package main

import (
	"testing"
	"time"

	"bean-watcher/internal/clock"
)

func TestJSTTodayFormat(t *testing.T) {
	// UTC 2026-06-20 23:00 == JST 2026-06-21 08:00
	c := clock.Fake{T: time.Date(2026, 6, 20, 23, 0, 0, 0, time.UTC)}
	got := jstToday(c)
	if got != "2026-06-21" {
		t.Errorf("got %q, want 2026-06-21", got)
	}
}
```

- [ ] **Step 2: テスト実行で失敗確認**

Run:
```bash
go test ./...
```
Expected: FAIL（jstToday 未定義）

- [ ] **Step 3: 実装**

Replace `main.go`:
```go
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"bean-watcher/internal/clock"
	"bean-watcher/internal/discord"
	"bean-watcher/internal/model"
	"bean-watcher/internal/notify"
	"bean-watcher/internal/record"
	"bean-watcher/internal/store"
)

const (
	configPath = "data/config.json"
	dataPath   = "data/data.json"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: bean-watcher <record|notify> [args]")
	}
	switch os.Args[1] {
	case "record":
		runRecord(os.Args[2:])
	case "notify":
		runNotify(os.Args[2:])
	default:
		log.Fatalf("unknown subcommand: %s", os.Args[1])
	}
}

func runRecord(args []string) {
	if len(args) < 1 {
		log.Fatal("usage: bean-watcher record <pour|clean|buy> ...")
	}
	sub := args[0]
	c := clock.Real{}
	cfg := mustLoadConfig(configPath)
	d := mustLoadData(dataPath)
	today := jstToday(c)
	d = record.Prune(d, cfg.AvgWindowDays, today)

	var err error
	switch sub {
	case "pour":
		fs := flag.NewFlagSet("pour", flag.ExitOnError)
		cups := fs.Int("cups", 1, "number of cups")
		fs.Parse(args[1:])
		d, err = record.PourCoffee(d, cfg, today, *cups)
	case "clean":
		fs := flag.NewFlagSet("clean", flag.ExitOnError)
		target := fs.String("target", "", "descaling|grinder")
		fs.Parse(args[1:])
		d, err = record.Clean(d, today, *target)
	case "buy":
		fs := flag.NewFlagSet("buy", flag.ExitOnError)
		grams := fs.Int("grams", 0, "grams purchased")
		fs.Parse(args[1:])
		if *grams <= 0 {
			log.Fatalf("invalid grams: must be positive, got %d", *grams)
		}
		d, err = record.AddBeans(d, cfg, today, *grams)
	default:
		log.Fatalf("unknown record action: %s", sub)
	}
	if err != nil {
		log.Fatalf("record: %v", err)
	}
	mustSaveData(dataPath, d)
}

func runNotify(args []string) {
	c := clock.Real{}
	cfg := mustLoadConfig(configPath)
	d := mustLoadData(dataPath)
	today := jstToday(c)
	d = record.Prune(d, cfg.AvgWindowDays, today)

	cur := notify.CurrentLevels(d, cfg, today)
	diff := notify.Diff(d.NotifyState, cur)

	if msg := notify.BuildMessage(d, cfg, cur, diff, today); msg != "" {
		webhook := os.Getenv("DISCORD_WEBHOOK_URL")
		if err := discord.Send(context.Background(), webhook, msg); err != nil {
			// 送信失敗時は notify_state を更新せず終了（次回リトライ）
			log.Fatalf("notify send: %v", err)
		}
	}

	// notify_state 更新（未設定のメンテナンスは元の値を維持）
	ns := model.NotifyState{
		Beans:     cur.Beans,
		Descaling: cur.Descaling,
		Grinder:   cur.Grinder,
	}
	if ns.Descaling == "" {
		ns.Descaling = d.NotifyState.Descaling
	}
	if ns.Grinder == "" {
		ns.Grinder = d.NotifyState.Grinder
	}
	d.NotifyState = ns
	mustSaveData(dataPath, d)
}

func jstToday(c clock.Clock) string {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.UTC
	}
	return c.Now().In(loc).Format("2006-01-02")
}

func mustLoadConfig(path string) model.Config {
	c, err := store.LoadConfig(path)
	if err != nil {
		log.Fatalf("load config %s: %v", path, err)
	}
	return c
}

func mustLoadData(path string) model.Data {
	d, err := store.LoadData(path)
	if err != nil {
		log.Fatalf("load data %s: %v", path, err)
	}
	return d
}

func mustSaveData(path string, d model.Data) {
	if err := store.SaveData(path, d); err != nil {
		log.Fatalf("save data %s: %v", path, err)
	}
}
```

- [ ] **Step 4: テスト合格確認**

Run:
```bash
go test ./...
```
Expected: 全パッケージ PASS

- [ ] **Step 5: go vet とビルド確認**

Run:
```bash
go vet ./... && go build -o bean-watcher .
```
Expected: エラーなし、`bean-watcher` バイナリ生成

- [ ] **Step 6: コミット**

```bash
git add main.go main_test.go
git commit -m "feat: main で record/notify サブコマンドを結合"
```

---

## Task 11: 初期データファイル

**Files:**
- Create: `data/config.json`
- Create: `data/data.json`

- [ ] **Step 1: config.json 作成**

Create `data/config.json`:
```json
{
  "grams_per_cup": 12,
  "low_days_threshold": 5,
  "critical_days_threshold": 2,
  "avg_window_days": 7,
  "fallback_cups_per_day": 2,
  "maintenance": {
    "descaling": { "threshold_days": 30, "threshold_shots": 200 },
    "grinder":   { "threshold_days": 7,  "threshold_shots": 30 }
  }
}
```

- [ ] **Step 2: data.json 作成（空状態）**

Create `data/data.json`:
```json
{
  "beans": { "remaining_grams": 0 },
  "usage": { "total_shots": 0, "daily_records": [] },
  "maintenance": {
    "descaling": { "last_date": "", "last_shots": 0 },
    "grinder":   { "last_date": "", "last_shots": 0 }
  },
  "notify_state": { "beans": "OK", "descaling": "OK", "grinder": "OK" }
}
```

- [ ] **Step 3: JSON 形式の検証**

Run:
```bash
go run . notify || true
```
Expected: 「load config / load data」の fatal が出ず、Webhook 未設定エラー（`notify send: discord webhook URL is empty`）または notify_state の保存成功で終了。いずれにせよ JSON 読込エラーが出ないことを確認。

- [ ] **Step 4: コミット**

```bash
git add data/config.json data/data.json
git commit -m "chore: 初期 config/data ファイルを追加"
```

---

## Task 12: GitHub Actions ワークフロー

**Files:**
- Create: `.github/workflows/record.yml`
- Create: `.github/workflows/notify.yml`
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: ci.yml 作成**

Create `.github/workflows/ci.yml`:
```yaml
name: ci

on:
  push:
    branches: [main, "feat/**"]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.21"
          cache: true
      - run: go vet ./...
      - run: go test ./... -race -cover
```

- [ ] **Step 2: record.yml 作成**

Create `.github/workflows/record.yml`:
```yaml
name: record

on:
  workflow_dispatch:
    inputs:
      action:
        description: "記録するアクション"
        required: true
        type: choice
        default: pour
        options:
          - pour
          - clean
          - buy
      cups:
        description: "杯数 (pour のみ)"
        required: false
        default: "1"
      target:
        description: "掃除対象 (clean のみ): descaling / grinder"
        required: false
        default: "descaling"
      grams:
        description: "購入量g (buy のみ)"
        required: false
        default: "200"

permissions:
  contents: write

concurrency:
  group: bean-watcher
  cancel-in-progress: false

jobs:
  record:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.21"
          cache: true
      - name: Build
        run: go build -o bean-watcher .
      - name: Run record
        run: |
          case "${{ github.event.inputs.action }}" in
            pour)  ./bean-watcher record pour  -cups=${{ github.event.inputs.cups }} ;;
            clean) ./bean-watcher record clean -target=${{ github.event.inputs.target }} ;;
            buy)   ./bean-watcher record buy   -grams=${{ github.event.inputs.grams }} ;;
          esac
      - name: Commit data
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git add data/data.json
          git commit -m "chore(data): ${{ github.event.inputs.action }} を記録 [skip ci]" || echo "no changes"
          git push
```

- [ ] **Step 3: notify.yml 作成**

Create `.github/workflows/notify.yml`:
```yaml
name: notify

on:
  schedule:
    - cron: "0 23 * * *"  # UTC 23:00 == JST 08:00
  workflow_dispatch:

permissions:
  contents: write

concurrency:
  group: bean-watcher
  cancel-in-progress: false

jobs:
  notify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.21"
          cache: true
      - name: Build
        run: go build -o bean-watcher .
      - name: Run notify
        env:
          DISCORD_WEBHOOK_URL: ${{ secrets.DISCORD_WEBHOOK_URL }}
        run: ./bean-watcher notify
      - name: Commit data
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git add data/data.json
          git commit -m "chore(data): notify_state 更新 [skip ci]" || echo "no changes"
          git push
```

- [ ] **Step 4: ワークフロー構文のローカル確認**

Run:
```bash
ls -la .github/workflows/
```
Expected: ci.yml, record.yml, notify.yml の3ファイル

- [ ] **Step 5: コミット**

```bash
git add .github/workflows/
git commit -m "ci: record/notify/ci ワークフローを追加"
```

---

## Task 13: README.md

**Files:**
- Create: `README.md`

- [ ] **Step 1: README 作成**

Create `README.md`（内容は Step 2 に完全な本文）。

- [ ] **Step 2: README 本文**

```markdown
# bean-watcher

全自動コーヒーメーカーの豆の残量とメンテナンス時期を管理し、Discord に通知するツール。GitHub Actions の定期実行とリポジトリ内 JSON だけで完全無料で運用できます。

## できること

- ☕ **豆の買い時通知**: 過去7日の消費ペースから残り日数を予測し、「あと約5日」「あと約2日」で Discord に通知。
- 🧽 **掃除タイミング通知**: クエン酸洗浄・ミル掃除を「経過日数」または「抽出回数」のどちらか超過で通知。
- 🔔 **1日1回のチェック**: 毎朝8時(JST)にcron実行。**レベルが変化した時だけ**通知（毎日同じメッセージは来ない）。

## セットアップ

### 1. リポジトリの準備
このリポジトリを自分の GitHub アカウントに用意（fork または新規作成）。

### 2. Discord Webhook の作成
1. Discord のサーバー設定 →「連携サービス」→「ウェブフック」→「新しいウェブフック」。
2. 通知したいチャンネルを選び、**ウェブフックURL**をコピー。
   （形式: `https://discord.com/api/webhooks/<ID>/<TOKEN>`）

### 3. Secret の登録
GitHub リポジトリの「Settings」→「Secrets and variables」→「Actions」→「New repository secret」:
- Name: `DISCORD_WEBHOOK_URL`
- Secret: コピーした Webhook URL

### 4. 設定の編集（任意）
`data/config.json` を自分の環境に合わせて変更:
- `grams_per_cup`: 1杯あたりの豆（g）。既定12。
- `low_days_threshold` / `critical_days_threshold`: 豆の警告日数。既定5日 / 2日。
- `maintenance.descaling.threshold_days/shots`: クエン酸洗浄周期。既定30日 / 200杯。
- `maintenance.grinder.threshold_days/shots`: ミル掃除周期。既定7日 / 30杯。

### 5. ワークフローの有効化
GitHub の「Actions」タブを開き、表示されるワークフローを「I understand my workflows, go ahead and enable them」で有効化。

## 使い方

GitHub の「Actions」タブから該当ワークフローを選び「Run workflow」:

| ワークフロー | いつ使う | 入力 |
|---|---|---|
| **record** → `pour` | コーヒーを淹れた | `cups`（既定1） |
| **record** → `clean` | 掃除した | `target`（descaling / grinder） |
| **record** → `buy` | 豆を買った | `grams`（購入量） |
| **notify** | （定期・手動） | なし。しきい値をチェックして通知 |

### 初回運用開始
1. 「record → `buy`」で `grams` に初期購入量（例: 300）を入れて実行 → 残量を登録。
2. 「record → `clean`」で `target=descaling` と `target=grinder` をそれぞれ1回ずつ実行 → 掃除の基準点を設定（これをしないと掃除通知は出ません）。
3. 以降は毎日コーヒーを淹れるたびに「record → `pour`」を、掃除したら「record → `clean`」を記録。notify は毎朝8時に自動実行されます。

## データ復元（data.json が壊れた場合）
`data/data.json` は git で管理されているため、GitHub の「Commits」履歴から1つ前の正常な版に戻せます:
```
git checkout <正常なコミット> -- data/data.json
git commit -m "fix: data.json を復元"
git push
```

## 仕組み
- **record** ワークフロー（手動）: `./bean-watcher record <action>` を実行し `data/data.json` を更新・コミット。
- **notify** ワークフロー（cron 0 23 * * * = JST 8時）: `./bean-watcher notify` がレベルを計算。前回通知レベルより厳しい方向への変化があれば Discord 通知し `notify_state` をコミット。
- **ci** ワークフロー（push/PR）: `go vet` と `go test` を実行。

## コスト
GitHub Actions の無料枠内で完結（公開リポジトリなら無制限、私有リポジトリでも月2000分以内）。外部サーバー・DB不要。
```

- [ ] **Step 3: コミット**

```bash
git add README.md
git commit -m "docs: README にセットアップ手順を追加"
```

---

## Task 14: 全体テストと最終確認

**Files:**
- なし（検証のみ）

- [ ] **Step 1: 全テスト実行**

Run:
```bash
go test ./... -race -cover
```
Expected: 全パッケージ PASS、カバレッジ表示

- [ ] **Step 2: go vet**

Run:
```bash
go vet ./...
```
Expected: エラーなし

- [ ] **Step 3: ローカル動作確認（data.json の更新）**

Run:
```bash
go build -o bean-watcher .
./bean-watcher record buy -grams=300
cat data/data.json
```
Expected: `remaining_grams` が 0 → 300 に更新される

- [ ] **Step 4: 通知のローカル確認**

Run:
```bash
./bean-watcher notify
```
Expected: `DISCORD_WEBHOOK_URL` 未設定なら `notify send: discord webhook URL is empty` で終了（JSON 読込は成功）。`data/data.json` の `notify_state` は更新されない（送信失敗時は保存しない）。

- [ ] **Step 5: リモートへプッシュ**

```bash
git push -u origin feat/bean-watcher
```
Expected: プッシュ成功。GitHub の Actions タブでワークフローが有効化される。

- [ ] **Step 6: 最終コミット（変更があれば）**

変更がある場合のみ:
```bash
git add -A
git commit -m "test: 全体テストと動作確認を完了"
git push
```

---

## 完了後の確認事項

- [ ] GitHub の Secrets に `DISCORD_WEBHOOK_URL` が登録済み。
- [ ] Actions タブで3つのワークフローが有効化済み。
- [ ] 「record → buy」で初期量を登録済み。
- [ ] 「record → clean」で descaling / grinder の基準点を設定済み。
- [ ] 「notify」を1回手動実行し、Discord に（警告があれば）通知が届くか確認。
