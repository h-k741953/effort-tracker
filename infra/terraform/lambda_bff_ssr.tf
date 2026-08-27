# BFF・SSR Lambda（Next.js Route Handler / OpenNext SSR）。
# ランタイムは Node のマネージドランタイム（AC-4-7・D-16。版は変数で
# 受け取り、apps/web が固定する版へ実 apply 時に一致させる）。
# OpenNext の導入・ビルド設定自体は本 Issue の非スコープ（D-13）。

resource "aws_iam_role" "bff_ssr" {
  name               = "${local.bff_ssr_function_name}-role"
  assume_role_policy = local.lambda_assume_role_policy
}

# AC-7-1: lambda:InvokeFunctionUrl はドメイン API の当該関数のみに限定する。
resource "aws_iam_role_policy" "bff_ssr_invoke_domain_api" {
  name = "invoke-domain-api-function-url"
  role = aws_iam_role.bff_ssr.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "lambda:InvokeFunctionUrl"
        Resource = aws_lambda_function.domain_api.arn
      }
    ]
  })
}

# AC-7-4: ログ出力は自身のロググループへの書き込みに限定する。
resource "aws_iam_role_policy" "bff_ssr_logs" {
  name = "write-own-log-group"
  role = aws_iam_role.bff_ssr.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.bff_ssr.arn}:*"
      }
    ]
  })
}

# AC-6-1・AC-6-3: 暗黙生成させず、関数名と食い違わない名前で明示的に作る。
resource "aws_cloudwatch_log_group" "bff_ssr" {
  name              = "/aws/lambda/${local.bff_ssr_function_name}"
  retention_in_days = 14 # docs/rules/cost-guardrails.md
}

resource "aws_lambda_function" "bff_ssr" {
  function_name = local.bff_ssr_function_name
  role          = aws_iam_role.bff_ssr.arn
  runtime       = var.bff_ssr_lambda_runtime
  handler       = "index.handler"
  filename      = var.bff_ssr_lambda_artifact_path

  reserved_concurrent_executions = 5 # docs/rules/cost-guardrails.md

  depends_on = [aws_cloudwatch_log_group.bff_ssr]
}

# AC-4-3: Function URL は AWS_IAM 認証(NONE を選ばない)。
resource "aws_lambda_function_url" "bff_ssr" {
  function_name      = aws_lambda_function.bff_ssr.function_name
  authorization_type = "AWS_IAM"
}

# CloudFront(OAC・SigV4 署名)からこの Function URL への到達を許可する。
resource "aws_lambda_permission" "bff_ssr_cloudfront" {
  statement_id           = "AllowCloudFrontOACInvoke"
  action                 = "lambda:InvokeFunctionUrl"
  function_name          = aws_lambda_function.bff_ssr.function_name
  principal              = "cloudfront.amazonaws.com"
  source_arn             = aws_cloudfront_distribution.main.arn
  function_url_auth_type = "AWS_IAM"
}
