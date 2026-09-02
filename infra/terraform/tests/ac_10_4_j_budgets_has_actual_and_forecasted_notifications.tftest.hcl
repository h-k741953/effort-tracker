# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは AWS Budgets の2本の通知（実績・予測）だけである。
# 期待値のうち上限額 月 $5・実績 80% / 予測 100% という閾値・本数が2本であること
# は docs/rules/cost-guardrails.md を読んで書いている（AC-10-4）。値を infra 側へ
# 二重に定義しない（AC-5-6）。通知先を持つことは AC-5-2 による。
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
