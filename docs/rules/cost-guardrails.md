# コストガードレール

デモ公開の前提条件。実装時に省略しない。根拠は `docs/adr/0002` `docs/adr/0003`。

| 項目 | 設定 |
|---|---|
| Lambda 予約同時実行数 | `reserved_concurrent_executions = 5` |
| Lambda Function URL | `authorization_type = "AWS_IAM"`（`NONE` は禁止） |
| AWS Budgets | 月 $5 で通知（実績80% / 予測100% の2本） |
| 使用禁止 | NAT Gateway / ALB / RDS / ECS |
| デモデータ | 日次リセット |
| Route Handler | レート制限 |
