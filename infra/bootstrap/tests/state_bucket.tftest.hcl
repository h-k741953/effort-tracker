# AC-10-4-i（state 側）: state バケットのパブリックアクセスブロック（AC-2-3）。
# 静的アセット側は
# infra/terraform/tests/ac_10_4_i_static_assets_bucket_blocks_all_public_access.tftest.hcl
# が持つ（二重管理を避ける）。

mock_provider "aws" {}

# --- AC-2-3: state バケットの保護 -------------------------------------------
run "state_bucket_blocks_all_public_access_and_has_versioning_and_encryption" {
  command = plan

  assert {
    condition     = aws_s3_bucket_public_access_block.state.block_public_acls == true
    error_message = "state バケットは block_public_acls = true でなければならない（AC-2-3）"
  }

  assert {
    condition     = aws_s3_bucket_public_access_block.state.block_public_policy == true
    error_message = "state バケットは block_public_policy = true でなければならない（AC-2-3）"
  }

  assert {
    condition     = aws_s3_bucket_public_access_block.state.ignore_public_acls == true
    error_message = "state バケットは ignore_public_acls = true でなければならない（AC-2-3）"
  }

  assert {
    condition     = aws_s3_bucket_public_access_block.state.restrict_public_buckets == true
    error_message = "state バケットは restrict_public_buckets = true でなければならない（AC-2-3）"
  }

  assert {
    condition     = aws_s3_bucket_versioning.state.versioning_configuration[0].status == "Enabled"
    error_message = "state バケットはバージョニングを有効にしなければならない（AC-2-3）"
  }

  assert {
    condition     = length(aws_s3_bucket_server_side_encryption_configuration.state.rule) > 0
    error_message = "state バケットはサーバ側暗号化を有効にしなければならない（AC-2-3）"
  }
}
