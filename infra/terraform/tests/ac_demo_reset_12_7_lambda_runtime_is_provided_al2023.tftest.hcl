# AC-12-7（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、リセット用 Lambda（4本目）の runtime が
# provided.al2023 であることだけである。成果物の別変数は ac_10_4_q、
# 予約同時実行数は ac_10_4_b、VPC は ac_10_4_m が持つ。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
# （aws_lambda_function.demo_data_reset）

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

# --- AC-12-7: リセット用 Lambda の runtime（provided.al2023） --------------
run "demo_data_reset_runtime_is_provided_al2023" {
  command = plan

  assert {
    condition     = aws_lambda_function.demo_data_reset.runtime == "provided.al2023"
    error_message = "リセット用 Lambda の runtime は provided.al2023 でなければならない（demo-data-reset.md AC-12-7）。暫定値・空文字は不可"
  }
}
