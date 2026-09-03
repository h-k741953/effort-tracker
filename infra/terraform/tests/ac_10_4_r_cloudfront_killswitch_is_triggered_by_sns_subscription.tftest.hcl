# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、CloudFront 遮断 Lambda が SNS トピックの lambda
# プロトコル購読から起動すること（購読の endpoint が遮断 Lambda の ARN である
# こと）だけである（AC-5-4）。
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
  public_origin                              = "https://public.example.test"
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
