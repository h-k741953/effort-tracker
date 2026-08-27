# tfstate backend（AC-2）。
#
# S3 backend の native lockfile を使い、DynamoDB のロックテーブルは
# 作らない（D-2・AC-2-1・AC-2-2）。この要求（use_lockfile が有効・
# DynamoDB のロック設定を持たないこと）は terraform test では機械検査
# されない（12-20。plan に現れない値であり、テスト実行時には backend が
# 上書きされるため）。担保はレビューと規律（D-24）。
#
# バケット名・キー・リージョンは実値を書かない（AC-2-5）。バケット自身は
# infra/bootstrap（AC-2-4）が作り、ここでは参照しない（D-11）。
# `terraform init -backend-config=<環境>.s3.tfbackend` で補う想定であり、
# `*.tfbackend` は .gitignore 対象（AC-2-5）。
terraform {
  backend "s3" {
    use_lockfile = true
  }
}
