#!/usr/bin/env bash
# Makefile の test-web / lint-web ターゲットの fixture テスト（Issue #9）。
#
# 仕様の単一情報源: docs/specs/web-app-scaffold.md AC-4。
#
# 【なぜこれが要るか】
#   AC-4-1〜AC-4-4 は「apps/web/package.json が無ければ SKIP して成功する」
#   分岐を削除し、代わりに失敗させる、という *Makefile の振る舞いの変更* を
#   要求する（2026-08-14 人間承認）。ロジックがシェル関数として切り出されて
#   いないため（Makefile 直書き）、check-review-trail.sh 等と同じ型の
#   「対象スクリプトへ環境変数を渡す」fixture は使えない。代わりに
#   `make <target> WEB_DIR=<fixture>` で対象ディレクトリを差し替える
#   （WEB_DIR は Makefile 冒頭で `WEB_DIR := apps/web` と定義された make 変数で、
#   コマンドラインからの上書きに対応している。実測済み）。
#
# 【対象種別が違うため test-scripts へ合流させない】
#   test-scripts（.github/scripts 内の CI チェッカ）とは対象が異なる
#   （検査対象はリポジトリ直下の Makefile 自体）。ただし
#   docs/specs/web-app-scaffold.md AC-4-6 が「web 向けの新しい make
#   ターゲットを増やさない」と定めているため、本ファイルを恒常的に
#   呼び出す新規 make ターゲットは作らない。make verify 経路への組み込み
#   （どの既存ターゲットへ合流させるかの選定と Makefile 編集）は
#   AC-4 本体（SKIP 除去・失敗化）の実装と不可分なため implementer 工程が行う。
#   本ファイルは単体で `bash .github/scripts/test-makefile-web.sh` として
#   実行し、Red / Green を確認できる（このコメント自体が申し送り）。
#
# 【npm install を要求しない】
#   正常系 fixture は `node_modules/.bin/tsc` に実行可能な shim を手書きし、
#   npm レジストリへ一切アクセスしない（P-3: devcontainer 内の npm 取得可否は
#   本テストの関心事ではない）。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

# run_case <名前> <target> <web_dir> <want_rc_kind> [期待する出力の部分文字列...]
#   want_rc_kind: "zero"（rc=0 を期待） | "nonzero"（rc!=0 を期待）
# 加えて、常に「出力に SKIP という語が含まれないこと」（AC-4-4）を検査する。
run_case() {
  local name="$1" target="$2" web_dir="$3" want="$4"; shift 4
  local out rc ok=1

  out="$(cd "$REPO_ROOT" && make "$target" "WEB_DIR=${web_dir}" 2>&1)"
  rc=$?

  if [ "$want" = "zero" ]; then
    [ "$rc" = 0 ] || ok=0
  else
    [ "$rc" != 0 ] || ok=0
  fi

  if grep -qF "SKIP" <<< "$out"; then
    ok=0
  fi

  local sub
  for sub in "$@"; do
    grep -qF -- "$sub" <<< "$out" || ok=0
  done

  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s (rc=%s)\n' "$name" "$rc"
  else
    fail=$((fail + 1))
    printf '  FAIL %s (rc=%s)\n' "$name" "$rc"
    sed 's/^/       | /' <<< "$out"
  fi
}

echo "==> test-makefile-web"

# --- fixture: package.json が無い ------------------------------------------
NO_PKG="${WORK}/no-package"
mkdir -p "$NO_PKG"

# AC-4-2: 対象が無ければ失敗（成功ではない）。
# AC-4-3: 黙って落ちない。何が無いか（package.json のパス）をログに出す。
# AC-4-4: 出力に SKIP の語が現れない（run_case が共通で検査）。
run_case "test-web: package.json が無ければ失敗" \
  test-web "$NO_PKG" nonzero "package.json"

run_case "lint-web: package.json が無ければ失敗" \
  lint-web "$NO_PKG" nonzero "package.json"

# --- fixture: package.json はあるが test/lint が失敗する --------------------
WITH_PKG_FAIL="${WORK}/with-package-fail"
mkdir -p "$WITH_PKG_FAIL"
cat > "${WITH_PKG_FAIL}/package.json" <<'EOF'
{
  "name": "fixture",
  "private": true,
  "scripts": {
    "test": "echo FIXTURE_TEST_CALLED && exit 1",
    "lint": "echo FIXTURE_LINT_CALLED && exit 1"
  }
}
EOF

# AC-4-1: SKIP 分岐が無く、実際に npm test / npm run lint が呼ばれること
# （FIXTURE_TEST_CALLED / FIXTURE_LINT_CALLED が出力に現れることで確認する）。
# lint はここでは意図的に失敗させ、`&&` の短絡で npx tsc --noEmit まで
# 到達させない（tsc の可否はネットワーク非依存の別ケースで検証する）。
run_case "test-web: package.json があれば npm test が実行される（失敗を伝播）" \
  test-web "$WITH_PKG_FAIL" nonzero "FIXTURE_TEST_CALLED"

run_case "lint-web: package.json があれば npm run lint が実行される（失敗を伝播）" \
  lint-web "$WITH_PKG_FAIL" nonzero "FIXTURE_LINT_CALLED"

# --- fixture: package.json があり test/lint/tsc すべて成功する --------------
WITH_PKG_OK="${WORK}/with-package-ok"
mkdir -p "${WITH_PKG_OK}/node_modules/.bin"
cat > "${WITH_PKG_OK}/package.json" <<'EOF'
{
  "name": "fixture",
  "private": true,
  "scripts": {
    "test": "echo FIXTURE_TEST_CALLED && exit 0",
    "lint": "echo FIXTURE_LINT_CALLED && exit 0"
  }
}
EOF
cat > "${WITH_PKG_OK}/node_modules/.bin/tsc" <<'EOF'
#!/usr/bin/env bash
echo FIXTURE_TSC_CALLED
exit 0
EOF
chmod +x "${WITH_PKG_OK}/node_modules/.bin/tsc"

run_case "test-web: 正常系はSKIPなしで成功する" \
  test-web "$WITH_PKG_OK" zero "FIXTURE_TEST_CALLED"

run_case "lint-web: 正常系はSKIPなしでlintとtscの両方が実行され成功する" \
  lint-web "$WITH_PKG_OK" zero "FIXTURE_LINT_CALLED" "FIXTURE_TSC_CALLED"

# --- 実リポジトリの apps/web ------------------------------------------------
# 本テスト作成時点（tester 工程）は apps/web が未スキャフォールドで
# package.json も無く、AC-4-2 により test-web / lint-web は失敗する状態
# だった（このコメント作成時点の Red はここで踏んでいた）。apps/web を
# スキャフォールドした now（implementer 工程）は、このコメントが元々予告した
# とおり「正常系」相当（rc=0・SKIPなし）に反転させる。package.json 自体は
# 存在するようになったため、その不在を示す部分文字列の検査は外す。
run_case "test-web: 実リポジトリの apps/web（AC-4-2 の本番適用）" \
  test-web "apps/web" zero

run_case "lint-web: 実リポジトリの apps/web（AC-4-2 の本番適用）" \
  lint-web "apps/web" zero

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
