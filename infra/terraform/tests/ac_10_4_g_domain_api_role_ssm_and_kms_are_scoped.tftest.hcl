# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、ドメイン API 実行ロールのポリシーだけである
# （SSM 取得の Resource がパラメータ名の変数から組み立てた ARN 1つに限定されて
# いること＝AC-8-3-1、kms:Decrypt が条件も限定も無い Resource = "*" でないこと
# ＝AC-8-3-2、いずれの文も Action にワイルドカードを含まないこと＝AC-7-3）。
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

# --- AC-10-4-g: ドメイン API 実行ロールの SSM / KMS 権限 --------------------
run "domain_api_role_ssm_and_kms_are_scoped" {
  command = plan

  assert {
    condition     = jsondecode(aws_iam_role_policy.domain_api_ssm.policy).Statement[0].Resource == "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${var.ssm_parameter_name}"
    error_message = "SSM 取得の Resource はパラメータ名の変数から組み立てた ARN 1つに限定し、\"*\" やワイルドカードを含んではならず、parameter/ の直後の / を重ねてもならない（AC-8-3-1）"
  }

  assert {
    condition     = jsondecode(aws_iam_role_policy.domain_api_ssm.policy).Statement[1].Resource != "*"
    error_message = "kms:Decrypt の文は条件も限定も無い Resource = \"*\" になってはならない（AC-8-3-2）"
  }

  # 10-4-g: AC-7-3 が持つ「〜のみ」の限定を検査へ写したもの。列挙が要求
  # どおりかは検査せず、広すぎないことだけを見る（12-10 と同型）。
  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.domain_api_ssm.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "ドメイン API 実行ロールのポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-7-3）"
  }
}
