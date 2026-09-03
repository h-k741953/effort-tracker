# AC-9-h（docs/specs/cognito-auth-infra.md）の検査。
#
# 本ファイルが検査するのは、アプリクライアントがちょうど1つ存在し、当該 User Pool に属していること（AC-5-1）。有効な ID プロバイダに、User Pool 本体と Google の双方が含まれること（AC-5-2） だけである。
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

run "app_client_has_both_identity_providers" {
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
    condition     = aws_cognito_user_pool_client.bff.user_pool_id == aws_cognito_user_pool.this.id
    error_message = "アプリクライアント（aws_cognito_user_pool_client.bff）は、この構成が作る User Pool に属していなければならない（AC-5-1）"
  }

  assert {
    condition     = contains(aws_cognito_user_pool_client.bff.supported_identity_providers, "COGNITO")
    error_message = "アプリクライアントは User Pool 本体（COGNITO）を有効な ID プロバイダに含めなければならない（AC-5-2）"
  }

  assert {
    condition     = contains(aws_cognito_user_pool_client.bff.supported_identity_providers, "Google")
    error_message = "アプリクライアントは Google を有効な ID プロバイダに含めなければならない（AC-5-2）"
  }
}
