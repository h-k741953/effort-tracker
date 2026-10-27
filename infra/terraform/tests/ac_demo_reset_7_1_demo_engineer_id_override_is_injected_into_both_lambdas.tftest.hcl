# AC-7-1・AC-7-5・AC-14-3（docs/specs/demo-data-reset.md）の検査 — 上書きした値の面。
#
# 本ファイルが検査するのは、変数 demo_engineer_id に**既定値と異なる値を与えたとき**、
# BFF/SSR Lambda とリセット用 Lambda の環境変数 DEMO_ENGINEER_ID の**両方**が、
# その与えた値になることである。片方が既定値（または別のリテラル）を直書きして
# いると、既定値だけを見る ac_demo_reset_7_1_*_default では通ってしまうため、
# 既定値と違う値を与えて「出所が1つの変数であること」を固定する（AC-7-1）。
# 注入の面の既定値は ac_demo_reset_7_1_*_default が持つ（片方だけを見て
# 「AC-7-1 は検査済み」と読まないこと）。
#
# 値はダミー（UUID の形でないもの）。実在の識別子ではない。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
# BFF/SSR Lambda の environment.variables には Cognito の id などの computed な値
# も混ざるため、ac_10_4_z と同じく override_resource で確定させる。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。

mock_provider "aws" {}

variables {
  aws_region                                 = "ap-northeast-1"
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "/effort-tracker/test/neon-connection-string"
  bff_ssr_lambda_artifact_path               = "testdata/bff-ssr.zip"
  domain_api_lambda_artifact_path            = "testdata/domain-api.zip"
  cloudfront_killswitch_lambda_artifact_path = "testdata/cloudfront-killswitch.zip"
  demo_data_reset_lambda_artifact_path       = "testdata/demo-data-reset.zip"
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
  public_origin                              = "https://public.example.test"
}

# --- AC-7-1・7-5: 与えた値が両 Lambda へ注入される ---------------------------
run "demo_engineer_id_override_is_injected_into_both_lambdas" {
  command = plan

  variables {
    demo_engineer_id = "demo-override-engineer-id"
  }

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

  override_resource {
    target = aws_lambda_function_url.domain_api
    values = {
      function_url = "https://dummy-domain-api-0000000000.lambda-url.ap-northeast-1.on.aws/"
    }
    override_during = plan
  }

  assert {
    condition     = aws_lambda_function.bff_ssr.environment[0].variables["DEMO_ENGINEER_ID"] == "demo-override-engineer-id"
    error_message = "BFF/SSR Lambda の環境変数 DEMO_ENGINEER_ID は、変数 demo_engineer_id に与えた値でなければならない（既定値・リテラルの直書きは不可＝AC-7-1）"
  }

  assert {
    condition     = aws_lambda_function.demo_data_reset.environment[0].variables["DEMO_ENGINEER_ID"] == "demo-override-engineer-id"
    error_message = "リセット用 Lambda の環境変数 DEMO_ENGINEER_ID は、変数 demo_engineer_id に与えた値でなければならない（既定値・リテラルの直書きは不可＝AC-7-1）"
  }
}
