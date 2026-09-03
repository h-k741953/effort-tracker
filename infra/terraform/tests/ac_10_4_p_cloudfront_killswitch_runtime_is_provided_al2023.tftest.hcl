# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、CloudFront 遮断 Lambda の runtime が provided.al2023
# であること（暫定値・空文字を許さないこと）だけである（AC-4-8・D-17）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# 構成（*.tf のリソース定義）は実装済みであり、各 run はその構成に対して実際に
# 評価される（AC-10-3 の Red は解消済み）。
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
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
  public_origin                              = "https://public.example.test"
}

# --- AC-10-4-p: 遮断 Lambda の runtime（provided.al2023） ------------------
run "cloudfront_killswitch_runtime_is_provided_al2023" {
  command = plan

  assert {
    condition     = aws_lambda_function.cloudfront_killswitch.runtime == "provided.al2023"
    error_message = "CloudFront 遮断 Lambda の runtime は provided.al2023 でなければならない（AC-4-8・D-17）。暫定値・空文字は不可"
  }
}
