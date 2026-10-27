# AC-12-2・AC-12-3-a・AC-11-5（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、デモデータのリセットを起動する EventBridge
# Scheduler のスケジュールだけである。
#   - 毎日 04:00、タイムゾーンは Asia/Tokyo（UTC に換算した値を書かない）
#   - 柔軟な時間枠を設けない（mode = OFF）
#   - 再試行回数 0（AC-11-5。Scheduler 側の設定の形は「再試行回数 0」）
#   - 呼び先はリセット用 Lambda 1つだけ。実行ロールは Scheduler が引き受ける
#     ロール（権限は ac_demo_reset_12_3_scheduler_role_*）
# 「スケジュールが1本だけ」「Function URL・リソースベースのポリシーが無い」は
# 構成に無いものの検査になるため assert しない（AC-12-10・infra-terraform.md 12-21）。
#
# cron の式は文字列の完全一致でなく、フィールドに分けて見る（04:00 の毎日を
# 表す書き方が複数あるため。分=0・時=4・月=*・年=*、日と曜日は * か ?）。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
# リセット用 Lambda と Scheduler ロールの arn は plan 時点で unknown になるため、
# override_resource でダミー値（架空のアカウント ID）に確定させる。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
# （aws_scheduler_schedule.demo_data_reset / aws_iam_role.demo_data_reset_scheduler）

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

# --- AC-12-2・AC-12-3-a・AC-11-5: スケジュール --------------------------------
run "demo_data_reset_schedule_is_daily_0400_asia_tokyo_without_retry_and_targets_reset_lambda" {
  command = plan

  override_resource {
    target = aws_lambda_function.demo_data_reset
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-demo-data-reset-test"
    }
    override_during = plan
  }

  override_resource {
    target = aws_iam_role.demo_data_reset_scheduler
    values = {
      arn = "arn:aws:iam::123456789012:role/effort-tracker-demo-data-reset-scheduler-test"
    }
    override_during = plan
  }

  # 取り違え検出用に、ほかの3本の arn も（互いに異なる）ダミー値に確定させる。
  override_resource {
    target = aws_lambda_function.domain_api
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-domain-api-test"
    }
    override_during = plan
  }

  override_resource {
    target = aws_lambda_function.bff_ssr
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-bff-ssr-test"
    }
    override_during = plan
  }

  override_resource {
    target = aws_lambda_function.cloudfront_killswitch
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-cloudfront-killswitch-test"
    }
    override_during = plan
  }

  # AC-12-2: 毎日 04:00。cron(分 時 日 月 曜日 年) = cron(0 4 * * ? *) 等。
  assert {
    condition     = startswith(aws_scheduler_schedule.demo_data_reset.schedule_expression, "cron(") && endswith(aws_scheduler_schedule.demo_data_reset.schedule_expression, ")")
    error_message = "スケジュールの式は cron(...) でなければならない（rate・at は不可。AC-12-2）"
  }

  assert {
    condition     = length(split(" ", trimsuffix(trimprefix(aws_scheduler_schedule.demo_data_reset.schedule_expression, "cron("), ")"))) == 6
    error_message = "cron の式は 6 フィールド（分 時 日 月 曜日 年）でなければならない（AC-12-2）"
  }

  assert {
    condition = alltrue([
      for f in [split(" ", trimsuffix(trimprefix(aws_scheduler_schedule.demo_data_reset.schedule_expression, "cron("), ")"))] :
      length(f) == 6 && f[0] == "0" && f[1] == "4" && f[3] == "*" && f[5] == "*" && contains(["*", "?"], f[2]) && contains(["*", "?"], f[4])
    ])
    error_message = "スケジュールは毎日 04:00（分=0・時=4・月=*・年=*、日と曜日は * か ?）でなければならない。UTC に換算した時刻を書かない（AC-12-2）"
  }

  assert {
    condition     = aws_scheduler_schedule.demo_data_reset.schedule_expression_timezone == "Asia/Tokyo"
    error_message = "スケジュールのタイムゾーンは Asia/Tokyo でなければならない（スケジュール側で指定する。UTC 換算の値を書かない＝AC-12-2）"
  }

  # AC-12-2: 実行時刻をずらす幅（柔軟な時間枠）を設けない。
  assert {
    condition     = length(aws_scheduler_schedule.demo_data_reset.flexible_time_window) == 1
    error_message = "flexible_time_window ブロックを明示しなければならない（必須ブロック。AC-12-2）"
  }

  assert {
    condition     = aws_scheduler_schedule.demo_data_reset.flexible_time_window[0].mode == "OFF"
    error_message = "柔軟な時間枠を設けない（flexible_time_window.mode = OFF。AC-12-2）"
  }

  assert {
    condition     = try(aws_scheduler_schedule.demo_data_reset.flexible_time_window[0].maximum_window_in_minutes, null) == null
    error_message = "柔軟な時間枠を設けないため maximum_window_in_minutes を与えてはならない（AC-12-2）"
  }

  # AC-12-3-a: 呼び先はリセット用 Lambda だけ（1つのターゲット）。
  assert {
    condition     = length(aws_scheduler_schedule.demo_data_reset.target) == 1
    error_message = "スケジュールのターゲットはちょうど1つ（リセット用 Lambda）でなければならない（AC-12-3-a）"
  }

  assert {
    condition     = aws_scheduler_schedule.demo_data_reset.target[0].arn == aws_lambda_function.demo_data_reset.arn
    error_message = "スケジュールの呼び先はリセット用 Lambda でなければならない。遮断 Lambda・ドメイン API・BFF/SSR を呼び先にしない（AC-12-3-a）"
  }

  assert {
    condition = alltrue([
      aws_scheduler_schedule.demo_data_reset.target[0].arn != aws_lambda_function.domain_api.arn,
      aws_scheduler_schedule.demo_data_reset.target[0].arn != aws_lambda_function.bff_ssr.arn,
      aws_scheduler_schedule.demo_data_reset.target[0].arn != aws_lambda_function.cloudfront_killswitch.arn,
    ])
    error_message = "スケジュールの呼び先に、ドメイン API・BFF/SSR・CloudFront 遮断 Lambda を指定してはならない（AC-12-3-a）"
  }

  # AC-12-3: Scheduler が引き受けるロールを渡す。
  assert {
    condition     = aws_scheduler_schedule.demo_data_reset.target[0].role_arn == aws_iam_role.demo_data_reset_scheduler.arn
    error_message = "スケジュールのターゲットは Scheduler が引き受けるロール（aws_iam_role.demo_data_reset_scheduler）を role_arn に指定しなければならない（AC-12-3）"
  }

  # AC-11-5: Scheduler の再試行をしない（再試行回数 0）。
  assert {
    condition     = length(aws_scheduler_schedule.demo_data_reset.target[0].retry_policy) == 1
    error_message = "retry_policy ブロックを明示しなければならない（未指定だと既定の再試行が掛かる＝AC-11-5）"
  }

  assert {
    condition     = aws_scheduler_schedule.demo_data_reset.target[0].retry_policy[0].maximum_retry_attempts == 0
    error_message = "スケジュールの再試行回数は 0 でなければならない（日中に再試行が走って利用者の入力を破棄しない＝AC-11-5）"
  }
}
