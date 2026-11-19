#!/usr/bin/env bash
# プロセス検査（spec-link / review-trail）の「SKIP／検査する／失敗」の判定（Issue #174）。
#
# 仕様: docs/specs/process-check-exclusion-axis.md（判定の定義・P-2・AC-6-7）
# 根拠: docs/adr/0025（除外軸は services/ apps/ infra/）
#
# 呼び出し: bash .github/scripts/check-exclusion-axis.sh <spec-link|review-trail>
#   PR_LABELS         ラベル名を1行に1つ（空文字はラベル無し。未設定は判定できない）
#   CHANGED_FILE_LIST 変更ファイルの一覧（1行1パス）を書いたファイルのパス
# 出力:
#   SKIP  -> 終了 0 / 標準出力 SKIP
#   CHECK -> 終了 0 / 標準出力 CHECK
#   失敗（review-trail のチェッカ自身のパスが無い）-> 終了 1 / 標準出力なし
#   判定できない（入力が読めない）-> 終了 2 / 標準出力なし
# 理由の文は標準エラー出力へ出す。
# チェッカ自身の2つのパスは、カレントディレクトリからの相対パスで確かめる。
set -uo pipefail

undetermined() {
  echo "判定できない: $1" >&2
  exit 2
}

# 1. 検査の名前と PR_LABELS
[ "$#" -eq 1 ] || undetermined "検査の名前は引数にちょうど1つ必要"
name="$1"
case "$name" in
  spec-link | review-trail) ;;
  *) undetermined "検査の名前は spec-link か review-trail" ;;
esac
[ "${PR_LABELS+set}" = set ] || undetermined "PR_LABELS が未設定"

# 2. no-spec ラベル（行全体の完全一致。一覧は読まない）
# <<< は末尾改行を足すため `|| [ -n ]` は通常不要だが、改行なし入力の取りこぼしを防ぐ守りとして残す
while IFS= read -r label || [ -n "$label" ]; do
  if [ "$label" = "no-spec" ]; then
    echo "SKIP: no-spec ラベルが付いている（仕様を要さない変更）" >&2
    echo "SKIP"
    exit 0
  fi
done <<< "$PR_LABELS"

# 3. review-trail のみ: チェッカ自身の2つのパスの実在（一覧を読む前）
if [ "$name" = "review-trail" ]; then
  for p in .github/scripts/check-review-trail.sh .github/workflows/review-trail.yml; do
    if [ ! -e "$p" ]; then
      {
        echo "NG: $p が存在しない。SKIP 除外軸のパスが実態とずれている（除外軸の劣化）。"
        echo "    移動・改名したなら check-exclusion-axis.sh のこの確認も追随させること。"
      } >&2
      exit 1
    fi
  done
fi

# 4. 変更ファイルの一覧
[ -n "${CHANGED_FILE_LIST:-}" ] || undetermined "CHANGED_FILE_LIST が未設定または空"
[ -f "$CHANGED_FILE_LIST" ] || undetermined "CHANGED_FILE_LIST が読めるファイルではない"
[ -r "$CHANGED_FILE_LIST" ] || undetermined "CHANGED_FILE_LIST が読めない（権限）"

# 5. review-trail のみ: チェッカ自身の変更は SKIP させない
# grep の終了コード: 0=一致 / 1=不一致 / 2以上=読み取り失敗等（判定できない。不一致に潰さない）
if [ "$name" = "review-trail" ]; then
  grep -qE '^\.github/(scripts/|workflows/review-trail\.yml$)' "$CHANGED_FILE_LIST"
  rc=$?
  case "$rc" in
    0)
      echo "検査を実行: チェッカ自身の変更を含む PR（SKIP させない）" >&2
      echo "CHECK"
      exit 0
      ;;
    1) ;;
    *) undetermined "変更ファイルの一覧を grep できない（終了コード $rc）" ;;
  esac
fi

# 6. 除外軸
grep -qE '^(services|apps|infra)/' "$CHANGED_FILE_LIST"
rc=$?
case "$rc" in
  0)
    echo "CHECK"
    exit 0
    ;;
  1) ;;
  *) undetermined "変更ファイルの一覧を grep できない（終了コード $rc）" ;;
esac
echo "SKIP: プロダクトコード等（services/ apps/ infra/）の変更なし" >&2
echo "SKIP"
exit 0
