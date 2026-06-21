# bean-watcher 設計仕様書

- **日付**: 2026-06-21
- **ステータス**: Review（ユーザーレビュー待ち）
- **対象**: 全自動コーヒーメーカーの豆消費・メンテナンス管理と Discord 通知

---

## 1. 概要

全自動コーヒーメーカーの運用を支援するバッチ型の自動化ツール。豆の残量を予測して「買い時」を、メンテナンス（クエン酸洗浄・ミル掃除）のタイミングを「掃除時」を、それぞれ Discord Webhook で通知する。サーバーコストをかけず、GitHub Actions（定期実行）とリポジトリ内 JSON（データ保存）だけで完全無料で構成する。

日々の「1杯淹れた」「掃除した」「豆を買った」は、GitHub の Actions 画面に並ぶ手動実行ボタン（`workflow_dispatch`）からワンタップで記録する。

## 2. 目的と背景

- 豆の買い忘れ・メンテナンスの怠りを防ぐ。
- 通知は「レベルが変化した時だけ」に絞り、毎日同じメッセージで煩わしくならないようにする。
- 外部サービス（サーバー、DB、有料 SaaS）を使わず、GitHub だけで完結する。

## 3. 機能要件

### 3.1 豆の残量管理
- 購入量（g）から日々の消費量（杯数 × `grams_per_cup`）を減算し、残量を予測する。
- 直近 `avg_window_days`（既定7日）の実績から1日平均杯数を算出し、残り日数を予測。
- 残り日数がしきい値を下回ったら Discord にリマインドを通知する。

### 3.2 メンテナンス管理
- クエン酸洗浝（descaling）とミル掃除（grinder）の2種類を管理。
- 「前回からの経過日数」**または**「前回からの抽出回数」のどちらかがしきい値を超えたら `DUE`（掃除不要）と判定し通知する。

### 3.3 入力インターフェース
- GitHub Actions の `workflow_dispatch` で3つのアクションをワンタップで記録する。
- 3アクション: コーヒーを淹れた / 掃除した / 豆を購入した。

## 4. 非機能要件

- **完全無料**: GitHub Actions の無料枠（公開リポジトリなら無制限、私有リポジトリなら月2000分）内で運用。外部サーバー・DB不要。
- **セキュア**: Discord Webhook URL は GitHub Secrets に格納し、リポジトリには一切コミットしない。
- **データの耐障害性**: 状態は `data/data.json` に都度コミット・プッシュされ、git 履歴で追跡可能。
- **言語/実行環境**: Go（`github.com/actions/setup-go` で実行）。外部依存は最小限、可能なら標準ライブラリのみ。
- **タイムゾーン**: すべて JST（Asia/Tokyo）で扱う。

## 5. システムアーキテクチャ

```
┌─────────────────────────────┐     ┌─────────────────────────────┐
│  record.yml (手動)          │     │  notify.yml (cron 毎朝8時)  │
│  workflow_dispatch          │     │  0 23 * * * (UTC=JST 8:00)  │
│  inputs: cups/target/grams  │     │                             │
└──────────┬──────────────────┘     └──────────┬──────────────────┘
           │                                    │
           ▼                                    ▼
   ┌───────────────────────────────────────────────────┐
   │  bean-watcher バイナリ (Go)                       │
   │  ./bean-watcher record  /  ./bean-watcher notify  │
   └──────────┬──────────────────────────┬─────────────┘
              │ 読み書き                  │ 読込 + 送信
              ▼                           ▼
   ┌─────────────────────┐      ┌─────────────────────┐
   │  data/data.json     │      │  Discord Webhook    │
   │  (commit & push)    │      │  (DISCORD_WEBHOOK_  │
   │                     │      │   URL from Secrets) │
   └─────────────────────┘      └─────────────────────┘
              ▲
              │ 設定読込
   ┌─────────────────────┐
   │  data/config.json   │
   │  (手動編集)         │
   └─────────────────────┘
```

**並行実行制御**: `record` と `notify` は同じ `data/data.json` を更新するため、両ワークフローで同一の concurrency group（`bean-watcher`）を指定し直列化する。これにより同時実行による競合・プッシュ衝突を防ぐ。

## 6. コンポーネント設計

Go パッケージ構成。各パッケージは単一責務で独立してテスト可能。

| パッケージ | 責務 | 主要な型/関数 |
|---|---|---|
| `internal/model` | データ構造の型定義。JSON タグ付き。 | `Config`, `Data`, `Beans`, `Usage`, `Maintenance`, `NotifyState`, `Level` |
| `internal/store` | `config.json` / `data.json` の読み書き。 | `LoadConfig(path)`, `LoadData(path)`, `SaveData(path, Data)` |
| `internal/clock` | 時刻取得の抽象化（テストで固定時刻を注入）。 | `Clock`, `realClock`, `fakeClock` |
| `internal/record` | 記録ロジック（純粋関数、Data を更新した新しい Data を返す）。 | `PourCoffee`, `Clean`, `AddBeans` |
| `internal/notify` | レベル計算・変化検知・メッセージ生成。 | `ComputeBeansLevel`, `ComputeMaintLevel`, `DiffLevels`, `BuildMessage` |
| `internal/discord` | Webhook 送信（`net/http`）。 | `Send(ctx, webhookURL, Message)` |
| `main` | サブコマンド (`record`/`notify`) のエントリポイント、引数解析、依存結合。 | `main()` |

**イミュータブル方針**（コーディング規約準拠）: `record`/`notify` のドメイン関数は引数の構造体を破壊せず、新しい構造体を返す。

## 7. データモデル

### 7.1 `data/config.json`（手動編集・変更少）

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

| 項目 | 意味 | 既定値 |
|---|---|---|
| `grams_per_cup` | 1杯あたりの豆（g） | 12 |
| `low_days_threshold` | 豆 LOW 警告の残り日数 | 5 |
| `critical_days_threshold` | 豆 CRITICAL 警告の残り日数 | 2 |
| `avg_window_days` | 平均消費を計算する直近日数 | 7 |
| `fallback_cups_per_day` | 履歴不足時の1日杯数（既定値） | 2 |
| `maintenance.*.threshold_days` | 掃除の経過日数しきい値 | descaling 30 / grinder 7 |
| `maintenance.*.threshold_shots` | 掃除の抽出回数しきい値 | descaling 200 / grinder 30 |

### 7.2 `data/data.json`（自動更新・変更多）

```json
{
  "beans": { "remaining_grams": 300 },
  "usage": {
    "total_shots": 0,
    "daily_records": [ { "date": "2026-06-20", "cups": 2 } ]
  },
  "maintenance": {
    "descaling": { "last_date": "2026-06-01", "last_shots": 0 },
    "grinder":   { "last_date": "2026-06-01", "last_shots": 0 }
  },
  "notify_state": {
    "beans": "OK",
    "descaling": "OK",
    "grinder": "OK"
  }
}
```

- `daily_records` は直近 `avg_window_days` 日分のみ保持し、`record`/`notify` の各実行でウィンドウより古いレコードをpruneする。
- `notify_state` は「前回通知したレベル」を保持し、レベル変化検知に使う。
- `remaining_grams` は `max(0, ...)` で0未満にはならないようクリップする。

### 7.3 初期セットアップ時のデータ

初回は `data/data.json` を下記の空状態でコミットする（README で案内）:

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

運用開始は「🛒 豆を購入した」で初期量を登録する。`last_date` が空のうちは掃除判定は「初回未設定のため通知しない」とし、最初の「掃除した」記録で基準点を設定する。

## 8. 入力インターフェース（`record` サブコマンド）

`record.yml` は `workflow_dispatch` で `action` を選ばせる。単一ワークフロー内で `action` 入力により分岐し、対応する引数で `./bean-watcher record` を呼ぶ。

| action 入力値 | 追加 input | record サブコマンド呼び出し | 動作 |
|---|---|---|---|
| `pour` | `cups`（既定1） | `record pour --cups <n>` | `remaining_grams -= cups×g/cup`（0未満クリップ）、`total_shots += cups`、当日 `daily_records` に加算（同日に複数回記録した場合は `cups` を合算） |
| `clean` | `target`（descaling/grinder） | `record clean --target <t>` | 対象の `last_date=今日`、`last_shots=total_shots`、`notify_state.<t>=OK` |
| `buy` | `grams` | `record buy --grams <g>` | `remaining_grams += grams`、`notify_state.beans` を現在残量から再計算 |

いずれも実行後に `data/data.json` をコミット・プッシュする。

## 9. 通知ロジック（`notify` サブコマンド）

### 9.1 レベル定義

**豆（`beans`）**: 予測残り日数で3段階。
```
# ウィンドウ = 今日を含めて過去 avg_window_days 日（今日から遡ってN日分）
windowRecords   = daily_records のうち [今日-N+1日 .. 今日] に入るレコード
totalCups       = windowRecords の cups の合計
hasWindowData   = windowRecords に1件以上のレコードがある

# 1日平均杯数（分母は固定で avg_window_days）
avgCups = hasWindowData ? totalCups / avg_window_days : fallback_cups_per_day

# 予測残り日数（0除算回避のため分母を微小値で下駄履き）
remainingDays = remaining_grams / (grams_per_cup × max(avgCups, 0.001))

if remainingDays <= critical_days_threshold: CRITICAL
elif remainingDays <= low_days_threshold:    LOW
else:                                        OK
```

補足: 平均の分母を「記録のあった日数」ではなく「ウィンドウ長（N日）」で割るのは、飲まなかった日も0杯として扱い、過大評価を防ぐため。記録が1件もない（運用開始直後など）場合のみ `fallback_cups_per_day` にフォールバックする。

**メンテナンス（descaling/grinder）**: 2段階。
```
if last_date == "":  DUE 未設定（通知しない、基準点待ち）
elif (今日 − last_date) > threshold_days
  OR (total_shots − last_shots) > threshold_shots:  DUE
else:  OK
```

レベルの大小関係: 豆は `OK(0) < LOW(1) < CRITICAL(2)`、メンテナンスは `OK(0) < DUE(1)`。

### 9.2 変化検知と通知

1. 現在の3レベル（beans/descaling/grinder）を計算。
2. `notify_state`（前回通知レベル）と比較。
3. **現在レベルが前回より厳しい方向に変わった項目**について Discord 通知。
4. 通知の有無にかかわらず、現在の3レベルを `notify_state` に保存してコミット（値が同じ場合も上書き）。

これにより「OK→LOW」で1回通知した後、LOWの間は毎日通知されず、「LOW→CRITICAL」で再通知される。掃除・購入で回復した場合は `notify_state` もOKに戻り（record サブコマンドで更新）、次に警告に達した時に再通知される。

### 9.3 Discord メッセージフォーマット例

通知は1回の Webhook で複数項目をまとめて1メッセージにする（連投を避ける）。

豆 LOW 例:
```
☕ **豆の買い時**: あと約5日でなくなりそうです（残り 60g）
週末に焙煎・購入を！
```

豆 CRITICAL 例:
```
🚨 **豆がもうすぐ切れます**: あと約2日（残り 24g）
早めの補充をお願いします！
```

掃除 DUE 例:
```
🧽 **掃除タイミング**: クエン酸洗浄の目安です（前回から35日 / 215杯）
```

複数同時例（ヘッダで件名をまとめる）:
```
🔔 **bean-watcher お知らせ** (2026-06-21)
• 🚨 豆がもうすぐ切れます: あと約2日（残り 24g）
• 🧽 掃除タイミング: ミル掃除の目安です（前回から8日）
```

## 10. GitHub Actions ワークフロー設計

### 10.1 `.github/workflows/record.yml`
- トリガー: `workflow_dispatch`（`action`, `cups`, `target`, `grams` の inputs）。
- ステップ: checkout → setup-go → `go build` → `./bean-watcher record <action> ...` → `data/data.json` の変更を commit & push。
- concurrency group: `bean-watcher`（直列化）。
- 権限: `contents: write`（コミット・プッシュ用）。

### 10.2 `.github/workflows/notify.yml`
- トリガー: `schedule: cron "0 23 * * *"`（UTC 23:00 = JST 8:00 毎日）＋ `workflow_dispatch`（手動テスト用）。
- ステップ: checkout → setup-go → `go build` → `./bean-watcher notify` → `notify_state` 変更があれば commit & push。
- 環境変数: `DISCORD_WEBHOOK_URL` を Secrets から注入。
- concurrency group: `bean-watcher`（直列化）。
- 権限: `contents: write`。

### 10.3 `.github/workflows/ci.yml`
- トリガー: `push`, `pull_request`。
- ステップ: `go vet`, `go test ./... -race -cover`。カバレッジ80%を目標。
- テスト不足時は後続のデプロイ/マージをブロック（保護ブランチ運用は任意）。

## 11. エラー処理・エッジケース

- **Webhook 送信失敗**: 通知失敗時は `notify_state` を更新せず終了（次回の cron で再通知される）。エラーは Actions のログに出力し非ゼロ終了。
- **JSON 読込失敗・形式不正**: 起動時に検証し、即座に非ゼロ終了＋ログ。`data.json` が破損した場合は git 履歴から復元（README に手順記載）。
- **残量のアンダーフロー**: `remaining_grams` は `max(0, ...)` でクリップ。
- **0除算**: 平均杯数0の場合は微小値で下駄を履かせ残り日数を巨大値（=OK）とする。
- **プッシュ衝突**: concurrency group で直列化し回避。万一衝突した場合は `git pull --rebase` 後再プッシュを1回だけリトライ。
- **初回（履歴なし）**: `fallback_cups_per_day` で予測。掃除 `last_date` 空は通知しない。
- **タイムゾーン**: Go 内で `Asia/Tokyo` を Load して日付計算。Load 失敗時は UTC にフォールバック（ログ警告）。

## 12. セキュリティ考慮

- Discord Webhook URL は GitHub Secrets のみ。コード・ログ・コミットに一切出力しない。
- URL が誤ってログに出ないよう、`discord.Send` は URL をマスクしたエラーメッセージを返す。
- `data.json` のコミットは `github-actions[bot]` の GITHUB_TOKEN を使い、個人トークンは使わない。
- 外部依存ライブラリは原則使わない（標準ライブラリのみ）。導入する場合は `go mod verify` と Dependabot を有効化。
- ユーザー入力（cups/grams）は整数にパースし範囲バリデーション（0未満・極端な値は拒否）。

## 13. テスト方針

TDD（testing.md 準拠）。目標カバレッジ80%以上。

| 対象 | テスト内容 |
|---|---|
| `internal/record` | `PourCoffee` の減算とクリップ、`Clean` のリセット、`AddBeans` の加算と再計算、入力バリデーション |
| `internal/notify` | レベル計算（OK/LOW/CRITICAL/DUE 境界）、平均杯数の算出とフォールバック、0除算、変化検知（上がった時だけ通知） |
| `internal/store` | JSON 読み書きの往復（ラウンドトリップ）、破損ファイルのエラー処理、`daily_records` のprune |
| `internal/discord` | HTTP リクエストの構築検証（httptest でサーバをモック）、送信失敗時のエラー、URL マスク |
| `internal/clock` | 固定時刻注入で日付計算を再現可能に |

エッジケーステスト: 残量0、履歴なし、`last_date` 空、杯数0の記録、閏日/月跨ぎの日付計算。

## 14. セットアップ手順（概要）

詳細は README.md に記載。要点:

1. リポジトリを fork/clone。
2. `data/config.json` を自分の環境に合わせて編集（1杯のg、しきい値）。
3. `data/data.json` を空状態でコミット。
4. Discord で Webhook を作成し、URL をリポジトリの Secrets に `DISCORD_WEBHOOK_URL` として登録。
5. Actions タブでワークフローを有効化。
6. 「🛒 豆を購入した」で初期量を登録して運用開始。

## 15. スコープ外（YAGNI）

本仕様では扱わない（必要になったら別仕様で検討）:
- 複数人・複数台コーヒーメーカーの管理。
- 銘柄・焙煎日・産地などの豆のメタデータ管理。
- グラフ・ダッシュボード表示。
- Discord Bot による双方向コマンド（常駐サーバーが必要になるため）。
- 自動バックアップ（git 履歴で十分）。

## 16. 用語集

- **shots（抽出回数）**: コーヒーを抽出した延べ杯数。1杯=1ショット。
- **descaling（クエン酸洗浝）**: 湯路の石灰化（スケール）除去。
- **grinder（ミル掃除）**: 豆挽き部の掃除。
- **DUE**: 掃除が必要な状態（しきい値超過）。
- **LOW/CRITICAL**: 豆残量の警告レベル。
