# AC-9-n（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、BFF・SSR Lambda の環境変数に、クライアントシークレットの変数の値が現れないこと（AC-8-2・AC-5-7） だけである。
#
# mock_provider を用い、実際の AWS API を呼ばない（P-6・P-9）。
#
# リソース名・変数名はこのテストが暫定的に固定するインターフェースである。
# 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。
#
# 実測（tester 工程）: aws_lambda_function.bff_ssr は #8 で既に存在するため、
# 「environment が空である」状態でも !contains(...) は真になり（空集合には
# 何も含まれないため）、実装が無いまま本 assert が pass してしまう
# （vacuous true）。これでは何も検証していないのと同じであるため、
# 「environment に値が実際に入っていること」を assert の前提として明示的に
# 併記し、実装が無い間は Red になるようにする。

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

run "bff_ssr_env_excludes_google_client_secret" {
  command = plan

  # BFF・SSR Lambda の environment.variables には、この構成が作る User
  # Pool・アプリクライアントの id が混ざる（COGNITO_USER_POOL_ID・
  # COGNITO_CLIENT_ID）。これらは provider が採番する computed 値であり、
  # plan 時点では unknown になる（ac_cognito_9_m と同型の理由）。ダミーの
  # ID で override_during = plan により確定させる。environment 自体・
  # var.google_client_secret・var.role_cookie_signing_key は override しない
  # （本 assert が実際に検査している対象であるため）。
  override_resource {
    target = aws_cognito_user_pool.this
    values = {
      id  = "ap-northeast-1_dummyPoolId"
      arn = "arn:aws:cognito-idp:ap-northeast-1:123456789012:userpool/ap-northeast-1_dummyPoolId"
    }
    override_during = plan
  }

  override_resource {
    target = aws_cognito_user_pool_client.bff
    values = {
      id = "dummyclientid0123456789abcdef"
    }
    override_during = plan
  }

  assert {
    condition = (
      length(try(aws_lambda_function.bff_ssr.environment[0].variables, {})) > 0
      && !contains(
        values(try(aws_lambda_function.bff_ssr.environment[0].variables, {})),
        var.google_client_secret
      )
    )
    error_message = "BFF・SSR Lambda の環境変数に、Google のクライアントシークレットの値が現れてはならない（AC-8-2・AC-5-7）。署名鍵（role_cookie_signing_key）は本 assert の対象に含めない（AC-9-n の除外・AC-8-6）。environment に値が1件も無い場合も、この assert は未実装として失敗する（vacuous true の回避）"
  }
}
