# AC-12-6（docs/specs/demo-data-reset.md）の検査。
#
# 本ファイルが検査するのは、リセット用 Lambda 実行ロールの権限が
# ①自身のロググループへの書き込み と ②接続文字列の SSM パラメータ1つの取得と
# その復号 だけであること（Action にワイルドカードを含まず、Resource = "*" を
# 置かない）である。②の限定の仕方は infra-terraform.md AC-8-3-1・AC-8-3-2
# （ac_10_4_g がドメイン API で検査している形）と同じにする。
# ログ書き込みの Resource が自身のロググループに限られることは ac_10_4_t が持つ。
#
# 「列挙が要求どおりか」でなく「許す Action がこの2つだけ」を見る。
# 構成に無いもの（ほかのポリシー・マネージドポリシーのアタッチ）は assert
# しない（infra-terraform.md 12-21 と同じ扱い。担保はレビューと規律）。
#
# mock_provider を用い、実際の AWS API を呼ばない（infra-terraform.md AC-10-2）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。
# （aws_iam_role_policy.demo_data_reset_ssm / aws_iam_role_policy.demo_data_reset_logs）

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

# --- AC-12-6 ②: SSM パラメータ1つの取得と復号に限る ------------------------
run "demo_data_reset_role_ssm_and_kms_are_scoped" {
  command = plan

  # 許す Action はこの2つだけ（取得と復号）。文が空・別の Action が混ざる形を拒む。
  assert {
    condition = toset(flatten([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_ssm.policy).Statement :
      try(tolist(stmt.Action), [stmt.Action])
    ])) == toset(["ssm:GetParameter", "kms:Decrypt"])
    error_message = "リセット用 Lambda 実行ロールの SSM 向けポリシーが許す Action は ssm:GetParameter と kms:Decrypt の2つだけでなければならない（AC-12-6 ②）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_ssm.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "リセット用 Lambda 実行ロールのポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-12-6）"
  }

  # ssm:GetParameter の文の Resource は、パラメータ名の変数から組み立てた ARN 1つ。
  assert {
    condition = length([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_ssm.policy).Statement :
      stmt if contains(try(tolist(stmt.Action), [stmt.Action]), "ssm:GetParameter")
    ]) == 1
    error_message = "ssm:GetParameter を許す文はちょうど1つでなければならない（AC-12-6 ②）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_ssm.policy).Statement :
      toset(try(tolist(stmt.Resource), [stmt.Resource])) == toset(["arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${var.ssm_parameter_name}"])
      if contains(try(tolist(stmt.Action), [stmt.Action]), "ssm:GetParameter")
    ])
    error_message = "SSM 取得の Resource はパラメータ名の変数から組み立てた ARN 1つに限定し、\"*\" やワイルドカードを含んではならない（AC-12-6 ②・infra-terraform.md AC-8-3-1）"
  }

  # kms:Decrypt の文の Resource は、alias/aws/ssm が指す実キーの ARN に限る（"*" でない）。
  assert {
    condition = length([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_ssm.policy).Statement :
      stmt if contains(try(tolist(stmt.Action), [stmt.Action]), "kms:Decrypt")
    ]) == 1
    error_message = "kms:Decrypt を許す文はちょうど1つでなければならない（AC-12-6 ②）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_ssm.policy).Statement :
      toset(try(tolist(stmt.Resource), [stmt.Resource])) == toset([data.aws_kms_alias.ssm.target_key_arn])
      if contains(try(tolist(stmt.Action), [stmt.Action]), "kms:Decrypt")
    ])
    error_message = "kms:Decrypt の Resource は alias/aws/ssm が指すキーの ARN 1つに限定しなければならず、\"*\" にしてはならない（AC-12-6 ②・infra-terraform.md AC-8-3-2）"
  }
}

# --- AC-12-6 ①: ログ書き込みに限る（Resource の限定は ac_10_4_t） -----------
run "demo_data_reset_role_logs_policy_only_writes_logs" {
  command = plan

  # ポリシーの Resource が当該ロググループの arn（plan 時点で unknown）から組み立てられる
  # ため、ダミー値（架空のアカウント ID）に確定させる（ac_10_4_t と同型）。
  override_resource {
    target = aws_cloudwatch_log_group.demo_data_reset
    values = {
      arn = "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/effort-tracker-demo-data-reset-test"
    }
    override_during = plan
  }

  assert {
    condition     = length(jsondecode(aws_iam_role_policy.demo_data_reset_logs.policy).Statement) > 0
    error_message = "リセット用 Lambda 実行ロールのログ書き込みポリシーは、文を1つ以上持たなければならない（AC-12-6 ①）"
  }

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.demo_data_reset_logs.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        startswith(action, "logs:") && !strcontains(action, "*")
      ])
    ])
    error_message = "リセット用 Lambda 実行ロールのログ書き込みポリシーの Action は logs: の書き込みに限り、ワイルドカードを含んではならない（AC-12-6 ①）"
  }
}
