# AC-9-p（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、アプリクライアントが許可する認証フローに認可コードが含まれ、implicit が含まれないこと。フローの列挙が「認可コードのみ」に収まっていること（AC-5-6 (iii)） だけである。
#
# mock_provider を用い、実際の AWS API を呼ばない（P-6・P-9）。
#
# リソース名・変数名はこのテストが暫定的に固定するインターフェースである。
# 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。

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
  public_origin                              = "https://public.example.test"
}

run "allowed_oauth_flows_is_authorization_code_only" {
  command = plan

  assert {
    condition     = contains(aws_cognito_user_pool_client.bff.allowed_oauth_flows, "code")
    error_message = "許可する認証フローに認可コード（code）を含めなければならない（AC-5-6 (iii)）"
  }

  assert {
    condition     = !contains(aws_cognito_user_pool_client.bff.allowed_oauth_flows, "implicit")
    error_message = "許可する認証フローに implicit を含めてはならない（AC-5-6 (iii)）"
  }

  assert {
    condition     = toset(aws_cognito_user_pool_client.bff.allowed_oauth_flows) == toset(["code"])
    error_message = "許可する認証フローの列挙は「認可コードのみ」に収まっていなければならない（AC-5-6 (iii)）"
  }
}
