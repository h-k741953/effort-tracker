# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、BFF/SSR Lambda の runtime が Node のマネージド
# ランタイムであり、provided.*（カスタムランタイム）でないことだけである
# （AC-4-7・D-16）。
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

# --- AC-10-4-n: BFF・SSR Lambda の runtime（Node マネージド） --------------
run "bff_ssr_runtime_is_a_node_managed_runtime" {
  command = plan

  assert {
    condition     = can(regex("^nodejs", aws_lambda_function.bff_ssr.runtime))
    error_message = "BFF/SSR Lambda の runtime は Node のマネージドランタイムでなければならない（AC-4-7・D-16）"
  }

  assert {
    condition     = !can(regex("^provided\\.", aws_lambda_function.bff_ssr.runtime))
    error_message = "BFF/SSR Lambda の runtime は provided.* （カスタムランタイム）であってはならない（AC-4-7・D-16）"
  }
}
