# デモデータ日次リセットの実行主体（demo-data-reset.md AC-11-5・AC-12・ADR 0022）。
# EventBridge Scheduler（毎日 04:00 JST）-> リセット用 Lambda の配線。
# 他3本とは別の成果物・別の実行ロール・別のロググループを持つ。
# Function URL・リソースベースのポリシーは作らない（起動は Scheduler のロール経由のみ）。
# VPC には置かない（NAT Gateway 禁止＝docs/rules/cost-guardrails.md）。

resource "aws_iam_role" "demo_data_reset" {
  name               = "${local.demo_data_reset_function_name}-role"
  assume_role_policy = local.lambda_assume_role_policy
}

# AC-12-6: 自身のロググループへの書き込みに限定する。
resource "aws_iam_role_policy" "demo_data_reset_logs" {
  name = "write-own-log-group"
  role = aws_iam_role.demo_data_reset.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.demo_data_reset.arn}:*"
      }
    ]
  })
}

# AC-12-6: SSM の当該パラメータの取得と復号のみ（ドメイン API と同じ形）。
resource "aws_iam_role_policy" "demo_data_reset_ssm" {
  name = "get-neon-connection-string-parameter"
  role = aws_iam_role.demo_data_reset.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "ssm:GetParameter"
        Resource = "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${var.ssm_parameter_name}"
      },
      {
        Effect   = "Allow"
        Action   = "kms:Decrypt"
        Resource = data.aws_kms_alias.ssm.target_key_arn
      }
    ]
  })
}

resource "aws_cloudwatch_log_group" "demo_data_reset" {
  name              = "/aws/lambda/${local.demo_data_reset_function_name}"
  retention_in_days = 14 # docs/rules/cost-guardrails.md
}

resource "aws_lambda_function" "demo_data_reset" {
  function_name = local.demo_data_reset_function_name
  role          = aws_iam_role.demo_data_reset.arn
  runtime       = "provided.al2023"
  handler       = "bootstrap"
  filename      = var.demo_data_reset_lambda_artifact_path

  reserved_concurrent_executions = 1 # docs/rules/cost-guardrails.md（遮断 Lambda と同じ1）

  # AC-12-9: 環境変数はこの2つだけ。接続文字列そのものは置かない。
  environment {
    variables = {
      NEON_CONNECTION_STRING_SSM_PARAMETER_NAME = var.ssm_parameter_name
      DEMO_ENGINEER_ID                          = var.demo_engineer_id
    }
  }

  depends_on = [aws_cloudwatch_log_group.demo_data_reset]
}

# AC-11-5: 非同期呼び出しの再試行を 0 にする（既定は 2 回）。
resource "aws_lambda_function_event_invoke_config" "demo_data_reset" {
  function_name          = aws_lambda_function.demo_data_reset.function_name
  maximum_retry_attempts = 0
}

# AC-12-3: Scheduler が引き受けるロール。信頼は scheduler.amazonaws.com だけ。
resource "aws_iam_role" "demo_data_reset_scheduler" {
  name = "${local.demo_data_reset_function_name}-scheduler-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Action    = "sts:AssumeRole"
        Principal = { Service = "scheduler.amazonaws.com" }
      }
    ]
  })
}

# AC-12-3: 権限はリセット用 Lambda の起動だけ。
resource "aws_iam_role_policy" "demo_data_reset_scheduler_invoke" {
  name = "invoke-demo-data-reset"
  role = aws_iam_role.demo_data_reset_scheduler.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "lambda:InvokeFunction"
        Resource = aws_lambda_function.demo_data_reset.arn
      }
    ]
  })
}

# AC-12-2: 毎日 04:00（Asia/Tokyo。UTC 換算の値を書かない）。柔軟な時間枠なし。
# AC-11-5: 再試行 0。スケジュールグループは作らない（既定グループ。AC-7-8）。
resource "aws_scheduler_schedule" "demo_data_reset" {
  name                         = local.demo_data_reset_function_name
  schedule_expression          = "cron(0 4 * * ? *)"
  schedule_expression_timezone = "Asia/Tokyo"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_function.demo_data_reset.arn
    role_arn = aws_iam_role.demo_data_reset_scheduler.arn

    retry_policy {
      maximum_retry_attempts = 0
    }
  }
}
