#!/usr/bin/env bash
# check-exclusion-axis.sh の fixture テスト（Issue #174）。
#
# 仕様: docs/specs/process-check-exclusion-axis.md
#   AC-1 を AC-6-5 の読み替えで、AC-2・AC-3・AC-5 の全行、AC-4、AC-6-4、
#   AC-6-7 の期待値の全行、AC-7-1 の理由の文を検査する。
#   PR 本文の検査（OK/NG）と、yml が結果を振り分けること（6-6 の後段）は
#   fixture に入らない（AC-8-6）。
#
# 入出力の形は AC-6-7 の条文のとおり:
#   呼び出し: bash .github/scripts/check-exclusion-axis.sh <検査の名前>
#   入力: PR_LABELS（1行1ラベル）、CHANGED_FILE_LIST（一覧を書いたファイルのパス）
#   出力: SKIP / CHECK（終了 0）、失敗（終了 1・出力なし）、判定できない（終了 2・出力なし）
#   2つのパスはカレントディレクトリからの相対パスで実在を確かめる。
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TARGET="${ROOT}/.github/scripts/check-exclusion-axis.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

report() { # <名前> <ok:0|1> [詳細]
  if [ "$2" = 1 ]; then
    pass=$((pass + 1))
    echo "PASS: $1"
  else
    fail=$((fail + 1))
    echo "FAIL: $1"
    [ -n "${3:-}" ] && printf '%s\n' "$3" | sed 's/^/      /'
  fi
}

# run_case <id> <期待: SKIP|CHECK|FAIL|UNDET> <PR_LABELS: UNSET|文字列> \
#          <一覧: UNSET|EMPTYSTR|MISSING|DIR|EMPTYFILE|内容(printf %b)> <2つのパス: BOTH|NONE|NO_SCRIPT|NO_YML> [引数...]
# 結果の標準エラー出力は LAST_ERR に残す。
LAST_ERR=""
run_case() {
  local id="$1" want="$2" labels="$3" list="$4" paths="$5"; shift 5
  local d="${WORK}/case-${id//\//_}"
  rm -rf "$d"; mkdir -p "$d/.github/scripts" "$d/.github/workflows"
  case "$paths" in
    BOTH|NO_YML) : > "$d/.github/scripts/check-review-trail.sh" ;;
  esac
  case "$paths" in
    BOTH|NO_SCRIPT) : > "$d/.github/workflows/review-trail.yml" ;;
  esac
  local listpath
  case "$list" in
    UNSET)     listpath="" ;;
    EMPTYSTR)  listpath="" ;;
    MISSING)   listpath="$d/no-such-list.txt" ;;
    DIR)       mkdir -p "$d/listdir"; listpath="$d/listdir" ;;
    EMPTYFILE) : > "$d/list.txt"; listpath="$d/list.txt" ;;
    *)         printf '%b' "$list" > "$d/list.txt"; listpath="$d/list.txt" ;;
  esac

  local -a envs=()
  [ "$labels" = UNSET ] || envs+=("PR_LABELS=$labels")
  [ "$list" = UNSET ] || envs+=("CHANGED_FILE_LIST=$listpath")

  local out rc
  out="$(cd "$d" && env -u PR_LABELS -u CHANGED_FILE_LIST "${envs[@]}" bash "$TARGET" "$@" 2> "$d/stderr")"
  rc=$?
  LAST_ERR="$(cat "$d/stderr")"

  local want_rc want_out ok=1
  case "$want" in
    SKIP)  want_rc=0; want_out="SKIP" ;;
    CHECK) want_rc=0; want_out="CHECK" ;;
    FAIL)  want_rc=1; want_out="" ;;
    UNDET) want_rc=2; want_out="" ;;
  esac
  [ "$rc" = "$want_rc" ] || ok=0
  [ "$out" = "$want_out" ] || ok=0
  report "$id: $want (引数: $*)" "$ok" "want rc=$want_rc out='$want_out' / got rc=$rc out='$out' err='${LAST_ERR}'"
}

# --- スクリプトの存在 -------------------------------------------------------
if [ -f "$TARGET" ]; then
  report "check-exclusion-axis.sh が存在する" 1
else
  report "check-exclusion-axis.sh が存在する" 0 "${TARGET} が無い"
fi

# --- AC-1 / AC-4 / AC-6-5: 除外軸の一致（両方の検査名で） ---------------------
# 形式: id|期待(CHECK=一致する,SKIP=一致しない)|一覧(printf %b)
axis_cases=(
  '1-1|CHECK|infra/terraform/iam_github_oidc.tf\n'
  '1-2|CHECK|infra/bootstrap/versions.tf\n'
  '1-3|CHECK|services/api/go.mod\n'
  '1-4|CHECK|apps/web/package.json\n'
  '1-5|SKIP|docs/specs/process-check-exclusion-axis.md\n'
  '1-6|SKIP|docs/infra/notes.md\n'
  '1-7|SKIP|infra-legacy/main.tf\n'
  '1-8|SKIP|infrastructure/main.tf\n'
  '1-9|SKIP|infra\n'
  '1-10|SKIP|Infra/main.tf\n'
  '1-11|SKIP|.github/workflows/ci.yml\n'
  '1-12|CHECK|docs/adr/0025-reference-model-with-infra-in-exclusion-axis.md\ninfra/terraform/budgets.tf\n'
  '1-13|CHECK|services/api/go.mod\napps/web/package.json\ninfra/terraform/locals.tf\n'
  '1-14|SKIP|docs/infra/notes.md\ninfra-legacy/main.tf\n'
  '1-15|SKIP|EMPTYFILE'
)
for c in "${axis_cases[@]}"; do
  IFS='|' read -r id want list <<< "$c"
  # 一覧に改行（\n）を含む行は read の区切りに影響しない（printf %b で展開する）。
  list="${c#*|*|}"
  if [ "$list" = EMPTYFILE ]; then lst=EMPTYFILE; else lst="$list"; fi
  for name in review-trail spec-link; do
    run_case "AC-6-5/AC-4-1 ${id} [${name}]" "$want" "" "$lst" BOTH "$name"
  done
done

# --- AC-2: review-trail（ラベル無し） ---------------------------------------
run_case "AC-2 2-1" CHECK "" 'infra/terraform/iam_github_oidc.tf\n' BOTH review-trail
run_case "AC-2 2-2" SKIP  "" 'docs/specs/process-check-exclusion-axis.md\n' BOTH review-trail
run_case "AC-2 2-3" SKIP  "" 'docs/infra/notes.md\ninfra-legacy/main.tf\n' BOTH review-trail
run_case "AC-2 2-4" CHECK "" '.github/scripts/check-review-trail.sh\n' BOTH review-trail
run_case "AC-2 2-5" CHECK "" '.github/workflows/review-trail.yml\ndocs/specs/process-check-exclusion-axis.md\n' BOTH review-trail
run_case "AC-2 2-6" SKIP  "" '.github/workflows/spec-link.yml\n' BOTH review-trail
run_case "AC-2 2-7" CHECK "" 'services/api/go.mod\n' BOTH review-trail
run_case "AC-2 2-8" FAIL  "" 'infra/terraform/budgets.tf\n' NO_SCRIPT review-trail
run_case "AC-2 2-9" CHECK "" '.github/scripts/check-exclusion-axis.sh\ndocs/specs/process-check-exclusion-axis.md\n' BOTH review-trail
run_case "AC-2 2-10" FAIL "" 'infra/terraform/budgets.tf\n' NO_YML review-trail

# --- AC-3: spec-link（ラベル無し。スクリプトの出力列） ------------------------
run_case "AC-3 3-1/3-2" CHECK "" 'infra/terraform/iam_github_oidc.tf\n' BOTH spec-link
run_case "AC-3 3-3" SKIP  "" 'docs/infra/notes.md\n' BOTH spec-link
run_case "AC-3 3-4" SKIP  "" 'infra-legacy/main.tf\n' BOTH spec-link
run_case "AC-3 3-5" CHECK "" 'services/api/go.mod\n' BOTH spec-link
run_case "AC-3 3-6" SKIP  "" '.github/workflows/spec-link.yml\n' BOTH spec-link
run_case "AC-3 3-7" SKIP  "" '.github/scripts/check-exclusion-axis.sh\n' BOTH spec-link

# --- AC-5: no-spec と infra/ ------------------------------------------------
INFRA='infra/terraform/iam_github_oidc.tf\n'
run_case "AC-5 5-1" SKIP  "no-spec" "$INFRA" BOTH review-trail
run_case "AC-5 5-3" SKIP  "no-spec" "$INFRA" BOTH spec-link
run_case "AC-5 5-4" SKIP  "no-spec" 'services/api/go.mod\n' BOTH review-trail
run_case "AC-5 5-5" SKIP  "no-spec" '.github/scripts/check-review-trail.sh\n' BOTH review-trail
run_case "AC-5 5-6" SKIP  "loop-extended" 'docs/specs/process-check-exclusion-axis.md\n' BOTH review-trail
run_case "AC-5 5-7" CHECK $'no-spec-draft\ndraft-no-spec' "$INFRA" BOTH review-trail
run_case "AC-5 5-8" CHECK $'no-spec-draft\ndraft-no-spec' "$INFRA" BOTH spec-link
run_case "AC-5 5-9" SKIP  $'loop-extended\nno-spec' "$INFRA" BOTH review-trail
run_case "AC-5 5-10" SKIP "no-spec" "$INFRA" NO_SCRIPT review-trail

# --- AC-6-7 の期待値 --------------------------------------------------------
DEF='infra/terraform/main.tf\n'
run_case "AC-6-7 6-7-1"  UNDET "" "$DEF" BOTH
run_case "AC-6-7 6-7-2"  UNDET "" "$DEF" BOTH ci
run_case "AC-6-7 6-7-3"  UNDET "" "$DEF" BOTH Spec-link
run_case "AC-6-7 6-7-4"  UNDET "" "$DEF" BOTH spec-link review-trail
run_case "AC-6-7 6-7-5"  UNDET UNSET "$DEF" BOTH review-trail
run_case "AC-6-7 6-7-6"  UNDET UNSET "$DEF" BOTH spec-link
run_case "AC-6-7 6-7-7"  UNDET "" UNSET BOTH review-trail
run_case "AC-6-7 6-7-8"  UNDET "" EMPTYSTR BOTH spec-link
run_case "AC-6-7 6-7-9"  UNDET "" MISSING BOTH spec-link
run_case "AC-6-7 6-7-10" UNDET "" DIR BOTH review-trail
run_case "AC-6-7 6-7-11" SKIP  "" EMPTYFILE BOTH review-trail
run_case "AC-6-7 6-7-12" SKIP  "" EMPTYFILE BOTH spec-link
run_case "AC-6-7 6-7-13" SKIP  "no-spec" MISSING BOTH review-trail
run_case "AC-6-7 6-7-14" FAIL  "" MISSING NO_SCRIPT review-trail
run_case "AC-6-7 6-7-15" CHECK "" 'infra/terraform/main.tf' BOTH spec-link
run_case "AC-6-7 6-7-16" CHECK "" "$DEF" NONE spec-link

# --- AC-7-1: SKIP の理由の文（標準エラー出力）が3つの範囲を挙げる ------------------
for name in review-trail spec-link; do
  run_case "AC-7-1 7-1 stdout [${name}]" SKIP "" 'docs/specs/process-check-exclusion-axis.md\n' BOTH "$name"
  ok=1
  for w in 'services/' 'apps/' 'infra/'; do
    case "$LAST_ERR" in *"$w"*) ;; *) ok=0 ;; esac
  done
  report "AC-7-1 7-1 理由の文が services/ apps/ infra/ を含む [${name}]" "$ok" "stderr='${LAST_ERR}'"
done

# --- AC-6-4: 2つの yml はスクリプトを呼び、自前の除外軸の判定を持たない ------------
# コメント行を除いた行だけを読む。除外軸の判定 = services/apps/infra のうち
# 2つ以上を「|」で並べた正規表現。
strip_comments() { sed -E 's/^[[:space:]]*#.*$//' "$1"; }
calls_script() { strip_comments "$1" | command grep -c 'check-exclusion-axis\.sh'; }
own_axis() {
  strip_comments "$1" | command grep -cE '(services|apps|infra)[|]|[|](services|apps|infra)'
}

for y in spec-link review-trail; do
  f="${ROOT}/.github/workflows/${y}.yml"
  n="$(calls_script "$f")"
  ok=0; [ "$n" -ge 1 ] && ok=1
  report "AC-6-4 ${y}.yml が check-exclusion-axis.sh を呼ぶ（${n}件）" "$ok" "呼び出しが0件"
  m="$(own_axis "$f")"
  ok=0; [ "$m" = 0 ] && ok=1
  report "AC-6-4 ${y}.yml が除外軸の判定を自前で持たない（${m}件）" "$ok" "自前の除外軸が${m}行ある"
done

# 陽性対照: 写しを壊すと検出器が反応すること（片方ずつ）
for y in spec-link review-trail; do
  f="${ROOT}/.github/workflows/${y}.yml"
  # (a) 自前の除外軸を書き足す
  cp "$f" "${WORK}/${y}.axis.yml"
  printf "          if ! grep -qE '^(services|apps|infra)/' <<< \"\$changed\"; then :; fi\n" >> "${WORK}/${y}.axis.yml"
  m="$(own_axis "${WORK}/${y}.axis.yml")"
  ok=0; [ "$m" -ge 1 ] && ok=1
  report "AC-6-4 陽性対照: ${y}.yml の写しに自前の除外軸を足すと検出される" "$ok" "検出 ${m}件"
  # (b) 呼び出しを消す
  sed 's/check-exclusion-axis\.sh/x.sh/g' "$f" > "${WORK}/${y}.nocall.yml"
  n="$(calls_script "${WORK}/${y}.nocall.yml")"
  ok=0; [ "$n" = 0 ] && ok=1
  report "AC-6-4 陽性対照: ${y}.yml の写しから呼び出しを消すと0件になる" "$ok" "呼び出し ${n}件"
done

echo
echo "pass=${pass} fail=${fail}"
[ "$fail" = 0 ]
