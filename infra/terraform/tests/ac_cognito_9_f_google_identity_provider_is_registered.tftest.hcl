# AC-9-f（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、Google のソーシャル ID プロバイダが1つ存在し、当該 User Pool に属していること（AC-4-1）。属性のマッピングが空でないこと（AC-4-4） だけである。
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

run "google_identity_provider_is_registered" {
  command = plan

  assert {
    condition     = aws_cognito_identity_provider.google.user_pool_id == aws_cognito_user_pool.this.id
    error_message = "Google の ID プロバイダ（aws_cognito_identity_provider.google）は、この構成が作る User Pool に属していなければならない（AC-4-1）"
  }

  assert {
    condition     = aws_cognito_identity_provider.google.provider_type == "Google"
    error_message = "provider_type は Google でなければならない（AC-4-1）"
  }

  assert {
    condition     = length(aws_cognito_identity_provider.google.attribute_mapping) > 0
    error_message = "Google から受け取る属性を User Pool の属性へ明示的に対応づけなければならない（attribute_mapping が空でないこと。AC-4-4）"
  }
}
