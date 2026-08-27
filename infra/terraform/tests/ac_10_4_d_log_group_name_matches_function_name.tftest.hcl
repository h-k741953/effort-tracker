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

# --- AC-10-4-d: ロググループ名と関数名の対応 --------------------------------
run "log_group_name_matches_function_name" {
  command = plan

  assert {
    condition     = aws_cloudwatch_log_group.bff_ssr.name == "/aws/lambda/${aws_lambda_function.bff_ssr.function_name}"
    error_message = "BFF/SSR のロググループ名は関数名から導出された /aws/lambda/<関数名> と一致しなければならない（暗黙生成の併存を防ぐ＝AC-6-3）"
  }

  assert {
    condition     = aws_cloudwatch_log_group.domain_api.name == "/aws/lambda/${aws_lambda_function.domain_api.function_name}"
    error_message = "ドメイン API のロググループ名は関数名から導出された /aws/lambda/<関数名> と一致しなければならない（AC-6-3）"
  }

  assert {
    condition     = aws_cloudwatch_log_group.cloudfront_killswitch.name == "/aws/lambda/${aws_lambda_function.cloudfront_killswitch.function_name}"
    error_message = "CloudFront 遮断 Lambda のロググループ名は関数名から導出された /aws/lambda/<関数名> と一致しなければならない（AC-6-3）"
  }
}
