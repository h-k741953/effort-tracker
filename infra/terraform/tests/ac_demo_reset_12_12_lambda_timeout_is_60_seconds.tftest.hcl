# AC-12-12・AC-12-10 ③（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、リセット用 Lambda の timeout が 60 秒であることだけである。
# 既定値（3 秒）に任せず明示すること。timeout は変数でないため、variables で上書きしない。
# 既存の3本の Lambda の timeout は対象の外（AC-13-11）。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
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

# --- AC-12-12: リセット用 Lambda の timeout（60 秒） ------------------------
run "demo_data_reset_timeout_is_60_seconds" {
  command = plan

  assert {
    condition     = aws_lambda_function.demo_data_reset.timeout == 60
    error_message = "リセット用 Lambda の timeout は 60 秒でなければならない（demo-data-reset.md AC-12-12）。未設定（既定 3 秒）・他の値は不可"
  }
}
