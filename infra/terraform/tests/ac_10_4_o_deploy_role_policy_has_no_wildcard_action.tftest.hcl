# AC-10（docs/specs/infra-terraform.md）の検査。
#
# 本ファイルが検査するのは、デプロイロールのポリシー
# （aws_iam_role_policy.github_actions_deploy）の本文である。
# - Action: "*" の文を持たないこと、Action が D-15 の列挙（AC-7-8）の
#   サービス接頭辞に収まること、scheduler: の Action が AC-7-8 の
#   scheduler の4つに限られること（2026-10-27 改訂）
# - IAM の権限の限定（10-4-o (1)〜(7)、AC-7-8 ①〜⑦）: iam: の Action の列挙・
#   Resource の形・iam:PassRole の文の形と渡し先の条件・自己保護の Deny・
#   for 式が空を回って緑になることの防止
# 期待値の持ち主は AC-7-8（D-15）。本ファイルは列挙の写しを持つが、
# 変更は AC-7-8 が先（下の 2026-11-10 追記の節も同じ）。
#
# mock_provider を用い、実際の AWS API を呼ばない（AC-10-2）。
# 構成（*.tf のリソース定義）は実装済みであり、各 run はその構成に対して実際に
# 評価される（AC-10-3 の Red は解消済み）。
#
# リソース名はこのテストが暫定的に固定するインターフェースである。実装側は
# 同じ名前で作ってよいし、都合が悪ければテストごと見直す。

mock_provider "aws" {}

variables {
  aws_region                                 = "ap-northeast-1"
  github_oidc_repo_owner                     = "h-k741953"
  github_oidc_repo_name                      = "effort-tracker"
  ssm_parameter_name                         = "/effort-tracker/test/neon-connection-string"
  bff_ssr_lambda_artifact_path               = "testdata/bff-ssr.zip"
  domain_api_lambda_artifact_path            = "testdata/domain-api.zip"
  cloudfront_killswitch_lambda_artifact_path = "testdata/cloudfront-killswitch.zip"
  demo_data_reset_lambda_artifact_path       = "testdata/demo-data-reset.zip"
  bff_ssr_lambda_runtime                     = "nodejs20.x"
  google_client_id                           = "000000000000-dummy.apps.googleusercontent.com"
  google_client_secret                       = "dummy-google-client-secret"
  cognito_callback_urls                      = ["https://example.test/api/auth/callback"]
  cognito_domain_prefix                      = "effort-tracker-dummy"
  role_cookie_signing_key                    = "dummy-role-cookie-signing-key-0123456789"
  public_origin                              = "https://public.example.test"
}

# --- 2026-11-10 追記（#143・ADR 0024）: IAM の権限の限定と自己保護の Deny ----
# 10-4-o (1)〜(7)。期待値の持ち主は AC-7-8 ①〜⑦（本ファイルは写しを持つが、
# 変更は AC-7-8 が先）。<account> は data.aws_caller_identity.current.account_id と
# 突き合わせ、リテラルのアカウント ID を書かない。
#
# 実装がポリシー本文に ARN を混ぜると、plan 下で本文が unknown になり
# jsondecode できない。そのため本ファイルのすべての run（既存を含む）へ、
# デプロイロールと凍結ポリシーの arn を override_resource で与える。
# ARN は実在の命名（...-github-actions-deploy / ...-budget-freeze-new-resources）と
# わざと食い違う目印付きのダミー値にする。(5)(iv) は、構成側の属性
# （aws_iam_role.github_actions_deploy.arn 等）と突き合わせて比べるため、
# 実在の命名と一致する override にすると別のロールを指す値との区別を失う。
override_resource {
  target = aws_iam_role.github_actions_deploy
  values = {
    arn = "arn:aws:iam::123456789012:role/effort-tracker-github-actions-deploy-override-marker"
  }
  override_during = plan
}

override_resource {
  target = aws_iam_policy.budget_freeze_new_resources
  values = {
    arn = "arn:aws:iam::123456789012:policy/effort-tracker-budget-freeze-new-resources-override-marker"
  }
  override_during = plan
}

# --- AC-10-4-o: デプロイロールのポリシー（Action = "*" を持たない） -------
run "deploy_role_policy_has_no_wildcard_action" {
  command = plan

  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement :
      stmt.Action != "*" && !contains(try(tolist(stmt.Action), [stmt.Action]), "*")
    ])
    error_message = "デプロイロールのポリシーは Action = \"*\" の文を持ってはならない（AC-7-8）"
  }

  # D-15 が限定列挙するサービス（S3 / CloudFront / Lambda / IAM / SSM /
  # Budgets / Logs / Scheduler）のサービス接頭辞に、Action が収まっていることを見る。
  # 列挙が十分かどうかは検査しない（12-10）。弾くのは裸の "*" だけであり、
  # s3:* のようなサービス内ワイルドカードは対象外（12-29・10-4-f/g/h とは
  # 別枠）。
  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        anytrue([
          for prefix in ["s3:", "cloudfront:", "lambda:", "iam:", "ssm:", "budgets:", "logs:", "scheduler:"] :
          startswith(action, prefix)
        ])
      ])
    ])
    error_message = "デプロイロールのポリシーの Action は D-15 の列挙（S3/CloudFront/Lambda/IAM/SSM/Budgets/Logs/Scheduler）のサービス接頭辞に収まらなければならない（AC-7-8）"
  }

  # 2026-10-27 改訂（#56・ADR 0022）: scheduler: で始まる Action は、スケジュールを
  # 作る（管理する）4つのいずれかに限る（AC-7-8・D-15）。scheduler:*、
  # スケジュールグループの操作（*ScheduleGroup）、タグの操作を拒む。
  # 列挙が十分かどうか（4つが実際に含まれるか）は検査しない（12-10）ので、
  # これは「広すぎないこと」だけを見る。
  assert {
    condition = alltrue([
      for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement :
      alltrue([
        for action in try(tolist(stmt.Action), [stmt.Action]) :
        !startswith(action, "scheduler:") || contains([
          "scheduler:CreateSchedule",
          "scheduler:GetSchedule",
          "scheduler:UpdateSchedule",
          "scheduler:DeleteSchedule",
        ], action)
      ])
    ])
    error_message = "デプロイロールのポリシーの scheduler: の Action は scheduler:CreateSchedule・GetSchedule・UpdateSchedule・DeleteSchedule の4つのいずれかに限らなければならない（scheduler:*・ScheduleGroup・タグの操作を含めない＝AC-7-8・10-4-o）"
  }

  # 空集合で緑にならないこと: 文が1つ以上あり、上の for 式が空を回っていない。
  assert {
    condition     = length(jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement) > 0
    error_message = "デプロイロールのポリシーは文を1つ以上持たなければならない（10-4-o の for 式が空集合で通らないようにする）"
  }
}

run "deploy_role_policy_iam_has_no_wildcard_action" {
  command = plan

  assert {
    condition     = alltrue([for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : alltrue([for a in try(tolist(stmt.Action), [stmt.Action]) : !startswith(a, "iam:") || !strcontains(a, "*")])])
    error_message = "いずれの文（Allow・Deny とも）も、iam: で始まり * を含む Action（iam:*・iam:Create* 等）を持ってはならない（10-4-o (1)・AC-7-8 ①）"
  }
}

run "deploy_role_policy_iam_allow_actions_are_enumerated_24" {
  command = plan

  assert {
    condition = alltrue([for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt.Effect != "Allow" || alltrue([for a in try(tolist(stmt.Action), [stmt.Action]) : !startswith(a, "iam:") || contains([
      "iam:CreateRole", "iam:GetRole", "iam:DeleteRole", "iam:UpdateAssumeRolePolicy",
      "iam:PutRolePolicy", "iam:GetRolePolicy", "iam:DeleteRolePolicy",
      "iam:CreatePolicy", "iam:GetPolicy", "iam:GetPolicyVersion", "iam:CreatePolicyVersion", "iam:DeletePolicy",
      "iam:CreateOpenIDConnectProvider", "iam:GetOpenIDConnectProvider", "iam:DeleteOpenIDConnectProvider",
      "iam:UpdateOpenIDConnectProviderThumbprint", "iam:AddClientIDToOpenIDConnectProvider", "iam:RemoveClientIDFromOpenIDConnectProvider",
      "iam:PassRole",
      "iam:ListRolePolicies", "iam:ListAttachedRolePolicies", "iam:ListInstanceProfilesForRole",
      "iam:ListPolicyVersions", "iam:DeletePolicyVersion",
    ], a)])])
    error_message = "Allow の文の iam: で始まる Action は、AC-7-8 ①の24個のいずれかに一致しなければならない（10-4-o (2)）"
  }
}

run "deploy_role_policy_iam_allow_resources_are_limited" {
  command = plan

  assert {
    condition     = alltrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && anytrue([for a in try(tolist(stmt.Action), [stmt.Action]) : startswith(a, "iam:")])] : alltrue([for r in try(tolist(stmt.Resource), [stmt.Resource]) : r != "*" && contains([format("arn:aws:iam::%s:role/effort-tracker-*", data.aws_caller_identity.current.account_id), format("arn:aws:iam::%s:policy/effort-tracker-*", data.aws_caller_identity.current.account_id), format("arn:aws:iam::%s:oidc-provider/token.actions.githubusercontent.com", data.aws_caller_identity.current.account_id)], r)])])
    error_message = "iam: の Action を持つ Allow の文の Resource は \"*\" でなく、AC-7-8 ②の3つの形（account は data.aws_caller_identity 由来）のいずれかだけからなる必要がある（10-4-o (3)）"
  }
}

run "deploy_role_policy_iam_passrole_is_limited_to_services" {
  command = plan

  assert {
    condition     = alltrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && contains(try(tolist(stmt.Action), [stmt.Action]), "iam:PassRole")] : tolist(try(tolist(stmt.Action), [stmt.Action])) == tolist(["iam:PassRole"])])
    error_message = "iam:PassRole を持つ Allow の文は、Action を iam:PassRole の1つだけとしなければならない（10-4-o (4)・AC-7-8 ③）"
  }

  assert {
    condition     = alltrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && contains(try(tolist(stmt.Action), [stmt.Action]), "iam:PassRole")] : tolist(try(tolist(stmt.Resource), [stmt.Resource])) == tolist([format("arn:aws:iam::%s:role/effort-tracker-*", data.aws_caller_identity.current.account_id)])])
    error_message = "iam:PassRole を持つ Allow の文の Resource は role/effort-tracker-* の形だけでなければならない（10-4-o (4)）"
  }

  assert {
    condition     = alltrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && contains(try(tolist(stmt.Action), [stmt.Action]), "iam:PassRole")] : contains(keys(stmt), "Condition") && contains(keys(stmt.Condition), "StringEquals") && contains(keys(stmt.Condition.StringEquals), "iam:PassedToService")])
    error_message = "iam:PassRole を持つ Allow の文は Condition.StringEquals[\"iam:PassedToService\"] を持たなければならない（10-4-o (4)）"
  }

  assert {
    condition     = alltrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && contains(try(tolist(stmt.Action), [stmt.Action]), "iam:PassRole")] : try(alltrue([for v in try(tolist(stmt.Condition.StringEquals["iam:PassedToService"]), [stmt.Condition.StringEquals["iam:PassedToService"]]) : contains(["lambda.amazonaws.com", "scheduler.amazonaws.com", "budgets.amazonaws.com"], v) && !strcontains(v, "*")]), false)])
    error_message = "iam:PassedToService の各値は lambda.amazonaws.com・scheduler.amazonaws.com・budgets.amazonaws.com のいずれかで、* を含んではならない（10-4-o (4)）"
  }

  assert {
    condition     = alltrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && contains(try(tolist(stmt.Action), [stmt.Action]), "iam:PassRole")] : !contains(keys(stmt), "Condition") || !contains(keys(stmt.Condition), "StringLike")])
    error_message = "iam:PassRole を持つ Allow の文は Condition.StringLike を持ってはならない（10-4-o (4)）"
  }
}

run "deploy_role_policy_has_self_protection_deny" {
  command = plan

  # AC-7-8 ⑤の Deny の10個の列挙（写し。持ち主は AC-7-8）。下の3つの assert が
  # var.self_protection_deny_actions として共有する。モジュールは同名の変数を
  # 宣言していない（テスト内だけの値）。
  variables {
    self_protection_deny_actions = ["iam:PutRolePolicy", "iam:DeleteRolePolicy", "iam:AttachRolePolicy", "iam:DetachRolePolicy", "iam:UpdateAssumeRolePolicy", "iam:DeleteRole", "iam:CreatePolicyVersion", "iam:DeletePolicyVersion", "iam:SetDefaultPolicyVersion", "iam:DeletePolicy"]
  }

  assert {
    condition     = length([for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Deny"]) >= 1
    error_message = "Effect が Deny の文が1つ以上なければならない（10-4-o (5)）"
  }

  assert {
    condition     = anytrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Deny"] : alltrue([for n in var.self_protection_deny_actions : contains(try(tolist(stmt.Action), [stmt.Action]), n)])])
    error_message = "Deny の文のうち少なくとも1つが、AC-7-8 ⑤の10個の Action をすべて含まなければならない（10-4-o (5)(i)）"
  }

  assert {
    condition     = anytrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Deny"] : alltrue([for n in var.self_protection_deny_actions : contains(try(tolist(stmt.Action), [stmt.Action]), n)]) && !anytrue([for a in try(tolist(stmt.Action), [stmt.Action]) : startswith(a, "iam:Get") || startswith(a, "iam:List")]) && !contains(keys(stmt), "Condition") && !anytrue([for r in try(tolist(stmt.Resource), [stmt.Resource]) : strcontains(r, "*")]) && toset(try(tolist(stmt.Resource), [stmt.Resource])) == toset([aws_iam_role.github_actions_deploy.arn, aws_iam_policy.budget_freeze_new_resources.arn]) && length(try(tolist(stmt.Resource), [stmt.Resource])) == 2])
    error_message = "Deny の文のうち少なくとも1つが、(i) 10個を含み (ii) iam:Get・iam:List を含まず (iii) Condition を持たず (iv) Resource が * を含まずデプロイロールと凍結ポリシーの ARN（構成側の属性）の2つだけでなければならない（10-4-o (5)）"
  }

  assert {
    condition     = anytrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Deny"] : alltrue([for n in var.self_protection_deny_actions : contains(try(tolist(stmt.Action), [stmt.Action]), n)]) && !anytrue([for a in try(tolist(stmt.Action), [stmt.Action]) : strcontains(a, "*")])])
    error_message = "自己保護の Deny 文の Action は * を含んではならない（10-4-o (5)・AC-7-8 ⑤）"
  }
}

run "deploy_role_policy_deny_does_not_catch_others" {
  command = plan

  assert {
    condition     = alltrue([for stmt in [for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Deny"] : alltrue([for r in try(tolist(stmt.Resource), [stmt.Resource]) : !strcontains(r, "effort-tracker-*")])])
    error_message = "Deny の文の Resource に role/effort-tracker-*・policy/effort-tracker-* の形（接頭辞のワイルドカード）を含んではならない（10-4-o (6)）"
  }
}

run "deploy_role_policy_iam_statements_are_not_vacuous" {
  command = plan

  assert {
    condition     = length([for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && anytrue([for a in try(tolist(stmt.Action), [stmt.Action]) : startswith(a, "iam:")])]) >= 1
    error_message = "iam: で始まる Action を持つ Allow の文が1つ以上なければならない（10-4-o (7)。(2)(3) の for 式が空を回って緑にならないため）"
  }

  assert {
    condition     = length([for stmt in jsondecode(aws_iam_role_policy.github_actions_deploy.policy).Statement : stmt if stmt.Effect == "Allow" && contains(try(tolist(stmt.Action), [stmt.Action]), "iam:PassRole")]) >= 1
    error_message = "iam:PassRole を持つ Allow の文が1つ以上なければならない（10-4-o (7)。(4) の for 式が空を回って緑にならないため）"
  }
}
