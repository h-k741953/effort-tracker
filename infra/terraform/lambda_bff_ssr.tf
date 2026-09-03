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

  # AC-8-1（cognito-auth-infra.md）: BFF がトークンを検証するために要する
  # 識別子（User Pool・アプリクライアント・リージョン）をこの構成が作る
  # Cognito リソースの属性から注入する（リテラルの再入力をしない＝AC-8-3）。
  # AC-8-6・AC-8-7: ロール切替 Cookie の署名鍵も、Q-F = (b) の決定により
  # 同じ環境変数へ注入する（infra-terraform.md AC-8-1 / D-12 が確立した
  # 「環境変数は SSM パラメータ名だけを持つ」型からの、署名鍵1つに限る
  # 意図的な逸脱。他の機密はこの型から外さない＝AC-8-2・AC-5-7）。
  environment {
    variables = {
      COGNITO_USER_POOL_ID    = aws_cognito_user_pool.this.id
      COGNITO_CLIENT_ID       = aws_cognito_user_pool_client.bff.id
      COGNITO_REGION          = var.aws_region
      COGNITO_DOMAIN_PREFIX   = var.cognito_domain_prefix
      ROLE_COOKIE_SIGNING_KEY = var.role_cookie_signing_key
      # AC-8-8・AC-8-9（cognito-auth-infra.md）: サインインの戻り先の組み立てに
      # 用いる公開オリジンも、同じ environment ブロック内で注入する
      # （bff-auth-termination.md AC-7-6。要求ヘッダから導かない代わり）。
      PUBLIC_ORIGIN = var.public_origin
    }
  }

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
