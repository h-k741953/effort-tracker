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

# --- AC-10-4-j: Budgets（本数・閾値・通知） ---------------------------------
# notification は nesting_mode = "set" で定義されており、添字 [0] では
# 参照できない（HCL の言語仕様）。各 Budget は notification ブロックを
# 1つしか持たないため、one() で集合を単一のオブジェクトへ変換してから
# 属性を参照する（検査する閾値・通知種別・比較演算子は変更しない）。
run "budgets_has_actual_and_forecasted_notifications" {
  command = plan

  assert {
    condition     = aws_budgets_budget.actual.limit_amount == "5"
    error_message = "AWS Budgets の上限額は月 $5（cost-guardrails.md）"
  }

  assert {
    condition     = one(aws_budgets_budget.actual.notification).threshold == 80
    error_message = "実績ベースの通知閾値は 80%（cost-guardrails.md）"
  }

  assert {
    condition     = one(aws_budgets_budget.actual.notification).notification_type == "ACTUAL"
    error_message = "1本目は実績（ACTUAL）ベースの通知でなければならない（cost-guardrails.md）"
  }

  assert {
    condition     = length(one(aws_budgets_budget.actual.notification).subscriber_sns_topic_arns) > 0 || length(one(aws_budgets_budget.actual.notification).subscriber_email_addresses) > 0
    error_message = "実績ベースの Budget は通知先を持たなければならない（AC-5-2）"
  }

  assert {
    condition     = aws_budgets_budget.forecasted.limit_amount == "5"
    error_message = "AWS Budgets の上限額は月 $5（cost-guardrails.md）"
  }

  assert {
    condition     = one(aws_budgets_budget.forecasted.notification).threshold == 100
    error_message = "予測ベースの通知閾値は 100%（cost-guardrails.md）"
  }

  assert {
    condition     = one(aws_budgets_budget.forecasted.notification).notification_type == "FORECASTED"
    error_message = "2本目は予測（FORECASTED）ベースの通知でなければならない（cost-guardrails.md）"
  }

  assert {
    condition     = length(one(aws_budgets_budget.forecasted.notification).subscriber_sns_topic_arns) > 0 || length(one(aws_budgets_budget.forecasted.notification).subscriber_email_addresses) > 0
    error_message = "予測ベースの Budget は通知先を持たなければならない（AC-5-2）"
  }
}
