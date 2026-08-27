# ドメイン API Lambda（provided.al2023。エントリポイントは
# services/api/cmd/bootstrap＝D-5）。Function URL は CloudFront の
# オリジンにしない（AC-4-4。ブラウザから到達しうる経路を作らない）。

resource "aws_iam_role" "domain_api" {
  name               = "${local.domain_api_function_name}-role"
  assume_role_policy = local.lambda_assume_role_policy
}

# AC-7-3・AC-8-3-1・AC-8-3-2: SSM の当該パラメータの取得と復号のみ。
resource "aws_iam_role_policy" "domain_api_ssm" {
  name = "get-neon-connection-string-parameter"
  role = aws_iam_role.domain_api.id

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

# AC-7-4: ログ出力は自身のロググループへの書き込みに限定する。
resource "aws_iam_role_policy" "domain_api_logs" {
  name = "write-own-log-group"
  role = aws_iam_role.domain_api.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.domain_api.arn}:*"
      }
    ]
  })
}

resource "aws_cloudwatch_log_group" "domain_api" {
  name              = "/aws/lambda/${local.domain_api_function_name}"
  retention_in_days = 14 # docs/rules/cost-guardrails.md
}

resource "aws_lambda_function" "domain_api" {
  function_name = local.domain_api_function_name
  role          = aws_iam_role.domain_api.arn
  runtime       = "provided.al2023"
  handler       = "bootstrap"
  filename      = var.domain_api_lambda_artifact_path

  reserved_concurrent_executions = 5 # docs/rules/cost-guardrails.md

  environment {
    variables = {
      # AC-8-1: 環境変数が持つのは SSM パラメータの名前であり、
      # 接続文字列そのものではない。
      NEON_CONNECTION_STRING_SSM_PARAMETER_NAME = var.ssm_parameter_name
    }
  }

  depends_on = [aws_cloudwatch_log_group.domain_api]
}

# AC-4-3: Function URL は AWS_IAM 認証(NONE を選ばない)。呼び出せるのは
# BFF・SSR Lambda の実行ロールのみ(AC-7-1)。CloudFront のオリジンには
# しない(AC-4-4)。
resource "aws_lambda_function_url" "domain_api" {
  function_name      = aws_lambda_function.domain_api.function_name
  authorization_type = "AWS_IAM"
}
