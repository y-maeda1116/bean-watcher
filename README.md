# bean-watcher

全自動コーヒーメーカーの豆の残量とメンテナンス時期を管理し、Discord に通知しつつ、ウェブダッシュボードでもいつでも確認できるツール。GitHub Actions とリポジトリ内 JSON だけで完全無料で運用できます。

## できること

- ☕ **豆の買い時通知**: 過去7日の消費ペースから残り日数を予測し、「あと約5日」「あと約2日」で Discord に通知。
- 🛒 **袋単位で管理**: 豆の購入を「袋」で記録（1袋＝`bag_grams`）。残量はグラムと袋の両方で表示。
- 🧽 **掃除タイミング通知**: クエン酸洗浄・ミル掃除を「経過日数」または「抽出回数」のどちらか超過で通知。
- 🔔 **1日1回のチェック**: 毎朝8時(JST)にcron実行。**レベルが変化した時だけ**通知（毎日同じメッセージは来ない）。
- 🌐 **ウェブダッシュボード**: 残量・買い時予測・減少推移・購入履歴・掃除状態をブラウザで常時確認（GitHub Pages・無料）。

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

### 4. GitHub Pages の有効化（ウェブダッシュボード用）
リポジトリの「Settings」→「Pages」→「Build and deployment」の **Source** を **「GitHub Actions」** に設定。`pages.yml` ワークフローが自動でデプロイします。

### 5. 設定の編集（任意）
`data/config.json` を自分の環境に合わせて変更:
- `grams_per_cup`: 1杯あたりの豆（g）。既定12。
- `bag_grams`: **1袋あたりの豆（g）**。既定200。購入・残量表示はこの単位。
- `low_days_threshold` / `critical_days_threshold`: 豆の警告日数。既定5日 / 2日。
- `maintenance.descaling.threshold_days/shots`: クエン酸洗浄周期。既定30日 / 200杯。
- `maintenance.grinder.threshold_days/shots`: ミル掃除周期。既定7日 / 30杯。

### 6. ワークフローの有効化
GitHub の「Actions」タブを開き、表示されるワークフローを「I understand my workflows, go ahead and enable them」で有効化。

## 使い方

### 入力（GitHub Actions の手動ボタン）
GitHub の「Actions」タブから該当ワークフローを選び「Run workflow」:

| ワークフロー | いつ使う | 入力 |
|---|---|---|
| **record** → `pour` | コーヒーを淹れた | `cups`（既定1） |
| **record** → `clean` | 掃除した | `target`（descaling / grinder） |
| **record** → `buy` | 豆を買った | `bags`（袋数。1袋＝`bag_grams`） |
| **notify** | （定期・手動） | なし。しきい値をチェックして通知 |

### 初回運用開始
1. 「record → `buy`」で `bags` に初期購入袋数（例: 1袋=200g なら `1`）を入れて実行 → 残量を登録。
2. 「record → `clean`」で `target=descaling` と `target=grinder` をそれぞれ1回ずつ実行 → 掃除の基準点を設定（これをしないと掃除通知は出ません）。
3. 以降は毎日コーヒーを淹れるたびに「record → `pour`」を、豆を買ったら「record → `buy`」を、掃除したら「record → `clean`」を記録。notify は毎朝8時に自動実行されます。

### ウェブダッシュボードの見方
`https://<あなたのGitHubユーザー名>.github.io/bean-watcher/` をブラウザで開く（Pages デプロイ後、数分で公開）:
- **残量カード**: 残りグラム / 袋数、ステータス（OK / 買い時 / もうすぐ切れる）
- **減少推移**: 直近7日の杯数グラフ
- **購入履歴**: 直近の購入（袋数・グラム）
- **メンテナンス**: クエン酸洗浄・ミル掃除の状態と経過

ダッシュボードは record/notify の実行で更新された `data/data.json` を表示します（入力は Actions ボタンから）。

## データ復元（data.json が壊れた場合）
`data/data.json` は git で管理されているため、GitHub の「Commits」履歴から1つ前の正常な版に戻せます:
```
git checkout <正常なコミット> -- data/data.json
git commit -m "fix: data.json を復元"
git push
```

## 仕組み
- **record** ワークフロー（手動）: `./bean-watcher record <action>` を実行し `data/data.json` を更新・コミット。残量の袋換算・予測日数なども計算して `summary` に保存。
- **notify** ワークフロー（cron `0 23 * * *` = JST 8時）: `./bean-watcher notify` がレベルを計算。前回通知レベルより厳しい方向への変化があれば Discord 通知し `notify_state` と `summary` をコミット。
- **pages** ワークフロー（`data/**` または `web/**` の push で発火）: `web/` と `data/data.json`・`data/config.json` を GitHub Pages にデプロイ。
- **ci** ワークフロー（push/PR、Go 変更時）: `go vet` と `go test` を実行。

## コスト
GitHub Actions と GitHub Pages の無料枠内で完結（公開リポジトリなら無制限）。外部サーバー・DB不要。

## セキュリティ
- Discord Webhook URL は GitHub Secrets のみ（コード・ログ・`data.json` に出力しない）。
- `data/data.json` は残量・履歴のみの公開情報。ウェブダッシュボードも認証なし（公開情報のため）。
