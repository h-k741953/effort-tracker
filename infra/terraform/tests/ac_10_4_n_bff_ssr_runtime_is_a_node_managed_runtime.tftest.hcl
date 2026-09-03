# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、BFF/SSR Lambda の runtime が Node のマネージド
# ランタイムであり、provided.*（カスタムランタイム）でないことだけである
# （AC-4-7・D-16）。
#
# **同じ AC-10-4-n を検査するファイルがもう1本ある。**
# ac_10_4_n_bff_ssr_runtime_default_is_a_node_managed_runtime.tftest.hcl が
# それで、両者は**与える変数が違う**。本ファイルは下の variables ブロックで
# bff_ssr_lambda_runtime を明示的に与えるため、**変数の既定値は plan に現れず、
# ここでは一切検査されない**。既定値の面はもう1本が担う（意図的に与えない）。
# 片方だけを見て「AC-10-4-n は検査済み」と読まないこと。
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
