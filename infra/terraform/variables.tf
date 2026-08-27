# 本体の構成が受け取る変数の宣言だけを置く（AC-1〜AC-9 の各項が指す変数）。
# 値の実体（cost-guardrails.md の数値・OIDC の owner/repo 等）はここには書かず、
# .tftest.hcl 側の variables ブロックまたは実 apply 時の tfvars が渡す。

variable "aws_region" {
  description = "既定リージョン（AC-3-1・D-3）"
  type        = string
  default     = "ap-northeast-1"
}

variable "github_oidc_repo_owner" {
  description = "GitHub OIDC 信頼ポリシーの sub に使う repo owner（AC-7-6）"
  type        = string
}

variable "github_oidc_repo_name" {
  description = "GitHub OIDC 信頼ポリシーの sub に使う repo name（AC-7-6）"
  type        = string
}

variable "ssm_parameter_name" {
  description = "Neon 接続文字列を保持する SSM パラメータ名（AC-8-1・D-4・D-12）"
  type        = string

  validation {
    # ARN は "parameter${var.ssm_parameter_name}" として組み立てる
    # （lambda_domain_api.tf）ため、パラメータ名の先頭に "/" が
    # 含まれていないと別のパラメータを指す ARN になる（AC-8-3-1）。
    condition     = startswith(var.ssm_parameter_name, "/")
    error_message = "ssm_parameter_name は \"/\" から始まる必要がある（AC-8-3-1）。"
  }
}

variable "bff_ssr_lambda_artifact_path" {
  description = "BFF/SSR Lambda の成果物パス（AC-4-5・D-13）"
  type        = string
}

variable "domain_api_lambda_artifact_path" {
  description = "ドメイン API Lambda の成果物パス（AC-4-5・D-13・AC-9-4）"
  type        = string
}

variable "cloudfront_killswitch_lambda_artifact_path" {
  description = "CloudFront 遮断 Lambda の成果物パス（AC-4-8・D-13・AC-9-8。ドメイン API とは別ファイル）"
  type        = string
}

variable "bff_ssr_lambda_runtime" {
  description = "BFF/SSR Lambda のランタイム識別子（AC-4-7・D-16。Node のマネージドランタイム。版は apps/web が固定する）"
  type        = string
  default     = "nodejs24.x"
}
