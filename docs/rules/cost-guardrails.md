# コストガードレール

デモ公開の前提条件。実装時に省略しない。根拠は `docs/adr/0013`（ホスティング）`docs/adr/0003`（IAM 認証）。

| 項目 | 設定 |
|---|---|
| Lambda 予約同時実行数 | `reserved_concurrent_executions = 5`（BFF/SSR・ドメイン API の**両 Lambda**） |
| Lambda Function URL | `authorization_type = "AWS_IAM"`（`NONE` は禁止） |
| AWS Budgets | 月 $5 で通知（実績80% / 予測100% の2本） |
| Budget Actions | 閾値到達で IAM / SCP の deny を適用（新規リソース作成を凍結。翌予算期に自動リバート） |
| CloudFront | ディストリビューション単位のハード上限は無い。従量遮断は Budget→SNS→Lambda で distribution 無効化（**数時間遅延あり**）。無料枠 1TB / 1000万req/月 |
| 使用禁止 | NAT Gateway / ALB / RDS / ECS |
| デモデータ | 日次リセット |
| Route Handler | レート制限（CloudFront 帯域の一次防御。WAF は $5 予算と二律背反のため非必須） |
