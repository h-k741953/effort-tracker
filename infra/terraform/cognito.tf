# Cognito 認証基盤（Issue #52・docs/specs/cognito-auth-infra.md）。
#
# #8（infra-terraform.md）の既存 *.tf は変更しない（AC-1-1）。唯一の例外は
# BFF・SSR Lambda の環境変数注入であり、それは lambda_bff_ssr.tf 側に書く
# （AC-1-2）。新しい provider は増やさない（AC-1-3）。

# --- 変数（AC-4-2・AC-4-3・AC-5-3・AC-5-6・AC-8-7） --------------------------

variable "google_client_id" {
  description = "Google OIDC のクライアント ID（AC-4-2）。terraform.tfvars（gitignore 済み）から与える"
  type        = string
}

variable "google_client_secret" {
  description = "Google OIDC のクライアントシークレット（AC-4-2・AC-4-3）。terraform.tfvars から与え、既定値を持たない"
  type        = string
  sensitive   = true
}

variable "cognito_callback_urls" {
  description = "アプリクライアントのサインイン戻り先（AC-5-3・AC-5-4）。実 URL は apply 時に与える"
  type        = list(string)
}

variable "cognito_domain_prefix" {
  description = "User Pool の既定ドメインのプレフィックス（AC-5-6 (ii)）。実値は apply 時に与える"
  type        = string
}

variable "role_cookie_signing_key" {
  description = "デモ用ロール切替 Cookie の署名鍵（AC-8-6・AC-8-7。bff-auth-termination.md Q-F = (b)）。terraform.tfvars から与え、既定値を持たない"
  type        = string
  sensitive   = true
}

locals {
  cognito_user_pool_name  = "effort-tracker-users"
  cognito_app_client_name = "effort-tracker-bff"
}

# --- User Pool 本体（AC-2・AC-3） -------------------------------------------

resource "aws_cognito_user_pool" "this" {
  name = local.cognito_user_pool_name

  # AC-2-2: メールアドレスでサインインできる。電話番号は識別子に含めない。
  username_attributes = ["email"]

  # AC-2-3: メールアドレスの本人確認はメールのみで行う（SMS を検証手段に選ばない）。
  auto_verified_attributes = ["email"]

  # AC-2-4: パスワードポリシーを構成側に明示する（provider の既定へ委ねない）。
  password_policy {
    minimum_length                   = 12
    require_lowercase                = true
    require_numbers                  = true
    require_symbols                  = true
    require_uppercase                = true
    temporary_password_validity_days = 7
  }

  # AC-3-1・AC-3-3・Q-2 = (b): MFA は TOTP を有効にし、適用は「任意」に留める。
  mfa_configuration = "OPTIONAL"

  software_token_mfa_configuration {
    enabled = true
  }

  # AC-3-2: SMS の送信設定（sms_configuration）は持たない（ブロック自体を書かない）。

  # AC-2-6・Q-5 = (b): 削除保護は無効。構成側に明示する。
  deletion_protection = "INACTIVE"

  # AC-7-2: 脅威対策・高度なセキュリティ機能を有効にしない。課金階層を上位にしない。
  user_pool_add_ons {
    advanced_security_mode = "OFF"
  }

  user_pool_tier = "LITE"
}

# --- Google OIDC 連携（AC-4） ------------------------------------------------

resource "aws_cognito_identity_provider" "google" {
  user_pool_id  = aws_cognito_user_pool.this.id
  provider_name = "Google"
  provider_type = "Google"

  provider_details = {
    client_id        = var.google_client_id
    client_secret    = var.google_client_secret
    authorize_scopes = "openid email profile" # AC-4-5: 本人確認に要る最小のスコープに限る
  }

  # AC-4-4: Google から受け取る属性を User Pool の属性へ明示的に対応づける。
  attribute_mapping = {
    email    = "email"
    username = "sub"
  }
}

# --- アプリクライアント（AC-5） ----------------------------------------------

resource "aws_cognito_user_pool_client" "bff" {
  name         = local.cognito_app_client_name
  user_pool_id = aws_cognito_user_pool.this.id

  # AC-5-5: アプリクライアントがシークレットを持つかどうかの値は構成側が持つ。
  # BFF がトークン交換をサーバー側で行う前提のうえで、シークレットの受け渡し
  # 経路（AC-5-7・AC-8-2 の SSM 型）を本 Issue で新設しないため、シークレット
  # を持たないクライアントとして構成する。
  generate_secret = false

  # AC-5-2: User Pool 本体（COGNITO）と Google の両方を有効にする。
  supported_identity_providers = ["COGNITO", "Google"]

  # AC-5-3・AC-5-4: 戻り先は変数から。ワイルドカードを含めない（構成側の責務。
  # var.cognito_callback_urls 自体にワイルドカードを与えないことは apply 時の運用）。
  callback_urls = var.cognito_callback_urls

  # AC-5-6 (iii): 認可コードのみ。implicit を許可しない。
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email"]

  # AC-5-5: トークンの有効期限を構成側に明示する（provider の既定へ委ねない）。
  access_token_validity  = 60
  id_token_validity      = 60
  refresh_token_validity = 30

  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "days"
  }
}

# --- User Pool ドメイン（AC-5-6 (i)(ii)・Q-1 = (a)） -------------------------

resource "aws_cognito_user_pool_domain" "this" {
  domain       = var.cognito_domain_prefix
  user_pool_id = aws_cognito_user_pool.this.id

  # AC-7-4: カスタムドメイン・ACM 証明書は使わない（certificate_arn を与えない）。
}
