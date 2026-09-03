# AC-9-g（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、Google の ID プロバイダが受け取るクライアントシークレットが、変数（google_client_secret）から来ており、リテラルの直書きでないこと（AC-4-2・AC-4-3） だけである。
#
# mock_provider を用い、実際の AWS API を呼ばない（P-6・P-9）。
#
# リソース名・変数名はこのテストが暫定的に固定するインターフェースである。
# 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。
#
# 限界（tester 工程の実測）: 変数の `sensitive = true` 宣言・既定値の
# 不在（AC-4-3）は、.tftest.hcl の assert condition から機械的に検査
# できない（実験で確認済み。missing value のエラーは run 全体を落とし、
# expect_failures では捕捉できない。sensitive 属性は plan/state の値に
# 現れない）。本 run が検査するのは値の出所（変数経由であること）まで
# である。この限界は AC-11-1 と同じ性質（作らないこと・宣言の属性は
# 機械検査できない）であり、reviewer と規律に委ねる。

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

run "google_client_secret_is_taken_from_variable" {
  command = plan

  # 変数から来ていること（リテラルの直書きでないこと）だけを見る。値そのもの
  # を期待値として書かない、という要求（P-4）は、ダミー値であっても var.
  # 経由で比較することで満たす（リテラルを2重に書かない）。
  assert {
    condition     = aws_cognito_identity_provider.google.provider_details["client_secret"] == var.google_client_secret
    error_message = "クライアントシークレットは変数（google_client_secret）から受け取らなければならない。値の実体は本 assert に書かない（AC-4-2）"
  }
}
