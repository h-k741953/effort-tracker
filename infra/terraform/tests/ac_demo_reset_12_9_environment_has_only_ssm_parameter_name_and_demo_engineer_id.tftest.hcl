# AC-12-9（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、リセット用 Lambda の環境変数が、接続文字列の SSM
# パラメータの名前とデモの技術者識別子の2つだけを持つこと（接続文字列・資格情報・
# AWS_ROLE_ARN を置かない）である。環境変数の名前はリセット用 cmd
# （services/api/cmd/demo-data-reset/main.go の demoEngineerIDEnv）と
# ドメイン API の既存の名前に揃える。名前の一致は機械検査されない
# （infra-terraform.md 12-28 と同型。担保はレビューと規律）。
#
# demo_engineer_id は variables ブロックで上書きしない（既定値を評価する）。
# 上書きした値の注入は ac_demo_reset_7_1_*_override が持つ。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
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

# --- AC-12-9: リセット用 Lambda の環境変数は2つだけ ------------------------
run "demo_data_reset_environment_has_only_ssm_parameter_name_and_demo_engineer_id" {
  command = plan

  assert {
    condition = aws_lambda_function.demo_data_reset.environment[0].variables == tomap({
      NEON_CONNECTION_STRING_SSM_PARAMETER_NAME = var.ssm_parameter_name
      DEMO_ENGINEER_ID                          = var.demo_engineer_id
    })
    error_message = "リセット用 Lambda の環境変数は NEON_CONNECTION_STRING_SSM_PARAMETER_NAME（= var.ssm_parameter_name）と DEMO_ENGINEER_ID（= var.demo_engineer_id）の2つだけでなければならない。接続文字列・資格情報・AWS_ROLE_ARN を置かない（AC-12-9）"
  }

  # 環境変数ブロックがちょうど1つあること（for 式や try が空に落ちて緑になる形を避ける）。
  assert {
    condition     = length(aws_lambda_function.demo_data_reset.environment) == 1
    error_message = "リセット用 Lambda は environment ブロックをちょうど1つ持たなければならない（AC-12-9）"
  }
}
