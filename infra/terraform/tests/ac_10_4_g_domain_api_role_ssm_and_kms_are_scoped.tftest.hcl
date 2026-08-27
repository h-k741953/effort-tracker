# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 期待値のうち docs/rules/cost-guardrails.md が持つもの（同時実行数・ログ保持日数・
# Budgets の本数と閾値・authorization_type）は同ファイルを読んで書いている
# （AC-10-4）。値を infra 側へ二重に定義しない（AC-5-6）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# 構成（*.tf のリソース定義）はまだ無いため、各 run は「未実装」を理由に
# 落ちる（AC-10-3 の Red）。
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
