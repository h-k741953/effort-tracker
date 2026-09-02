# AC-9-b（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、MFA の設定が無効でないこと・TOTP が有効であること・MFA の設定が「任意」に相当する値であり「必須」でないこと（AC-3-1・AC-3-3・Q-2 = (b)） だけである。
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

run "mfa_is_optional_totp" {
  command = plan

  assert {
    condition     = aws_cognito_user_pool.this.mfa_configuration == "OPTIONAL"
    error_message = "MFA の設定は「任意」（OPTIONAL 相当）でなければならない。「無効」（OFF）でも「必須」（ON）でもない（AC-3-1・AC-3-3・Q-2 = (b)）"
  }

  assert {
    condition = (
      length(aws_cognito_user_pool.this.software_token_mfa_configuration) > 0 &&
      one(aws_cognito_user_pool.this.software_token_mfa_configuration).enabled == true
    )
    error_message = "TOTP（ソフトウェアトークン MFA）を有効にしなければならない（AC-3-1）"
  }
}
