# AC-9-k（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、脅威対策・高度なセキュリティに相当する設定が有効でないこと、および課金階層の属性が上位でないこと（AC-7-2） だけである。
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

run "advanced_security_is_not_enabled" {
  command = plan

  assert {
    condition = (
      length(aws_cognito_user_pool.this.user_pool_add_ons) == 0 ||
      one(aws_cognito_user_pool.this.user_pool_add_ons).advanced_security_mode == "OFF"
    )
    error_message = "脅威対策・高度なセキュリティ機能（advanced_security_mode）を有効にしてはならない（AC-7-2）"
  }

  # user_pool_tier は provider スキーマ上 optional かつ computed であり、
  # 構成側が明示していなければ plan 時点で unknown のままで比較が評価
  # できない。上位階層でないことを検査するには、構成側が値を明示している
  # ことを要する（AC-2-4 と同型）。
  assert {
    condition     = aws_cognito_user_pool.this.user_pool_tier != "PLUS"
    error_message = "User Pool の課金階層（user_pool_tier）は上位（PLUS。脅威対策を含む階層）であってはならない（AC-7-2）"
  }
}
