# AC-8-3-1（docs/specs/infra-terraform.md）の検査。
#
# variables.tf の ssm_parameter_name は、ARN を
# "arn:aws:ssm:<region>:<account>:parameter${var.ssm_parameter_name}" として
# 組み立てる（lambda_domain_api.tf）ため、先頭が "/" でない値を渡すと
# "parameter/" の直後にパラメータ名の先頭 "/" が重ならず、別のパラメータを
# 指す ARN になってしまう。これを防ぐ variable "validation" ブロックが
# variables.tf に既にある。本ファイルはその検証が実際に効くこと
# （先頭 "/" の無い値を拒否すること）を確認する。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# 変数検証の失敗は expect_failures で拾う。

mock_provider "aws" {}

variables {
  aws_region                                 = "ap-northeast-1"
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "effort-tracker/test/neon-connection-string"
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

# --- AC-8-3-1: 先頭 "/" の無い ssm_parameter_name は変数検証で拒否される ----
run "ssm_parameter_name_without_leading_slash_is_rejected" {
  command = plan

  expect_failures = [
    var.ssm_parameter_name,
  ]
}
