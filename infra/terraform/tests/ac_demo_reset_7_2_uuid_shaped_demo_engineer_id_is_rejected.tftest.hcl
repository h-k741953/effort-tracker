# AC-7-2（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、変数 demo_engineer_id に UUID の形の値を与えると、
# Terraform の変数検証が拒否することだけである（Cognito の sub が取りうる形と
# 衝突させない＝Q-5）。検証の手段は仕様工程の定め（variable の validation）。
# UUID の形でない値が通ることは ac_demo_reset_7_1_*（既定値・上書きの両面）が
# 持つ。
#
# 値は RFC 4122 の例に近いダミー（実在の sub ではない）。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
# （variable "demo_engineer_id"）

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

# --- AC-7-2: UUID の形の demo_engineer_id は変数検証で拒否される -------------
run "uuid_shaped_demo_engineer_id_is_rejected" {
  command = plan

  variables {
    demo_engineer_id = "123e4567-e89b-12d3-a456-426614174000"
  }

  expect_failures = [
    var.demo_engineer_id,
  ]
}
