# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 期待値のうち docs/rules/cost-guardrails.md が持つもの（同時実行数・ログ保持日数・
# Budgets の本数と閾値・authorization_type）は同ファイルを読んで書いている
# （AC-10-4）。値を infra 側へ二重に定義しない（AC-5-6）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# 構成（*.tf のリソース定義）はまだ無いため、各 run は「未実装」を理由に
# 落ちる（AC-10-3 の Red）。
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

# --- AC-10-4-r: 遮断 Lambda の起動経路（SNS 購読1つ） ----------------------
# 「それ以外のイベント源を持たない」ことの網羅的な不在確認は 12-7 と同型の
# 限界により対象外とする。ここでは SNS サブスクリプションが存在し、遮断
# Lambda を対象としていることまでを assert する。
run "cloudfront_killswitch_is_triggered_by_sns_subscription" {
  command = plan

  # aws_lambda_function.cloudfront_killswitch.arn は provider が採番する
  # computed 値で、plan 時点では unknown になり比較できない（AC-10-2 の
  # mock_provider + command = plan を維持したまま、参照先だけダミー値で
  # 確定させる）。ARN はダミー（架空のアカウントID）であり、実在の値ではない。
  override_resource {
    target = aws_lambda_function.cloudfront_killswitch
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-cloudfront-killswitch-test"
    }
    override_during = plan
  }

  assert {
    condition     = aws_sns_topic_subscription.cloudfront_killswitch.protocol == "lambda"
    error_message = "遮断 Lambda は SNS トピックの lambda プロトコル購読から起動しなければならない（AC-5-4）"
  }

  assert {
    condition     = aws_sns_topic_subscription.cloudfront_killswitch.endpoint == aws_lambda_function.cloudfront_killswitch.arn
    error_message = "SNS サブスクリプションの endpoint は遮断 Lambda の ARN でなければならない（AC-5-4）"
  }
}
