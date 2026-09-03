# AC-9-l（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、User Pool のドメインがちょうど1つ存在し、当該 User Pool に属していること（AC-5-6 (i)）。そのドメインが ACM 証明書の ARN を参照していないこと（AC-7-4）。プレフィックスが変数から与えられており、リテラルの直書きでないこと（AC-5-6 (ii)） だけである。
#
# mock_provider を用い、実際の AWS API を呼ばない（P-6・P-9）。
#
# リソース名・変数名はこのテストが暫定的に固定するインターフェースである。
# 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。

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

run "domain_is_default_domain_from_variable" {
  command = plan

  # User Pool の id は provider が採番する computed 値であり、plan 時点では
  # unknown になる（ac_cognito_9_m と同型の理由）。ダミーの ID で
  # override_during = plan により確定させる。
  override_resource {
    target = aws_cognito_user_pool.this
    values = {
      id  = "ap-northeast-1_dummyPoolId"
      arn = "arn:aws:cognito-idp:ap-northeast-1:123456789012:userpool/ap-northeast-1_dummyPoolId"
    }
    override_during = plan
  }

  assert {
    condition     = aws_cognito_user_pool_domain.this.user_pool_id == aws_cognito_user_pool.this.id
    error_message = "User Pool のドメイン（aws_cognito_user_pool_domain.this）は、この構成が作る User Pool に属していなければならない（AC-5-6 (i)）"
  }

  assert {
    condition     = aws_cognito_user_pool_domain.this.certificate_arn == null
    error_message = "ドメインは ACM 証明書の ARN を参照してはならない（カスタムドメインでないこと。既定ドメインに限る＝AC-7-4）"
  }

  assert {
    condition     = aws_cognito_user_pool_domain.this.domain == var.cognito_domain_prefix
    error_message = "ドメインのプレフィックスは変数（cognito_domain_prefix）から与えられていなければならない。リテラルの直書きでない（AC-5-6 (ii)）"
  }
}
