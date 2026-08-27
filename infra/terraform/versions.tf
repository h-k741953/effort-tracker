# Terraform 本体・provider の版の要求だけを書く。
# 版の実値は docs/specs/infra-terraform.md P-3 / AC-1-6 により
# .devcontainer/Dockerfile と .github/workflows/ci.yml が現況として持つ。
# ここに書き写さない。「terraform test（D-6）・use_lockfile（D-2）を
# 解釈できる版であること」という制約だけを表す。
terraform {
  required_version = ">= 1.9.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.61"
    }
  }
}
