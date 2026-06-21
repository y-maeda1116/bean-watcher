# bean-watcher

全自動コーヒーメーカーの豆の残量とメンテナンス時期を管理し、Discord に通知するツール。GitHub Actions の定期実行とリポジトリ内 JSON だけで完全無料で運用できます。

## できること

- ☕ **豆の買い時通知**: 過去7日の消費ペースから残り日数を予測し、「あと約5日」「あと約2日」で Discord に通知。
- 🧽 **掃除タイムング通知**: クエン酸洗浄・ミル掃除を「経過日数」または「抽出回数」のどちらか超過で通知。
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
