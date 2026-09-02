# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、3つの Lambda 実行ロールのログ書き込みポリシーだけで
# ある（いずれの文も Action にワイルドカードを含まないこと、Resource が自身の
# ロググループに限定され他の関数のロググループへ届かないこと＝AC-7-4）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
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

# --- AC-10-4-t: 3本の実行ロールのログ書き込みポリシー -----------------------
run "log_write_policies_have_no_wildcard_action_and_are_scoped_to_own_log_group" {
  command = plan

  # aws_cloudwatch_log_group.*.arn は provider が採番する computed 値で、
  # plan 時点では unknown になり比較できない（AC-10-2 の mock_provider +
  # command = plan を維持したまま、参照先だけダミー値で確定させる）。
  # ARN はダミー（架空のアカウントID）であり、実在の値ではない。
  # 3本を異なる値にすることで、他の関数のロググループへ届いていないこと
  # （境界を越えていないこと）を startswith で検出できるようにする。
  override_resource {
    target = aws_cloudwatch_log_group.bff_ssr
    values = {
      arn = "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/effort-tracker-bff-ssr-test"
    }
    override_during = plan
  }

  override_resource {
    target = aws_cloudwatch_log_group.domain_api
    values = {
      arn = "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/effort-tracker-domain-api-test"
    }
    override_during = plan
  }

  override_resource {
    target = aws_cloudwatch_log_group.cloudfront_killswitch
    values = {
      arn = "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/effort-tracker-cloudfront-killswitch-test"
    }
    override_during = plan
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.bff_ssr_logs.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "BFF/SSR 実行ロールのログ書き込みポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-7-4）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.bff_ssr_logs.policy).Statement :
      stmt.Resource != "*" && startswith(stmt.Resource, aws_cloudwatch_log_group.bff_ssr.arn)
    ])
    error_message = "BFF/SSR 実行ロールのログ書き込みポリシーの Resource は自身のロググループに限定しなければならず、\"*\" にしてはならず、他の関数のロググループへ届いてはならない（AC-7-4）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.domain_api_logs.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "ドメイン API 実行ロールのログ書き込みポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-7-4）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.domain_api_logs.policy).Statement :
      stmt.Resource != "*" && startswith(stmt.Resource, aws_cloudwatch_log_group.domain_api.arn)
    ])
    error_message = "ドメイン API 実行ロールのログ書き込みポリシーの Resource は自身のロググループに限定しなければならず、\"*\" にしてはならず、他の関数のロググループへ届いてはならない（AC-7-4）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.cloudfront_killswitch_logs.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "遮断 Lambda 実行ロールのログ書き込みポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-7-4）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.cloudfront_killswitch_logs.policy).Statement :
      stmt.Resource != "*" && startswith(stmt.Resource, aws_cloudwatch_log_group.cloudfront_killswitch.arn)
    ])
    error_message = "遮断 Lambda 実行ロールのログ書き込みポリシーの Resource は自身のロググループに限定しなければならず、\"*\" にしてはならず、他の関数のロググループへ届いてはならない（AC-7-4）"
  }
}
