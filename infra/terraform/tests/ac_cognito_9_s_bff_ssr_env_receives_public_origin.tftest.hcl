# AC-9-s（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、BFF・SSR Lambda の環境変数に公開オリジンが注入されており（未設定でなく、空でなく）、その値が当該変数（public_origin）から与えられていること（リテラルの直書き・暫定値でないこと。AC-8-8・AC-8-9） だけである。
#
# mock_provider を用い、実際の AWS API を呼ばない（P-6・P-9）。
#
# リソース名・変数名はこのテストが暫定的に固定するインターフェースである。
# 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。
#
# **値そのものを assert しない**（AC-9-s。ダミーであれ期待値として書かない
# ＝P-4・9-g と同型）。右辺は var.public_origin の参照であり、リテラルの
# URL ではない。
#
# 限界（AC-9-s・AC-11-15）: 変数の宣言側（既定値を持たないこと＝AC-8-9 (i)）は
# 本 run では観測できず、AC-9 前文の例外は AC-9-g / AC-9-r の 2 条に限るため、
# 本条を静的チェッカ（AC-12）へ足さない。帰結として「既定値を持たない」は
# 機械検査されず、担保はレビューと規律である（要求としては緩めない）。
# あわせて、ここで渡した公開オリジンと Cognito に登録された戻り先
# （AC-5-3・AC-9-i）が同じ公開オリジンを指すことも検査されない（AC-11-15）。

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

run "bff_ssr_env_receives_public_origin" {
  command = plan

  # BFF・SSR Lambda の environment.variables には、この構成が作る User
  # Pool・アプリクライアントの id が混ざる（COGNITO_USER_POOL_ID・
  # COGNITO_CLIENT_ID）。これらは provider が採番する computed 値であり、
  # plan 時点では unknown になる（ac_cognito_9_m と同型の理由）。ダミーの
  # ID で override_during = plan により確定させる。environment 自体・
  # var.public_origin は override しない（本 assert が実際に検査している
  # 対象であるため）。
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
    condition = contains(
      values(try(aws_lambda_function.bff_ssr.environment[0].variables, {})),
      var.public_origin
    )
    error_message = "BFF・SSR Lambda の環境変数は、サインインの戻り先の組み立てに要する公開オリジンを変数（public_origin）から受け取らなければならない。リテラルの直書き・暫定値でない（AC-8-8・AC-8-9）"
  }
}
