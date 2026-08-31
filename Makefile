# effort-tracker 検証ハーネス
#
# 設計と運用ルールは docs/harness/verification-loop.md を参照。
#
# 【原則】CI（.github/workflows/ci.yml）はここと同一のターゲットを呼ぶこと。
#         CI 側にだけ存在する検査を作らない。作った瞬間、ローカルの Green が
#         信用できなくなる。

SHELL := /bin/bash
.DEFAULT_GOAL := help

API_DIR := services/api
WEB_DIR := apps/web
TF_DIR  := infra/terraform
TF_BOOTSTRAP_DIR := infra/bootstrap
MODULE  := github.com/h-k741953/effort-tracker/services/api

.PHONY: help
help: ## このヘルプを表示
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# =============================================================================
# 集約ターゲット
# =============================================================================

.PHONY: verify
verify: lint test check-domain-deps check-skills check-go-module-pins scan-secrets ## 全検査（コミット前・PR前に実行）
	@echo ""
	@echo "==> verify: 全検査を通過"

.PHONY: test
test: test-api test-web test-tf test-scripts test-hooks test-commands test-skills test-go-module-pins ## 全レイヤーのテスト

.PHONY: lint
lint: lint-api lint-web lint-tf ## 全レイヤーの Lint / 型チェック

.PHONY: fmt
fmt: ## 全レイヤーの自動整形
	@cd $(API_DIR) && go fmt ./...
	@if [ -d $(TF_DIR) ]; then terraform -chdir=$(TF_DIR) fmt -recursive; fi

# =============================================================================
# Go / services/api
# =============================================================================

.PHONY: test-api
test-api: ## Go のテスト
	@echo "==> test-api"
	@cd $(API_DIR) && go test ./...

.PHONY: lint-api
lint-api: ## Go の Lint（golangci-lint + go vet）
	@echo "==> lint-api"
	@cd $(API_DIR) && go vet ./...
	@cd $(API_DIR) && golangci-lint run ./...

# ドメイン層の依存検査。
#
# CLAUDE.md は「ドメイン層は Go の標準ライブラリのみに依存する」と定めている。
# この規約は人間のレビューに任せると必ず腐る（import が1行増える変更は最も
# 見落とされやすい）ため、機械的に検査する。
#
# 判定には go list の .Standard フラグを使う。パス中のドットの有無で
# 標準ライブラリを判別する方式は誤り。net/http などが内部で
# vendor/golang.org/x/... を引き込むため誤検知する。
#
# -test を付けること。付けないとテストファイルの import が出力されず、
# ドメインのテストが任意の外部ライブラリを import しても素通りする（ADR 0007）。
# テストバリアント（workmonth.test 等）の ImportPath は $(MODULE)/internal/domain/
# で始まるため、既存のフィルタがそのまま効く。追加の除外は要らない。
#
# go list を2回呼ぶこと。ADR 0007 の主張は2段になっており、1回では表せない。
#   本体   (-test なし): 標準ライブラリのみ。例外なし
#   テスト (-test あり): 標準ライブラリ + go-cmp のみ
# -test 込みの出力に許可リストを1回かけるだけでは、本体の import とテストの
# import を区別できず、本体が go-cmp を import しても通ってしまう。実測で踏んだ。
#
# go-cmp をテストにのみ許すのは、テスト専用ライブラリだからである（ADR 0007）。
# パッケージドキュメント自身が "It is intended to only be used in tests" と
# 明示している。2つ目を許す判断には ADR の置換が要る。黙って増やさないこと。
#
# go list の終了コードを必ず見ること。パイプに直接繋ぐと、終了コードは grep の
# ものになり `|| true` が握り潰す。-deps 単体ならエラー時も既知の import パスを
# 出力するので実害は出ないが、-test はテストパッケージを読めないと
# 「can't load test package」と言って何も出力せずに落ちる。パイプが空になり、
# 検査は OK を返す。実測で踏んだ（＝偽の Green）。
# 検査対象を読めなかったことは、違反が無いことを意味しない。
.PHONY: check-domain-deps
check-domain-deps: ## ドメイン層が標準ライブラリのみに依存しているか検査（テスト含む）
	@echo "==> check-domain-deps"
	@cd $(API_DIR) && \
	pkgs="$$(go list ./internal/domain/... 2>/dev/null || true)"; \
	if [ -z "$$pkgs" ]; then \
	  echo "  NG: ドメイン層のパッケージが見つからない。検査対象が無い状態を"; \
	  echo "      成功として扱わないため、これを失敗とする。"; \
	  exit 1; \
	fi; \
	if ! raw="$$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/domain/... 2>&1)"; then \
	  echo "  NG: go list がドメイン層の依存を解決できなかった:"; \
	  echo "$$raw" | sed 's/^/    /'; \
	  echo "  → 読めなかった状態を成功として扱わないため、これを失敗とする。"; \
	  exit 1; \
	fi; \
	violations="$$(echo "$$raw" \
	  | grep -v '^$$' \
	  | grep -v '^$(MODULE)/internal/domain/' || true)"; \
	if [ -n "$$violations" ]; then \
	  echo "  NG: ドメイン層の【本体】が標準ライブラリ以外に依存している:"; \
	  echo "$$violations" | sed 's/^/    - /'; \
	  echo "  → 本体に例外は無い。go-cmp もテスト専用であり本体では使えない（ADR 0007）。"; \
	  echo "  → docs/adr/ と CLAUDE.md を確認すること。緩める判断には人間の承認が要る。"; \
	  exit 1; \
	fi; \
	if ! rawtest="$$(go list -deps -test -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/domain/... 2>&1)"; then \
	  echo "  NG: go list がドメイン層のテストの依存を解決できなかった:"; \
	  echo "$$rawtest" | sed 's/^/    /'; \
	  echo "  → 読めなかった状態を成功として扱わないため、これを失敗とする。"; \
	  exit 1; \
	fi; \
	tviolations="$$(echo "$$rawtest" \
	  | grep -v '^$$' \
	  | grep -v '^$(MODULE)/internal/domain/' \
	  | grep -v '^github.com/google/go-cmp/' || true)"; \
	if [ -n "$$tviolations" ]; then \
	  echo "  NG: ドメイン層の【テスト】が標準ライブラリ・go-cmp 以外に依存している:"; \
	  echo "$$tviolations" | sed 's/^/    - /'; \
	  echo "  → テストに許されるのは go-cmp だけ（ADR 0007）。testify 等は入れない。"; \
	  echo "  → docs/adr/ と CLAUDE.md を確認すること。緩める判断には人間の承認が要る。"; \
	  exit 1; \
	fi; \
	echo "  OK: $$(echo "$$pkgs" | wc -l) パッケージ（本体=標準ライブラリのみ / テスト=+go-cmp）"

# =============================================================================
# Next.js / apps/web
#
# apps/web はスキャフォールド済み（Issue #9）。SKIP 分岐は持たない。
# 「対象（apps/web/package.json）が無いので黙って成功」は verification-loop.md が
# 禁じる偽の Green であり、かつ検査対象が無い状態は「未着手」ではなく
# 「壊れている」ことを意味する（docs/specs/web-app-scaffold.md AC-4-2、
# 2026-08-14 人間承認）。check-domain-deps と同じ扱いで、対象なしを失敗とする。
# =============================================================================

.PHONY: test-web
test-web: ## Next.js のテスト
	@echo "==> test-web"
	@if [ ! -f $(WEB_DIR)/package.json ]; then \
	  echo "  NG: $(WEB_DIR)/package.json が無い。apps/web はスキャフォールド済みである前提のため、"; \
	  echo "      対象が無い状態を成功として扱わない（docs/specs/web-app-scaffold.md AC-4-2）。"; \
	  exit 1; \
	else \
	  cd $(WEB_DIR) && npm test; \
	fi

.PHONY: lint-web
lint-web: ## Next.js の Lint と型チェック
	@echo "==> lint-web"
	@if [ ! -f $(WEB_DIR)/package.json ]; then \
	  echo "  NG: $(WEB_DIR)/package.json が無い。apps/web はスキャフォールド済みである前提のため、"; \
	  echo "      対象が無い状態を成功として扱わない（docs/specs/web-app-scaffold.md AC-4-2）。"; \
	  exit 1; \
	else \
	  cd $(WEB_DIR) && npm run lint && npx tsc --noEmit; \
	fi

# =============================================================================
# Terraform / infra
# =============================================================================

.PHONY: lint-tf
lint-tf: ## Terraform の整形チェックと検証（infra/terraform・infra/bootstrap の両方）
	@echo "==> lint-tf"
	@for dir in $(TF_DIR) $(TF_BOOTSTRAP_DIR); do \
	  if [ ! -d "$$dir" ] || [ -z "$$(find $$dir -name '*.tf' -print -quit 2>/dev/null)" ]; then \
	    echo "  SKIP: $$dir に *.tf が無い（IaC 未着手）"; \
	  else \
	    echo "  -- $$dir --"; \
	    terraform -chdir=$$dir fmt -check -recursive -diff && \
	    terraform -chdir=$$dir init -backend=false -input=false > /dev/null && \
	    terraform -chdir=$$dir validate || exit 1; \
	  fi; \
	done

.PHONY: test-tf
test-tf: ## Terraform の terraform test（infra/terraform・infra/bootstrap の両方。AWS API を呼ばない）
	@echo "==> test-tf"
	@for dir in $(TF_DIR) $(TF_BOOTSTRAP_DIR); do \
	  if [ ! -d "$$dir" ] || [ -z "$$(find $$dir -name '*.tf' -print -quit 2>/dev/null)" ]; then \
	    echo "  SKIP: $$dir に *.tf が無い（IaC 未着手）"; \
	  else \
	    echo "  -- $$dir --"; \
	    terraform -chdir=$$dir init -backend=false -input=false > /dev/null && \
	    terraform -chdir=$$dir test -no-color || exit 1; \
	  fi; \
	done

# =============================================================================
# Lambda 成果物のビルド + zip 化（docs/specs/infra-terraform.md AC-9-2〜9-5・
# AC-9-8〜9-9）
#
# provided.al2023 は zip 直下の実行ファイル "bootstrap" を起動する
# （AC-9-2・AC-9-8）。したがって zip の中身はディレクトリ接頭辞の無い単一の
# エントリ "bootstrap" でなければならない。エントリ名を素の "bootstrap" に
# するため、生成物のディレクトリへ cd してから zip 化する（cd せずに
# パスを渡すと、zip エントリ名にディレクトリ接頭辞が付いてしまう）。
#
# zip CLI は devcontainer に無い（unzip / zipinfo / python3 -m zipfile は
# 入っている）。devcontainer のリビルド無しで完結させるため、標準ライブラリ
# だけで足りる `python3 -m zipfile -c` で zip を作る（ファイルの実行権限
# ビットを保持することを実測済み）。
#
# 生成物は $(LAMBDA_ARTIFACT_DIR) 配下に置き、.gitignore で無視する
# （AC-9-4・AC-9-9。コミットしない）。ビルドと zip 作成は Terraform の中で
# 行わない（AC-4-5）。Terraform 側は成果物のパスを変数で受け取るだけである
# （D-13）。
# =============================================================================

LAMBDA_ARTIFACT_DIR      := lambda-artifacts
DOMAIN_API_BUILD_DIR      := $(LAMBDA_ARTIFACT_DIR)/domain-api
DOMAIN_API_ZIP            := $(DOMAIN_API_BUILD_DIR)/bootstrap.zip
KILLSWITCH_BUILD_DIR      := $(LAMBDA_ARTIFACT_DIR)/cloudfront-killswitch
KILLSWITCH_ZIP            := $(KILLSWITCH_BUILD_DIR)/bootstrap.zip

.PHONY: build-lambda-domain-api
build-lambda-domain-api: ## ドメイン API Lambda を Linux 向けにクロスコンパイルし zip 化（AC-9-2・AC-9-3）
	@echo "==> build-lambda-domain-api"
	@mkdir -p $(DOMAIN_API_BUILD_DIR)
	@# go build -o は出力先に既存ファイルがあるとその権限モードを引き継ぐため、
	@# 事前に削除して「実行ビット付きの新規ファイル」を毎回保証する。
	@rm -f $(DOMAIN_API_BUILD_DIR)/bootstrap
	@cd $(API_DIR) && GOOS=linux GOARCH=amd64 go build -o ../../$(DOMAIN_API_BUILD_DIR)/bootstrap ./cmd/bootstrap
	@rm -f $(DOMAIN_API_ZIP)
	@cd $(DOMAIN_API_BUILD_DIR) && python3 -m zipfile -c bootstrap.zip bootstrap
	@echo "  -> $(DOMAIN_API_ZIP)"

.PHONY: build-lambda-cloudfront-killswitch
build-lambda-cloudfront-killswitch: ## CloudFront 遮断 Lambda を Linux 向けにクロスコンパイルし zip 化（AC-9-8・AC-9-9）
	@echo "==> build-lambda-cloudfront-killswitch"
	@mkdir -p $(KILLSWITCH_BUILD_DIR)
	@# go build -o は出力先に既存ファイルがあるとその権限モードを引き継ぐため、
	@# 事前に削除して「実行ビット付きの新規ファイル」を毎回保証する。
	@rm -f $(KILLSWITCH_BUILD_DIR)/bootstrap
	@cd $(API_DIR) && GOOS=linux GOARCH=amd64 go build -o ../../$(KILLSWITCH_BUILD_DIR)/bootstrap ./cmd/cloudfront-killswitch
	@rm -f $(KILLSWITCH_ZIP)
	@cd $(KILLSWITCH_BUILD_DIR) && python3 -m zipfile -c bootstrap.zip bootstrap
	@echo "  -> $(KILLSWITCH_ZIP)"

# =============================================================================
# .github/scripts のロジック
#
# プロセス検査（spec-link / review-trail）の *対象* は PR のメタデータで、
# ローカルには存在しない。だが *チェッカのロジック* は入力を差し替えれば
# 完全にローカル実行でき、ここは「ローカル再現が必須」の側である。
# この区別を付けないと、チェッカのバグだけが検査不能地帯に残る。
#
# 実際に踏んだ: 往復見出しを `###` ちょうどに限定していたため `## 往復 4` が
# 黙殺され、上限超過（＝人間へ上げるべき停止条件）が OK で通っていた。
# さらに `#{1,6}` の ERE 区間指定は mawk（GitHub runner の既定 awk）が解釈せず、
# 修正が黙って無効化される。どちらも fixture でしか捕まらない。
# =============================================================================

#
# 【例外: test-makefile-web.sh / test-apps-web-contract.sh をここへ合流させる】
#   この2本の対象は本来「.github/scripts 内の CI プロセスチェッカ」ではなく
#   Makefile 自身の test-web/lint-web の振る舞いと apps/web の設定契約であり、
#   上記の区分（対象種別ごとにターゲットを分ける）からは外れる
#   （docs/specs/web-app-scaffold.md AC-4-6 が「web 向けの新しい make
#   ターゲットを増やさない／verify・test・lint の依存関係を変えない」と定めて
#   おり、専用ターゲットを新設できない）。test-web / lint-web 自身の recipe に
#   埋め込むことも選べない: test-makefile-web.sh は内部で
#   `make test-web WEB_DIR=...` / `make lint-web WEB_DIR=...` を複数回呼ぶため、
#   埋め込むと自己再帰で無限ループする（実装工程で検証済み）。消去法で、
#   既存ターゲットの中で対象種別の区分から外れる侵食が最小のここへ合流させる。
#
.PHONY: test-scripts
test-scripts: ## .github/scripts のチェッカを fixture で検査（apps/web の Makefile 契約・設定契約を含む）
	@echo "==> test-scripts"
	@bash .github/scripts/test-check-review-trail.sh
	@bash .github/scripts/test-makefile-web.sh
	@bash .github/scripts/test-apps-web-contract.sh "$(CURDIR)/$(WEB_DIR)"
	@bash .github/scripts/test-gitleaks-allowlist.sh "$(CURDIR)/.gitleaks.toml"
	@bash .github/scripts/test-ci-terraform-test-target.sh
	@bash .github/scripts/test-makefile-lambda-package.sh

# =============================================================================
# .claude/hooks のロジック
#
# .github/scripts と同じ理由（上記コメント参照）でローカル再現が必須の側だが、
# 対象は CI のスクリプトではなく Claude Code のハーネス資産（UserPromptSubmit
# hook）であり、置き場所が .github/scripts と異なる（docs/adr/0011,
# docs/specs/orchestrator-entry-hook.md）。ディレクトリが違うものを
# test-scripts ターゲットへ合流させると、このコメント区分（「.github/scripts
# のロジック」）と実体がずれるため、ターゲットを分けた。
# =============================================================================

.PHONY: test-hooks
test-hooks: ## .claude/hooks のチェッカを fixture で検査
	@echo "==> test-hooks"
	@bash .claude/hooks/test-check-prompt-entry.sh

# =============================================================================
# .claude/scripts のロジック
#
# .github/scripts / .claude/hooks と同じ理由（上記コメント参照）でローカル
# 再現が必須の側だが、対象はスラッシュコマンドの機械判定部（`/issue` の
# issue-gate.sh）であり、置き場所が .github/scripts とも .claude/hooks とも
# 異なる（docs/specs/issue-command.md「対象」節「なぜ make ターゲットを
# 分け〜」）。.github/scripts（CI スクリプト）・.claude/hooks（hook）・
# .claude/scripts（スラッシュコマンドの機械判定部）は対象種別が異なるため、
# Makefile のコメント区分と実体をずらさないよう test-scripts / test-hooks へ
# 合流させず test-commands を新設する。
# =============================================================================

.PHONY: test-commands
test-commands: ## .claude/scripts のチェッカ（スラッシュコマンドの機械判定部）を fixture で検査
	@echo "==> test-commands"
	@bash .claude/scripts/test-issue-gate.sh

# =============================================================================
# .claude/skills のチェッカ（Skill 資産の形式検査）
#
# check-skills は .claude/skills/*/SKILL.md が (1) frontmatter に name/description を
# 持ち (2) 本文に docs/ 参照を1つ以上持つ、を機械検査する（docs/specs/skills.md
# AC-5）。プロンプト資産で機械検査できるのは「参照が張られているか」までである
# （docs/harness/commands-and-skills.md 軸3）。
#
# check-domain-deps のように Makefile へ直書きせずスクリプトへ切り出したのは、
# AC-5 が fixture でのローカル再現を要求するため（Issue #30 の穴を踏まない）。
# .claude/skills は他の .claude/ 資産と対象種別が異なるため、Makefile の
# コメント区分と実体をずらさないよう test-scripts / test-hooks / test-commands へ
# 合流させず、専用の check-skills / test-skills を新設する。
# =============================================================================

.PHONY: check-skills
check-skills: ## .claude/skills/*/SKILL.md の形式（frontmatter + docs/ 参照）を検査
	@echo "==> check-skills"
	@bash .claude/scripts/check-skills.sh .claude/skills

.PHONY: test-skills
test-skills: ## check-skills.sh のロジックを fixture で検査
	@echo "==> test-skills"
	@bash .claude/scripts/test-check-skills.sh

# =============================================================================
# Go モジュール pin の整合検査（.devcontainer/Dockerfile ⇔ services/api/go.mod）
#
# check-go-module-pins は、Dockerfile の ARG pin（GOMODCACHE を温めるための
# 事前取得に使う版）と go.mod の direct require が一致しているかを検査する
# （docs/specs/devcontainer-go-module-policy.md AC-5〜AC-7）。ARG は go.mod の
# 追随側の二重管理であり、放置すれば必ずずれる（同仕様 P-3）。
#
# test-scripts へ合流させないのは、対象種別が違うため。test-scripts が対象と
# するのは CI から呼ばれる *プロセス検査のチェッカ*（往復証跡の形式等）だが、
# こちらは **リポジトリ内の設定ファイル間の整合検査**であり、プロセスも
# 外部入力も見ない。check-skills / test-skills と同じ理由で、Makefile の
# コメント区分と実体をずらさないよう専用ターゲットへ分ける。
# =============================================================================

.PHONY: check-go-module-pins
check-go-module-pins: ## Dockerfile の ARG pin と go.mod の direct require の整合を検査
	@echo "==> check-go-module-pins"
	@bash .github/scripts/check-go-module-pins.sh .devcontainer/Dockerfile services/api/go.mod

.PHONY: test-go-module-pins
test-go-module-pins: ## check-go-module-pins.sh のロジックを fixture で検査
	@echo "==> test-go-module-pins"
	@bash .github/scripts/test-check-go-module-pins.sh

# =============================================================================
# セキュリティ
# =============================================================================

.PHONY: scan-secrets
scan-secrets: ## gitleaks によるシークレット検出（作業ツリー + 履歴）
	@echo "==> scan-secrets"
	@gitleaks dir . --no-banner --redact
	@gitleaks git . --no-banner --redact
