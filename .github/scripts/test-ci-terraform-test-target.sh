#!/usr/bin/env bash
# CI（.github/workflows/ci.yml）の terraform ジョブが
# `make test-tf` を呼ぶことを検査する fixture テスト（Issue #8, C-1）。
#
# 仕様の単一情報源:
#   docs/specs/infra-terraform.md
#     AC-1-3「terraform test を make のターゲットから実行でき、make test の
#             経路に入ること。CI 側にだけ存在する検査を作らない」
#     AC-1-4「CI は AC-1-2・AC-1-3 と同一の make ターゲットを呼ぶ。
#             CI に独自のコマンド列を書かない」
#
# 【実測済みの欠陥（C-1）】
#   本テスト作成時点で infra/terraform/tests/ 配下に .tftest.hcl が 20 本
#   存在し、Makefile 側には `test-tf` ターゲットが既に追加されている
#   （`make test` の前提にも入っている）。しかし
#   .github/workflows/ci.yml の terraform ジョブが呼ぶのは
#   `make lint-tf` だけで、`make test-tf` を呼ぶ行が無い。したがって
#   20 本の terraform test は CI で一度も実行されていない。
#
# 【検査対象を Makefile ではなく ci.yml に絞る理由】
#   test-tf ターゲット自体（Makefile 側）は本 Issue の対象外ファイルであり
#   （オーケストレーターの指示により Makefile は「検査される側」）、かつ
#   既に Makefile へ追加済み（実測: `test: test-api test-web test-tf ...`）。
#   欠けているのは CI 側の配線だけなので、ci.yml のみを検査する。
#
# 【なぜ terraform 呼び出しでなく grep で静的に検査するか】
#   実際に `make test-tf` を本テストの中で実行すると、並行して進行中の
#   infra/terraform・infra/bootstrap の内容（tests/ を含む）に本テストの
#   成否が引きずられ、C-1（CI の配線漏れ）とは無関係な理由で Red/Green が
#   揺れる。本テストの関心は「ci.yml が正しいコマンドを呼ぶ配線になって
#   いるか」であり、ci.yml の静的な記述だけを検査する。
#
# 単体実行: bash .github/scripts/test-ci-terraform-test-target.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CI_YML="${REPO_ROOT}/.github/workflows/ci.yml"

pass=0
fail=0

ok() { pass=$((pass + 1)); printf '  ok   %s\n' "$1"; }
ng() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; shift; printf '       | %s\n' "$@"; }

echo "==> test-ci-terraform-test-target"

if [ ! -f "$CI_YML" ]; then
  ng "ci.yml が存在する" "expected: file exists at $CI_YML" "actual: not found"
  echo ""
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi

# terraform ジョブのブロックだけを切り出す（先頭 2 桁インデントの
# "terraform:" キーから、次の 2 桁インデントのトップレベルキーの手前まで）。
# 本ワークフローでは terraform ジョブが最後のジョブだが、将来ジョブが
# 追加されても巻き込まないよう終端条件を明示している。
job_block="$(awk '
  $0 ~ /^  terraform:$/ { found=1 }
  found {
    if (started && $0 ~ /^  [A-Za-z0-9_-]+:/) { exit }
    started=1
    print
  }
' "$CI_YML")"

if [ -z "$job_block" ]; then
  ng "ci.yml に terraform ジョブが存在する" \
    "expected: a job block starting with '  terraform:'" \
    "actual: not found in $CI_YML"
  echo ""
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
ok "ci.yml に terraform ジョブが存在する"

# ジョブブロック内の run: の値（1行形式のみを対象。複数行の run: | が
# 現れたらそれ自体が AC-1-4 の「独自のコマンド列」を疑う対象として扱う）。
mapfile -t run_lines < <(printf '%s\n' "$job_block" | command grep -E '^\s*run:' )

if printf '%s\n' "${run_lines[@]:-}" | command grep -qE '^\s*run:\s*\|'; then
  ng "terraform ジョブの run: が単一コマンドの形である（複数行ブロックでない）" \
    "expected: each 'run:' step is a single make invocation" \
    "actual: found a multi-line 'run: |' block" \
    "$(printf '%s\n' "${run_lines[@]}")"
fi

mapfile -t run_values < <(printf '%s\n' "${run_lines[@]:-}" | sed -E 's/^\s*run:\s*//' | sed -E 's/[[:space:]]+$//')

# --- AC-1-3: make test-tf を呼ぶ行が存在する ---------------------------------
if printf '%s\n' "${run_values[@]:-}" | command grep -qxF "make test-tf"; then
  ok "terraform ジョブが 'make test-tf' を呼ぶ（AC-1-3）"
else
  ng "terraform ジョブが 'make test-tf' を呼ぶ（AC-1-3）" \
    "expected: a 'run: make test-tf' step in the terraform job" \
    "actual run values found: $(printf '%s / ' "${run_values[@]:-}")"
fi

# --- 回帰防止: make lint-tf を呼ぶ行が引き続き存在する（AC-1-2 の配線）-------
if printf '%s\n' "${run_values[@]:-}" | command grep -qxF "make lint-tf"; then
  ok "terraform ジョブが 'make lint-tf' を呼ぶ（AC-1-2、回帰防止）"
else
  ng "terraform ジョブが 'make lint-tf' を呼ぶ（AC-1-2、回帰防止）" \
    "expected: a 'run: make lint-tf' step in the terraform job" \
    "actual run values found: $(printf '%s / ' "${run_values[@]:-}")"
fi

# --- AC-1-4: run: の値はすべて make ターゲット呼び出しに限る（独自のコマンド
#     列を書かない）。allowlist はこの2つだけとする。 --------------------------
bad=0
for v in "${run_values[@]:-}"; do
  case "$v" in
    "make lint-tf"|"make test-tf") ;;
    *) bad=1; printf '       | 想定外の run 値: %s\n' "$v" ;;
  esac
done
if [ "$bad" = 0 ] && [ "${#run_values[@]}" -gt 0 ]; then
  ok "terraform ジョブの run: は make ターゲット呼び出しのみ（AC-1-4、独自コマンド列を書かない）"
else
  fail=$((fail + 1))
  printf '  FAIL %s\n' "terraform ジョブの run: は make ターゲット呼び出しのみ（AC-1-4、独自コマンド列を書かない）"
  printf '       | expected: every run value is one of {make lint-tf, make test-tf}\n'
fi

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
