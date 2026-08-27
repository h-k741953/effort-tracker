# GitHub Actions からの OIDC 連携(AC-7-5〜AC-7-9)。
# ランタイム向けの ID プロバイダ(外部 IdP との federation)は作らない
# (docs/rules/security.md・AC-11-3)。

resource "aws_iam_openid_connect_provider" "github_actions" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
  # GitHub の OIDC 発行者の TLS 証明書チェーン(DigiCert Global Root G2)の
  # サムプリント(機密情報ではない。AWS/GitHub の公開ドキュメントに
  # 掲載されている既知の値)。
  thumbprint_list = [
    "1c58a3a8518e8759bf075b76b750d4f2df264fcd",
  ]
}

# AC-7-6: sub は repo:[OWNER]/[REPO]:environment:production に完全一致で
# 固定する(StringEquals・単一値。ワイルドカードにしない)。
# AC-7-7: aud も条件に含める(sub だけで足りるとしない)。
#
# aws_iam_policy_document ではなく jsonencode を直接使う: json 属性は
# provider 側の計算による完全な computed 値であり、mock_provider の下では
# 実際の merge ロジックを経由せず無効な JSON を返す(実測済み)。
# jsonencode は Terraform core の組み込み関数であり mock の影響を受けない。
#
# Principal.Federated は aws_iam_openid_connect_provider.github_actions.arn
# を直接参照せず、data.aws_caller_identity と既知のリテラル(プロバイダの
# URL からスキームを除いた形)から組み立てる: OIDC プロバイダの ARN は
# 決定的だが provider スキーマ上は computed 属性であり、他リソースの
# computed 属性を自分の引数に混ぜると command = plan 下で
# 全体が unknown になり assert が評価不能になる(実測済み)。
resource "aws_iam_role" "github_actions_deploy" {
  name = "effort-tracker-github-actions-deploy"

  # AC-7-5: このロールは github_actions プロバイダを信頼の起点とするため、
  # 依存関係を構成側に明示して持つ(依存グラフから読み取れる形に保つ)。
  # Principal.Federated は上のコメントのとおりリテラルから組み立てており
  # プロバイダの属性を参照しないため、depends_on で辺を明示する
  # (この行を消すと apply がプロバイダの作成順序に依存して失敗しうる)。
  depends_on = [aws_iam_openid_connect_provider.github_actions]

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = "sts:AssumeRoleWithWebIdentity"
        Principal = {
          Federated = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:oidc-provider/token.actions.githubusercontent.com"
        }
        Condition = {
          StringEquals = {
            "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
            "token.actions.githubusercontent.com:sub" = "repo:${var.github_oidc_repo_owner}/${var.github_oidc_repo_name}:environment:production"
          }
        }
      }
    ]
  })
}

# AC-7-8: 本仕様が作るサービスに限定して列挙する(D-15)。
# Action = "*" を書かない。管理者相当のポリシーを付けない。
resource "aws_iam_role_policy" "github_actions_deploy" {
  name = "deploy-permissions"
  role = aws_iam_role.github_actions_deploy.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "DeployScopedServices"
        Effect = "Allow"
        Action = [
          "s3:*",
          "cloudfront:*",
          "lambda:*",
          "iam:*",
          "ssm:*",
          "budgets:*",
          "logs:*",
        ]
        Resource = "*"
      }
    ]
  })
}
