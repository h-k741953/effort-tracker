# AWS Budgets(AC-5-2)。値の実体は docs/rules/cost-guardrails.md
# (月 $5・実績80%／予測100%の2本)。通知先は AC-5-4 の SNS トピックを使う
# (Budget → SNS → 遮断 Lambda の配線と同じトピック。値を二重に定義しない
# ＝AC-5-6)。

resource "aws_budgets_budget" "actual" {
  name         = "effort-tracker-actual"
  budget_type  = "COST"
  limit_amount = "5" # docs/rules/cost-guardrails.md
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator       = "GREATER_THAN"
    threshold                 = 80 # docs/rules/cost-guardrails.md
    threshold_type            = "PERCENTAGE"
    notification_type         = "ACTUAL"
    subscriber_sns_topic_arns = [aws_sns_topic.cost_alert.arn]
  }
}

resource "aws_budgets_budget" "forecasted" {
  name         = "effort-tracker-forecasted"
  budget_type  = "COST"
  limit_amount = "5" # docs/rules/cost-guardrails.md
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator       = "GREATER_THAN"
    threshold                 = 100 # docs/rules/cost-guardrails.md
    threshold_type            = "PERCENTAGE"
    notification_type         = "FORECASTED"
    subscriber_sns_topic_arns = [aws_sns_topic.cost_alert.arn]
  }
}

# AC-5-3: Budget Actions(閾値到達で IAM の deny を適用し、新規リソース
# 作成を凍結する。翌予算期に自動リバートするのは Budget Actions の標準
# 挙動)。対象は GitHub Actions デプロイロール(実際にリソースを作成できる
# 唯一の主体)。

resource "aws_iam_policy" "budget_freeze_new_resources" {
  name = "effort-tracker-budget-freeze-new-resources"

  # D-15 が列挙する各サービスの「新規作成」系アクションを拒否する。
  # 既存リソースの読み取り・更新・デプロイ以外の操作系(削除等)は
  # このポリシーの対象外(凍結の趣旨は「新規リソース作成」に限る)。
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "DenyNewResourceCreation"
        Effect = "Deny"
        Action = [
          "s3:CreateBucket",
          "cloudfront:CreateDistribution",
          "cloudfront:CreateOriginAccessControl",
          "lambda:CreateFunction",
          "iam:CreateRole",
          "iam:CreatePolicy",
          "iam:CreateOpenIDConnectProvider",
          "ssm:PutParameter",
          "budgets:CreateBudget",
          "budgets:CreateBudgetAction",
          "logs:CreateLogGroup",
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role" "budget_action_execution" {
  name = "effort-tracker-budget-action-execution"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Action    = "sts:AssumeRole"
        Principal = { Service = "budgets.amazonaws.com" }
      }
    ]
  })
}

# Budget Actions の実行ロールが要る最小権限: 対象ロール(デプロイロール)への
# 凍結ポリシーの着脱のみ。
resource "aws_iam_role_policy" "budget_action_execution" {
  name = "attach-detach-freeze-policy"
  role = aws_iam_role.budget_action_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "iam:AttachRolePolicy",
          "iam:DetachRolePolicy",
          "iam:ListAttachedRolePolicies",
        ]
        Resource = aws_iam_role.github_actions_deploy.arn
      }
    ]
  })
}

resource "aws_budgets_budget_action" "freeze_on_forecast" {
  budget_name        = aws_budgets_budget.forecasted.name
  action_type        = "APPLY_IAM_POLICY"
  approval_model     = "AUTOMATIC"
  notification_type  = "FORECASTED"
  execution_role_arn = aws_iam_role.budget_action_execution.arn

  action_threshold {
    action_threshold_type  = "PERCENTAGE"
    action_threshold_value = 100 # docs/rules/cost-guardrails.md
  }

  definition {
    iam_action_definition {
      policy_arn = aws_iam_policy.budget_freeze_new_resources.arn
      roles      = [aws_iam_role.github_actions_deploy.name]
    }
  }

  subscriber {
    address           = aws_sns_topic.cost_alert.arn
    subscription_type = "SNS"
  }
}
