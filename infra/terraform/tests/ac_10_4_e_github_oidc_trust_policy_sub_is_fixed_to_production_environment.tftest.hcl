# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 期待値のうち docs/rules/cost-guardrails.md が持つもの（同時実行数・ログ保持日数・
# Budgets の本数と閾値・authorization_type）は同ファイルを読んで書いている
# （AC-10-4）。値を infra 側へ二重に定義しない（AC-5-6）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# 構成（*.tf のリソース定義）はまだ無いため、各 run は「未実装」を理由に
# 落ちる（AC-10-3 の Red）。
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
