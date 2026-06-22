# bean-watcher ウェブダッシュボード拡張 仕様書

- **日付**: 2026-06-22
- **ステータス**: Approved（ユーザー一任）
- **対象**: bean-watcher へのウェブダッシュボード追加と袋単位管理

---

## 1. 概要

既存の Discord 通知（push 型）に加え、**GitHub Pages の静的ダッシュボード**（pull 型・無料）を追加する。豆の残量・買い時予測・減少推移・購入履歴をブラウザでいつでも確認できる。あわせて豆の購入を「袋単位」で管理できるようにする。

Discord 通知は既存のまま継続。ウェブは**閲覧専用**とし、入力は既存の GitHub Actions の手動ボタン（`record` ワークフロー）を継続して使う。これによりサーバー不要・完全無料・安全（シークレットをブラウザに置かない）を保つ。

## 2. 目的と背景

- Discord 通知は「届いた瞬間」しか見られない。常時確認できるダッシュボードが欲しい。
- 購入を直感的な「袋」で管理したい（グラム直接入力は事務的）。
- 買い時の予測を視覚的に把握したい。

## 3. 機能要件

### 3.1 袋単位での購入管理
- `config.json` の `bag_grams`（1袋あたりのグラム、既定200）を基準に、購入を袋数で入力する。
- 購入履歴（日付・袋数・グラム）を `data.json` の `purchases` に保持する。
- 残量はグラムと袋換算の両方で表示する。

### 3.2 ウェブダッシュボード（閲覧専用）
- 残量（g ＋ 残り袋数）、買い時ステータス（OK/LOW/CRITICAL の色分け）
- 予測残り日数（過去7日の平均消費から）
- 減少推移（直近 `daily_records` の簡易グラフ）
- 購入履歴（`purchases` のリスト）
- 掃除ステータス（descaling/grinder のレベルと経過日数）
- `data/data.json` と `data/config.json` を fetch して表示。

### 3.3 表示用サマリの計算
- レベル計算（豆 OK/LOW/CRITICAL、掃除 OK/DUE）は Go 側で行い、結果を `data.json` の `summary` に書き出す。JavaScript は表示のみ（ロジックの重複を避ける）。

## 4. 非機能要件

- **完全無料**: GitHub Pages は無料。追加サーバー/DB 不要。
- **セキュア**: Webhook URL は `data.json` に含めない（Secrets のみ）。`data.json` は残量・履歴のみの公開情報。
- **技術**: ウェブはバニラ JavaScript（フレームワーク不要・外部依存ゼロ）。CDN 不使用（オフライン・プライバシーフレンドリ）。
- **タイムゾーン**: JST。

## 5. アーキテクチャ

```
既存:  record.yml / notify.yml  →  Go バイナリ  →  data.json (commit)
                                                        │
本拡張で追加:                                            ▼
                                              summary/purchases 更新
                                                        │
                                              pages.yml (push to main,
                                                paths: data/**, web/**)
                                                        │
                                                        ▼
                                              GitHub Pages (静的ホスト)
                                                        ▲
                                              web/index.html + app.js
                                              (data.json, config.json を fetch)
                                                        ▲
                                              ブラウザ（閲覧専用）
```

**データフロー**:
1. ユーザーが `record`（pour/clean/buy）を実行 → Go が `data.json` を更新（`summary`・`purchases` 含む）→ コミット・プッシュ。
2. `notify`（毎朝8時）も同様に `summary` を更新してコミット。
3. `pages.yml` が `data/**` または `web/**` の変更を検知し、`web/` の内容と `data.json`/`config.json` を GitHub Pages にデプロイ。
4. ブラウザが Pages から `index.html` を読み込み、`data.json` を fetch して表示。

## 6. データモデル

### 6.1 `data/config.json`（拡張）
```json
{
  "grams_per_cup": 12,
  "low_days_threshold": 5,
  "critical_days_threshold": 2,
  "avg_window_days": 7,
  "fallback_cups_per_day": 2,
  "bag_grams": 200,
  "maintenance": {
    "descaling": { "threshold_days": 30, "threshold_shots": 200 },
    "grinder":   { "threshold_days": 7,  "threshold_shots": 30 }
  }
}
```
追加: `bag_grams`（1袋のグラム、既定200）。

### 6.2 `data/data.json`（拡張）
```json
{
  "beans": { "remaining_grams": 300 },
  "usage": {
    "total_shots": 5,
    "daily_records": [ { "date": "2026-06-22", "cups": 2 } ]
  },
  "maintenance": {
    "descaling": { "last_date": "2026-06-01", "last_shots": 0 },
    "grinder":   { "last_date": "2026-06-01", "last_shots": 0 }
  },
  "purchases": [
    { "date": "2026-06-22", "bags": 1, "grams": 200 }
  ],
  "summary": {
    "remaining_grams": 300,
    "remaining_bags": 1.5,
    "beans_level": "OK",
    "predicted_days": 12.5,
    "avg_cups_per_day": 2.0,
    "descaling_level": "OK",
    "descaling_days_elapsed": 21,
    "descaling_shots_elapsed": 5,
    "grinder_level": "OK",
    "grinder_days_elapsed": 21,
    "grinder_shots_elapsed": 5,
    "updated_at": "2026-06-22"
  },
  "notify_state": { "beans": "OK", "descaling": "OK", "grinder": "OK" }
}
```
追加: `purchases`（購入履歴）、`summary`（表示用計算結果）。

## 7. コンポーネント設計（Go 側）

| パッケージ | 変更 | 内容 |
|---|---|---|
| `internal/model` | 拡張 | `Config.BagGrams`、`Data.Purchases`、`Purchase{Date,Bags,Grams}`、`Summary` 構造体、`Data.Summary` 追加 |
| `internal/summary` | **新規** | `Compute(d Data, cfg Config, today string) Summary`。level パッケージを利用し、残量・予測日数・レベル・経過日数を計算する純粋関数 |
| `internal/record` | 拡張 | `AddBags(d, cfg, date, bags)` を追加（`bags × bag_grams` を加算、`purchases` に履歴追加）。`AddBeans(grams 直接)` は廃止し bags に統一 |
| `main` | 拡張 | `runRecord`/`runNotify` の保存前に `summary.Compute` を呼び `Data.Summary` を更新 |

`summary.Compute` は `level.Beans`/`level.Maintenance` を再利用し、JS 側への値（`predicted_days`、`avg_cups_per_day`、経過日数・回数など）を計算して返す。

## 8. 入力インターフェースの変更（`record buy`）

| 変更前 | 変更後 |
|---|---|
| `record buy -grams=300` | `record buy -bags=2`（2袋 × bag_grams） |

`record.yml` の `grams` input を `bags`（既定1）に変更。run ステップの case 分岐も `-bags` に更新。

互換性: `grams` 入力は廃止。袋単位が要件なので bags に統一（YAGNI）。

## 9. ウェブUI設計

### 9.1 ファイル構成
```
web/
├── index.html    レイアウト・表示枠
├── app.js        data.json/config.json fetch ＋ 表示
└── style.css     スタイル（軽量・ダーク対応なしでまず単一テーマ）
```

### 9.2 表示項目とレイアウト
```
┌─────────────────────────────────────┐
│  bean-watcher                  更新日 │
├─────────────────────────────────────┤
│  ☕ 残量      [300g / 1.5袋]   ●OK    │
│  予測: あと約 12日 (1日平均 2.0杯)    │
├─────────────────────────────────────┤
│  📉 減少推移（直近7日の杯数バーチャート）│
├─────────────────────────────────────┤
│  🛒 購入履歴                          │
│   6/22  1袋 (200g)                   │
│   6/08  2袋 (400g)                   │
├─────────────────────────────────────┤
│  🧽 メンテナンス                      │
│   クエン酸洗浄  ●OK  経過21日 / 5杯   │
│   ミル掃除     ●OK  経過21日 / 5杯   │
└─────────────────────────────────────┘
```
ステータス色: OK=緑、LOW=黄、CRITICAL/DUE=赤。

### 9.3 app.js の振る舞い
- 起動時に `fetch('data.json')` と `fetch('config.json')`（Pages ルートに配置されるため相対パス）。
- `summary` を読みカード表示、`purchases` を履歴リスト、`usage.daily_records` をグラフ描画。
- fetch 失敗時は「データを読み込めませんでした」を表示。

## 10. ホスティング（GitHub Pages）

### 10.1 デプロイ構成
`pages.yml` を新規追加:
- トリガー: `push` to `main`、`paths: ['data/**', 'web/**']` ＋ `workflow_dispatch`。
- ステップ: `web/` の中身（index.html, app.js, style.css）と `data/data.json`・`data/config.json` を Pages ルートに平坦化して1つの artifact にまとめる（`data/data.json`→`/data.json`, `data/config.json`→`/config.json`）。`actions/deploy-pages` でデプロイ。これにより `index.html` と同階層に `data.json`/`config.json` が置かれ、`app.js` は `fetch('data.json')`/`fetch('config.json')` で取得できる。
- `permissions: { contents: read, pages: write, id-token: write }`。
- concurrency group: `pages`、`cancel-in-progress: false` はせず `true`（最新のみ残す）。

### 10.2 `[skip ci]` の調整
現状 `record.yml`/`notify.yml` の data コミットに `[skip ci]` が付いており Pages デプロイがスキップされる。**`[skip ci]` を外す**。代わりに `ci.yml` のトリガーに `paths` フィルタ（`**/*.go`, `go.mod`, `go.sum`）を追加し、data.json 更新では Go テストが走らないようにする。これで data.json 更新 → pages.yml が発火 → Pages 更新、の流れが成立する。

## 11. Discord 通知への影響

- `notify` の `BuildMessage` は豆メッセージに袋換算を追加（例:「あと約5日でなくなりそうです（残り 60g / 0.3袋）」）。`summary` から残り袋数を取得。
- 通知頻度・レベル判定ロジックは変更なし。

## 12. エラー処理・エッジケース

- **data.json に summary が無い（旧形式）**: app.js は `summary` 欠落時にフォールバック表示（「—」）。
- **fetch 失敗**: app.js がエラーメッセージ表示。Pages デプロイ未完了時も同様。
- **袋数0以下**: `AddBags` がエラー（既存のバリデーション踏襲）。
- **bag_grams が0**: summary の袋換算で0除算回避（残り袋数を巨大値または「—」）。
- **purchases が空**: 履歴欄に「まだ購入記録がありません」。
- **Pages デプロイ失敗**: pages.yml のログで確認。data.json 自体は main にあるので復旧容易。

## 13. テスト方針

| 対象 | テスト内容 |
|---|---|
| `internal/summary` | `Compute` のレベル計算・予測日数・経過日数・袋換算。level と同等の結果になることを検証 |
| `internal/record` | `AddBags` の加算・`purchases` 追加・bags<=0 拒否・イミュータビリティ |
| `internal/model` | `Purchase`/`Summary` の JSON ラウンドトリップ |
| ウェブUI | 手動確認（ブラウザ）。JS テストは YAGNI で追加しない |

目標カバレッジ 80%以上（Go 側）。

## 14. セットアップ追加手順（README へ追記）

1. GitHub リポジトリ Settings → Pages → Source を「GitHub Actions」に設定。
2. `record → buy` で袋数を入力（1袋＝`config.json` の `bag_grams`）。
3. Pages の URL（`https://<user>.github.io/bean-watcher/`）にアクセスしてダッシュボード確認。

## 15. スコープ外（YAGNI）

- ウェブからの入力（書き込み）。入力は record ワークフロー継続。
- 複数銘柄の袋グラム違い管理（`bag_grams` は全局共通）。
- ダークモード・レスポンシブの高度な調整（最小限のモバイル対応のみ）。
- 認証（ダッシュボードは公開情報のみ）。
- リアルタイム更新（Pages はデプロイ時点のスナップショット）。

## 16. 用語集
- **bag_grams**: 1袋あたりの豆のグラム数。
- **summary**: Go が計算して data.json に書き出す表示用サマリ。JS はこれを読むだけ。
