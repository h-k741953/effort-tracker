# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、GitHub OIDC の信頼ポリシーの sub 条件だけである
# （StringEquals で repo:[OWNER]/[REPO]:environment:production に完全一致させ、
# StringLike によるワイルドカード一致を使わないこと＝AC-7-6・D-7）。
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
}

# --- AC-10-4-e: GitHub OIDC の信頼ポリシー ----------------------------------
run "github_oidc_trust_policy_sub_is_fixed_to_production_environment" {
  command = plan

  assert {
    condition     = jsondecode(aws_iam_role.github_actions_deploy.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:${var.github_oidc_repo_owner}/${var.github_oidc_repo_name}:environment:production"
    error_message = "信頼ポリシーの sub は StringEquals で repo:[OWNER]/[REPO]:environment:production に完全一致で固定しなければならない（AC-7-6・D-7）"
  }

  assert {
    condition     = !can(jsondecode(aws_iam_role.github_actions_deploy.assume_role_policy).Statement[0].Condition.StringLike)
    error_message = "信頼ポリシーの sub 条件は StringLike（ワイルドカード一致）を使ってはならない（AC-7-6）"
  }
}
