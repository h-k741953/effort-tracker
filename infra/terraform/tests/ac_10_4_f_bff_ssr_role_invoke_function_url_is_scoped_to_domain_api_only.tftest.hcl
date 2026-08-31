# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、BFF/SSR 実行ロールのポリシーだけである
# （lambda:InvokeFunctionUrl の Resource がドメイン API の当該関数 ARN に限定
# されていること、いずれの文も Action にワイルドカードを含まないこと＝AC-7-1）。
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

# --- AC-10-4-f: BFF 実行ロールの InvokeFunctionUrl 権限 ---------------------
run "bff_ssr_role_invoke_function_url_is_scoped_to_domain_api_only" {
  command = plan

  # aws_lambda_function.domain_api.arn は provider が採番する computed 値で、
  # plan 時点では unknown になり比較できない（AC-10-2 の mock_provider +
  # command = plan を維持したまま、参照先だけダミー値で確定させる）。
  # ARN はダミー（架空のアカウントID）であり、実在の値ではない。
  override_resource {
    target = aws_lambda_function.domain_api
    values = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:effort-tracker-domain-api-test"
    }
    override_during = plan
  }

  assert {
    condition     = jsondecode(aws_iam_role_policy.bff_ssr_invoke_domain_api.policy).Statement[0].Resource == aws_lambda_function.domain_api.arn
    error_message = "BFF/SSR 実行ロールの lambda:InvokeFunctionUrl の Resource はドメイン API の当該関数 ARN に限定し、\"*\" にしてはならない（AC-7-1）"
  }

  # 10-4-f: AC-7-1 が持つ「〜のみ」の限定を検査へ写したもの。列挙が要求
  # どおりかは検査せず、広すぎないことだけを見る（12-10 と同型）。
  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.bff_ssr_invoke_domain_api.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !strcontains(action, "*")
      ])
    ])
    error_message = "BFF/SSR 実行ロールのポリシーは、いずれの文も Action にワイルドカード（*）を含んではならない（AC-7-1）"
  }
}
