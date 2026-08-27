# CloudFront ディストリビューション（AC-4-2）。
#
# 静的アセットは S3 オリジン（OAC 経由）、SSR / Route Handler は
# BFF・SSR Lambda の Function URL オリジン（OAC による SigV4 署名。
# authorization_type = NONE を選ばない＝AC-4-3・ADR 0003）へ振り分ける。
# ドメイン API の Function URL はここのオリジンにしない（AC-4-4）。
# 代替ドメイン名は設定せず、viewer 証明書は CloudFront の既定証明書
# とする（ACM 証明書の ARN を参照しない＝AC-4-2・D-14）。

resource "aws_cloudfront_origin_access_control" "static_assets" {
  name                              = "effort-tracker-static-assets"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_origin_access_control" "bff_ssr" {
  name                              = "effort-tracker-bff-ssr"
  origin_access_control_origin_type = "lambda"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

locals {
  # Lambda Function URL のドメイン部分（スキームと末尾の "/" を除いた形）。
  bff_ssr_function_url_domain = trimsuffix(trimprefix(aws_lambda_function_url.bff_ssr.function_url, "https://"), "/")
}

resource "aws_cloudfront_distribution" "main" {
  enabled         = true
  is_ipv6_enabled = true
  comment         = "effort-tracker"

  origin {
    domain_name              = aws_s3_bucket.static_assets.bucket_regional_domain_name
    origin_id                = "static-assets"
    origin_access_control_id = aws_cloudfront_origin_access_control.static_assets.id
  }

  origin {
    domain_name              = local.bff_ssr_function_url_domain
    origin_id                = "bff-ssr"
    origin_access_control_id = aws_cloudfront_origin_access_control.bff_ssr.id

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  # 既定の振る舞い: SSR / Route Handler（BFF・SSR Lambda）へ。
  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "bff-ssr"
    viewer_protocol_policy = "redirect-to-https"

    # マネージド キャッシュポリシー "CachingDisabled"（SSR は都度動的）。
    cache_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
    # マネージド オリジンリクエストポリシー "AllViewerExceptHostHeader"。
    origin_request_policy_id = "b689b0a8-53d0-40ab-baf2-68738e2966ac"
  }

  # 静的アセットは S3 オリジンから配信する。
  ordered_cache_behavior {
    path_pattern           = "/_next/static/*"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "static-assets"
    viewer_protocol_policy = "redirect-to-https"

    # マネージド キャッシュポリシー "CachingOptimized"。
    cache_policy_id = "658327ea-f89d-4fab-a63d-7e88639e58f6"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  # 代替ドメイン名（aliases）は設定しない。既定証明書を使う（AC-4-2・D-14）。
  # 未設定のままだと null になり length() が期待通りに動かないため、
  # 空リストを明示する（AC-10-4-l）。
  aliases = []

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}
