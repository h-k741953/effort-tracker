# AC-9-o（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、削除保護に相当する属性が構成側に明示されており、その値が「有効」でないこと（AC-2-6・Q-5 = (b)） だけである。
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

run "deletion_protection_is_not_active" {
  command = plan

  assert {
    condition     = aws_cognito_user_pool.this.deletion_protection != null
    error_message = "削除保護（deletion_protection）を構成側に明示しなければならない。provider の既定へ暗黙に委ねない（AC-2-6）"
  }

  assert {
    condition     = aws_cognito_user_pool.this.deletion_protection != "ACTIVE"
    error_message = "削除保護（deletion_protection）は無効でなければならない（Q-5 = (b)。日次リセット前提での作り直しやすさを優先する）"
  }
}
