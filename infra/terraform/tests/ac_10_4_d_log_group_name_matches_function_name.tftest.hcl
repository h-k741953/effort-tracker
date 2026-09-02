# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、3つの Lambda のロググループ名が関数名から導出した
# /aws/lambda/<関数名> と一致すること（暗黙に生成されるロググループが併存しない
# こと＝AC-6-3）だけである。
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

# --- AC-10-4-d: ロググループ名と関数名の対応 --------------------------------
run "log_group_name_matches_function_name" {
  command = plan

  assert {
    condition     = aws_cloudwatch_log_group.bff_ssr.name == "/aws/lambda/${aws_lambda_function.bff_ssr.function_name}"
    error_message = "BFF/SSR のロググループ名は関数名から導出された /aws/lambda/<関数名> と一致しなければならない（暗黙生成の併存を防ぐ＝AC-6-3）"
  }

  assert {
    condition     = aws_cloudwatch_log_group.domain_api.name == "/aws/lambda/${aws_lambda_function.domain_api.function_name}"
    error_message = "ドメイン API のロググループ名は関数名から導出された /aws/lambda/<関数名> と一致しなければならない（AC-6-3）"
  }

  assert {
    condition     = aws_cloudwatch_log_group.cloudfront_killswitch.name == "/aws/lambda/${aws_lambda_function.cloudfront_killswitch.function_name}"
    error_message = "CloudFront 遮断 Lambda のロググループ名は関数名から導出された /aws/lambda/<関数名> と一致しなければならない（AC-6-3）"
  }
}
