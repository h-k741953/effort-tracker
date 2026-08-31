# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 期待値のうち docs/rules/cost-guardrails.md が持つもの（同時実行数・ログ保持日数・
# Budgets の本数と閾値・authorization_type）は同ファイルを読んで書いている
# （AC-10-4）。値を infra 側へ二重に定義しない（AC-5-6）。
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
}

# --- AC-10-4-x: Budget の側から SNS トピックへの発行の許可 -------------------
#
# 検査するのは「許可が存在し、当該トピックを対象としていること」までであり、
# その許可で実際に発行が通るかは検査しない（10-4-x。破れは 12-33 が持つ）。
run "sns_topic_policy_allows_budgets_to_publish" {
  command = plan

  # aws_sns_topic.cost_alert.arn は provider が採番する computed 値で、
  # plan 時点では unknown になり比較できない（AC-10-2 の mock_provider +
  # command = plan を維持したまま、参照先だけダミー値で確定させる）。ARN は
  # ダミー（架空のトピック）であり、実在の値ではない。検査対象である
  # aws_sns_topic_policy.cost_alert の policy 自体は override しない —
  # 検査対象の値を検査側が供給すると、どんな実装でも緑になる偽 Green に
  # なるため。
  override_resource {
    target = aws_sns_topic.cost_alert
    values = {
      arn = "arn:aws:sns:ap-northeast-1:123456789012:effort-tracker-cost-alert-test"
    }
    override_during = plan
  }

  # 「Budget の側からの発行を許す文」を、AWS Budgets がリソースベースの
  # ポリシーで通知先へ発行する際に用いるサービスプリンシパル
  # (budgets.amazonaws.com) を主体とし、SNS:Publish を許す文として特定する
  # （同じ主体はこの構成の budgets.tf 側で aws_iam_role.budget_action_execution
  # の信頼ポリシーが既に用いており、本仕様固有の当て推量ではない）。
  assert {
    condition = length([
      for stmt in jsondecode(aws_sns_topic_policy.cost_alert.policy).Statement :
      stmt
      if try(stmt.Effect, "") == "Allow"
      && contains(
        try(tolist(stmt.Principal.Service), [try(stmt.Principal.Service, "")]),
        "budgets.amazonaws.com"
      )
      && contains(try(tolist(stmt.Action), [stmt.Action]), "SNS:Publish")
    ]) > 0
    error_message = "SNS トピックには、Budget の側(budgets.amazonaws.com)からの SNS:Publish を許す文が存在しなければならない（AC-5-4・10-4-x）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_sns_topic_policy.cost_alert.policy).Statement :
      stmt.Resource == aws_sns_topic.cost_alert.arn
      if try(stmt.Effect, "") == "Allow"
      && contains(
        try(tolist(stmt.Principal.Service), [try(stmt.Principal.Service, "")]),
        "budgets.amazonaws.com"
      )
      && contains(try(tolist(stmt.Action), [stmt.Action]), "SNS:Publish")
    ])
    error_message = "Budget の側からの発行を許す文の対象は、この構成が作る当該トピックそのもの(aws_sns_topic.cost_alert.arn)でなければならず、暫定値・無関係な値・別のリソースを指す値であってはならない（AC-5-4・10-4-x）"
  }
}
