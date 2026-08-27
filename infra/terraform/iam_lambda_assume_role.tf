# 3本の Lambda が共通で使う信頼ポリシー(lambda.amazonaws.com が引き受ける)。
#
# aws_iam_policy_document ではなく jsonencode を直接使う: json 属性は
# provider 側の computed 値であり、mock_provider の下では実際の merge
# ロジックを経由せず無効な JSON を返す(実測済み)。jsonencode は
# Terraform core の組み込み関数であり mock の影響を受けない。
locals {
  lambda_assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Action    = "sts:AssumeRole"
        Principal = { Service = "lambda.amazonaws.com" }
      }
    ]
  })
}
