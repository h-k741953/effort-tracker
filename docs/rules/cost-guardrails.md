# コストガードレール

デモ公開の前提条件。実装時に省略しない。根拠は `docs/adr/0013`（ホスティング）`docs/adr/0003`（IAM 認証）。

| 項目 | 設定 |
|---|---|
| Lambda 予約同時実行数 | BFF/SSR・ドメイン API の**両 Lambda**は `reserved_concurrent_executions = 5`。**CloudFront 遮断 Lambda**（下記の従量遮断回路）のみ `reserved_concurrent_executions = 1`。**デモデータのリセット用 Lambda**（下記「デモデータ」の行。ADR 0022）は `reserved_concurrent_executions = 1`（2026-10-27 追加。上の「のみ」は BFF/SSR・ドメイン API の `5` との対比であり、リセット用 Lambda を 1 から除く意味ではない。4本とも明示し、未設定を残さない） |
| Lambda Function URL | `authorization_type = "AWS_IAM"`（`NONE` は禁止） |
| AWS Budgets | 月 $5 で通知（実績80% / 予測100% の2本） |
| Budget Actions | 閾値到達で IAM / SCP の deny を適用（新規リソース作成を凍結。翌予算期に自動リバート） |
| CloudFront | ディストリビューション単位のハード上限は無い。従量遮断は Budget→SNS→Lambda で distribution 無効化（**数時間遅延あり**）。無料枠 1TB / 1000万req/月 |
| CloudWatch Logs | ロググループの保持期間 `retention_in_days = 14`（全 Lambda。無期限保持にしない） |
| 使用禁止 | NAT Gateway / ALB / RDS / ECS |
| デモデータ | 日次リセット（EventBridge Scheduler から呼ぶリセット専用の Lambda で行う。ADR 0022・`docs/specs/demo-data-reset.md`） |
| Route Handler | レート制限（CloudFront 帯域の一次防御。WAF は $5 予算と二律背反のため非必須） |
