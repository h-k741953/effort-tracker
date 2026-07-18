#!/usr/bin/env bash
# 往復証跡（PR コメント）の形式・往復上限を検査する（Issue #25）。
#
# review-trail.yml から呼ばれるプロセス検査の本体。PR 本文（環境変数 PR_BODY）に、
# 実装↔レビューの往復が verification-loop.md「実装↔レビューループ」の定型フォーマットで
# 記録されているか、往復数が「ループの上限値」表の上限内かを機械的に確認する。
#
# 【この検査の限界 ― 記録しか見ない】
#   見るのは *PR に記録された* 往復ブロックの形式と往復数であって、実セッションの
#   実装↔レビュー往復そのものではない。記録をごまかせば擦り抜ける（外部が書き込める
#   場所を正解にしない、という ADR 0004 と同じ留意）。指摘レベルの分類（Critical/
#   Warning/Info の妥当性）と2段承認ゲートの成立は artifact に残らず、検査できない。
#
# 入力:
#   PR_BODY           … PR 本文（review-trail.yml が env 経由で渡す。injection 回避のため引数にしない）
#   REVIEW_LOOP_DOC   … 上限値の単一情報源（既定 docs/harness/verification-loop.md）
set -euo pipefail

DOC="${REVIEW_LOOP_DOC:-docs/harness/verification-loop.md}"

# --- 上限値を単一情報源（verification-loop.md「ループの上限値」表）から読む ---------
# スクリプトにハードコードしない（Issue #25 / ADR 0010 §A の閾値は人間の承認事項）。
# 読み取れなければ「単一情報源の書式が動いた」合図として fail する。黙って素通りさせない。
#
# マークダウン表のセル（行頭 `| 実装↔レビューの往復 |`）にアンカーする。行内一致だと
# 「実装↔レビューの往復…」を含む散文行にも当たり、将来そこへ太字数字が書かれると
# head -n1 が誤値を黙って拾う（＝単一情報源を検査自身が壊す）。表の行に限定して防ぐ。
cap="$(grep -E '^\|[[:space:]]*実装↔レビューの往復[[:space:]]*\|' "$DOC" \
  | grep -oE '\*\*[0-9]+\*\*' \
  | grep -oE '[0-9]+' \
  | head -n1 || true)"

if [ -z "${cap:-}" ]; then
  echo "NG: 往復上限を $DOC「ループの上限値」から読み取れなかった。"
  echo "    行『実装↔レビューの往復 | **N** | ...』が見つからない。表の書式が変わった可能性。"
  echo "    上限はスクリプトにハードコードせず単一情報源から読む方針（Issue #25）。"
  exit 1
fi

# --- PR 本文の往復ブロックを検証 ---------------------------------------------------
# CRLF を落として awk へ。ラベルは前後の空白に寛容に一致させる（整形のブレを許す）。
if printf '%s' "${PR_BODY:-}" | tr -d '\r' | awk -v cap="$cap" '
  function flush(   miss) {
    if (!in_block) return
    miss=""
    if (!has_c)   miss = miss " Critical"
    if (!has_w)   miss = miss " Warning"
    if (!has_i)   miss = miss " Info"
    if (!has_r)   miss = miss " 対応"
    if (!has_u)   miss = miss " 未解決"
    if (miss != "") {
      msgs = msgs sprintf("  往復 %d: 必須行が欠けている →%s\n", curn, miss)
      bad = 1
    }
  }
  /^[ \t]*###[ \t]+往復[ \t]+[0-9]+/ {
    flush()
    in_block = 1; count++
    s = $0
    sub(/^[ \t]*###[ \t]+往復[ \t]+/, "", s); sub(/[^0-9].*/, "", s)
    curn = s + 0
    if (curn > maxn) maxn = curn
    has_c = has_w = has_i = has_r = has_u = 0
    next
  }
  in_block {
    if ($0 ~ /^[ \t]*[-*][ \t]*Critical[ \t]*:/)  has_c = 1
    else if ($0 ~ /^[ \t]*[-*][ \t]*Warning[ \t]*:/)  has_w = 1
    else if ($0 ~ /^[ \t]*[-*][ \t]*Info[ \t]*:/)     has_i = 1
    else if ($0 ~ /^[ \t]*[-*][ \t]*対応[ \t]*:/)     has_r = 1
    else if ($0 ~ /^[ \t]*[-*][ \t]*未解決[ \t]*:/)   has_u = 1
  }
  END {
    flush()
    if (count == 0) {
      msgs = msgs "  『### 往復 N』ブロックが本文に1つも無い\n"
      bad = 1
    }
    if (maxn > cap) {
      msgs = msgs sprintf("  往復 %d が上限 %d を超えている（収束せず＝握り潰さず人間へ上げる状況）\n", maxn, cap)
      bad = 1
    }
    if (bad) {
      printf("NG: 往復証跡（PR コメント）の形式・往復数が要件を満たさない\n")
      printf("%s", msgs)
      exit 1
    }
    printf("OK: 往復ブロック %d 個 / 最大 往復 %d ≤ 上限 %d / 各ブロックに必須5行そろい\n", count, maxn, cap)
  }
'; then
  exit 0
fi

echo ""
echo "  往復ごとに次のフォーマットで PR にコメントを残すこと（verification-loop.md「実装↔レビューループ」）:"
echo ""
echo "    ### 往復 N"
echo "    - Critical: <件数> — <要点。無ければ「なし」>"
echo "    - Warning : <件数> — <要点。無ければ「なし」>"
echo "    - Info    : <件数> — <要点。無ければ「なし」>"
echo "    - 対応    : <この往復で implementer が直した点>"
echo "    - 未解決  : <次往復へ持ち越す点。無ければ「なし」>"
echo ""
echo "  往復が上限を超えたら握り潰さず人間へ上げる（verification-loop.md「停止条件」）。"
echo "  仕様を要さない変更なら no-spec ラベルを付けること。"
exit 1
