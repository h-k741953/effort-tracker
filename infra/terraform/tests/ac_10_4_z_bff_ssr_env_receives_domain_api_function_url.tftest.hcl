# AC-10-4-z（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、BFF・SSR Lambda の環境変数へ、この構成が作る
# ドメイン API の Function URL が注入されていること（未設定でなく、空でなく、
# 暫定値・無関係な値・別のリソース〔BFF・SSR Lambda 自身の Function URL を含む〕
# を指す値でないこと）だけである（AC-4-9。bff-wiring.md D 節 U-6）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
#
# 環境変数の「名前」と「Function URL の値」は本仕様が持たず、構成側が持つ
# （期待値を仕様から書き写して固定しない）。したがって本 run は特定のキー名を
# 固定せず、BFF・SSR Lambda の environment.variables を values() で丸ごと見て、
# (1) 空でない値が少なくとも1つ存在すること、(2) 値の集合が当該 Function URL の
# 属性（aws_lambda_function_url.domain_api.function_url）を含むこと、
# (3) BFF・SSR Lambda 自身の Function URL の属性を、ドメイン API の呼び先として
# 含まないこと（2つの URL を区別できる別々のダミー値で確定させて突き合わせる）、
# の3つを assert する（10-4-s と同型）。
#
# 限界（12-38）: 注入する環境変数の名前と、apps/web 側が読む名前の一致は
# 検査されない。apps/web 側が読むこと・欠落時の扱いは bff-wiring.md AC-3 の
# Vitest が持つ。
#
# リソース名はこのテストが暫定的に固定するインターフェースである（既存の
# 構成のリソース名 aws_lambda_function_url.domain_api / .bff_ssr に依る）。

mock_provider "aws" {}

variables {
  aws_region                                 = "ap-northeast-1"
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "/effort-tracker/test/neon-connection-string"
  bff_ssr_lambda_artifact_path               = "testdata/bff-ssr.zip"
  domain_api_lambda_artifact_path            = "testdata/domain-api.zip"
  cloudfront_killswitch_lambda_artifact_path = "testdata/cloudfront-killswitch.zip"
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
  public_origin                              = "https://public.example.test"
}

run "bff_ssr_env_receives_domain_api_function_url" {
  command = plan

  # Function URL は provider が採番する computed 値で、plan 時点では unknown に
  # なり比較できない（mock_provider + command = plan を維持したまま、参照先だけ
  # ダミー値で確定させる）。ドメイン API と BFF・SSR Lambda の URL は、取り違えを
  # 検出できるよう別々の（架空の）値にする。
  override_resource {
    target = aws_lambda_function_url.domain_api
    values = {
      function_url = "https://dummy-domain-api-0000000000.lambda-url.ap-northeast-1.on.aws/"
    }
    override_during = plan
  }

  override_resource {
    target = aws_lambda_function_url.bff_ssr
    values = {
      function_url = "https://dummy-bff-ssr-1111111111.lambda-url.ap-northeast-1.on.aws/"
    }
    override_during = plan
  }

  # BFF・SSR Lambda の environment.variables には Cognito の User Pool・
  # アプリクライアントの id も混ざる（computed。ac_cognito_9_m・9_s と同型の理由）。
  override_resource {
    target = aws_cognito_user_pool.this
    values = {
      id  = "ap-northeast-1_dummyPoolId"
      arn = "arn:aws:cognito-idp:ap-northeast-1:123456789012:userpool/ap-northeast-1_dummyPoolId"
    }
    override_during = plan
  }

  override_resource {
    target = aws_cognito_user_pool_client.bff
    values = {
      id = "dummyclientid0123456789abcdef"
    }
    override_during = plan
  }

  assert {
    condition = length([
      for v in values(try(aws_lambda_function.bff_ssr.environment[0].variables, {})) :
      v if v != null && v != ""
    ]) > 0
    error_message = "BFF・SSR Lambda の環境変数に、空でない値が注入されていなければならない（AC-4-9・10-4-z）"
  }

  assert {
    condition = contains(
      values(try(aws_lambda_function.bff_ssr.environment[0].variables, {})),
      aws_lambda_function_url.domain_api.function_url
    )
    error_message = "BFF・SSR Lambda の環境変数へ、この構成が作るドメイン API の Function URL（aws_lambda_function_url.domain_api.function_url）そのものが注入されていなければならない。リテラル・暫定値・無関係な値であってはならない（AC-4-9・10-4-z）"
  }

  assert {
    condition = !contains(
      values(try(aws_lambda_function.bff_ssr.environment[0].variables, {})),
      aws_lambda_function_url.bff_ssr.function_url
    )
    error_message = "BFF・SSR Lambda 自身の Function URL（aws_lambda_function_url.bff_ssr.function_url）を、ドメイン API の呼び先として注入してはならない（AC-4-9・10-4-z）"
  }
}
