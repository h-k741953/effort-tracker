# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、CloudFront が既定証明書を使い（ACM 証明書の ARN を
# 参照せず）、代替ドメイン名（aliases）を持たないことだけである（AC-4-2・D-14）。
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

# --- AC-10-4-l: CloudFront の viewer 証明書（既定証明書・aliases 空） ------
run "cloudfront_uses_default_certificate_and_no_aliases" {
  command = plan

  assert {
    condition     = aws_cloudfront_distribution.main.viewer_certificate[0].cloudfront_default_certificate == true
    error_message = "CloudFront は既定証明書を使わなければならない（ACM 証明書の ARN を参照しない＝AC-4-2・D-14）"
  }

  assert {
    condition     = length(aws_cloudfront_distribution.main.aliases) == 0
    error_message = "CloudFront の代替ドメイン名（aliases）は空でなければならない（AC-4-2・D-14）"
  }
}
