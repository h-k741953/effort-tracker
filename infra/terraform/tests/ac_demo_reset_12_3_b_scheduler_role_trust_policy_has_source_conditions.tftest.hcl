# AC-12-3-b・AC-12-10 ④（docs/specs/demo-data-reset.md）の検査。
#
# Scheduler 用ロール（aws_iam_role.demo_data_reset_scheduler）の信頼ポリシーのうち、
# 主体が scheduler.amazonaws.com の文について、次を独立した assert で確かめる。
#   (i)   そのような文が1つ以上ある（空集合で緑にならない）
#   (ii)  各文の Condition.StringEquals["aws:SourceAccount"] が呼び出し元アカウントの属性と一致
#   (iii) 各文の Condition.StringEquals["aws:SourceArn"] が
#         arn:aws:scheduler:<region>:<account>:schedule-group/default と一致
#   (iv)  同じ文が Condition.StringLike・Condition.ArnLike を持たない
# <account> はリテラルで書かず data.aws_caller_identity.current.account_id と突き合わせる。
# aws:SourceArn の実際の値が AWS の渡す値と一致することは確かめない（AC-13-16）。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。

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

run "scheduler_role_trust_policy_has_source_account_and_source_arn_conditions" {
  command = plan

  # (i)
  assert {
    condition     = length([for s in jsondecode(aws_iam_role.demo_data_reset_scheduler.assume_role_policy).Statement : s if toset(try(tolist(s.Principal.Service), [s.Principal.Service])) == toset(["scheduler.amazonaws.com"])]) > 0
    error_message = "主体が scheduler.amazonaws.com の信頼ポリシーの文が1つ以上なければならない（AC-12-10 ④ (i)）"
  }

  # (ii)
  assert {
    condition = alltrue([
      for s in jsondecode(aws_iam_role.demo_data_reset_scheduler.assume_role_policy).Statement :
      try(s.Condition.StringEquals["aws:SourceAccount"], null) == data.aws_caller_identity.current.account_id
      if toset(try(tolist(s.Principal.Service), [s.Principal.Service])) == toset(["scheduler.amazonaws.com"])
    ])
    error_message = "各文の Condition.StringEquals[\"aws:SourceAccount\"] は自アカウント（data.aws_caller_identity.current.account_id）と一致しなければならない（AC-12-3-b ①・AC-12-10 ④ (ii)）"
  }

  # (iii)
  assert {
    condition = alltrue([
      for s in jsondecode(aws_iam_role.demo_data_reset_scheduler.assume_role_policy).Statement :
      try(s.Condition.StringEquals["aws:SourceArn"], null) == "arn:aws:scheduler:${var.aws_region}:${data.aws_caller_identity.current.account_id}:schedule-group/default"
      if toset(try(tolist(s.Principal.Service), [s.Principal.Service])) == toset(["scheduler.amazonaws.com"])
    ])
    error_message = "各文の Condition.StringEquals[\"aws:SourceArn\"] は arn:aws:scheduler:<region>:<account>:schedule-group/default と一致しなければならない（AC-12-3-b ②・AC-12-10 ④ (iii)）"
  }

  # (iv)
  assert {
    condition = alltrue([
      for s in jsondecode(aws_iam_role.demo_data_reset_scheduler.assume_role_policy).Statement :
      !can(s.Condition.StringLike) && !can(s.Condition.ArnLike)
      if toset(try(tolist(s.Principal.Service), [s.Principal.Service])) == toset(["scheduler.amazonaws.com"])
    ])
    error_message = "Scheduler ロールの信頼ポリシーは Condition.StringLike・Condition.ArnLike を持ってはならない（AC-12-3-b ③・AC-12-10 ④ (iv)）"
  }
}
