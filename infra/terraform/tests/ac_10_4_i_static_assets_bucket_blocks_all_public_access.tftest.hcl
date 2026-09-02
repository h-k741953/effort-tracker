# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、静的アセット S3 バケットのパブリックアクセス遮断の
# 4設定（block_public_acls / block_public_policy / ignore_public_acls /
# restrict_public_buckets）だけである（AC-4-1）。
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
}

# --- AC-10-4-i: S3（静的アセット）パブリックアクセスブロック --------------
# state バケットのパブリックアクセスブロックは AC-2-4 により本体の state で
# 管理せず別ディレクトリで切り出すため、infra/bootstrap/tests/state_bucket.tftest.hcl
# が検査する。ここでは静的アセットバケットのみを検査する。
run "static_assets_bucket_blocks_all_public_access" {
  command = plan

  assert {
    condition     = aws_s3_bucket_public_access_block.static_assets.block_public_acls == true
    error_message = "静的アセット S3 バケットは block_public_acls = true でなければならない（AC-4-1）"
  }

  assert {
    condition     = aws_s3_bucket_public_access_block.static_assets.block_public_policy == true
    error_message = "静的アセット S3 バケットは block_public_policy = true でなければならない（AC-4-1）"
  }

  assert {
    condition     = aws_s3_bucket_public_access_block.static_assets.ignore_public_acls == true
    error_message = "静的アセット S3 バケットは ignore_public_acls = true でなければならない（AC-4-1）"
  }

  assert {
    condition     = aws_s3_bucket_public_access_block.static_assets.restrict_public_buckets == true
    error_message = "静的アセット S3 バケットは restrict_public_buckets = true でなければならない（AC-4-1）"
  }
}
