# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、CloudFront 遮断 Lambda の実行ロールのポリシーだけで
# ある（Resource が当該ディストリビューションに限定されていること、いずれの文も
# Action にワイルドカードを含まないこと＝AC-7-2）。
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

# --- AC-10-4-h: 遮断 Lambda のポリシー（対象ディストリビューション限定） ---
run "cloudfront_killswitch_role_is_scoped_to_the_distribution" {
  command = plan

  # aws_cloudfront_distribution.main.arn は provider が採番する computed 値で、
  # plan 時点では unknown になり比較できない（AC-10-2 の mock_provider +
  # command = plan を維持したまま、参照先だけダミー値で確定させる）。
  # ARN はダミー（架空のアカウントID・ディストリビューションID）であり、実在の値ではない。
  override_resource {
    target = aws_cloudfront_distribution.main
    values = {
      arn = "arn:aws:cloudfront::123456789012:distribution/EDFDVBD6EXAMPLE"
    }
    override_during = plan
  }

  assert {
    condition     = jsondecode(aws_iam_role_policy.cloudfront_killswitch_cloudfront.policy).Statement[0].Resource == aws_cloudfront_distribution.main.arn
    error_message = "遮断 Lambda の実行ロールは当該ディストリビューションに限定しなければならない（AC-7-2）"
  }

  # 10-4-h: AC-7-2 が持つ「〜のみ」の限定を検査へ写したもの。列挙が要求
  # どおりかは検査せず、広すぎないことだけを見る（12-10 と同型）。
  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.cloudfront_killswitch_cloudfront.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "遮断 Lambda の実行ロールのポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-7-2）"
  }
}
