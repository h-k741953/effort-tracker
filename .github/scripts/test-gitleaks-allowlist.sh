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

# =============================================================================
# Part C: AC-12 — `.gitleaks.toml` の構造をホワイトリスト型で機械検査する
#
#   AC-11 の訂正（11-9-c）から派生。「許可された記述はこれだけ」というホワ
#   イトリスト型（12-1）で、12-2 の許可集合と 12-3 の必須テストベクタを検査
#   する。Part A / Part B（AC-11 の動的検査）を置き換えず併存させる（12-6-b）。
#
#   collect_structure_violations() は 12-2-a〜h の許可集合との照合を行う
#   本体。往復5の時点で 3-a〜3-k 相当（2-a〜2-g の一部）は実装済みだが、
#   2026-08-18 訂正で追加された次の要求は【本テスト作成時点ではまだ判定に
#   反映されていない】:
#     - 12-2-d-1: `paths` の値が既定形の文字列と完全一致すること（内容の
#       完全一致。型だけの検査では 3-l/3-m/3-n を捕捉できない）
#     - 12-2-b 補足 / 12-2-c 補足: `[extend]` / `useDefault` / `[[allowlists]]`
#       の「不在」も違反にすること（3-o/3-p/3-q。現状は複数存在の上限しか
#       見ておらず、不在は検査されない）
#     - 12-2-d-2: `paths` 行の行末コメントを不合格にすること（3-r。現状は
#       全行の行末コメントを一律に許している）
#     - 12-2-h: 行頭インデントを許可外にすること（3-t。現状は trim してか
#       ら判定しており、インデントの有無を見ていない）
#   そのため 3-l/3-m/3-n/3-o/3-p/3-q/3-r/3-t は Red になる。3-s（値の前後
#   空白）は is_single_literal_array の正規表現が前後空白込みの文字列を
#   リテラル文字列として受理してしまうため、これも Red になる。
#   3-a/3-b（合格を期待）と、訂正後の 3-j（`paths` 行以外の行末コメントは
#   合格を期待）は緑のまま。この切り分けにより、Red が「対象未実装」による
#   ものであることを確かめる（本ファイル冒頭のコメントと同じ設計）。
#   実装（12-2-d-1 / 2-b 補足 / 2-d-2 / 2-h の判定ロジックそのもの）は本テ
#   スト工程の範囲外。次工程（implementer）が collect_structure_violations()
#   へ反映する。
# =============================================================================

# collect_structure_violations <dir>
#   グローバル配列 VIOLATIONS へ、12-2 の許可集合から外れた記述と、
#   12-2-g が禁じる `.gitleaksignore` の存在を積む。
#   <dir>/.gitleaks.toml と <dir>/.gitleaksignore を見る。
collect_structure_violations() {
  local dir="$1"
  local -a violations=()
  local toml="${dir}/.gitleaks.toml"
  local ignorefile="${dir}/.gitleaksignore"

  # --- 12-2-g: .gitleaksignore の不在 -----------------------------------
  if [ -f "$ignorefile" ]; then
    violations+=("${ignorefile} が存在する（12-2-g: 除外の入口は .gitleaks.toml の1つに固定）")
  fi

  if [ ! -f "$toml" ]; then
    violations+=("${toml} が存在しない")
    VIOLATIONS=("${violations[@]}")
    return
  fi

  # --- 行末コメント／引用符内の '#' を取り違えない除去（12-2-f） --------
  #   '（リテラル文字列）と "（他の文字列）を独立にトグルし、いずれの
  #   引用符の外で最初に現れた '#' 以降を切り捨てる。三連引用符
  #   ('''...''') も1文字ずつの奇偶トグルで正しく開閉が扱える。
  strip_comment() {
    local line="$1" out="" i ch qc="" len
    len=${#line}
    for ((i = 0; i < len; i++)); do
      ch="${line:i:1}"
      if [ -n "$qc" ]; then
        out+="$ch"
        [ "$ch" = "$qc" ] && qc=""
      else
        if [ "$ch" = "'" ] || [ "$ch" = '"' ]; then
          qc="$ch"
          out+="$ch"
        elif [ "$ch" = "#" ]; then
          break
        else
          out+="$ch"
        fi
      fi
    done
    printf '%s' "$out"
  }

  # line_has_trailing_comment: 引用符外に '#' が現れるかどうかを真偽で返す
  #   （paths 行の行末コメント判定・12-2-d-2 に使う）。strip_comment と
  #   同じトークナイズだが、`$(...)` を介さず直接呼び出して判定する
  #   （$(...) はサブシェルを生成し、グローバル変数への代入が呼び出し元
  #   へ伝播しないため、真偽は終了コードで返す）。
  line_has_trailing_comment() {
    local line="$1" i ch qc="" len
    len=${#line}
    for ((i = 0; i < len; i++)); do
      ch="${line:i:1}"
      if [ -n "$qc" ]; then
        [ "$ch" = "$qc" ] && qc=""
      else
        if [ "$ch" = "'" ] || [ "$ch" = '"' ]; then
          qc="$ch"
        elif [ "$ch" = "#" ]; then
          return 0
        fi
      fi
    done
    return 1
  }

  # --- 12-2-d / 12-2-d-1: 「要素1個のリテラル文字列の配列」を判定し、
  #     引用符を剥がした値そのものを標準出力へ返す（呼び出し側で既定値と
  #     の完全一致を比較する）。値の内容自体は一切加工しない
  #     （trim・正規化・パターン照合をしない。12-2-d-1-c / d-1-d）。
  extract_single_literal_value() {
    local val="$1" inner
    inner="$(printf '%s' "$val" | sed -E 's/^\[(.*)\]$/\1/')"
    [ "$inner" != "$val" ] || return 1
    printf '%s' "$inner" | grep -q ',' && return 1
    # 角括弧の内側・引用符の外側にある空白は TOML の記法上の空白であり
    # 値ではないため、ここでのみ trim してよい（12-2-d-1-c）。
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

  # 許可する paths の値はこの1つだけ（12-2-d-1-a）。禁止パターンの列挙
  # ではなく、許可する文字列そのものを1つに固定して比較する
  # （12-2-d-1-d）。
  local PATHS_DEFAULT_VALUE='^apps/web/\.next/'

  local extend_count=0 allowlists_count=0
  # --- 12-2-i: 「ちょうど1つ」はキーの出現回数で数える（真偽フラグでは
  #     2回目の代入が1回目を吸収してしまい、重複キー行を検知できない）。
  #     記法（ドット付きキー／テーブル内）をまたいで通算し（i-d）、値や
  #     記法が許可外の行も出現回数に数える（i-f）。
  local usedefault_occurrences=0 paths_occurrences=0
  # --- 12-2-i-g（往復7・W-1）: 重複違反のメッセージに2件目以降の行番号を
  #     載せるため、出現ごとの行番号も配列で貯める（判定ロジックは
  #     usedefault_occurrences / paths_occurrences のままで変更しない）。
  local -a usedefault_linenos=() paths_linenos=()
  local current_table=""   # "" | extend | allowlists | other
  local line stripped trimmed lineno=0 had_comment=0

  while IFS= read -r line || [ -n "$line" ]; do
    lineno=$((lineno + 1))

    # --- 12-2-h: 行頭のインデントを許可しない（記法違いの抜けを塞ぐ）----
    #   コメント／空行を含む「許可される記述の行」すべてが対象。加工前の
    #   生の行に対して判定する（trim してから見ると有無が分からない）。
    if [[ "$line" =~ [^[:space:]] ]] && [[ "$line" =~ ^[[:space:]] ]]; then
      violations+=("行${lineno}: 行頭にインデントがある（12-2-h）: $(printf '%s' "$line" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')")
      continue
    fi

    stripped="$(strip_comment "$line")"
    if line_has_trailing_comment "$line"; then had_comment=1; else had_comment=0; fi
    trimmed="$(printf '%s' "$stripped" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
    [ -z "$trimmed" ] && continue   # 空行／コメントのみの行（2-a）

    if [ "$trimmed" = "[extend]" ]; then
      extend_count=$((extend_count + 1))
      current_table="extend"
      if [ "$extend_count" -gt 1 ]; then
        violations+=("行${lineno}: [extend] テーブルが複数（2-b）: ${trimmed}")
      fi
      continue
    fi

    if [ "$trimmed" = "[[allowlists]]" ]; then
      allowlists_count=$((allowlists_count + 1))
      current_table="allowlists"
      if [ "$allowlists_count" -gt 1 ]; then
        violations+=("行${lineno}: [[allowlists]] テーブルが複数（2-b）: ${trimmed}")
      fi
      continue
    fi

    # 許可されないテーブル見出し（[allowlist] / [[rules]] /
    # [[rules.allowlists]] / [allowlists.xxx] 等）
    if [[ "$trimmed" =~ ^\[\[?[A-Za-z0-9_.-]+\]\]?$ ]]; then
      violations+=("行${lineno}: 許可されないテーブル見出し（2-b）: ${trimmed}")
      current_table="other"
      continue
    fi

    # トップレベル（まだテーブル見出しが出ていない状態）でのドット付き
    # キー（2-e: extend.xxx / allowlists.xxx をテーブル内記法と同一視）
    if [ "$current_table" = "" ] && [[ "$trimmed" =~ ^([A-Za-z0-9_-]+)\.([A-Za-z0-9_-]+)[[:space:]]*=[[:space:]]*(.*)$ ]]; then
      local tbl="${BASH_REMATCH[1]}" key="${BASH_REMATCH[2]}" val="${BASH_REMATCH[3]}"
      val="$(printf '%s' "$val" | sed -E 's/[[:space:]]+$//')"
      case "$tbl" in
        extend)
          if [ "$key" = "useDefault" ]; then
            usedefault_occurrences=$((usedefault_occurrences + 1))
            usedefault_linenos+=("$lineno")
            if [ "$val" != "true" ]; then
              violations+=("行${lineno}: ドット付きキー extend.${key} は許可されない（2-c/2-e）: ${trimmed}")
            fi
          else
            violations+=("行${lineno}: ドット付きキー extend.${key} は許可されない（2-c/2-e）: ${trimmed}")
          fi
          ;;
        allowlists)
          if [ "$key" = "paths" ]; then
            paths_occurrences=$((paths_occurrences + 1))
            paths_linenos+=("$lineno")
            local dotpv dotrc
            dotpv="$(extract_single_literal_value "$val")"; dotrc=$?
            if [ "$dotrc" -ne 0 ]; then
              violations+=("行${lineno}: ドット付きキー allowlists.paths は要素1個のリテラル文字列配列ではない（2-d/2-e）: ${trimmed}")
            elif [ "$had_comment" = "1" ]; then
              violations+=("行${lineno}: paths 行に行末コメントがある（12-2-d-2）: ${trimmed}")
            elif [ "$dotpv" != "$PATHS_DEFAULT_VALUE" ]; then
              violations+=("行${lineno}: paths の値が既定値（${PATHS_DEFAULT_VALUE}）と完全一致しない（12-2-d-1）: ${trimmed}")
            fi
          else
            violations+=("行${lineno}: ドット付きキー allowlists.${key} は許可されない（2-d/2-e）: ${trimmed}")
          fi
          ;;
        *)
          violations+=("行${lineno}: 許可されないドット付きキー（2-e）: ${trimmed}")
          ;;
      esac
      continue
    fi

    # 通常のキー = 値（テーブル内）
    if [[ "$trimmed" =~ ^([A-Za-z0-9_-]+)[[:space:]]*=[[:space:]]*(.*)$ ]]; then
      local key2="${BASH_REMATCH[1]}" val2="${BASH_REMATCH[2]}"
      val2="$(printf '%s' "$val2" | sed -E 's/[[:space:]]+$//')"
      case "$current_table" in
        extend)
          if [ "$key2" = "useDefault" ]; then
            usedefault_occurrences=$((usedefault_occurrences + 1))
            usedefault_linenos+=("$lineno")
            if [ "$val2" != "true" ]; then
              violations+=("行${lineno}: [extend] 内の許可外キー/値（2-c）: ${trimmed}")
            fi
          else
            violations+=("行${lineno}: [extend] 内の許可外キー/値（2-c）: ${trimmed}")
          fi
          ;;
        allowlists)
          if [ "$key2" = "paths" ]; then
            paths_occurrences=$((paths_occurrences + 1))
            paths_linenos+=("$lineno")
            local pv2 rc2
            pv2="$(extract_single_literal_value "$val2")"; rc2=$?
            if [ "$rc2" -ne 0 ]; then
              violations+=("行${lineno}: [[allowlists]] 内の許可外キー/値（2-d）: ${trimmed}")
            elif [ "$had_comment" = "1" ]; then
              violations+=("行${lineno}: paths 行に行末コメントがある（12-2-d-2）: ${trimmed}")
            elif [ "$pv2" != "$PATHS_DEFAULT_VALUE" ]; then
              violations+=("行${lineno}: paths の値が既定値（${PATHS_DEFAULT_VALUE}）と完全一致しない（12-2-d-1）: ${trimmed}")
            fi
          else
            violations+=("行${lineno}: [[allowlists]] 内の許可外キー/値（2-d）: ${trimmed}")
          fi
          ;;
        other)
          violations+=("行${lineno}: 許可されないテーブル内のキー（2-b）: ${trimmed}")
          ;;
        "")
          violations+=("行${lineno}: トップレベルの裸キーは許可されない（2-e）: ${trimmed}")
          ;;
      esac
      continue
    fi

    # ここまでのいずれの許可形にも一致しない記述
    violations+=("行${lineno}: 許可集合に無い記述（12-2）: ${trimmed}")
  done < "$toml"

  # --- 2-b 補足 / 2-c 補足 / 2-d: 不在も違反にする ------------------------
  #   上限（複数存在）だけでなく、下限（不在）も見る。[extend] が消える
  #   と組み込みルールが全滅して何も検出しないのに緑になる（W-b）ため、
  #   これを機械で捕捉する。
  if [ "$extend_count" -eq 0 ]; then
    violations+=("[extend] テーブルが存在しない（2-b 補足）")
  fi
  if [ "$allowlists_count" -eq 0 ]; then
    violations+=("[[allowlists]] テーブルが存在しない（2-b 補足）")
  fi
  # --- 12-2-i: useDefault / paths それぞれの出現回数がちょうど1であること
  #     （0件=不在、2件以上=重複キー行。いずれも違反。値が既定値と一致して
  #     いても2件以上なら相殺しない＝i-c）。記法をまたいで通算済み（i-d）。
  if [ "$usedefault_occurrences" -eq 0 ]; then
    violations+=("[extend] 内に useDefault キーが存在しない（12-2-c 補足）")
  elif [ "$usedefault_occurrences" -gt 1 ]; then
    # 12-2-i-g: 2件目以降の行番号をメッセージに載せる（判定=出現回数の
    # 比較は上の -gt 1 のまま。ここは文言の組み立てのみ）。
    local usedefault_dup_linenos="" i
    for ((i = 1; i < ${#usedefault_linenos[@]}; i++)); do
      usedefault_dup_linenos="${usedefault_dup_linenos}行${usedefault_linenos[$i]}: "
    done
    violations+=("${usedefault_dup_linenos}[extend] 内で useDefault キーが ${usedefault_occurrences} 回出現している（12-2-i: 出現回数はちょうど1でなければ不合格）")
  fi
  if [ "$paths_occurrences" -eq 0 ]; then
    violations+=("[[allowlists]] 内に paths キーが存在しない（12-2-d 補足）")
  elif [ "$paths_occurrences" -gt 1 ]; then
    # 12-2-i-g: 2件目以降の行番号をメッセージに載せる（判定=出現回数の
    # 比較は上の -gt 1 のまま。ここは文言の組み立てのみ）。
    local paths_dup_linenos="" j
    for ((j = 1; j < ${#paths_linenos[@]}; j++)); do
      paths_dup_linenos="${paths_dup_linenos}行${paths_linenos[$j]}: "
    done
    violations+=("${paths_dup_linenos}[[allowlists]] 内で paths キーが ${paths_occurrences} 回出現している（12-2-i: 出現回数はちょうど1でなければ不合格）")
  fi

  VIOLATIONS=("${violations[@]}")
}

# run_structure_case <名前> <dir> <want:satisfy|violate>
#   「合格」= 違反0件。「不合格」= 違反1件以上（12-3 冒頭の定義）。
#   メッセージの具体的な文言は実装依存のため、件数のみで判定する。
run_structure_case() {
  local name="$1" dir="$2" want="$3"
  local -a VIOLATIONS=()
  collect_structure_violations "$dir"

  local ok=1
  if [ "$want" = "satisfy" ]; then
    [ "${#VIOLATIONS[@]}" -eq 0 ] || ok=0
  else
    [ "${#VIOLATIONS[@]}" -gt 0 ] || ok=0
  fi

  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s（違反 %s 件）\n' "$name" "${#VIOLATIONS[@]}"
  else
    fail=$((fail + 1))
    printf '  FAIL %s（期待=%s, 実際の違反 %s 件）\n' "$name" "$want" "${#VIOLATIONS[@]}"
  fi
}

# run_structure_case_min <名前> <dir> <最小件数>
#   run_structure_case の二値判定（違反0件か1件以上か）では、重複を
#   「値の不一致」等の別の違反1件と区別できない（12-3-v / 3-y が要求する
#   のは「重複としても違反が積まれること」＝件数の下限）。この呼び出し方は
#   違反件数が <最小件数> 件以上であることを検査する。
#   collect_structure_violations() 自体は変更しない。
run_structure_case_min() {
  local name="$1" dir="$2" min="$3"
  local -a VIOLATIONS=()
  collect_structure_violations "$dir"

  if [ "${#VIOLATIONS[@]}" -ge "$min" ]; then
    pass=$((pass + 1))
    printf '  ok   %s（違反 %s 件 >= %s 件）\n' "$name" "${#VIOLATIONS[@]}" "$min"
  else
    fail=$((fail + 1))
    printf '  FAIL %s（期待=%s 件以上, 実際の違反 %s 件）\n' "$name" "$min" "${#VIOLATIONS[@]}"
  fi
}

# run_structure_case_dup_lineno <名前> <dir> <key> <2件目以降の行番号...>
#   AC-12-2-i-g（W-1。往復7）: 重複キー違反のメッセージには「2件目以降の
#   行番号」と「当該キーが重複している旨」を出すことを要求している
#   （docs/specs/web-app-scaffold.md 530行目 / 12-6-c）。
#   run_structure_case / run_structure_case_min は件数のみで判定しており
#   （文言は実装依存として明記して見ていない。611行目）、i-g の要求を検査
#   するベクタが1本も無かった。本関数はその穴を塞ぐ別の呼び出し方であり、
#   run_structure_case / run_structure_case_min と collect_structure_violations()
#   自体は変更しない。
#
#   判定: VIOLATIONS の中から「${key} キーが」かつ「12-2-i」を含むメッセージ
#   （＝出現回数の重複違反）がちょうど1件あることを確かめたうえで、その
#   メッセージに <2件目以降の行番号> それぞれについて「行${n}:」という
#   部分文字列が含まれることを確かめる。書式は同ファイル内の他の違反
#   メッセージ（462/475/484/492行目）と同じ「行${lineno}: …」を踏襲し、
#   新しい書式を発明しない。
run_structure_case_dup_lineno() {
  local name="$1" dir="$2" key="$3"
  shift 3
  local -a expected_linenos=("$@")
  local -a VIOLATIONS=()
  collect_structure_violations "$dir"

  local -a matches=()
  local v
  for v in "${VIOLATIONS[@]+"${VIOLATIONS[@]}"}"; do
    if [[ "$v" == *"${key} キーが"* ]] && [[ "$v" == *"12-2-i"* ]]; then
      matches+=("$v")
    fi
  done

  local ok=1 detail=""
  if [ "${#matches[@]}" -ne 1 ]; then
    ok=0
    detail="${key} の重複違反メッセージが1件ではない（実際 ${#matches[@]} 件）"
  else
    local msg="${matches[0]}" n
    for n in "${expected_linenos[@]}"; do
      if [[ "$msg" != *"行${n}:"* ]]; then
        ok=0
        detail="メッセージに「行${n}:」が含まれない（12-2-i-g）: ${msg}"
      fi
    done
  fi

  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s\n' "$name"
  else
    fail=$((fail + 1))
    printf '  FAIL %s（%s）\n' "$name" "$detail"
  fi
}

# mk_struct_dir <name> <.gitleaks.toml の内容(heredoc相当)>
#   $WORK_ROOT 配下に fixture 用の擬似リポジトリ・ディレクトリを作り、
#   そのパスを返す（.gitleaksignore は置かない）。
STRUCT_DIR_SEQ=0
mk_struct_dir() {
  local name="$1"
  STRUCT_DIR_SEQ=$((STRUCT_DIR_SEQ + 1))
  local d="${WORK_ROOT}/struct-${STRUCT_DIR_SEQ}-${name}"
  mkdir -p "$d"
  printf '%s' "$d"
}

echo "==> test-gitleaks-allowlist (Part C: AC-12 構造検査)"

# =============================================================================
# 2-k の1件（AC-12-2-k / 12-2-k-1。AC-11-11 の1件）の fixture 組み立て部品
#
#   【テスト側の部品。検査本体（collect_structure_violations）ではない】
#   3-fx-a: 12-3 の「正常形」は 2-k の1件を含む。既存の不合格ベクタ（3-c〜3-y）
#           も 2-k の1件を含む正常形から組む。2-k の1件は fixture の末尾に置く
#           （既存 fixture の行番号を保つため。3-u/3-w/3-x の i-g の行番号は不変）。
#   3-fx-b: 値は、検査コードが持つ「1つの写し」（K_SIG_VALUE）から実行時に組み立てる。
#           変異（置換・切り詰め）も実行時にこの写しから作り、値の写しを fixture
#           ごとに増やさない（AC-11-11-c）。値の持ち主はテストの字面と
#           .gitleaks.toml であり、ここは写し（AC-12-L8 (3)）。
#           検査本体が 12-2-k-1-c の完全一致を比べる相手も、この変数 1 つとする。
#   値そのもの（64 桁 16 進）を docs に書かない（AC-11-11）。
# =============================================================================

K_SIG_VALUE='85d96828115b5dc0cfc3bd16ad9e210dd772bbebba041836c64533a82be05ead'
IFS= read -r K_PATHS_VALUE <<'EOF'
^apps/web/src/lib/lambda-client\.test\.ts$
EOF
K_RULES_HDR='[[rules]]'
K_RA_HDR='[[rules.allowlists]]'
K_ID_LINE='id = "generic-api-key"'
K_COND_LINE='condition = "AND"'
printf -v K_PATHS_LINE "paths = ['''%s''']" "$K_PATHS_VALUE"
printf -v K_REGEXES_LINE "regexes = ['''%s''']" "$K_SIG_VALUE"

# k_emit <行>: 行を出力する。@DEL@ は「この行を置かない」の意味。
k_emit() { [ "$1" = "@DEL@" ] || printf '%s\n' "$1"; }

# k_block: 2-k の1件（正しい形）を標準出力へ書く。KB_* の環境変数で行ごとに差し替える。
#   KB_RULES_HDR / KB_ID / KB_RA_HDR / KB_COND / KB_PATHS / KB_REGEXES
k_block() {
  k_emit "${KB_RULES_HDR:-$K_RULES_HDR}"
  k_emit "${KB_ID:-$K_ID_LINE}"
  printf '\n'
  k_emit "${KB_RA_HDR:-$K_RA_HDR}"
  k_emit "${KB_COND:-$K_COND_LINE}"
  k_emit "${KB_PATHS:-$K_PATHS_LINE}"
  k_emit "${KB_REGEXES:-$K_REGEXES_LINE}"
}

# append_k <file>: 空行 + 2-k の1件を末尾に足す（KB_* で差し替え可能）。
append_k() { { printf '\n'; k_block; } >> "$1"; }

# write_base <file>: 2-k を除いた旧来の正常形（[extend] + [[allowlists]]）。
write_base() {
  cat > "$1" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
}

# k_literal_line <key> <値>: `key = ['''値''']`
k_literal_line() { printf "%s = ['''%s''']" "$1" "$2"; }

# count_violations <dir>: 構造検査の違反件数を標準出力へ返す。
count_violations() {
  local -a VIOLATIONS=()
  collect_structure_violations "$1"
  printf '%s' "${#VIOLATIONS[@]}"
}

# run_structure_case_vs_baseline <名前> <dir>
#   3-a（正常形）の違反件数 BASELINE_COUNT を超えることを検査する。
#   Green（実装後）では BASELINE_COUNT=0 なので「違反 1 件以上」と同値であり、
#   run_structure_case の violate と期待は同じ（緩めていない）。
#   Red（実装前）では、旧検査が 2-k の 6 行そのものを違反に数えるため、
#   「2-k の 6 行が違反だから不合格に見える」だけの偽の Green を弾く
#   （ベクタの変更が増やした違反だけを数える）。判定は件数のみ。
BASELINE_COUNT=0
run_structure_case_vs_baseline() {
  local name="$1" dir="$2"
  local -a VIOLATIONS=()
  collect_structure_violations "$dir"
  if [ "${#VIOLATIONS[@]}" -gt "$BASELINE_COUNT" ]; then
    pass=$((pass + 1))
    printf '  ok   %s（違反 %s 件 > 正常形 %s 件）\n' "$name" "${#VIOLATIONS[@]}" "$BASELINE_COUNT"
  else
    fail=$((fail + 1))
    printf '  FAIL %s（期待=正常形の %s 件を超える, 実際の違反 %s 件）\n' "$name" "$BASELINE_COUNT" "${#VIOLATIONS[@]}"
  fi
}

# --- 3-a: 現行の正常形（2-k の1件を含む。2026-10-16 改定）--------------------
#   3-aa は「3-a と同一の fixture（別に用意しない）」ため、この 1 件が 3-aa を兼ねる。
D_3A="$(mk_struct_dir 3a)"
write_base "${D_3A}/.gitleaks.toml"
append_k "${D_3A}/.gitleaks.toml"
run_structure_case "3-a / 3-aa 正常形（[extend] + [[allowlists]] + 2-k の1件）" "$D_3A" satisfy
BASELINE_COUNT="$(count_violations "$D_3A")"

# --- 3-b: リポジトリ直下の実ファイル ----------------------------------------
REAL_DIR="$(dirname "$REAL_TOML")"
run_structure_case "3-b / 3-aa 実リポジトリの .gitleaks.toml（回帰）" "$REAL_DIR" satisfy

# --- 3-c: 既存の [extend] テーブル内に disabledRules を1行足す --------------
#   新しい [extend] テーブルを作らない（12-5-b：TOML パースエラーで落ちる
#   形を捕捉の根拠にしない。3-c は TOML として妥当）。
D_3C="$(mk_struct_dir 3c)"
cat > "${D_3C}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true
disabledRules = ["generic-api-key"]

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3C}/.gitleaks.toml"
run_structure_case "3-c 既存 [extend] 内に disabledRules を1行追加" "$D_3C" violate

# --- 3-d: 2本目の [[allowlists]] --------------------------------------------
D_3D="$(mk_struct_dir 3d)"
cat > "${D_3D}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']

[[allowlists]]
paths = ['''^docs/\.next/''']
EOF
append_k "${D_3D}/.gitleaks.toml"
run_structure_case "3-d 2本目の [[allowlists]] を追加" "$D_3D" violate

# --- 3-e: [[rules]]（既存ルール id の上書きを含む）--------------------------
#   2026-10-16 注記: 2-k の1件とずれた形のまま不合格。追加する [[rules]] は
#   id が別（aws-access-token）で、id 以外のキー（description / regex）を持つ。
#   fixture を 2-k の1件と一致させない（末尾に足す 2-k の1件とは別の [[rules]]）。
D_3E="$(mk_struct_dir 3e)"
cat > "${D_3E}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']

[[rules]]
id = "aws-access-token"
description = "disabled"
regex = '''x^'''
EOF
append_k "${D_3E}/.gitleaks.toml"
run_structure_case "3-e [[rules]] で既存ルール id を上書き（2-k の1件とずれた形）" "$D_3E" violate

# --- 3-f: [allowlist]（単数形・非配列）--------------------------------------
D_3F="$(mk_struct_dir 3f)"
cat > "${D_3F}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']

[allowlist]
paths = ['''^vendor/''']
EOF
append_k "${D_3F}/.gitleaks.toml"
run_structure_case "3-f [allowlist]（単数形）を追加" "$D_3F" violate

# --- 3-g: [[rules]] + [[rules.allowlists]] ----------------------------------
#   2026-10-16 注記: 2-k の1件とずれた形のまま不合格。追加する
#   [[rules.allowlists]] は paths の値が別（^vendor/）で、condition と regexes を欠く。
D_3G="$(mk_struct_dir 3g)"
cat > "${D_3G}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']

[[rules]]
id = "generic-api-key"

[[rules.allowlists]]
paths = ['''^vendor/''']
EOF
append_k "${D_3G}/.gitleaks.toml"
run_structure_case "3-g [[rules]] + [[rules.allowlists]]（ルール個別の迂回。2-k の1件とずれた形）" "$D_3G" violate

# --- 3-h: [[allowlists]] に regexes を追加 ----------------------------------
#   2026-10-16 注記: トップレベルの [[allowlists]] の regexes は 2-k の1件とは別の
#   場所（値も別: AKIA…。condition も無い）。不合格のまま。
D_3H="$(mk_struct_dir 3h)"
cat > "${D_3H}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
regexes = ['''AKIA[0-9A-Z]{16}''']
EOF
append_k "${D_3H}/.gitleaks.toml"
run_structure_case "3-h [[allowlists]] に regexes を追加（2-k の1件とは別の場所）" "$D_3H" violate

# --- 3-i: トップレベルのドット付きキー --------------------------------------
#   [extend] を見出し（[extend]）ではなくドット付きキーだけで書く。TOML の
#   解決規則上、見出し [extend] とトップレベルの extend.* を同一ファイルで
#   併用すると構文エラーになるため（実測: tomllib で "Cannot declare
#   ('extend',) twice"）、ドット付きキーのみで [extend] と同等の内容
#   （useDefault=true と禁止キー disabledRules）を表現する。12-2-e が言う
#   「記法を変えれば通る、という抜け」の再現であり、内容は 3-c と同値。
D_3I="$(mk_struct_dir 3i)"
cat > "${D_3I}/.gitleaks.toml" <<'EOF'
extend.useDefault = true
extend.disabledRules = ["generic-api-key"]

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3I}/.gitleaks.toml"
run_structure_case "3-i トップレベルのドット付きキー extend.disabledRules" "$D_3I" violate

# --- 3-j: `paths` 行以外の各行に行末コメント（誤検知しないこと）------------
#   2026-08-18 訂正: 旧 3-j は「各行に」付けたものを合格としていたが、
#   `paths` 行に行末コメントを付けると値の切り出しが失敗し実装と矛盾して
#   いた（12-2-d-2 / W-a）。`paths` 行の行末コメントは 3-r で別途「不合格」
#   を確かめる。本ケースは `paths` 行**以外**（[extend] / useDefault /
#   [[allowlists]]）に付けたものが合格することだけを確かめる。
#   2026-10-16 注記: 「各行」は 2-d-2-b の 3 行を指し、2-k の 6 行を含まない
#   （2-k の1件は末尾に、行末コメント無しの正しい形で置く）。
D_3J="$(mk_struct_dir 3j)"
cat > "${D_3J}/.gitleaks.toml" <<'EOF'
[extend]  # 理由
useDefault = true  # 理由

[[allowlists]]  # 理由
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3J}/.gitleaks.toml"
run_structure_case "3-j paths 行以外の行末コメントは誤検知しない（12-2-d-2-b。2-k の6行は含まない）" "$D_3J" satisfy

# --- 3-k: .gitleaksignore が存在する ----------------------------------------
D_3K="$(mk_struct_dir 3k)"
write_base "${D_3K}/.gitleaks.toml"
append_k "${D_3K}/.gitleaks.toml"
: > "${D_3K}/.gitleaksignore"
run_structure_case "3-k .gitleaksignore が存在する（12-2-g）" "$D_3K" violate

# =============================================================================
# 12-3 の追加ベクタ（2026-08-18。完全一致・不在・行末コメントの検証）
#   期待値はいずれも「不合格」（12-3 の追加ベクタ表 前文）。
# =============================================================================

# --- 3-l: paths の値に選択肢を追加（往復4で 26/26 通過が実測された形。C-3）--
D_3L="$(mk_struct_dir 3l)"
cat > "${D_3L}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/|^infra/prod''']
EOF
append_k "${D_3L}/.gitleaks.toml"
run_structure_case "3-l paths の値に選択肢を追加（C-3。既定値と完全一致しない）" "$D_3L" violate

# --- 3-m: paths の値の末尾を緩和（同型だが未実測の形）----------------------
D_3M="$(mk_struct_dir 3m)"
cat > "${D_3M}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/.*''']
EOF
append_k "${D_3M}/.gitleaks.toml"
run_structure_case "3-m paths の値の末尾を緩和（既定値と完全一致しない）" "$D_3M" violate

# --- 3-n: paths の値から `^` を欠落（往復4の M1 の回帰。12-4-d と同一の値）-
#   12-4-d が要求する「`^` を削った fixture」と同一のものであり、両者を
#   別々に用意しない（12-3 追加ベクタ表・3-n の行）。実体はここで1度だけ
#   定義し、12-4 側（BAD_NO_ANCHOR_TOML）はこのファイルをそのまま再利用する。
#   2-k の1件は末尾に足す（12-4 側が読むのは先頭の paths 行なので影響しない）。
NO_ANCHOR_TOML="${WORK_ROOT}/no-anchor.gitleaks.toml"
cat > "$NO_ANCHOR_TOML" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''apps/web/\.next/''']
EOF
append_k "$NO_ANCHOR_TOML"
D_3N="$(mk_struct_dir 3n)"
cp "$NO_ANCHOR_TOML" "${D_3N}/.gitleaks.toml"
run_structure_case "3-n paths の値から '^' が欠落（M1 の回帰。既定値と完全一致しない）" "$D_3N" violate

# --- 3-o: [extend] ブロックごと削除（W-b。既定ルール全滅が緑になる形）------
D_3O="$(mk_struct_dir 3o)"
cat > "${D_3O}/.gitleaks.toml" <<'EOF'
[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3O}/.gitleaks.toml"
run_structure_case "3-o [extend] ブロックごと削除（W-b。2-b 補足）" "$D_3O" violate

# --- 3-p: [extend] は残し useDefault=true の行だけ削除（3-o と同じ実害）----
D_3P="$(mk_struct_dir 3p)"
cat > "${D_3P}/.gitleaks.toml" <<'EOF'
[extend]

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3P}/.gitleaks.toml"
run_structure_case "3-p useDefault = true の行だけ削除（12-2-c 補足）" "$D_3P" violate

# --- 3-q: [[allowlists]] ブロックごと削除（除外範囲の変更は人間の決定事項）-
D_3Q="$(mk_struct_dir 3q)"
cat > "${D_3Q}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true
EOF
append_k "${D_3Q}/.gitleaks.toml"
run_structure_case "3-q [[allowlists]] ブロックごと削除（2-b 補足）" "$D_3Q" violate

# --- 3-r: paths 行に行末コメント（W-a。旧 3-j はこれを合格としていた）------
D_3R="$(mk_struct_dir 3r)"
cat > "${D_3R}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']  # 理由
EOF
append_k "${D_3R}/.gitleaks.toml"
run_structure_case "3-r paths 行に行末コメント（12-2-d-2-a。W-a）" "$D_3R" violate

# --- 3-s: paths の値の前後に空白（trim ではなくバイト列の完全一致であること）
D_3S="$(mk_struct_dir 3s)"
cat > "${D_3S}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = [''' ^apps/web/\.next/''']
EOF
append_k "${D_3S}/.gitleaks.toml"
run_structure_case "3-s paths の値の前後に空白（12-2-d-1-c。trim 後ではなくバイト列一致）" "$D_3S" violate

# --- 3-t: 各行に行頭インデント（記法違いの抜けを塞ぐ。2-e と同じ趣旨）------
#   「正常形の各行」: 2-k の1件の 6 行にもインデントを付ける（3-fx-a の正常形は
#   2-k の1件を含むため）。空行はそのまま。
D_3T="$(mk_struct_dir 3t)"
cat > "${D_3T}/.gitleaks.toml" <<'EOF'
  [extend]
  useDefault = true

  [[allowlists]]
  paths = ['''^apps/web/\.next/''']
EOF
{ printf '\n'; k_block | sed -E 's/^(.+)$/  \1/'; } >> "${D_3T}/.gitleaks.toml"
run_structure_case "3-t 各行に行頭インデント（12-2-h）" "$D_3T" violate

# =============================================================================
# 12-3 の追加ベクタ（2026-08-18。重複キー行 = Info-3 の検証）
#   3-u〜3-y は「不合格」、3-z は「合格」を期待する（12-3 追加ベクタ表・
#   重複キー行の節、前文）。
#
#   本群の fixture は TOML として構文不正（重複キーは TOML 仕様が禁じる）
#   であり、tomllib.loads() と gitleaks は必ずパースに失敗する。この例外は
#   5-d が重複キー行の群に限って認めるものであり、5-e のとおり合否は
#   collect_structure_violations() の単体呼び出し（run_structure_case が
#   内部で行う）の違反件数で測る。「gitleaks が落ちた」「パーサが ERROR を
#   返した」ことを合格の証拠にしない。
#
#   各 fixture が「意図した重複以外の構文誤りを持たないこと」は tomllib と
#   gitleaks の双方で確認済み（tester 工程の作業ログ）。いずれも
#   "key ... is already defined"（3-u/3-v/3-w/3-y）または
#   "key table already exists as a allowlists, but should be an array table"
#   （3-x。ドット付きキーとテーブル見出しの重複）という、意図した重複その
#   ものを指すエラー1種類だけが出ることを確かめた。
#   （2026-10-16: 2-k の1件は末尾に足す。重複の行番号は変わらない。）
# =============================================================================

# --- 3-u: [[allowlists]] 内に paths 2行、両方とも既定値と完全一致 ----------
#   現行検査が素通りする形（実測 0 件）。値の一致で相殺しないことを確かめる
#   （12-2-i-b / i-c）。
D_3U="$(mk_struct_dir 3u)"
cat > "${D_3U}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3U}/.gitleaks.toml"
run_structure_case "3-u [[allowlists]] 内に paths 2行（両方とも既定値。12-2-i-b/i-c）" "$D_3U" violate
# --- 3-u i-g: 違反メッセージに2件目の行番号が入ること（往復7・W-1） --------
#   D_3U を流用（新しい fixture を作らない。12-3-n の趣旨）。2行目の
#   paths は fixture の6行目にある。
run_structure_case_dup_lineno "3-u (12-2-i-g) paths 重複メッセージに2件目の行番号（行6）が入る" "$D_3U" paths 6

# --- 3-v: [[allowlists]] 内に paths 2行、2行目の値だけ既定値と異なる -------
#   現行でも 12-2-d-1 で1件（値の不一致）は捕捉されるが、捕捉の根拠が
#   「重複」ではなく「値の不一致」だけだった。重複としても違反が積まれる
#   こと（違反2件以上）を run_structure_case_min で確かめる（12-2-i-b）。
#   run_structure_case の二値判定（violate = 1件以上）では 1件と2件を
#   区別できず、この対を検証できない。
D_3V="$(mk_struct_dir 3v)"
cat > "${D_3V}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
paths = ['''^apps/web/\.next/|^infra/prod''']
EOF
append_k "${D_3V}/.gitleaks.toml"
run_structure_case_min "3-v [[allowlists]] 内に paths 2行（2行目のみ既定値と不一致。12-2-i-b。違反2件以上）" "$D_3V" 2

# --- 3-w: [extend] 内に useDefault = true を2行 ----------------------------
#   現行検査が素通りする形（実測 0 件）。paths 側だけを直して終わらせない
#   ための対（12-2-i-a / i-b）。
D_3W="$(mk_struct_dir 3w)"
cat > "${D_3W}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3W}/.gitleaks.toml"
run_structure_case "3-w [extend] 内に useDefault = true を2行（12-2-i-a/i-b）" "$D_3W" violate
# --- 3-w i-g: 違反メッセージに2件目の行番号が入ること（往復7・W-1） --------
#   D_3W を流用（新しい fixture を作らない）。2行目の useDefault は
#   fixture の3行目にある。
run_structure_case_dup_lineno "3-w (12-2-i-g) useDefault 重複メッセージに2件目の行番号（行3）が入る" "$D_3W" useDefault 3

# --- 3-x: トップレベル allowlists.paths ＋ [[allowlists]] 内 paths（混在）--
#   記法をまたいで通算すること（合計2件）を確かめる（12-2-i-d）。トップ
#   レベルのキーはテーブル見出しの前にしか書けないため、[[allowlists]] より
#   前に置く。現行検査が素通りする形（実測 0 件）。
D_3X="$(mk_struct_dir 3x)"
cat > "${D_3X}/.gitleaks.toml" <<'EOF'
allowlists.paths = ['''^apps/web/\.next/''']

[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3X}/.gitleaks.toml"
run_structure_case "3-x トップレベル allowlists.paths と [[allowlists]] 内 paths の混在（12-2-i-d）" "$D_3X" violate
# --- 3-x i-g: 記法をまたいでも2件目の行番号が入ること（往復7・W-1） -------
#   D_3X を流用（新しい fixture を作らない）。1件目はトップレベルの
#   ドット付きキー（1行目）、2件目は [[allowlists]] 内の paths（7行目）。
#   記法が違っても2件目として数えた行番号が出ることを確かめる（i-d と
#   i-g の合わせ技）。
run_structure_case_dup_lineno "3-x (12-2-i-g) 記法混在でも2件目の行番号（行7）が入る" "$D_3X" paths 7

# --- 3-y: [[allowlists]] 内に paths 2行、1行目を許可外の形にする -----------
#   許可外の行も出現回数に数えること（「片方を壊せば重複が1件に見える」
#   抜けが無いこと）を確かめる（12-2-i-f）。1行目は既定値と異なる値にする
#   （12-3 追加ベクタ表の例示のうち「値が既定値と異なる」の形を採る）。
#   期待は違反2件以上（重複の1件 + 1行目が許可外の形である1件）。
#   run_structure_case の二値判定では「片方を壊せば重複が1件に見える」抜けを
#   検出できないため run_structure_case_min で検証する。
D_3Y="$(mk_struct_dir 3y)"
cat > "${D_3Y}/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
paths = ['''^apps/web/\.next/|^infra/prod''']
paths = ['''^apps/web/\.next/''']
EOF
append_k "${D_3Y}/.gitleaks.toml"
run_structure_case_min "3-y [[allowlists]] 内に paths 2行（1行目が許可外の形。12-2-i-f。違反2件以上）" "$D_3Y" 2

# --- 3-z: 3-a・3-b・3-j の期待値は「合格」のまま（回帰。12-2-i-h）----------
#   本変更は許可集合を狭めるものであり、既に合格と定めていた形を不合格に
#   しない。新しい fixture は置かず、上で既に実行済みの 3-a / 3-b / 3-j の
#   run_structure_case 呼び出し（いずれも satisfy 期待）がそのまま 3-z の
#   確認を兼ねる（fixture の重複を避ける。12-3-n の経緯と同じ扱い）。

# =============================================================================
# 12-3 の追加ベクタ（2026-10-16。AC-11-11 の1件 = 2-k / 2-l の検証）
#   既存ベクタの期待値は 1 つも変えない（3-a・3-b・3-j は合格、それ以外は不合格）。
#   期待はすべて「不合格」。判定は run_structure_case_vs_baseline（3-a の違反件数を
#   超えること。Green では「1 件以上」と同値）。値は K_SIG_VALUE から実行時に組み立てる
#   （3-fx-b）。3-ba〜3-bd は重複キー行であり 12-5-d の例外（TOML として構文不正）。
#   合否は構造検査の単体呼び出しの違反件数で測る（3-fx-c / 12-5-e）。
#   3-aa は 3-a・3-b の呼び出しが兼ねる（上記）。
# =============================================================================

# 値の変異（3-fx-b。いずれも K_SIG_VALUE から実行時に作る）
SIG_FIRST="${K_SIG_VALUE:0:1}"
if [ "$SIG_FIRST" = "0" ]; then SIG_REPL="1"; else SIG_REPL="0"; fi
SIG_MUT_SAMELEN="${SIG_REPL}${K_SIG_VALUE:1}"       # 1 文字を別の 16 進文字へ（長さを保つ）
SIG_MUT_TRUNC="${K_SIG_VALUE%?}"                    # 末尾 1 文字を削る（部分文字列）
SIG_OTHER64="$(printf 'a%.0s' $(seq 1 64))"         # 別の 16 進 64 文字（3-ap の 2 要素目）

# vec_kb <id> <説明>: 3-a の基準形（write_base）に、環境変数 KB_* で
#   差し替えた 2-k の1件を足した fixture を作り、vs_baseline で検査する。
vec_kb() {
  local id="$1" desc="$2" d
  d="$(mk_struct_dir "$id")"
  write_base "${d}/.gitleaks.toml"
  append_k "${d}/.gitleaks.toml"
  run_structure_case_vs_baseline "${id} ${desc}" "$d"
}

KB_REGEXES="$(k_literal_line regexes "$SIG_MUT_SAMELEN")" \
  vec_kb 3-ab "regexes の値の 1 文字を別の 16 進文字へ（長さを保つ。k-1-c/k-1-d）"
KB_REGEXES="$(k_literal_line regexes "$SIG_MUT_TRUNC")" \
  vec_kb 3-ac "regexes の値の末尾 1 文字を削る（正しい値の部分文字列。k-1-d）"
KB_PATHS="$(k_literal_line paths '^apps/web/src/lib/')" \
  vec_kb 3-ad "[[rules.allowlists]] の paths をディレクトリへ拡大（k-1-b）"
KB_PATHS="$(k_literal_line paths "${K_PATHS_VALUE%\$}")" \
  vec_kb 3-ae "paths の値から末尾の \$ を欠く（k-1-b）"
KB_COND='@DEL@' \
  vec_kb 3-af "condition 行を削除（2-k の不在）"
KB_COND='condition = "OR"' \
  vec_kb 3-ag "condition の値を \"OR\" に（k-1-a）"
KB_COND='condition = "and"' \
  vec_kb 3-ah "condition の値を \"and\"（小文字）に（k-1-a。大文字小文字を同一視しない）"

# --- 3-ai: 2-k の1件をもう1組、内容同一のまま足す --------------------------
D_3AI="$(mk_struct_dir 3ai)"
write_base "${D_3AI}/.gitleaks.toml"
append_k "${D_3AI}/.gitleaks.toml"
append_k "${D_3AI}/.gitleaks.toml"
run_structure_case_vs_baseline "3-ai 2-k の1件をもう1組（内容同一。[[rules]] が2つ。2-b）" "$D_3AI"

# --- 3-aj: 同じ [[rules]] の下に 2本目の [[rules.allowlists]] を内容同一で足す
D_3AJ="$(mk_struct_dir 3aj)"
write_base "${D_3AJ}/.gitleaks.toml"
append_k "${D_3AJ}/.gitleaks.toml"
{
  printf '\n'
  k_emit "$K_RA_HDR"
  k_emit "$K_COND_LINE"
  k_emit "$K_PATHS_LINE"
  k_emit "$K_REGEXES_LINE"
} >> "${D_3AJ}/.gitleaks.toml"
run_structure_case_vs_baseline "3-aj 2本目の [[rules.allowlists]] を内容同一で足す（2-b）" "$D_3AJ"

KB_ID='id = "aws-access-token"' \
  vec_kb 3-ak "id の値を \"aws-access-token\" に（k-1-a）"
KB_ID="${K_ID_LINE}"$'\n'"regex = '''x^'''" \
  vec_kb 3-al "[[rules]] に regex = '''x^''' を足す（2-k。既定ルールの検出を殺す形）"
KB_REGEXES="${K_REGEXES_LINE}"$'\n''regexTarget = "line"' \
  vec_kb 3-am "[[rules.allowlists]] に regexTarget = \"line\" を足す（2-k。照合の対象を広げる形）"

# --- 3-an: 2-k の1件ごと削除（2026-10-16 改定前の 3-a の形）----------------
D_3AN="$(mk_struct_dir 3an)"
write_base "${D_3AN}/.gitleaks.toml"
run_structure_case_vs_baseline "3-an 2-k の1件ごと削除（改定前の 3-a の形。2-b の不在）" "$D_3AN"

# --- 3-ao: [[rules.allowlists]] ブロックだけ削除（[[rules]] の id は残す）----
D_3AO="$(mk_struct_dir 3ao)"
write_base "${D_3AO}/.gitleaks.toml"
KB_RA_HDR='@DEL@' KB_COND='@DEL@' KB_PATHS='@DEL@' KB_REGEXES='@DEL@' append_k "${D_3AO}/.gitleaks.toml"
run_structure_case_vs_baseline "3-ao [[rules.allowlists]] ブロックだけ削除（2-b の不在）" "$D_3AO"

# --- 2-b のテーブル本数の判定を、キーの出現回数の検査から切り離して測るベクタ
#   【spec に番号が無い。2-b（テーブルは各ちょうど1つ。多いことも少ないことも
#   違反）と 2-l（キーの数え方）から導いた。2026-10-16 人間の決定】
#   3-ai / 3-aj / 3-an / 3-ao はキーを伴うため、本数の判定を潰してもキーの
#   出現回数（id / condition / paths / regexes）の検査が同じ fixture を捕捉し、
#   本数の判定の判別力を測れない。以下はキーの出現回数を 1 のまま保つ形。
#   - 2b-empty-rules : 3-a の末尾にキーを持たない空の 2 本目の [[rules]]（本数 > 1）
#   - 2b-empty-ra    : 3-a の末尾にキーを持たない空の 2 本目の [[rules.allowlists]]（本数 > 1）
#   - 2b-nohdr-rules : [[rules]] の見出しだけを欠く（id 行は残る。2-b の不在）
#   - 2b-nohdr-ra    : [[rules.allowlists]] の見出しだけを欠く（3 キーは残る。2-b の不在）
D_2BER="$(mk_struct_dir 2b-empty-rules)"
write_base "${D_2BER}/.gitleaks.toml"
append_k "${D_2BER}/.gitleaks.toml"
{ printf '\n'; k_emit "$K_RULES_HDR"; } >> "${D_2BER}/.gitleaks.toml"
run_structure_case_vs_baseline "2b-empty-rules 2-b キーを持たない空の 2 本目の [[rules]]（2-b / 2-l。キー出現回数は 1 のまま）" "$D_2BER"

D_2BEA="$(mk_struct_dir 2b-empty-ra)"
write_base "${D_2BEA}/.gitleaks.toml"
append_k "${D_2BEA}/.gitleaks.toml"
{ printf '\n'; k_emit "$K_RA_HDR"; } >> "${D_2BEA}/.gitleaks.toml"
run_structure_case_vs_baseline "2b-empty-ra 2-b キーを持たない空の 2 本目の [[rules.allowlists]]（2-b / 2-l。キー出現回数は 1 のまま）" "$D_2BEA"

KB_RULES_HDR='@DEL@' \
  vec_kb 2b-nohdr-rules "[[rules]] の見出しだけを欠く（id 行は残る。2-b の不在）"
KB_RA_HDR='@DEL@' \
  vec_kb 2b-nohdr-ra "[[rules.allowlists]] の見出しだけを欠く（3 キーは残る。2-b の不在）"

KB_REGEXES="regexes = ['''${K_SIG_VALUE}''', '''${SIG_OTHER64}''']" \
  vec_kb 3-ap "regexes を要素 2 個に（正しい値と別の 16 進 64 文字。k-1-c）"
KB_PATHS="paths = ['''${K_PATHS_VALUE}''', '''^apps/web/src/lib/x\\.ts\$''']" \
  vec_kb 3-aq "[[rules.allowlists]] の paths を要素 2 個に（k-1-b）"
KB_REGEXES="${K_REGEXES_LINE}  # 理由" \
  vec_kb 3-ar "regexes 行に行末コメント（k-1-e）"
KB_ID="${K_ID_LINE}  # 理由" \
  vec_kb 3-as "id 行に行末コメント（k-1-e）"
KB_REGEXES="$(k_literal_line regexes " ${K_SIG_VALUE} ")" \
  vec_kb 3-at "regexes の値の前後に空白（k-1-d。trim しない）"
KB_PATHS="$(k_literal_line paths " ${K_PATHS_VALUE} ")" \
  vec_kb 3-au "[[rules.allowlists]] の paths の値の前後に空白（k-1-d）"

# --- 3-av: 2-k の 6 行に行頭のインデント（他の行はそのまま）-----------------
D_3AV="$(mk_struct_dir 3av)"
write_base "${D_3AV}/.gitleaks.toml"
{ printf '\n'; k_block | sed -E 's/^(.+)$/  \1/'; } >> "${D_3AV}/.gitleaks.toml"
run_structure_case_vs_baseline "3-av 2-k の6行に行頭インデント（k-1-e / 2-h）" "$D_3AV"

# --- 3-aw: [[rules.allowlists]] 見出しを置かず、[[rules]] 内のドット付きキー ---
D_3AW="$(mk_struct_dir 3aw)"
write_base "${D_3AW}/.gitleaks.toml"
{
  printf '\n'
  k_emit "$K_RULES_HDR"
  k_emit "$K_ID_LINE"
  printf 'allowlists.%s\n' "$K_COND_LINE"
  printf 'allowlists.%s\n' "$K_PATHS_LINE"
  printf 'allowlists.%s\n' "$K_REGEXES_LINE"
} >> "${D_3AW}/.gitleaks.toml"
run_structure_case_vs_baseline "3-aw [[rules]] 内に allowlists.* のドット付きキーで同じ値（2-e）" "$D_3AW"

# --- 3-ax: [[rules]] 内の allowlists = [{ … }] インライン表 ------------------
D_3AX="$(mk_struct_dir 3ax)"
write_base "${D_3AX}/.gitleaks.toml"
{
  printf '\n'
  k_emit "$K_RULES_HDR"
  k_emit "$K_ID_LINE"
  printf "allowlists = [{ condition = \"AND\", paths = ['''%s'''], regexes = ['''%s'''] }]\n" "$K_PATHS_VALUE" "$K_SIG_VALUE"
} >> "${D_3AX}/.gitleaks.toml"
run_structure_case_vs_baseline "3-ax [[rules]] 内に allowlists = [{…}] のインライン表で同じ値（2-e）" "$D_3AX"

# --- 3-ay: トップレベルの rules = [{ id = …, allowlists = [{ … }] }] ----------
#   トップレベルのキーはテーブル見出しの前に置く。[[rules]] / [[rules.allowlists]] は置かない。
D_3AY="$(mk_struct_dir 3ay)"
printf "rules = [{ id = \"generic-api-key\", allowlists = [{ condition = \"AND\", paths = ['''%s'''], regexes = ['''%s'''] }] }]\n\n" "$K_PATHS_VALUE" "$K_SIG_VALUE" > "${D_3AY}/.gitleaks.toml"
write_base "${D_3AY}/base.tmp"
cat "${D_3AY}/base.tmp" >> "${D_3AY}/.gitleaks.toml"
rm -f "${D_3AY}/base.tmp"
run_structure_case_vs_baseline "3-ay トップレベルの rules = [{…}] のインライン表で同じ値（2-e）" "$D_3AY"

# --- 3-az: paths と regexes を基本文字列（"…"）で、解釈後の値が正しい値と一致するように
#   paths の `\\.` は TOML の基本文字列では `\.` に解釈される（解釈後の値は K_PATHS_VALUE と一致）。
IFS= read -r PATHS_BASIC_LINE <<'EOF'
paths = ["^apps/web/src/lib/lambda-client\\.test\\.ts$"]
EOF
KB_PATHS="$PATHS_BASIC_LINE" KB_REGEXES="regexes = [\"${K_SIG_VALUE}\"]" \
  vec_kb 3-az "paths と regexes を基本文字列（\"…\"）で書く（解釈後の値は一致。k-1-b / k-1-c）"

# --- 3-ba〜3-bd: 重複キー行（12-5-d の例外。同一の値を 2 行）-----------------
KB_ID="${K_ID_LINE}"$'\n'"${K_ID_LINE}" \
  vec_kb 3-ba "[[rules]] 内に id 行を 2 行（同一の値。2-l。重複キー行）"
KB_COND="${K_COND_LINE}"$'\n'"${K_COND_LINE}" \
  vec_kb 3-bb "[[rules.allowlists]] 内に condition 行を 2 行（同一の値。2-l）"
KB_PATHS="${K_PATHS_LINE}"$'\n'"${K_PATHS_LINE}" \
  vec_kb 3-bc "[[rules.allowlists]] 内に paths 行を 2 行（同一の値。2-l）"
KB_REGEXES="${K_REGEXES_LINE}"$'\n'"${K_REGEXES_LINE}" \
  vec_kb 3-bd "[[rules.allowlists]] 内に regexes 行を 2 行（同一の値。2-l）"

echo ""

# =============================================================================
# 12-4: `paths` の正規表現に先頭アンカーがあること（位置ずれベクタ）
#
#   11-4 の表は変更しない（4-b）。11-4 のどのパスも位置ずれを含まないため、
#   `^` を削っても 11-4 は全件通ってしまう（= 素通りの原因）。12-4 は
#   `vendor/apps/web/.next/...` のような「末尾にプレフィックスとして
#   一致してしまう」位置ずれのケースを別表として追加する。
# =============================================================================

VECTOR4_PATHS=(
  "vendor/apps/web/.next/prerender-manifest.json"
  "x/apps/web/.next/a.json"
  "../apps/web/.next/a.json"
  "apps/web/.next/prerender-manifest.json"
)
VECTOR4_EXPECT=(
  nomatch nomatch nomatch match
)

# collect_anchor_violations <toml_file>
#   extract_paths_regex（Part A で定義済み）を再利用し、12-4 の位置ずれ
#   ベクタと突き合わせる。
collect_anchor_violations() {
  local toml="$1"
  local -a violations=()
  local regex rc
  regex="$(extract_paths_regex "$toml")"
  rc=$?
  if [ "$rc" -ne 0 ] || [ -z "$regex" ]; then
    violations+=("paths の正規表現を ${toml} から抽出できない")
    VIOLATIONS=("${violations[@]}")
    return
  fi
  local i path expect got
  for i in "${!VECTOR4_PATHS[@]}"; do
    path="${VECTOR4_PATHS[$i]}"
    expect="${VECTOR4_EXPECT[$i]}"
    if printf '%s' "$path" | grep -Eq -- "$regex"; then
      got=match
    else
      got=nomatch
    fi
    if [ "$got" != "$expect" ]; then
      violations+=("12-4 位置ずれベクタ不一致: ${path} は期待=${expect} 実際=${got}（regex=${regex}）")
    fi
  done
  VIOLATIONS=("${violations[@]}")
}

# run_anchor_case <名前> <toml_file> <want:satisfy|violate>
run_anchor_case() {
  local name="$1" toml="$2" want="$3"
  local -a VIOLATIONS=()
  collect_anchor_violations "$toml"
  local ok=1 joined
  joined="$(printf '%s\n' "${VIOLATIONS[@]+"${VIOLATIONS[@]}"}")"

  if [ "$want" = "satisfy" ]; then
    [ "${#VIOLATIONS[@]}" -eq 0 ] || ok=0
  else
    [ "${#VIOLATIONS[@]}" -gt 0 ] || ok=0
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

# --- 4-c/4-d 対: 正常形（アンカー済み）は位置ずれベクタも全て満たす --------
run_anchor_case "12-4 正常形（^ アンカー済み）は位置ずれベクタを満たす" "$GOOD_TOML" satisfy

# --- 4-c/4-d 対: `^` を削った fixture は位置ずれベクタで不合格になる -------
#   実体は 3-n（Part C）で定義済みの NO_ANCHOR_TOML をそのまま再利用する
#   （12-3 追加ベクタ表・3-n の行が要求する「同一のものであり、両者を
#   別々に用意しない」）。
BAD_NO_ANCHOR_TOML="$NO_ANCHOR_TOML"
run_anchor_case "12-4 異常系: '^' を削ると位置ずれベクタ（vendor/... 等）に誤って一致する" "$BAD_NO_ANCHOR_TOML" violate

# --- 実リポジトリの .gitleaks.toml（AC-12-4 の本番適用） -------------------
# Part A（11-4, 216行目）が実ファイルに適用しているのと同じ理由。
# collect_anchor_violations を fixture だけに適用していると、本番の `.gitleaks.toml`
# から `^` が抜けても検査は緑のままになる（4-a/4-b が求める検査が回帰的に本番へ
# 効いていることを、正常形・`^` 削除 fixture との対で確かめる意図に合わせる）。
run_anchor_case "実リポジトリの .gitleaks.toml（AC-12-4 の本番適用）" "$REAL_TOML" satisfy
echo ""

# =============================================================================
# Part D: AC-11-11 — .next の外の 1 件（2-j の期待値の Signature）の除外
#
#   11-11-d: 実際の .gitleaks.toml を使った実測の組（11-6-a と同じ設計）。
#   11-11-g: パスの正規表現の意味のベクタ（11-4 と同じ位置づけ。Part A 参照）。
#   11-11-f: 一時ディレクトリで完結し、新しい make ターゲットを増やさない。
#   値は K_SIG_VALUE（Part C の「写し」）から実行時に組み立てる（11-11-c）。
#   「別の値」は実行時に生成し、実体を残さない（11-11-e）。
# =============================================================================

echo "==> test-gitleaks-allowlist (Part D: AC-11-11 の1件)"

# 2-j と同じ文脈（`Signature=` に続けて値を置く行）。lambda-client.test.ts の
# 2-j の行と同じ形（Authorization の文字列の一部）で、値だけを差し替える。
SIG_CTX_PREFIX='      "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature='
SIG_CTX_SUFFIX='",'
K_TEST_RELPATH="apps/web/src/lib/lambda-client.test.ts"
K_OTHER_RELPATH="apps/web/src/lib/other.test.ts"

# run_sig_case <名前> <toml（空=config 無し）> <relpath> <value> <want:1=検出される/0=検出されない> [rule_id]
run_sig_case() {
  local name="$1" toml="$2" relpath="$3" value="$4" want="$5" rule="${6:-}"
  local work ec report log ok=1
  work="$(mktemp -d -p "$WORK_ROOT")"
  report="${work}.report.json"
  log="${work}.out.log"
  mkdir -p "$(dirname "${work}/${relpath}")"
  printf '%s%s%s\n' "$SIG_CTX_PREFIX" "$value" "$SIG_CTX_SUFFIX" > "${work}/${relpath}"
  if [ -n "$toml" ]; then cp "$toml" "${work}/.gitleaks.toml"; fi
  ( cd "$work" && gitleaks dir . --no-banner --redact --report-format json --report-path "$report" ) > "$log" 2>&1
  ec=$?

  if [ "$want" = "1" ]; then
    [ "$ec" -ne 0 ] || ok=0
    if [ -n "$rule" ]; then
      grep -qF "\"RuleID\": \"${rule}\"" "$report" 2>/dev/null || ok=0
    fi
  else
    [ "$ec" -eq 0 ] || ok=0
  fi

  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s（EXIT=%s）\n' "$name" "$ec"
  else
    fail=$((fail + 1))
    local want_desc
    if [ "$want" = "1" ]; then want_desc="!=0（検出される${rule:+, ルール ${rule}}）"; else want_desc="=0（検出されない）"; fi
    printf '  FAIL %s（期待 EXIT %s, 実際 EXIT=%s）\n' "$name" "$want_desc" "$ec"
    sed 's/^/       /' "$log"
    if [ -f "$report" ]; then grep -E '"RuleID"|"File"' "$report" | sed 's/^/       /'; fi
  fi
}

# 11-11-e: 「別の値」は、generic-api-key に検知されることを先に実測して選ぶ
#   （config 無し・同じ文脈・当該ファイルで検知され、ルール ID が generic-api-key）。
#   実体は実行時に組み立てる。20 回試して 1 つも検知されなければ OTHER_SIG は空にし、
#   対照を FAIL にする（11-6-b-1 と同じ設計）。
OTHER_SIG=""
for _try in $(seq 1 20); do
  cand="$(LC_ALL=C tr -dc '0-9a-f' < /dev/urandom | head -c64)"
  [ "$cand" != "$K_SIG_VALUE" ] || continue
  cwork="$(mktemp -d -p "$WORK_ROOT")"
  mkdir -p "$(dirname "${cwork}/${K_TEST_RELPATH}")"
  printf '%s%s%s\n' "$SIG_CTX_PREFIX" "$cand" "$SIG_CTX_SUFFIX" > "${cwork}/${K_TEST_RELPATH}"
  if ( cd "$cwork" && gitleaks dir . --no-banner --redact --report-format json --report-path "${cwork}.report.json" ) >/dev/null 2>&1; then
    :
  elif grep -qF '"RuleID": "generic-api-key"' "${cwork}.report.json" 2>/dev/null; then
    OTHER_SIG="$cand"
    break
  fi
done
if [ -n "$OTHER_SIG" ]; then
  pass=$((pass + 1))
  echo "  ok   control: 別の値は config 無しで generic-api-key に検知される（11-11-e。値は実行時に生成）"
else
  fail=$((fail + 1))
  echo "  FAIL control: 別の値が generic-api-key に検知されなかった（20 回試行。11-11-e）"
fi

# --- control: config 無しでは当該の値も検知される（値が除外の必要な形であることの裏付け）
run_sig_case "control: config 無しで当該の値は検出される（generic-api-key）" "" "$K_TEST_RELPATH" "$K_SIG_VALUE" 1 generic-api-key

# --- 11-11-d-1: 当該ファイル・当該の値 → 検出されない -----------------------
run_sig_case "11-11-d-1 実 .gitleaks.toml: lambda-client.test.ts の当該の値は検出されない" "$REAL_TOML" "$K_TEST_RELPATH" "$K_SIG_VALUE" 0

# --- 11-11-d-2: 当該ファイル以外・当該の値 → 検出される（generic-api-key）----
run_sig_case "11-11-d-2 実 .gitleaks.toml: 当該ファイル以外（other.test.ts）の同じ行は検出される" "$REAL_TOML" "$K_OTHER_RELPATH" "$K_SIG_VALUE" 1 generic-api-key

# --- 11-11-d-3: 当該ファイル・別の値 → 検出される（generic-api-key）---------
if [ -n "$OTHER_SIG" ]; then
  run_sig_case "11-11-d-3 実 .gitleaks.toml: lambda-client.test.ts の別の値は検出される" "$REAL_TOML" "$K_TEST_RELPATH" "$OTHER_SIG" 1 generic-api-key
else
  fail=$((fail + 1))
  echo "  FAIL 11-11-d-3 実行不能（別の値を選べていない。上の control を参照）"
fi

# --- 対照: 2-k の1件を持たない設定（3-an の形）では d-1 の行は検出される ----
#   d-1 の「検出されない」が、2-k の1件の効果であって、そもそも検出されない
#   構成のせいではないことを示す（11-6-a と同じ「対で確かめる」設計）。
run_sig_case "対照: 2-k の1件を持たない設定では d-1 の行は検出される（d-1 は 2-k の効果）" "${D_3AN}/.gitleaks.toml" "$K_TEST_RELPATH" "$K_SIG_VALUE" 1 generic-api-key

echo ""

# =============================================================================
# 11-11-g: パスの正規表現の意味のベクタ（[[rules.allowlists]] の paths）
#   Part A（11-4）と同じ位置づけ。完全一致（12-2-k-1）は値が 1 つに固定されて
#   いることしか見ず、その 1 つが当該ファイルだけを表すかを見ない（12-L6 と同じ）。
# =============================================================================

VECTOR11G_PATHS=(
  "apps/web/src/lib/lambda-client.test.ts"
  "apps/web/src/lib/lambda-client.ts"
  "apps/web/src/lib/lambda-client.test.tsx"
  "apps/web/src/lib/lambda-client.test.ts.bak"
  "apps/web/src/lib/lambda-clientXtestXts"
  "vendor/apps/web/src/lib/lambda-client.test.ts"
  "apps/web/src/lib/other.test.ts"
)
VECTOR11G_EXPECT=(
  match
  nomatch nomatch nomatch nomatch nomatch nomatch
)

# extract_rules_paths_regex <toml>: [[rules.allowlists]] 見出しより後の最初の paths 行から
#   正規表現を取り出す（Part A の extract_paths_regex を、見出し以降だけに適用する）。
extract_rules_paths_regex() {
  local toml="$1" tail_file
  [ -f "$toml" ] || return 1
  tail_file="$(mktemp -p "$WORK_ROOT")"
  awk '/^\[\[rules\.allowlists\]\]/ { f = 1; next } f' "$toml" > "$tail_file"
  extract_paths_regex "$tail_file"
}

# run_k_path_case <名前> <toml_file> <want:satisfy|violate> [違反に含まれるべき部分文字列...]
run_k_path_case() {
  local name="$1" toml="$2" want="$3"; shift 3
  local -a violations=()
  local regex rc i path expect got
  regex="$(extract_rules_paths_regex "$toml")"
  rc=$?
  if [ "$rc" -ne 0 ] || [ -z "$regex" ]; then
    violations+=("[[rules.allowlists]] の paths の正規表現を ${toml} から抽出できない")
  else
    for i in "${!VECTOR11G_PATHS[@]}"; do
      path="${VECTOR11G_PATHS[$i]}"
      expect="${VECTOR11G_EXPECT[$i]}"
      if printf '%s' "$path" | grep -Eq -- "$regex"; then got=match; else got=nomatch; fi
      if [ "$got" != "$expect" ]; then
        violations+=("11-11-g テストベクタ不一致: ${path} は期待=${expect} 実際=${got}（regex=${regex}）")
      fi
    done
  fi

  local ok=1 joined
  joined="$(printf '%s\n' "${violations[@]+"${violations[@]}"}")"
  if [ "$want" = "satisfy" ]; then
    [ "${#violations[@]}" -eq 0 ] || ok=0
  else
    [ "${#violations[@]}" -gt 0 ] || ok=0
    local sub
    for sub in "$@"; do
      grep -qF -- "$sub" <<< "$joined" || ok=0
    done
  fi

  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s（違反 %s 件）\n' "$name" "${#violations[@]}"
  else
    fail=$((fail + 1))
    printf '  FAIL %s（期待=%s, 実際の違反 %s 件）\n' "$name" "$want" "${#violations[@]}"
    local v
    for v in "${violations[@]+"${violations[@]}"}"; do
      printf '       - %s\n' "$v"
    done
  fi
}

echo "==> test-gitleaks-allowlist (11-11-g: [[rules.allowlists]] の paths のパス判定)"

run_k_path_case "11-11-g 正常系: 2-k の paths（正しい形）は7本のベクタを満たす" "${D_3A}/.gitleaks.toml" satisfy

# 陽性対照（ベクタが判別力を持つこと。Part A の「'.' をエスケープしない」と同じ設計）
G_NO_DOLLAR="$(mk_struct_dir 11g-nodollar)"
write_base "${G_NO_DOLLAR}/.gitleaks.toml"
KB_PATHS="$(k_literal_line paths "${K_PATHS_VALUE%\$}")" append_k "${G_NO_DOLLAR}/.gitleaks.toml"
run_k_path_case "11-11-g 異常系: 末尾の \$ が無いと .test.tsx / .test.ts.bak を巻き込む" "${G_NO_DOLLAR}/.gitleaks.toml" violate \
  "lambda-client.test.tsx" "lambda-client.test.ts.bak"

G_NO_CARET="$(mk_struct_dir 11g-nocaret)"
write_base "${G_NO_CARET}/.gitleaks.toml"
KB_PATHS="$(k_literal_line paths "${K_PATHS_VALUE#^}")" append_k "${G_NO_CARET}/.gitleaks.toml"
run_k_path_case "11-11-g 異常系: 先頭の ^ が無いと vendor/... を巻き込む" "${G_NO_CARET}/.gitleaks.toml" violate \
  "vendor/apps/web/src/lib/lambda-client.test.ts"

G_UNESCAPED="$(mk_struct_dir 11g-unescaped)"
write_base "${G_UNESCAPED}/.gitleaks.toml"
KB_PATHS="$(k_literal_line paths '^apps/web/src/lib/lambda-client.test.ts$')" append_k "${G_UNESCAPED}/.gitleaks.toml"
run_k_path_case "11-11-g 異常系: '.' をエスケープしないと lambda-clientXtestXts を巻き込む" "${G_UNESCAPED}/.gitleaks.toml" violate \
  "lambda-clientXtestXts"

run_k_path_case "11-11-g 2-k の1件が無い設定では paths を抽出できない" "${D_3AN}/.gitleaks.toml" violate "抽出できない"

run_k_path_case "実リポジトリの .gitleaks.toml（11-11-g の本番適用）" "$REAL_TOML" satisfy


echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
