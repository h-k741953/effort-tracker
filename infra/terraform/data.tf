# AC-8-3-1: SSM パラメータの ARN をリージョン・アカウント ID から組み立てる
# ために要る data ソース。実値を構成にもドキュメントにも直書きしない
# （AC-2-5）。
data "aws_caller_identity" "current" {}

# AC-8-3-2: SecureString の復号鍵は AWS 管理鍵 alias/aws/ssm（D-4）。
# 顧客管理鍵を作らない（AC-11-11）。kms:Decrypt の Resource は、この
# エイリアスが指す実キーの ARN に限定する（エイリアス ARN 自体は
# IAM の Resource として認可の単位にならないため）。
data "aws_kms_alias" "ssm" {
  name = "alias/aws/ssm"
}
