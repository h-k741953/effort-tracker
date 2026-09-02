# AC-9-r（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、署名鍵の変数（role_cookie_signing_key）が既定値を持たず変数から与えられていること、および BFF・SSR Lambda の環境変数が当該変数から値を受け取っていること（リテラルの直書き・暫定値でないこと。AC-8-6・AC-8-7） だけである。
#
# mock_provider を用い、実際の AWS API を呼ばない（P-6・P-9）。
#
# リソース名・変数名はこのテストが暫定的に固定するインターフェースである。
# 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。
#
# 限界（tester 工程の実測）: 変数の `sensitive = true` 宣言・既定値の
# 不在（AC-8-7）は、AC-9-g と同じ理由で .tftest.hcl の assert condition
# から機械的に検査できない（実験で確認済み）。本 run が検査するのは
# 値の出所（変数経由であること）までである。

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

run "bff_ssr_env_receives_role_cookie_signing_key" {
  command = plan

  assert {
    condition = contains(
      values(try(aws_lambda_function.bff_ssr.environment[0].variables, {})),
      var.role_cookie_signing_key
    )
    error_message = "BFF・SSR Lambda の環境変数は、ロール切替 Cookie の署名鍵を変数（role_cookie_signing_key）から受け取らなければならない。リテラルの直書き・暫定値でない（AC-8-6）"
  }
}
