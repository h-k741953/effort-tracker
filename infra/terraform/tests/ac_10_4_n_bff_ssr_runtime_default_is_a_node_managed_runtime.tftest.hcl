# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、bff_ssr_lambda_runtime 変数を**明示的に与えなかった
# とき**（＝変数の既定値が使われるとき）にも、BFF/SSR Lambda の runtime が Node の
# マネージドランタイムであり、provided.*（カスタムランタイム）でないことだけで
# ある（AC-4-7・D-16）。
#
# **同じ AC-10-4-n を検査するファイルがもう1本ある。**
# ac_10_4_n_bff_ssr_runtime_is_a_node_managed_runtime.tftest.hcl がそれで、
# あちらは bff_ssr_lambda_runtime を**与えたうえで**同じ2つの assert を評価する。
# 両者は与える変数だけが違い、担当する面が違う。片方だけを見て「AC-10-4-n は
# 検査済み」と読まないこと。
#
# そのファイルを含む他の run はいずれも variables ブロックで
# bff_ssr_lambda_runtime を上書きしているため、**既定値そのものは一度も plan に
# 現れない**。既定値を Node 以外・provided.* へ書き換えても、上書きしている限り
# どの run も緑のまま通る。本ファイルは上書きしないことで、その面を検査に載せる。
#
# **版そのものの一致は検査しない**（12-12。版の持ち主は apps/web であり、
# 既定値の具体的な文字列を期待値へ書かない）。検査するのは「Node のマネージド
# ランタイムであること」までである（AC-10-4-n と同じ範囲）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。

mock_provider "aws" {}

# bff_ssr_lambda_runtime は**意図的に与えない**（既定値を評価させるため）。
# 既定値を持たない変数だけを与える。
variables {
  aws_region                                 = "ap-northeast-1"
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "/effort-tracker/test/neon-connection-string"
  bff_ssr_lambda_artifact_path               = "testdata/bff-ssr.zip"
  domain_api_lambda_artifact_path            = "testdata/domain-api.zip"
  cloudfront_killswitch_lambda_artifact_path = "testdata/cloudfront-killswitch.zip"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
  public_origin                              = "https://public.example.test"
}

# --- AC-10-4-n: BFF・SSR Lambda の runtime の既定値（Node マネージド） -------
run "bff_ssr_runtime_default_is_a_node_managed_runtime" {
  command = plan

  assert {
    condition     = can(regex("^nodejs", aws_lambda_function.bff_ssr.runtime))
    error_message = "bff_ssr_lambda_runtime の既定値は Node のマネージドランタイムでなければならない（AC-4-7・D-16）"
  }

  assert {
    condition     = !can(regex("^provided\\.", aws_lambda_function.bff_ssr.runtime))
    error_message = "bff_ssr_lambda_runtime の既定値は provided.*（カスタムランタイム）であってはならない（AC-4-7・D-16）"
  }
}
