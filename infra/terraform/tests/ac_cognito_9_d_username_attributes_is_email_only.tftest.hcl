# AC-9-d（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、メールアドレスでサインインできる設定であること。電話番号がサインインの識別子に含まれないこと（AC-2-2） だけである。
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
}

run "username_attributes_is_email_only" {
  command = plan

  assert {
    condition     = contains(aws_cognito_user_pool.this.username_attributes, "email")
    error_message = "メールアドレス（email）でサインインできなければならない（AC-2-2）"
  }

  assert {
    condition     = !contains(aws_cognito_user_pool.this.username_attributes, "phone_number")
    error_message = "電話番号（phone_number）をサインインの識別子に含めてはならない（AC-2-2・AC-3-2）"
  }
}
