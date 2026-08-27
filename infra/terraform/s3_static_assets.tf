# 静的アセット用 S3 バケット（AC-4-1）。
#
# パブリックアクセスを全面ブロックし、CloudFront の OAC からのみ読める
# ようにする。ウェブサイトホスティング機能は有効にしない（公開エンド
# ポイントを増やさない）。バケット名は実値を書かず、衝突を避けるため
# prefix + 自動サフィックスで生成する。
resource "aws_s3_bucket" "static_assets" {
  bucket_prefix = "effort-tracker-static-"
}

resource "aws_s3_bucket_public_access_block" "static_assets" {
  bucket = aws_s3_bucket.static_assets.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# CloudFront（OAC）からのみ GetObject を許可する。Resource には当該
# ディストリビューションを SourceArn 条件で限定する（AC-4-1）。
resource "aws_s3_bucket_policy" "static_assets" {
  bucket = aws_s3_bucket.static_assets.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowCloudFrontOACRead"
        Effect    = "Allow"
        Principal = { Service = "cloudfront.amazonaws.com" }
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.static_assets.arn}/*"
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = aws_cloudfront_distribution.main.arn
          }
        }
      }
    ]
  })
}
