# AC-9-m（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、BFF・SSR Lambda の環境変数に、この構成が作る User Pool とアプリクライアントの識別子が注入されていること。未設定でなく、空でなく、暫定値・無関係な値でないこと（AC-8-1・AC-8-3） だけである。
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

run "bff_ssr_env_receives_user_pool_and_client_ids" {
  command = plan

  # User Pool・アプリクライアントの id は provider が採番する computed 値で
  # あり、plan 時点では unknown になる（ac_10_4_s と同型の理由）。ダミーの
  # ID で override_during = plan により確定させる。
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
      aws_cognito_user_pool.this.id
    )
    error_message = "BFF・SSR Lambda の環境変数に、この構成が作る User Pool の識別子が注入されていなければならない（AC-8-1・AC-8-3）"
  }

  assert {
    condition = contains(
      values(try(aws_lambda_function.bff_ssr.environment[0].variables, {})),
      aws_cognito_user_pool_client.bff.id
    )
    error_message = "BFF・SSR Lambda の環境変数に、この構成が作るアプリクライアントの識別子が注入されていなければならない（AC-8-1・AC-8-3）"
  }
}
