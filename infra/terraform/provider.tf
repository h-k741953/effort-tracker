# 既定 provider のみ（AC-3-1・AC-3-2）。alias 付き provider は置かない。
provider "aws" {
  region = var.aws_region
}
