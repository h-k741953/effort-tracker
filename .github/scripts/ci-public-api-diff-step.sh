#!/usr/bin/env bash
# 公開 API 差分の警告 step の本体（Issue #93）。
#
# .github/workflows/ci.yml の go ジョブ（`Go (lint / test / domain-deps)`）
# から呼ばれる。仕様の単一情報源: docs/specs/public-api-diff-check.md AC-7。
#
# 【ローカル再現の範囲 — スクリプト全体が CI 専用なのではない】
#   このスクリプトのうち `SKIP` 判定の2分岐（AC-7-7）は fixture の対象で
#   あり、`make test-public-api-diff` がローカルで踏む（AC-9-10）。
#   $GITHUB_STEP_SUMMARY も fixture が一時ファイルを渡して読む。
#   ローカル再現の対象外なのは base 側の git ツリーを取得する経路だけで
#   ある（AC-8-6）。抽出器（services/api/cmd/exportlist）と比較器
#   （check-public-api-diff.sh）自体は `make test-api` /
#   `make test-public-api-diff` で完全にローカル再現できる。
#
# 【常に exit 0 で終わる（AC-7-2）】
#   このジョブは ruleset protect-main の必須チェックである（P-2）。
#   step が非ゼロで終わるとブロックが実現してしまう（ADR 0018 決定1が
#   採らないと決めた形）。continue-on-error には依存しない。
#
# 【終了コードを握り潰さない（AC-7-3）】
#   抽出器・比較器の終了コードはすべて一度変数へ受けてから verdict へ
#   写像する。`| ... || true` のようなパイプでの握り潰しはしない。
#
# 入力（env、workflow 側が渡す）:
#   PAD_EVENT_NAME … github.event_name
#   PAD_BASE_SHA   … github.event.pull_request.base.sha（PR イベント以外は空）
set -uo pipefail

DETAIL_FILE="$(mktemp)"
CLEANUP_DIRS=()
# CLEANUP_WORKTREES: `git worktree add` で登録したディレクトリ。ただの
# rm -rf では .git/worktrees/ 側の登録が残る（AC-7-15）ため、
# `git worktree remove` で登録の解除ごと消す。
CLEANUP_WORKTREES=()
cleanup() {
  local d
  for d in "${CLEANUP_WORKTREES[@]:-}"; do
    [ -n "$d" ] && git worktree remove --force "$d" > /dev/null 2>&1
  done
  for d in "${CLEANUP_DIRS[@]:-}"; do
    [ -n "$d" ] && rm -rf "$d"
  done
}
trap cleanup EXIT

# emit_step_summary: verdict と理由、詳細行を $GITHUB_STEP_SUMMARY へ書く。
# ジョブログ側は finish() が別途 echo する（AC-7-4: 両方へ出す）。
emit_step_summary() {
  local verdict="$1" reason="$2"
  {
    echo "### 公開 API 差分の警告（Issue #93）"
    echo ""
    echo "VERDICT: ${verdict}"
    echo ""
    echo "${reason}"
    if [ -s "$DETAIL_FILE" ]; then
      echo ""
      echo '```'
      cat "$DETAIL_FILE"
      echo '```'
    fi
  } >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
}

# finish <verdict> <reason>: ジョブログ・$GITHUB_STEP_SUMMARY の両方へ
# verdict を出し（AC-7-4: OK / SKIP でも黙らない）、WARN / INDETERMINATE
# のときだけ ::warning:: を出し（AC-7-5）、常に exit 0 で終わる（AC-7-2）。
finish() {
  local verdict="$1" reason="$2"
  echo "VERDICT: ${verdict}"
  echo "${reason}"
  if [ -s "$DETAIL_FILE" ]; then
    cat "$DETAIL_FILE"
  fi
  emit_step_summary "$verdict" "$reason"
  if [ "$verdict" = "WARN" ] || [ "$verdict" = "INDETERMINATE" ]; then
    echo "::warning::公開 API 差分チェック: ${verdict}. ${reason}"
  fi
  exit 0
}

# --- AC-7-7 / AC-7-8: SKIP その1 — PR 文脈でない実行 ---------------------------
if [ "${PAD_EVENT_NAME:-}" != "pull_request" ]; then
  finish "SKIP" "PR 文脈でない実行のため、ベースラインが無い（比較を行わない）。"
fi

# --- AC-7-7 / AC-7-8: SKIP その2 — ベース側の ref が取得できない --------------
if [ -z "${PAD_BASE_SHA:-}" ]; then
  finish "SKIP" "ベース側の ref を取得できない（ベースラインが無い）。"
fi

# --- AC-7-9 / AC-7-10: ベース側ツリーを作業ツリーとは別のディレクトリへ ---------
# git checkout / git stash で作業ツリーを切り替えない。
BASE_DIR="$(mktemp -d)"
CLEANUP_DIRS+=("$BASE_DIR")
if ! git worktree add --detach "$BASE_DIR" "$PAD_BASE_SHA" > "$DETAIL_FILE" 2>&1; then
  finish "SKIP" "ベース側 ref（${PAD_BASE_SHA}）を取得できない（ベースラインが無い）。"
fi
CLEANUP_WORKTREES+=("$BASE_DIR")
: > "$DETAIL_FILE"

# --- AC-7-11: head 側の抽出器を1つビルドし、両側の抽出に使う -------------------
BIN_DIR="$(mktemp -d)"
CLEANUP_DIRS+=("$BIN_DIR")
EXPORTLIST_BIN="${BIN_DIR}/exportlist"
if ! ( cd services/api && go build -o "$EXPORTLIST_BIN" ./cmd/exportlist ) > "$DETAIL_FILE" 2>&1; then
  finish "INDETERMINATE" "抽出器（services/api/cmd/exportlist）のビルドに失敗した。"
fi
: > "$DETAIL_FILE"

OLD_TSV="$(mktemp)"
NEW_TSV="$(mktemp)"
OLD_ERR="$(mktemp)"
NEW_ERR="$(mktemp)"

OLD_RC=0
"$EXPORTLIST_BIN" "${BASE_DIR}/services/api" > "$OLD_TSV" 2> "$OLD_ERR" || OLD_RC=$?

NEW_RC=0
"$EXPORTLIST_BIN" "services/api" > "$NEW_TSV" 2> "$NEW_ERR" || NEW_RC=$?

OLD_LINES="$(wc -l < "$OLD_TSV" | tr -d ' ')"
NEW_LINES="$(wc -l < "$NEW_TSV" | tr -d ' ')"

# --- AC-7-12: 抽出器が非ゼロ・抽出結果が0行 → INDETERMINATE（OK へ倒さない） ----
if [ "$OLD_RC" -ne 0 ] || [ "$NEW_RC" -ne 0 ] || [ "$OLD_LINES" -eq 0 ] || [ "$NEW_LINES" -eq 0 ]; then
  {
    echo "ベース側抽出: rc=${OLD_RC} lines=${OLD_LINES}"
    cat "$OLD_ERR"
    echo "head 側抽出: rc=${NEW_RC} lines=${NEW_LINES}"
    cat "$NEW_ERR"
  } > "$DETAIL_FILE"
  finish "INDETERMINATE" "抽出器が非ゼロで終わった、または抽出結果が0行だった。"
fi

# --- 比較（AC-4 / AC-5）。旧側=ベース、新側=head。 -----------------------------
# 比較器の出力は stdout（VERDICT と詳細行 — AC-7-13）と stderr（人間・AI
# 向けの説明 — AC-9-1 等）の2系統。DETAIL_FILE は片方で上書きせず、両方を
# 保持する（AC-7-16）。
COMPARE_ERR_FILE="$(mktemp)"
COMPARE_RC=0
COMPARE_OUT="$(.github/scripts/check-public-api-diff.sh "$OLD_TSV" "$NEW_TSV" 2>"$COMPARE_ERR_FILE")" || COMPARE_RC=$?
{
  printf '%s\n' "$COMPARE_OUT"
  cat "$COMPARE_ERR_FILE"
} > "$DETAIL_FILE"

# --- AC-7-12 / AC-7-13: 比較器の verdict を step の verdict へ写像する ---------
case "$COMPARE_RC" in
  0)
    finish "OK" "公開 API の集合とシグネチャは base…head で変わっていない。"
    ;;
  3)
    finish "WARN" "公開 API に差分がある（ADDED または SIGNATURE_CHANGED。詳細行は上を参照）。見るべき差分の候補であり、判定ではない。"
    ;;
  *)
    finish "INDETERMINATE" "比較器が INDETERMINATE を返した（一覧を読めなかった等）。"
    ;;
esac
