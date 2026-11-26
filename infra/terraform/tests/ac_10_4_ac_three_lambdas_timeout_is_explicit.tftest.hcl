# AC-10（docs/specs/infra-terraform.md）10-4-ac・AC-4-10 の検査。
#
# 既存の3本の Lambda の timeout を、3本それぞれ別の assert で確かめる。
#   BFF・SSR Lambda = 10 秒 / ドメイン API Lambda = 5 秒 / CloudFront 遮断 Lambda = 30 秒
# 期待値は秒数のリテラルで書き、var.* と突き合わせない（12-36）。
# timeout は変数でないため variables で上書きしない。未設定（既定 3 秒）では落ちる。
# リセット用 Lambda（60 秒）は docs/specs/demo-data-reset.md AC-12-10 ③が持つので、ここでは assert しない。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# （aws_lambda_function.bff_ssr / domain_api / cloudfront_killswitch）

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

run "bff_ssr_timeout_is_10_seconds" {
  command = plan

  assert {
    condition     = aws_lambda_function.bff_ssr.timeout == 10
    error_message = "BFF・SSR Lambda の timeout は 10 秒でなければならない（infra-terraform.md AC-4-10）。未設定（既定 3 秒）・他の値は不可"
  }
}

run "domain_api_timeout_is_5_seconds" {
  command = plan

  assert {
    condition     = aws_lambda_function.domain_api.timeout == 5
    error_message = "ドメイン API Lambda の timeout は 5 秒でなければならない（infra-terraform.md AC-4-10）。未設定（既定 3 秒）・他の値は不可"
  }
}

run "cloudfront_killswitch_timeout_is_30_seconds" {
  command = plan

  assert {
    condition     = aws_lambda_function.cloudfront_killswitch.timeout == 30
    error_message = "CloudFront 遮断 Lambda の timeout は 30 秒でなければならない（infra-terraform.md AC-4-10）。未設定（既定 3 秒）・他の値は不可"
  }
}
