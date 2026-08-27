# 各 Lambda の関数名を1箇所で持つ。ロググループ名（AC-6-3）はここから
# 導出し、関数名と食い違わせない。aws_lambda_function.*.function_name の
# 属性を参照する形にすると「ロググループ→関数名→ロググループ」の循環に
# なるため、明示的な名前をここで固定し、両リソースがこの値を参照する。
locals {
  bff_ssr_function_name               = "effort-tracker-bff-ssr"
  domain_api_function_name            = "effort-tracker-domain-api"
  cloudfront_killswitch_function_name = "effort-tracker-cloudfront-killswitch"
}
