# AC-11-5（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、リセット用 Lambda の非同期呼び出しの再試行回数が
# 0 であることだけである（Scheduler 側の再試行回数 0 は
# ac_demo_reset_12_2_*）。理由は「記録するだけ」という決定に合わせ、日中に
# 再試行が走って利用者の入力を破棄しないようにするためである。
#
# 手段: aws_lambda_function_event_invoke_config の maximum_retry_attempts = 0
# （demo-data-reset.md は手段を定めておらず、tester 工程が選んだ）。
# 未指定の既定は 2 回の再試行なので、明示の 0 であることを assert する。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
# リセット用 Lambda の arn は plan 時点で unknown になるため、override_resource で
# ダミー値に確定させる。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
# （aws_lambda_function_event_invoke_config.demo_data_reset）

mock_provider "aws" {}

variables {
  aws_region                                 = "ap-northeast-1"
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "/effort-tracker/test/neon-connection-string"
  bff_ssr_lambda_artifact_path               = "testdata/bff-ssr.zip"
  domain_api_lambda_artifact_path            = "testdata/domain-api.zip"
  cloudfront_killswitch_lambda_artifact_path = "testdata/cloudfront-killswitch.zip"
  demo_data_reset_lambda_artifact_path       = "testdata/demo-data-reset.zip"
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
  public_origin                              = "https://public.example.test"
}

# --- AC-11-5: Lambda の非同期呼び出しの再試行回数 0 -------------------------
run "demo_data_reset_async_invocation_has_no_retry" {
  command = plan

  override_resource {
    target = aws_lambda_function.demo_data_reset
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-demo-data-reset-test"
    }
    override_during = plan
  }

  assert {
    condition     = aws_lambda_function_event_invoke_config.demo_data_reset.maximum_retry_attempts == 0
    error_message = "リセット用 Lambda の非同期呼び出しの再試行回数は 0 でなければならない（AC-11-5。未指定の既定は 2 回）"
  }

  # 設定の掛かり先がリセット用 Lambda であること（別の関数に掛けて緑にしない）。
  assert {
    condition = contains(
      [aws_lambda_function.demo_data_reset.function_name, aws_lambda_function.demo_data_reset.arn],
      aws_lambda_function_event_invoke_config.demo_data_reset.function_name
    )
    error_message = "非同期呼び出しの再試行回数 0 は、リセット用 Lambda（aws_lambda_function.demo_data_reset）に掛けなければならない（AC-11-5）"
  }
}
