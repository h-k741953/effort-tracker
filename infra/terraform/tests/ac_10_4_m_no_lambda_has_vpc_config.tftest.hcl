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

# --- AC-10-4-m: いずれの Lambda も VPC 設定を持たない -----------------------
run "no_lambda_has_vpc_config" {
  command = plan

  assert {
    condition     = length(aws_lambda_function.bff_ssr.vpc_config) == 0
    error_message = "BFF/SSR Lambda は VPC に配置してはならない（AC-4-6）"
  }

  assert {
    condition     = length(aws_lambda_function.domain_api.vpc_config) == 0
    error_message = "ドメイン API Lambda は VPC に配置してはならない（AC-4-6）"
  }

  assert {
    condition     = length(aws_lambda_function.cloudfront_killswitch.vpc_config) == 0
    error_message = "CloudFront 遮断 Lambda は VPC に配置してはならない（AC-4-6）"
  }
}
