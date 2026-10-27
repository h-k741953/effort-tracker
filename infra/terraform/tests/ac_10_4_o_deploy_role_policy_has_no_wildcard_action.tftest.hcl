# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、デプロイロールのポリシーの Action だけである
# （Action = "*" の文を持たないこと、D-15 が列挙するサービス接頭辞
# （S3/CloudFront/Lambda/IAM/SSM/Budgets/Logs）に収まること＝AC-7-8）。
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
  demo_data_reset_lambda_artifact_path       = "testdata/demo-data-reset.zip"
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
  public_origin                              = "https://public.example.test"
}

# --- AC-10-4-o: デプロイロールのポリシー（Action = "*" を持たない） -------
run "deploy_role_policy_has_no_wildcard_action" {
  command = plan

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement :
      stmt.Action != "*" && !contains(try(tolist(stmt.Action), [stmt.Action]), "*")
    ])
    error_message = "デプロイロールのポリシーは Action = \"*\" の文を持ってはならない（AC-7-8）"
  }

  # D-15 が限定列挙するサービス（S3 / CloudFront / Lambda / IAM / SSM /
  # Budgets / Logs）のサービス接頭辞に、Action が収まっていることを見る。
  # 列挙が十分かどうかは検査しない（12-10）。弾くのは裸の "*" だけであり、
  # s3:* のようなサービス内ワイルドカードは対象外（12-29・10-4-f/g/h とは
  # 別枠）。
  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        anytrue([
          for prefix in ["s3:", "cloudfront:", "lambda:", "iam:", "ssm:", "budgets:", "logs:", "scheduler:"] :
          startswith(action, prefix)
        ])
      ])
    ])
    error_message = "デプロイロールのポリシーの Action は D-15 の列挙（S3/CloudFront/Lambda/IAM/SSM/Budgets/Logs/Scheduler）のサービス接頭辞に収まらなければならない（AC-7-8）"
  }

  # 2026-10-27 改訂（#56・ADR 0022）: scheduler: で始まる Action は、スケジュールを
  # 作る（管理する）4つのいずれかに限る（AC-7-8・D-15）。scheduler:*、
  # スケジュールグループの操作（*ScheduleGroup）、タグの操作を拒む。
  # 列挙が十分かどうか（4つが実際に含まれるか）は検査しない（12-10）ので、
  # これは「広すぎないこと」だけを見る。
  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !startswith(action, "scheduler:") || contains([
          "scheduler:CreateSchedule",
          "scheduler:GetSchedule",
          "scheduler:UpdateSchedule",
          "scheduler:DeleteSchedule",
        ], action)
      ])
    ])
    error_message = "デプロイロールのポリシーの scheduler: の Action は scheduler:CreateSchedule・GetSchedule・UpdateSchedule・DeleteSchedule の4つのいずれかに限らなければならない（scheduler:*・ScheduleGroup・タグの操作を含めない＝AC-7-8・10-4-o）"
  }

  # 空集合で緑にならないこと: 文が1つ以上あり、上の for 式が空を回っていない。
  assert {
    condition     = length(jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement) > 0
    error_message = "デプロイロールのポリシーは文を1つ以上持たなければならない（10-4-o の for 式が空集合で通らないようにする）"
  }
}
