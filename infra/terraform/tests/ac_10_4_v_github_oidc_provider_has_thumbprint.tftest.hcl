# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、GitHub OIDC プロバイダが空でない thumbprint_list を
# 持つことだけである（AC-7-5）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。

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

# --- AC-10-4-v: GitHub OIDC プロバイダのサムプリント ------------------------
# 値が発行者の現況と一致するかは検査しない（12-30 が持つ限界）。ここで見るのは
# thumbprint_list が存在し、空でないことまで（AC-7-5）。
run "github_oidc_provider_has_thumbprint" {
  command = plan

  assert {
    condition     = length(aws_iam_openid_connect_provider.github_actions.thumbprint_list) > 0
    error_message = "GitHub OIDC プロバイダは thumbprint_list を持ち、それが空であってはならない（AC-7-5）"
  }
}
