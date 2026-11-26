# AC-10（docs/specs/infra-terraform.md）10-4-ab の検査（2026-11-25 追加・#162）。
#
# 本ファイルが検査するのは、凍結ポリシー（aws_iam_policy.budget_freeze_new_resources。
# AC-5-3 の Deny の側）の本文が、AC-5-3 の 2026-11-25 追記（凍結の3つ）
# （scheduler:CreateSchedule・sns:CreateTopic・sns:Subscribe）をすべて含む Deny の文を
# 持つことである。期待値の持ち主は AC-5-3（本ファイルは写しを持つが、変更は AC-5-3 が先）。
# 10-4-aa（Cognito の4つ）は別ファイルが持ち、本ファイルはそれを弱めない。
# 射程は3つだけで、sns:SetTopicAttributes 等が含まれないことは検査しない。
# 凍結が適用済みのロールで実際に効くことは検査しない（12-3）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。

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

# --- AC-10-4-ab: 凍結ポリシーの Scheduler・SNS の作成 ---------------------
run "budget_freeze_policy_denies_scheduler_and_sns_creation_actions" {
  command = plan

  variables {
    scheduler_sns_creation_actions = ["scheduler:CreateSchedule", "sns:CreateTopic", "sns:Subscribe"]
  }

  # Deny の文のうち少なくとも1つが3つをすべて含む。Deny の文が無ければ anytrue([]) = false で落ちる
  # （空集合で緑にならない）。1つでも欠ければ落ちる。
  assert {
    condition = anytrue([
      for stmt in [for s in jsondecode(aws_iam_policy.budget_freeze_new_resources.policy).Statement : s if s.Effect == "Deny"] :
      alltrue([for n in var.scheduler_sns_creation_actions : contains(try(tolist(stmt.Action), [stmt.Action]), n)])
    ])
    error_message = "凍結ポリシーの Deny の文のうち少なくとも1つが、scheduler:CreateSchedule・sns:CreateTopic・sns:Subscribe の3つをすべて含まなければならない（AC-5-3・10-4-ab）"
  }
}
