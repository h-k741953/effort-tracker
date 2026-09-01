# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、aws_region 変数を**明示的に与えなかったとき**
# （＝変数の既定値が使われるとき）に、既定のリージョンが ap-northeast-1 で
# あることだけである（AC-3-1・D-3・AC-10-4-y）。
#
# provider ブロックの region そのものは plan に現れず検査できない（12-37）。
# 観測できるのは、aws_region の既定値を用いて構成が組み立てる plan 上の値まで
# である。ここでは、ドメイン API 実行ロールの SSM ポリシーが持つ ARN
# （arn:aws:ssm:${var.aws_region}:...）を代理として用いる —— この ARN は
# aws_region をそのまま埋め込んで組み立てられており（lambda_domain_api.tf）、
# リージョン値の持ち主を増やさない（AC-3-4）。
#
# **同じ ARN を assert する run がもう1本ある**
# （ac_10_4_g_domain_api_role_ssm_and_kms_are_scoped.tftest.hcl）。あちらは
# aws_region を**与えたうえで** var.aws_region を右辺に使い、SSM/KMS の
# スコープが要求どおりであることを検査する。**あちらの assert は
# var.aws_region を動的に埋め込むため、既定値が書き換わっても常に自己無矛盾
# で緑のままになる** —— 既定値そのものの検査にはならない。本ファイルは
# aws_region を与えず、右辺にリージョン名のリテラル ap-northeast-1 を書くこと
# で、既定値そのものを検査に載せる（AC-10-4-y が明示的に要求するリテラル）。
# 両者は担当する面が違い、片方だけを見て「aws_region は検査済み」と読まない
# こと。
#
# 上書きする run しか無い状態では、既定値は一度も plan に現れない（12-36）。
# 既存の run はいずれも variables ブロックで aws_region を上書きしており、
# 既定値を書き換えても全 run が緑のまま通ってしまう。本ファイルは上書きしない
# ことで、その面を検査に載せる。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。

mock_provider "aws" {}

# aws_region は**意図的に与えない**（既定値を評価させるため）。
# 評価対象を aws_region の既定値ただ1つに絞るため、与えないのは aws_region
# だけである（AC-10-4-y）。bff_ssr_lambda_runtime も既定値を持つ変数だが、
# ここでは他の run と同じ "nodejs20.x" を入力として与える —— これは値を
# 固定して plan を成立させるための入力であって期待値ではなく、版を固定する
# 意図は無い（版の持ち主は apps/web＝D-16・12-12）。
variables {
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "/effort-tracker/test/neon-connection-string"
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  bff_ssr_lambda_artifact_path               = "testdata/bff-ssr.zip"
  domain_api_lambda_artifact_path            = "testdata/domain-api.zip"
  cloudfront_killswitch_lambda_artifact_path = "testdata/cloudfront-killswitch.zip"
}

# --- AC-10-4-y: aws_region の既定値（ap-northeast-1） ------------------------
run "aws_region_default_is_ap_northeast_1" {
  command = plan

  assert {
    condition     = jsondecode(aws_iam_role_policy.domain_api_ssm.policy).Statement[0].Resource == "arn:aws:ssm:ap-northeast-1:${data.aws_caller_identity.current.account_id}:parameter${var.ssm_parameter_name}"
    error_message = "aws_region の既定値は ap-northeast-1 でなければならない（AC-3-1・D-3）"
  }
}
