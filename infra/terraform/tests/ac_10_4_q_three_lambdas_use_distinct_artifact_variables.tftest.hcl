# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、3つの Lambda が互いに別々の成果物パス変数から zip を
# 受け取ること（遮断 Lambda がドメイン API の zip を使い回さないこと）だけである
# （AC-4-5・AC-4-8・D-13）。
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
}

# --- AC-10-4-q: 成果物の受け渡し（3本が別々の変数から。killswitch は domain-api と別） ---
run "three_lambdas_use_distinct_artifact_variables" {
  command = plan

  variables {
    bff_ssr_lambda_artifact_path               = "testdata/a-bff-ssr.zip"
    domain_api_lambda_artifact_path            = "testdata/b-domain-api.zip"
    cloudfront_killswitch_lambda_artifact_path = "testdata/c-cloudfront-killswitch.zip"
  }

  assert {
    condition     = aws_lambda_function.bff_ssr.filename == "testdata/a-bff-ssr.zip"
    error_message = "BFF/SSR Lambda は変数 bff_ssr_lambda_artifact_path から成果物パスを受け取らなければならない（AC-4-5・D-13）"
  }

  assert {
    condition     = aws_lambda_function.domain_api.filename == "testdata/b-domain-api.zip"
    error_message = "ドメイン API Lambda は変数 domain_api_lambda_artifact_path から成果物パスを受け取らなければならない（AC-4-5・D-13）"
  }

  assert {
    condition     = aws_lambda_function.cloudfront_killswitch.filename == "testdata/c-cloudfront-killswitch.zip"
    error_message = "CloudFront 遮断 Lambda は変数 cloudfront_killswitch_lambda_artifact_path から成果物パスを受け取らなければならない（AC-4-8・D-13）"
  }

  assert {
    condition     = aws_lambda_function.cloudfront_killswitch.filename != aws_lambda_function.domain_api.filename
    error_message = "CloudFront 遮断 Lambda はドメイン API と同じ成果物を指してはならない（AC-4-8・D-13。ドメイン API の zip を使い回さない）"
  }
}
