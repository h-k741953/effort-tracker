# AC-2-4: state バケット自身は本体（infra/terraform）の state で管理しない。
# 別ディレクトリの最小構成として切り出す。ここは本体の backend を参照しない
# （bootstrap 自身は当面ローカル state。AC-1-2 の整形・検証の対象に含める）。
terraform {
  required_version = ">= 1.9.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.61"
    }
  }
}
