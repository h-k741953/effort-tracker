#!/usr/bin/env bash
# apps/web の package.json / vitest 設定 / テストファイルの契約テスト（Issue #9）。
#
# 仕様の単一情報源: docs/specs/web-app-scaffold.md AC-2（scripts 契約）・
# AC-3（テスト基盤は Vitest。0件で緑にしない・意味のあるテストを最低1件置く）。
#
# 【対象ディレクトリを引数で受け取る（fixture 差し替え可能にする）】
#   .github/scripts/check-go-module-pins.sh と同じ理由。実リポジトリの
#   apps/web を検査対象にしつつ、擬似ディレクトリでも同じロジックを
#   再現できるようにする。$1 = 検査対象ディレクトリ（apps/web 相当。必須）。
#
# 【ここで検査するのは静的な契約のみ】
#   AC-3-3/AC-3-4（意味のあるテストが実際に Vitest で Red を踏めること）の
#   *動的な実行* は、apps/web 自体が未スキャフォールドの現時点では検証できない
#   （docs/specs/web-app-scaffold.md P-3 の懸念と create-next-app 相当の
#   スキャフォールドを本テスト工程が先取りしないため）。本スクリプトは
#   「意味のあるテストファイルが存在し、自明恒真式（expect(true).toBe(true) 等）
#   でないこと」までを静的パターンで検査するに留める。実際に Vitest で実行して
#   Red を踏めることの確認は、implementer が apps/web をスキャフォールドした
#   直後に改めて行うことを申し送る（引き継ぎ事項）。
#
# 【新規 make ターゲットは作らない（AC-4-6 と同じ理由をここにも適用）】
#   本ファイルは単体で `bash .github/scripts/test-apps-web-contract.sh apps/web`
#   として実行し、Red / Green を確認する。make verify への組み込みは
#   implementer 工程が既存の集約ターゲットへ合流させる形で行う。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

pass=0
fail=0

# collect_violations <target_dir>
#   target_dir 配下の契約違反を1行1件、グローバル配列 VIOLATIONS へ積む。
#   判定ロジックそのもの（何が違反か）と、テストとしての合否判定
#   （run_contract_case）を分離する。分離しないと、異常系 fixture で
#   「違反が正しく検出された」ことが、そのままスクリプト自身の FAIL として
#   誤集計される（実際に踏んだ設計ミス）。
collect_violations() {
  local dir="$1"
  local -a violations=()
  local pkg="${dir}/package.json"

  if [ ! -f "$pkg" ]; then
    violations+=("package.json が無い（${pkg}）")
  else
    # AC-2-1: scripts.lint / scripts.test が存在する
    local script_test script_lint
    script_test="$(node -e "try{const p=require('${pkg}');process.stdout.write(p.scripts&&p.scripts.test||'')}catch(e){}" 2>/dev/null)"
    script_lint="$(node -e "try{const p=require('${pkg}');process.stdout.write(p.scripts&&p.scripts.lint||'')}catch(e){}" 2>/dev/null)"

    [ -n "$script_test" ] || violations+=("scripts.test が無い")
    [ -n "$script_lint" ] || violations+=("scripts.lint が無い")

    # AC-2-2 / AC-3-1: watch せず非対話。`vitest run` を使う（素の vitest 単体は NG）。
    if [ -n "$script_test" ]; then
      case "$script_test" in
        *"vitest run"*) : ;;
        *) violations+=("scripts.test が 'vitest run' を使っていない（watch モードでハングしうる）: ${script_test}") ;;
      esac
      case "$script_test" in
        *"--watch"*) violations+=("scripts.test に --watch が含まれる（非対話で終了しない）") ;;
      esac
      # AC-3-2: passWithNoTests を有効にしない
      case "$script_test" in
        *"passWithNoTests"*) violations+=("scripts.test に passWithNoTests が含まれる（0件でも緑になる）") ;;
      esac
    fi

    # AC-2-5: lint の --fix を既定にしない
    if [ -n "$script_lint" ]; then
      case "$script_lint" in
        *"--fix"*) violations+=("scripts.lint に --fix が含まれる（検査が自動修正で緑になる）") ;;
      esac
    fi

    # AC-3-1: テストランナーは Vitest。Jest を入れない。
    local has_vitest has_jest
    has_vitest="$(node -e "try{const p=require('${pkg}');const d=Object.assign({},p.dependencies,p.devDependencies);process.stdout.write(('vitest' in d)?'yes':'')}catch(e){}" 2>/dev/null)"
    has_jest="$(node -e "try{const p=require('${pkg}');const d=Object.assign({},p.dependencies,p.devDependencies);process.stdout.write(('jest' in d)?'yes':'')}catch(e){}" 2>/dev/null)"
    [ "$has_vitest" = "yes" ] || violations+=("devDependencies/dependencies に vitest が無い")
    [ "$has_jest" != "yes" ] || violations+=("devDependencies/dependencies に jest が含まれる（ADR 0007 §5 は Vitest 指定）")
  fi

  # AC-2-3: package-lock.json をコミットする
  [ -f "${dir}/package-lock.json" ] || violations+=("package-lock.json が無い（npm ci が失敗する）")

  # AC-3-6: vitest 設定は apps/web 配下に置く。リポジトリ直下へ出さない。
  local vconf found_vconf=""
  for vconf in vitest.config.ts vitest.config.mts vitest.config.js vitest.config.mjs vitest.config.cts; do
    if [ -f "${dir}/${vconf}" ]; then
      found_vconf="${dir}/${vconf}"
      break
    fi
  done
  if [ -z "$found_vconf" ]; then
    violations+=("vitest.config.* が ${dir} 配下に無い")
  else
    grep -q "passWithNoTests" "$found_vconf" && \
      violations+=("${found_vconf} に passWithNoTests の記述がある（0件でも緑になりうる）")
  fi
  local root_vconf
  for vconf in vitest.config.ts vitest.config.mts vitest.config.js vitest.config.mjs vitest.config.cts; do
    if [ -f "${REPO_ROOT}/${vconf}" ]; then
      violations+=("vitest.config.* がリポジトリ直下にある（apps/web 配下に置くべき）")
    fi
  done

  # AC-3-3: 意味のあるテストを最低1件置く（静的判定: expect(true).toBe(true) の
  # ような自明恒真式だけのファイルを「意味がある」とはみなさない）。
  local test_files
  test_files="$(find "$dir" -type f \( -name '*.test.ts' -o -name '*.test.tsx' -o -name '*.spec.ts' -o -name '*.spec.tsx' \) 2>/dev/null | grep -v /node_modules/ || true)"
  if [ -z "$test_files" ]; then
    violations+=("*.test.ts(x) / *.spec.ts(x) が1件も無い")
  else
    local meaningful=0
    local f
    while IFS= read -r f; do
      [ -z "$f" ] && continue
      if grep -qE 'expect\(true\)\.toBe\(true\)|expect\(1\)\.toBe\(1\)' "$f"; then
        continue
      fi
      if grep -q 'expect(' "$f"; then
        meaningful=1
      fi
    done <<< "$test_files"
    [ "$meaningful" = 1 ] || violations+=("テストファイルはあるが、意味のある expect() を含むものが無い（自明恒真式のみ）")
  fi

  VIOLATIONS=("${violations[@]}")
}

# run_contract_case <名前> <target_dir> <want> [違反に含まれるべき部分文字列...]
#   want: "satisfy"     … 違反ゼロを期待（正常系）
#         "violate"     … 違反が1件以上あり、かつ渡した部分文字列をすべて含むことを期待
run_contract_case() {
  local name="$1" dir="$2" want="$3"; shift 3
  local -a VIOLATIONS=()
  collect_violations "$dir"
  local ok=1 joined
  joined="$(printf '%s\n' "${VIOLATIONS[@]+"${VIOLATIONS[@]}"}")"

  if [ "$want" = "satisfy" ]; then
    [ "${#VIOLATIONS[@]}" -eq 0 ] || ok=0
  else
    [ "${#VIOLATIONS[@]}" -gt 0 ] || ok=0
    local sub
    for sub in "$@"; do
      grep -qF -- "$sub" <<< "$joined" || ok=0
    done
  fi

  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s（違反 %s 件）\n' "$name" "${#VIOLATIONS[@]}"
  else
    fail=$((fail + 1))
    printf '  FAIL %s（期待=%s, 実際の違反 %s 件）\n' "$name" "$want" "${#VIOLATIONS[@]}"
    local v
    for v in "${VIOLATIONS[@]+"${VIOLATIONS[@]}"}"; do
      printf '       - %s\n' "$v"
    done
  fi
}

echo "==> test-apps-web-contract"

# --- fixture: 何も無い --------------------------------------------------
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "${WORK}/empty"
run_contract_case "異常系: 何も無いディレクトリは全違反" "${WORK}/empty" violate \
  "package.json が無い"

# --- fixture: 正常系（すべて満たす）----------------------------------------
GOOD="${WORK}/good"
mkdir -p "${GOOD}/src/app"
cat > "${GOOD}/package.json" <<'EOF'
{
  "name": "fixture-web",
  "private": true,
  "scripts": {
    "test": "vitest run",
    "lint": "eslint ."
  },
  "devDependencies": {
    "vitest": "^2.0.0"
  }
}
EOF
: > "${GOOD}/package-lock.json"
cat > "${GOOD}/vitest.config.ts" <<'EOF'
import { defineConfig } from "vitest/config";
export default defineConfig({ test: {} });
EOF
cat > "${GOOD}/src/app/next.config.test.ts" <<'EOF'
import { describe, it, expect } from "vitest";
import config from "../../next.config";

describe("next.config", () => {
  it("does not use static export (ADR 0013 / P-6)", () => {
    expect(config.output).not.toBe("export");
  });
});
EOF
run_contract_case "正常系: 契約をすべて満たす fixture は違反ゼロ" "$GOOD" satisfy

# --- fixture: passWithNoTests が混入 ----------------------------------------
BAD_PASSNOTESTS="${WORK}/bad-passwithnotests"
mkdir -p "${BAD_PASSNOTESTS}/src"
cp "${GOOD}/package.json" "${BAD_PASSNOTESTS}/package.json"
: > "${BAD_PASSNOTESTS}/package-lock.json"
cat > "${BAD_PASSNOTESTS}/vitest.config.ts" <<'EOF'
import { defineConfig } from "vitest/config";
export default defineConfig({ test: { passWithNoTests: true } });
EOF
cp "${GOOD}/src/app/next.config.test.ts" "${BAD_PASSNOTESTS}/src/next.config.test.ts" 2>/dev/null || true
mkdir -p "${BAD_PASSNOTESTS}/src/app"
cp "${GOOD}/src/app/next.config.test.ts" "${BAD_PASSNOTESTS}/src/app/next.config.test.ts"
run_contract_case "異常系: vitest.config.ts の passWithNoTests は検出される" "$BAD_PASSNOTESTS" violate \
  "passWithNoTests"

# --- fixture: 自明恒真式のみのテスト -----------------------------------------
BAD_TRIVIAL="${WORK}/bad-trivial"
mkdir -p "${BAD_TRIVIAL}/src"
cp "${GOOD}/package.json" "${BAD_TRIVIAL}/package.json"
: > "${BAD_TRIVIAL}/package-lock.json"
cp "${GOOD}/vitest.config.ts" "${BAD_TRIVIAL}/vitest.config.ts"
cat > "${BAD_TRIVIAL}/src/trivial.test.ts" <<'EOF'
import { describe, it, expect } from "vitest";
describe("trivial", () => {
  it("is always true", () => {
    expect(true).toBe(true);
  });
});
EOF
run_contract_case "異常系: 自明恒真式のみのテストは意味が無いとして検出される" "$BAD_TRIVIAL" violate \
  "自明恒真式のみ"

# --- fixture: vitest run を使っていない（watch モードのまま）----------------
BAD_WATCH="${WORK}/bad-watch"
mkdir -p "${BAD_WATCH}/src"
: > "${BAD_WATCH}/package-lock.json"
cp "${GOOD}/vitest.config.ts" "${BAD_WATCH}/vitest.config.ts"
cp "${GOOD}/src/app/next.config.test.ts" "${BAD_WATCH}/src/next.config.test.ts" 2>/dev/null || true
mkdir -p "${BAD_WATCH}/src/app"
cp "${GOOD}/src/app/next.config.test.ts" "${BAD_WATCH}/src/app/next.config.test.ts"
cat > "${BAD_WATCH}/package.json" <<'EOF'
{
  "name": "fixture-web",
  "private": true,
  "scripts": {
    "test": "vitest",
    "lint": "eslint ."
  },
  "devDependencies": {
    "vitest": "^2.0.0"
  }
}
EOF
run_contract_case "異常系: scripts.test が素の vitest（watch）は検出される" "$BAD_WATCH" violate \
  "vitest run"

# --- 実リポジトリの apps/web -------------------------------------------------
# 現状（本テスト作成時点）は apps/web が未スキャフォールドのため違反ゼロには
# ならない（AC-2/AC-3 の Red 実測そのもの）。スキャフォールド後は
# run_contract_case ... satisfy 相当（違反ゼロ）になることを期待する
# 回帰ケースとして機能する。
TARGET_DIR="${1:-${REPO_ROOT}/apps/web}"
run_contract_case "実リポジトリの apps/web の契約（AC-2/AC-3 の本番適用）" "$TARGET_DIR" satisfy

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
