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
MODULE  := github.com/h-k741953/effort-tracker/services/api

.PHONY: help
help: ## このヘルプを表示
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# =============================================================================
# 集約ターゲット
# =============================================================================

.PHONY: verify
verify: lint test check-domain-deps scan-secrets ## 全検査（コミット前・PR前に実行）
	@echo ""
	@echo "==> verify: 全検査を通過"

.PHONY: test
test: test-api test-web ## 全レイヤーのテスト

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
# go-cmp のみ例外として許す。テスト専用ライブラリであり、標準 testing が唯一
# 苦手とする構造体比較だけを埋めるため（ADR 0007）。2つ目を許す判断には
# ADR の置換が要る。ここを黙って増やさないこと。
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
	if ! raw="$$(go list -deps -test -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/domain/... 2>&1)"; then \
	  echo "  NG: go list がドメイン層の依存を解決できなかった:"; \
	  echo "$$raw" | sed 's/^/    /'; \
	  echo "  → 読めなかった状態を成功として扱わないため、これを失敗とする。"; \
	  exit 1; \
	fi; \
	violations="$$(echo "$$raw" \
	  | grep -v '^$$' \
	  | grep -v '^$(MODULE)/internal/domain/' \
	  | grep -v '^github.com/google/go-cmp/' || true)"; \
	if [ -n "$$violations" ]; then \
	  echo "  NG: ドメイン層が標準ライブラリ以外に依存している:"; \
	  echo "$$violations" | sed 's/^/    - /'; \
	  echo "  → 本体は標準ライブラリのみ、テストは標準ライブラリ + go-cmp のみ（ADR 0007）。"; \
	  echo "  → docs/adr/ と CLAUDE.md を確認すること。緩める判断には人間の承認が要る。"; \
	  exit 1; \
	fi; \
	echo "  OK: $$(echo "$$pkgs" | wc -l) パッケージ、標準ライブラリのみに依存（テスト含む）"

# =============================================================================
# Next.js / apps/web
#
# 未スキャフォールドの間は明示的に SKIP を表示する。
# 「検査対象が無いので黙って成功」は verification-loop.md が禁じる偽の Green。
# 見えるログを残すことで、通っているのか素通りなのかを区別できるようにする。
# =============================================================================

.PHONY: test-web
test-web: ## Next.js のテスト
	@echo "==> test-web"
	@if [ ! -f $(WEB_DIR)/package.json ]; then \
	  echo "  SKIP: $(WEB_DIR)/package.json が無い（Next.js 未スキャフォールド）"; \
	else \
	  cd $(WEB_DIR) && npm test; \
	fi

.PHONY: lint-web
lint-web: ## Next.js の Lint と型チェック
	@echo "==> lint-web"
	@if [ ! -f $(WEB_DIR)/package.json ]; then \
	  echo "  SKIP: $(WEB_DIR)/package.json が無い（Next.js 未スキャフォールド）"; \
	else \
	  cd $(WEB_DIR) && npm run lint && npx tsc --noEmit; \
	fi

# =============================================================================
# Terraform / infra
# =============================================================================

.PHONY: lint-tf
lint-tf: ## Terraform の整形チェックと検証
	@echo "==> lint-tf"
	@if [ ! -d $(TF_DIR) ] || [ -z "$$(find $(TF_DIR) -name '*.tf' -print -quit 2>/dev/null)" ]; then \
	  echo "  SKIP: $(TF_DIR) に *.tf が無い（IaC 未着手）"; \
	else \
	  terraform -chdir=$(TF_DIR) fmt -check -recursive -diff && \
	  terraform -chdir=$(TF_DIR) init -backend=false -input=false > /dev/null && \
	  terraform -chdir=$(TF_DIR) validate; \
	fi

# =============================================================================
# セキュリティ
# =============================================================================

.PHONY: scan-secrets
scan-secrets: ## gitleaks によるシークレット検出（作業ツリー + 履歴）
	@echo "==> scan-secrets"
	@gitleaks dir . --no-banner --redact
	@gitleaks git . --no-banner --redact
