# AC-9-j（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、有効期限に相当する属性が構成側に明示されている（存在し、provider の既定へ委ねていない）こと。値の妥当性までは検査しない（AC-5-5） だけである。
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

run "token_validity_is_explicit" {
  command = plan

  # access_token_validity は provider スキーマ上 optional かつ computed
  # (provider 側の既定へ委ねられる)属性である。構成側が明示的に値を与えて
  # いなければ plan 時点で値は unknown のままであり、"!= null" の比較すら
  # 評価できず run がエラーで落ちる（構成側が明示していないことの検出＝
  # AC-2-4 と同型の「provider の既定へ暗黙に委ねない」の裏返し）。
  assert {
    condition     = aws_cognito_user_pool_client.bff.access_token_validity != null
    error_message = "トークンの有効期限（access_token_validity）を構成側に明示しなければならない。provider の既定へ暗黙に委ねない（AC-5-5）"
  }
}
