# AC-9-i（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、戻り先が変数から与えられており、リテラルの直書きでないこと（AC-5-3）。ワイルドカード等で任意の URL を許す形でないこと（AC-5-4） だけである。
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

run "callback_urls_come_from_variable_without_wildcard" {
  command = plan

  assert {
    condition     = toset(aws_cognito_user_pool_client.bff.callback_urls) == toset(var.cognito_callback_urls)
    error_message = "戻り先（callback_urls）は変数（cognito_callback_urls）から与えられていなければならない。リテラルの直書きでない（AC-5-3）"
  }

  assert {
    condition     = !anytrue([for u in aws_cognito_user_pool_client.bff.callback_urls : strcontains(u, "*")])
    error_message = "戻り先はワイルドカード等で任意の URL を許す形であってはならない（AC-5-4。オープンリダイレクタを作らない）"
  }
}
