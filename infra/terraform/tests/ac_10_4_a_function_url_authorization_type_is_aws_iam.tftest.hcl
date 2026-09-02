# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、BFF/SSR とドメイン API の2つの Lambda Function URL の
# authorization_type だけである。期待値の AWS_IAM（NONE 禁止）は
# docs/rules/cost-guardrails.md を読んで書いている（AC-10-4）。値を infra 側へ
# 二重に定義しない（AC-5-6）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# 構成（*.tf のリソース定義）は実装済みであり、各 run はその構成に対して実際に
# 評価される（AC-10-3 の Red は解消済み）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。

mock_provider "aws" {}

variables {
  aws_region                                 = "ap-northeast-1"
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "/effort-tracker/test/neon-connection-string"
  bff_ssr_lambda_artifact_path               = "testdata/bff-ssr.zip"
  domain_api_lambda_artifact_path            = "testdata/domain-api.zip"
  cloudfront_killswitch_lambda_artifact_path = "testdata/cloudfront-killswitch.zip"
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
}

# --- AC-10-4-a: 両 Function URL の authorization_type -----------------------
run "function_url_authorization_type_is_aws_iam" {
  command = plan

  assert {
    condition     = aws_lambda_function_url.bff_ssr.authorization_type == "AWS_IAM"
    error_message = "BFF/SSR Lambda の Function URL は authorization_type = AWS_IAM でなければならない（NONE 禁止。cost-guardrails.md）"
  }

  assert {
    condition     = aws_lambda_function_url.domain_api.authorization_type == "AWS_IAM"
    error_message = "ドメイン API Lambda の Function URL は authorization_type = AWS_IAM でなければならない（NONE 禁止。cost-guardrails.md）"
  }
}
