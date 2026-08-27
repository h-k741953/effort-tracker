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

# --- AC-10-4-s: 遮断 Lambda への遮断対象の受け渡し（環境変数への注入） -----
#
# 環境変数の「名前」と「ID の値」は本仕様（AC-9-12・10-4-s）が持たず、構成側が
# 持つ。したがって本 run は特定のキー名を書き写して固定しない。遮断 Lambda の
# environment.variables を values() で丸ごと見て、(1) 未設定でなく空でない値が
# 少なくとも1つ存在すること、(2) その値の集合が当該ディストリビューションの
# ID（aws_cloudfront_distribution.main.id）を含むこと、の両方を assert する
# （AC-4-2：この構成が作るディストリビューションは1つ）。
run "cloudfront_killswitch_receives_distribution_id_via_env" {
  command = plan

  # aws_cloudfront_distribution.main.id は provider が採番する computed 値で、
  # plan 時点では unknown になり比較できない（AC-10-2 の mock_provider +
  # command = plan を維持したまま、参照先だけダミー値で確定させる）。ID・ARN は
  # いずれもダミー（架空のディストリビューション ID・ARN）であり、実在の値では
  # ない。arn も併せて確定させないと、同じディストリビューションを参照する
  # 他リソース（CloudFront 向け lambda 権限の source_arn）が不正な ARN 形式の
  # まま plan され、本 run と無関係な理由で fail する。
  override_resource {
    target = aws_cloudfront_distribution.main
    values = {
      id  = "EDFDVBD6EXAMPLE"
      arn = "arn:aws:cloudfront::123456789012:distribution/EDFDVBD6EXAMPLE"
    }
    override_during = plan
  }

  assert {
    condition = length([
      for v in values(try(aws_lambda_function.cloudfront_killswitch.environment[0].variables, {})) :
      v if v != null && v != ""
    ]) > 0
    error_message = "遮断 Lambda の環境変数に、遮断対象のディストリビューション ID が注入されていなければならない（未設定でなく、空でないこと＝AC-9-12・10-4-s）"
  }

  assert {
    condition = contains(
      values(try(aws_lambda_function.cloudfront_killswitch.environment[0].variables, {})),
      aws_cloudfront_distribution.main.id
    )
    error_message = "遮断 Lambda の環境変数へ注入された値は、この構成が作る当該ディストリビューションそのもの（aws_cloudfront_distribution.main.id）を指していなければならない。暫定値・無関係な値・別のリソースを指す値であってはならない（AC-4-2・10-4-s）"
  }
}
