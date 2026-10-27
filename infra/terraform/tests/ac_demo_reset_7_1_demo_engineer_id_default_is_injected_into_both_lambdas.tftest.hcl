# AC-7-1・AC-7-5・AC-14-3（docs/specs/demo-data-reset.md）の検査 — 既定値の面。
#
# 本ファイルが検査するのは、変数 demo_engineer_id を**明示的に与えなかったとき**
# （＝変数の既定値が使われるとき）にも、(1) 値が空でなく、(2) Cognito の sub が
# 取りうる形（UUID の形）に当たらず、(3) BFF/SSR Lambda とリセット用 Lambda の
# 環境変数 DEMO_ENGINEER_ID の両方へ、同じ変数の値が注入されることである。
#
# **同じ AC-7-1・7-5 を検査するファイルがもう1本ある。**
# ac_demo_reset_7_1_demo_engineer_id_override_is_injected_into_both_lambdas
# がそれで、あちらは demo_engineer_id を**与えたうえで**同じ注入を評価する。
# 他の run は多くが variables ブロックで他の変数を上書きしており、既定値を
# 上書きすると既定値そのものは一度も plan に現れない。本ファイルは
# demo_engineer_id を上書きしないことで、既定値の面を検査に載せる。
# （値そのものは期待値に書かない。値の持ち主は構成側である＝AC-7-3）
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
# BFF/SSR Lambda の environment.variables には Cognito の id などの computed な値
# も混ざるため、ac_10_4_z と同じく override_resource で確定させる。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
# （variable "demo_engineer_id" / 環境変数名 DEMO_ENGINEER_ID）

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

# demo_engineer_id は**意図的に与えない**（既定値を評価させるため）。
# --- AC-7-1・7-2・7-5: 既定値の注入と形 --------------------------------------
run "demo_engineer_id_default_is_injected_into_both_lambdas_and_is_not_uuid_shaped" {
  command = plan

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
    condition     = length(var.demo_engineer_id) > 0
    error_message = "demo_engineer_id の既定値は空であってはならない（AC-7-1）"
  }

  assert {
    condition     = !can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", var.demo_engineer_id))
    error_message = "demo_engineer_id の既定値は UUID の形（Cognito の sub が取りうる形）に当たってはならない（AC-7-2）"
  }

  assert {
    condition     = aws_lambda_function.bff_ssr.environment[0].variables["DEMO_ENGINEER_ID"] == var.demo_engineer_id
    error_message = "BFF/SSR Lambda の環境変数 DEMO_ENGINEER_ID は変数 demo_engineer_id の値でなければならない（AC-7-1・7-5）"
  }

  assert {
    condition     = aws_lambda_function.demo_data_reset.environment[0].variables["DEMO_ENGINEER_ID"] == var.demo_engineer_id
    error_message = "リセット用 Lambda の環境変数 DEMO_ENGINEER_ID は変数 demo_engineer_id の値でなければならない（AC-7-1・7-5）"
  }

  assert {
    condition     = aws_lambda_function.bff_ssr.environment[0].variables["DEMO_ENGINEER_ID"] == aws_lambda_function.demo_data_reset.environment[0].variables["DEMO_ENGINEER_ID"]
    error_message = "BFF/SSR とリセット用 Lambda の DEMO_ENGINEER_ID は同じ値でなければならない（出所を2つにしない＝AC-7-1）"
  }
}
