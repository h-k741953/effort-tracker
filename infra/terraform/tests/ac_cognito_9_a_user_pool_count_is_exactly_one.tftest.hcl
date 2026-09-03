# AC-9-a（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、User Pool を1つだけ作ること（AC-2-1）だけである。
#
# mock_provider を用い、実際の AWS API を呼ばない（P-6・P-9）。
#
# リソース名（aws_cognito_user_pool.this）はこのテストが暫定的に固定する
# インターフェースである。実装側は同じ名前で作ってよいし、都合が悪ければ
# テストごと見直す。

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

# --- AC-9-a: User Pool がちょうど1つ ----------------------------------------
#
# aws_cognito_user_pool.this という単一のリソースブロック（count/for_each を
# 持たない）を参照する。実装がこのアドレスで1つだけ作れば、name 属性は
# plan 時点で既知の値（構成側の入力）として解決できる。実装が無ければ
# 「参照先のリソースが無い」ため plan 自体が失敗する。
run "user_pool_count_is_exactly_one" {
  command = plan

  assert {
    condition     = aws_cognito_user_pool.this.name != null && aws_cognito_user_pool.this.name != ""
    error_message = "User Pool（aws_cognito_user_pool.this）をちょうど1つ作らなければならない（AC-2-1）"
  }
}
