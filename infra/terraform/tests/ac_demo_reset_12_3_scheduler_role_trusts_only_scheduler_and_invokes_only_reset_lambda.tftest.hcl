# AC-12-3（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、EventBridge Scheduler が引き受けるロール
# （aws_iam_role.demo_data_reset_scheduler）だけである。
#   - 信頼ポリシーの主体は EventBridge Scheduler のサービス
#     （scheduler.amazonaws.com）だけ
#   - 許すのはリセット用 Lambda の呼び出し（lambda:InvokeFunction）だけ。
#     Resource は当該関数に限り "*" にしない。Action にワイルドカードを含めない
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
# リセット用 Lambda の arn は plan 時点で unknown になるため、override_resource で
# ダミー値（架空のアカウント ID）に確定させる。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
# （aws_iam_role.demo_data_reset_scheduler /
#   aws_iam_role_policy.demo_data_reset_scheduler_invoke）

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

# --- AC-12-3: Scheduler が引き受けるロールの信頼ポリシーと権限 ---------------
run "demo_data_reset_scheduler_role_trusts_only_scheduler_and_invokes_only_reset_lambda" {
  command = plan

  override_resource {
    target = aws_lambda_function.demo_data_reset
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-demo-data-reset-test"
    }
    override_during = plan
  }

  # 信頼ポリシー: 文が1つ以上あり、主体は Service = scheduler.amazonaws.com だけ。
  assert {
    condition     = length(jsondecode(aws_iam_role.demo_data_reset_scheduler.assume_role_policy).Statement) > 0
    error_message = "Scheduler ロールの信頼ポリシーは文を1つ以上持たなければならない（AC-12-3）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role.demo_data_reset_scheduler.assume_role_policy).Statement :
      toset(keys(stmt.Principal)) == toset(["Service"])
    ])
    error_message = "Scheduler ロールの信頼ポリシーの Principal は Service だけでなければならない（AWS アカウント・Federated・\"*\" を含めない＝AC-12-3）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role.demo_data_reset_scheduler.assume_role_policy).Statement :
      toset(try(tolist(stmt.Principal.Service), [stmt.Principal.Service])) == toset(["scheduler.amazonaws.com"])
    ])
    error_message = "Scheduler ロールの信頼ポリシーの主体は scheduler.amazonaws.com だけでなければならない（lambda.amazonaws.com など他のサービスを含めない＝AC-12-3）"
  }

  # 権限: lambda:InvokeFunction だけ。Resource はリセット用 Lambda の ARN 1つ。
  assert {
    condition     = length(jsondecode(aws_iam_role_policy.demo_data_reset_scheduler_invoke.policy).Statement) > 0
    error_message = "Scheduler ロールの権限ポリシーは文を1つ以上持たなければならない（AC-12-3）"
  }

  assert {
    condition = toset(flatten([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_scheduler_invoke.policy).Statement :
      try(tolist(stmt.Action), [stmt.Action])
    ])) == toset(["lambda:InvokeFunction"])
    error_message = "Scheduler ロールが許す Action は lambda:InvokeFunction だけでなければならない。ワイルドカードを含めない（AC-12-3）"
  }

  assert {
    condition = toset(flatten([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_scheduler_invoke.policy).Statement :
      try(tolist(stmt.Resource), [stmt.Resource])
    ])) == toset([aws_lambda_function.demo_data_reset.arn])
    error_message = "Scheduler ロールの Resource はリセット用 Lambda の ARN 1つに限り、\"*\" にしてはならない。ほかの Lambda を含めない（AC-12-3）"
  }
}
