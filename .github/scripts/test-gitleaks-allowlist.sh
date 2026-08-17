#!/usr/bin/env bash
# gitleaks の apps/web/.next allowlist（AC-11）の Red テスト。
#
# 仕様の単一情報源: docs/specs/web-app-scaffold.md AC-11。
#   - Part A（AC-11-4）: `.gitleaks.toml` の `[[allowlists]]` `paths` 正規表現に
#     対する「パス判定」のテスト。11-4 のテストベクタ表をそのまま落とす。
#   - Part B（AC-11-6 / 11-6-b 補足）: `[extend] useDefault = true` が実際に
#     効いていることの機械検査。AC-11-6-a が要求する「`.next` の外で検出さ
#     れる」と「`.next` 配下で検出されない」の**対**で実測する。片方だけでは
#     useDefault の欠落による偽 Green と、正しい実装を区別できない。
#
# 【対象は REPO_ROOT/.gitleaks.toml（$1 で差し替え可能。既定は REPO_ROOT）】
#   `.gitleaks.toml` は本テスト作成時点では存在しない（AC-11-1 は実装工程の
#   仕事）。実装前は Part A / Part B の「実リポジトリ」ケースが Red になる。
#   fixture ケース（GOOD / BAD）は、その Red が「対象未実装」によるものであり
#   チェッカ自体の構文崩れではないことを、正常系が緑であることで切り分ける
#   ために置く（test-apps-web-contract.sh と同じ設計）。
#
# 【ダミーは実行時に組み立てる。実体をリポジトリに書かない（AC-11-6-b/6-c）】
#   AWS アクセスキー ID 形式のダミーを本ファイルに書かない
#   （`make scan-secrets` 自身が本ファイルを検出して落ちるため）。
#   実測した事実（gitleaks 8.30.1）: AWS のアクセスキー ID は Base32 に近い
#   文字種 [A-Z2-7]（0/1/8/9 を含まない）を使っており、組み込みルール
#   `aws-access-token` はその文字種を要求する。[A-Z0-9]（0/1/8/9 を含む）で
#   生成すると検出が不安定になる（実測で 20 回中 6 回しか検出されなかった）。
#   これは AC-11-6-b 補足が警告する「形式が合っていても検出されない」の一種
#   であり、b-1（検出されることを先に実測してから採用する）に従い
#   [A-Z2-7] を採用した（15/15 で検出されることを実測済み）。
#
# 【新しい make ターゲットは作らない（AC-11-6-e）】
#   本ファイルは単体で
#   `bash .github/scripts/test-gitleaks-allowlist.sh [.gitleaks.toml のパス]`
#   として実行し、Red / Green を確認する。Makefile の test-scripts へ合流
#   させる（実装は tester 工程では行わない）。
#
# 【CI への申し送り】
#   `.github/workflows/ci.yml` の `scripts` ジョブ（test-scripts を呼ぶ）は
#   本テスト作成時点で gitleaks をインストールしていない（gitleaks の
#   インストールは別ジョブ `secrets` のみ）。本テストを test-scripts へ
#   合流させたことで、CI 側にも gitleaks のインストールが必要になる可能性が
#   ある。この対応は本テスト工程（tester）の範囲外とし、`.gitleaks.toml` を
#   追加する実装工程への申し送り事項とする。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
REAL_TOML="${1:-${REPO_ROOT}/.gitleaks.toml}"

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "gitleaks が見つからない。本テストは動的検査のため gitleaks が必須（devcontainer の前提）。" >&2
  exit 1
fi

pass=0
fail=0

WORK_ROOT="$(mktemp -d)"
trap 'rm -rf "$WORK_ROOT"' EXIT

echo "==> test-gitleaks-allowlist"

# =============================================================================
# Part A: AC-11-4 のテストベクタ表 — `paths` 正規表現のパス判定
# =============================================================================

# 11-4 の表をそのまま並べる（1行1件。行3は「配下の任意の深さのファイル」の
# 代表例として深いパスを1つ採用する）。
VECTOR_PATHS=(
  "apps/web/.next/prerender-manifest.json"
  "apps/web/.next/server/server-reference-manifest.json"
  "apps/web/.next/a/b/c/deep.json"
  "apps/web/src/lib/lambda-client.ts"
  "apps/web/next.config.ts"
  "apps/web/.next.bak/prerender-manifest.json"
  "apps/web/xnext/a.json"
  "apps/web/node_modules/pkg/.next/a.json"
  "docs/.next/a.json"
  "services/api/.next/a.json"
  "infra/terraform/main.tf"
)
VECTOR_EXPECT=(
  match match match
  nomatch nomatch nomatch nomatch nomatch nomatch nomatch nomatch
)

# extract_paths_regex <toml_file>
#   `.gitleaks.toml` の `paths = [...]` 行から正規表現を1本だけ取り出す。
#   対応する引用形式は 11-3 が要求するリテラル文字列（'''...''' / '...'）
#   のみ（fact d: 基本文字列 """...""" は実際の gitleaks では読み込みに
#   失敗するため対応しない）。要素が複数（カンマを含む）の場合は 11-3(a)
#   違反として抽出失敗を返す。1行に収まらない複数行配列にも対応しない
#   （実装工程が単純な1行配列を採る前提。既知の限界）。
extract_paths_regex() {
  local toml="$1"
  [ -f "$toml" ] || return 1
  local raw
  raw="$(grep -E '^[[:space:]]*paths[[:space:]]*=' "$toml" | head -n1)"
  [ -n "$raw" ] || return 1
  local inner
  inner="$(printf '%s' "$raw" | sed -E 's/^[^=]*=[[:space:]]*\[(.*)\][[:space:]]*$/\1/')"
  [ "$inner" != "$raw" ] || return 1
  if printf '%s' "$inner" | grep -q ','; then
    return 1
  fi
  inner="$(printf '%s' "$inner" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
  if [[ "$inner" =~ ^\'\'\'(.*)\'\'\'$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
    return 0
  elif [[ "$inner" =~ ^\'(.*)\'$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

# collect_path_violations <toml_file>
#   グローバル配列 VIOLATIONS へ、11-4 の期待とズレた行を積む。
collect_path_violations() {
  local toml="$1"
  local -a violations=()
  local regex rc
  regex="$(extract_paths_regex "$toml")"
  rc=$?
  if [ "$rc" -ne 0 ] || [ -z "$regex" ]; then
    violations+=("paths の正規表現を ${toml} から抽出できない（.gitleaks.toml が無い/要素が複数/引用形式が非対応 のいずれか）")
    VIOLATIONS=("${violations[@]}")
    return
  fi
  local i path expect got
  for i in "${!VECTOR_PATHS[@]}"; do
    path="${VECTOR_PATHS[$i]}"
    expect="${VECTOR_EXPECT[$i]}"
    if printf '%s' "$path" | grep -Eq -- "$regex"; then
      got=match
    else
      got=nomatch
    fi
    if [ "$got" != "$expect" ]; then
      violations+=("11-4 テストベクタ不一致: ${path} は期待=${expect} 実際=${got}（regex=${regex}）")
    fi
  done
  VIOLATIONS=("${violations[@]}")
}

# run_path_case <名前> <toml_file> <want> [違反に含まれるべき部分文字列...]
run_path_case() {
  local name="$1" toml="$2" want="$3"; shift 3
  local -a VIOLATIONS=()
  collect_path_violations "$toml"
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

# --- fixture: 既定形 regex（^apps/web/\.next/）は11-4を全て満たす -----------
GOOD_TOML="${WORK_ROOT}/good.gitleaks.toml"
cat > "$GOOD_TOML" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
run_path_case "正常系: 既定形 regex（アンカー済み・エスケープ済み）は11-4の全ベクタを満たす" "$GOOD_TOML" satisfy

# --- fixture: `.` をエスケープし忘れると xnext を誤って除外する -------------
BAD_UNESCAPED_TOML="${WORK_ROOT}/bad-unescaped.gitleaks.toml"
cat > "$BAD_UNESCAPED_TOML" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/.next/''']
EOF
run_path_case "異常系: '.' をエスケープしない regex は apps/web/xnext/a.json まで誤って除外する（11-3-c / 11-4）" "$BAD_UNESCAPED_TOML" violate \
  "apps/web/xnext/a.json"

# --- fixture: paths の要素が複数（11-3-a 違反）は抽出できない ---------------
MULTI_TOML="${WORK_ROOT}/multi.gitleaks.toml"
cat > "$MULTI_TOML" <<'EOF'
[[allowlists]]
paths = ['''^apps/web/\.next/''', '''^docs/\.next/''']
EOF
run_path_case "異常系: paths 要素が複数だと抽出できない（11-3-a）" "$MULTI_TOML" violate \
  "抽出できない"

# --- fixture: `.gitleaks.toml` が存在しない ---------------------------------
run_path_case "異常系: .gitleaks.toml が存在しないディレクトリでは抽出できない" "${WORK_ROOT}/does-not-exist.gitleaks.toml" violate \
  "抽出できない"

# --- 実リポジトリの .gitleaks.toml ------------------------------------------
# 本テスト作成時点（AC-11-1 未実装）では .gitleaks.toml が存在せず、上と
# 同じ理由で Red になる。実装後は「正常系」と同じ意味で満たすことを期待する
# 回帰ケースとして機能する。
run_path_case "実リポジトリの .gitleaks.toml（AC-11-4 の本番適用）" "$REAL_TOML" satisfy

echo ""

# =============================================================================
# Part B: AC-11-6 / 11-6-b 補足 — useDefault = true が効いていることの機械検査
# =============================================================================

DUMMY_KEYNAME="AWS_ACCESS_KEY_ID"
# gen_dummy: AWS アクセスキー ID 形式（AKIA + 大文字英数字16文字）のダミーを
# 実行時に組み立てる。文字種は [A-Z2-7]（0/1/8/9 を含まない）。理由は本ファイル
# 冒頭のコメント参照（実測により [A-Z0-9] は検出が不安定と判明したため）。
gen_dummy() {
  local suffix
  suffix="$(LC_ALL=C tr -dc 'A-Z2-7' < /dev/urandom | head -c16)"
  printf 'AKIA%s' "$suffix"
}

# 11-4 の表にある実在の相対パスをそのまま使う（Part A との整合を取り、新しい
# パスを創作しない）。
OUTSIDE_PATH="apps/web/src/lib/lambda-client.ts"
INSIDE_PATH="apps/web/.next/prerender-manifest.json"

write_good_toml() {
  cat > "$1" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
}

write_bad_toml_no_usedefault() {
  # AC-11-2 が警告する最悪の失敗モード（useDefault を書き忘れると組み込み
  # ルールが無効化される）をそのまま再現する fixture。
  cat > "$1" <<'EOF'
[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
}

write_real_toml() {
  if [ -f "$REAL_TOML" ]; then
    cp "$REAL_TOML" "$1"
  fi
  # REAL_TOML が存在しない場合（本テスト作成時点の実態）は、config を置かない
  # ＝ gitleaks の組み込みルールのみで動く状態のまま検証する。
}

# run_dir_case <名前> <writer_fn_or_empty> <relpath> <want:1=検出される/0=検出されない>
run_dir_case() {
  local name="$1" writer="$2" relpath="$3" want="$4"
  local work dummy ec
  work="$(mktemp -d -p "$WORK_ROOT")"
  mkdir -p "$(dirname "${work}/${relpath}")"
  dummy="$(gen_dummy)"
  printf '%s=%s\n' "$DUMMY_KEYNAME" "$dummy" > "${work}/${relpath}"
  if [ -n "$writer" ]; then
    "$writer" "${work}/.gitleaks.toml"
  fi
  ( cd "$work" && gitleaks dir . --no-banner --redact ) > "${work}/out.log" 2>&1
  ec=$?

  local ok=1
  if [ "$want" = "1" ]; then
    [ "$ec" -ne 0 ] || ok=0
  else
    [ "$ec" -eq 0 ] || ok=0
  fi

  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s（EXIT=%s）\n' "$name" "$ec"
  else
    fail=$((fail + 1))
    local want_desc
    if [ "$want" = "1" ]; then want_desc="!=0（検出される）"; else want_desc="=0（検出されない）"; fi
    printf '  FAIL %s（期待 EXIT %s, 実際 EXIT=%s）\n' "$name" "$want_desc" "$ec"
    sed 's/^/       /' "${work}/out.log"
  fi
}

# --- control: config 無し。ダミーの検出可能性そのものの裏付け（11-6-b-1）---
run_dir_case "control: config 無しで .next の外のダミーは検出される（ダミー選定の裏付け・11-6-b-1）" "" "$OUTSIDE_PATH" 1

# --- GOOD config: AC-11-6-a のペア（正しい実装を想定した正常系）------------
run_dir_case "正常系: useDefault=true + allowlist → .next の外は検出される（11-6-a-(1)）" write_good_toml "$OUTSIDE_PATH" 1
run_dir_case "正常系: useDefault=true + allowlist → .next 配下は検出されない（11-6-a-(2)）" write_good_toml "$INSIDE_PATH" 0

# --- BAD config: useDefault を落とすと偽 Green になる（AC-11-2 の再現）-----
run_dir_case "異常系: useDefault を落とすと .next の外も検出されなくなる（偽Green再現・AC-11-2）" write_bad_toml_no_usedefault "$OUTSIDE_PATH" 0
run_dir_case "異常系: useDefault を落とすと .next 配下も検出されない（allowlistが効いたのではなく全ルールが死んでいるだけ）" write_bad_toml_no_usedefault "$INSIDE_PATH" 0

# --- 実リポジトリの .gitleaks.toml（AC-11-6-a の本番適用）-------------------
# 本テスト作成時点では .gitleaks.toml が存在しないため、.next 配下も除外
# されず検出されてしまい、2件目が Red になる（AC-11-1 が未実装のため）。
run_dir_case "実リポジトリの .gitleaks.toml: .next の外は検出される（11-6-a-(1) 本番適用）" write_real_toml "$OUTSIDE_PATH" 1
run_dir_case "実リポジトリの .gitleaks.toml: .next 配下は検出されない（11-6-a-(2) 本番適用）" write_real_toml "$INSIDE_PATH" 0

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
