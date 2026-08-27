# CloudFront 従量遮断回路の実行主体（AC-4-8・AC-5-4・D-17・D-18）。
# Budget -> SNS -> 遮断 Lambda の配線。遮断 Lambda はドメイン API とは
# 別の成果物・別の実行ロール・別のロググループを持つ(AC-4-8)。

resource "aws_iam_role" "cloudfront_killswitch" {
  name               = "${local.cloudfront_killswitch_function_name}-role"
  assume_role_policy = local.lambda_assume_role_policy
}

# AC-7-2: 当該ディストリビューションに対する取得・更新のみ。
# 他の CloudFront 操作(作成・削除・他ディストリビューションの操作)を含めない。
resource "aws_iam_role_policy" "cloudfront_killswitch_cloudfront" {
  name = "disable-distribution"
  role = aws_iam_role.cloudfront_killswitch.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "cloudfront:GetDistributionConfig",
          "cloudfront:UpdateDistribution",
        ]
        Resource = aws_cloudfront_distribution.main.arn
      }
    ]
  })
}

# AC-7-4: ログ出力は自身のロググループへの書き込みに限定する。
resource "aws_iam_role_policy" "cloudfront_killswitch_logs" {
  name = "write-own-log-group"
  role = aws_iam_role.cloudfront_killswitch.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.cloudfront_killswitch.arn}:*"
      }
    ]
  })
}

resource "aws_cloudwatch_log_group" "cloudfront_killswitch" {
  name              = "/aws/lambda/${local.cloudfront_killswitch_function_name}"
  retention_in_days = 14 # docs/rules/cost-guardrails.md
}

resource "aws_lambda_function" "cloudfront_killswitch" {
  function_name = local.cloudfront_killswitch_function_name
  role          = aws_iam_role.cloudfront_killswitch.arn
  runtime       = "provided.al2023" # D-17
  handler       = "bootstrap"
  filename      = var.cloudfront_killswitch_lambda_artifact_path

  reserved_concurrent_executions = 1 # docs/rules/cost-guardrails.md（他2本と異なる値）

  # AC-5-4・AC-9-12・10-4-s: 遮断対象のディストリビューションは、この構成が
  # 作る当該ディストリビューション（aws_cloudfront_distribution.main）を
  # リソース参照で渡す。ID のリテラルを書かない。
  #
  # 環境変数名は services/api/cmd/cloudfront-killswitch/distribution_id_env.go
  # が読む名前と一致していなければならない。この一致は機械検査されない
  # （12-28）。担保はレビューと規律。
  environment {
    variables = {
      CLOUDFRONT_DISTRIBUTION_ID = aws_cloudfront_distribution.main.id
    }
  }

  depends_on = [aws_cloudwatch_log_group.cloudfront_killswitch]
}

# AC-5-4: Budget からの通知を受け取る SNS トピック。起動のイベント源は
# この SNS の1本だけとし、他(EventBridge のスケジュール等)を足さない
# (AC-11-22。「それ以外を持たないこと」は 12-21 により機械検査しない)。
resource "aws_sns_topic" "cost_alert" {
  name = "effort-tracker-cost-alert"
}

resource "aws_sns_topic_subscription" "cloudfront_killswitch" {
  topic_arn = aws_sns_topic.cost_alert.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.cloudfront_killswitch.arn
}

resource "aws_lambda_permission" "cloudfront_killswitch_sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.cloudfront_killswitch.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.cost_alert.arn
}
