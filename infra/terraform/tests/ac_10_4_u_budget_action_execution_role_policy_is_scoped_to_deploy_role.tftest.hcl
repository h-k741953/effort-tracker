# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、Budget Action の実行ロールのポリシーだけである
# （いずれの文も Action にワイルドカードを含まないこと、Resource が deny を
# 付け外しする相手＝デプロイロールに限定されていること＝AC-5-3）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
#
# 本 run の対象は Budget Action の実行ロール側のポリシーだけである。閾値
# 到達時に適用される凍結側のポリシー（Effect が Deny の側）は対象外であり、
# そちらを広く書くことは凍結の目的そのものなので、本 run では検査しない
# （10-4-u）。

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

# --- AC-10-4-u: Budget Action の実行ロールのポリシー -------------------------
run "budget_action_execution_role_policy_is_scoped_to_deploy_role" {
  command = plan

  # aws_iam_role.github_actions_deploy.arn は provider が採番する computed
  # 値で、plan 時点では unknown になり比較できない（AC-10-2 の
  # mock_provider + command = plan を維持したまま、参照先だけダミー値で
  # 確定させる）。ARN はダミー（架空のアカウントID）であり、実在の値ではない。
  override_resource {
    target = aws_iam_role.github_actions_deploy
    values = {
      arn = "arn:aws:iam::123456789012:role/effort-tracker-github-actions-deploy-test"
    }
    override_during = plan
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.budget_action_execution.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "Budget Action の実行ロールのポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-5-3）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.budget_action_execution.policy).Statement :
      stmt.Resource != "*" && stmt.Resource == aws_iam_role.github_actions_deploy.arn
    ])
    error_message = "Budget Action の実行ロールのポリシーの Resource は deny を付け外しする相手（デプロイロール）に限定しなければならず、\"*\" にしてはならない（AC-5-3）"
  }
}
