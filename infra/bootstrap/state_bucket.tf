# infra/terraform の Terraform state を保管する S3 バケット（D-11）。
# ロックは DynamoDB を使わず S3 ネイティブの lockfile 機能に拠る（D-2。
# infra/terraform/backend.tf 側の use_lockfile = true と対）。

resource "aws_s3_bucket" "state" {
  bucket_prefix = "effort-tracker-tfstate-"
}

# AC-2-3: パブリックアクセスを一切許可しない。
resource "aws_s3_bucket_public_access_block" "state" {
  bucket = aws_s3_bucket.state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# AC-2-3: state の上書き事故に備えてバージョニングを有効にする。
resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id

  versioning_configuration {
    status = "Enabled"
  }
}

# AC-2-3: サーバ側暗号化（S3 管理鍵で足りる。state に長期認証情報を
# 書かない運用のため、KMS 管理鍵までは要求しない）。
resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}
