#!/usr/bin/env bash
# UserPromptSubmit hook 本体（Issue #30 / ADR 0011）。
#
# 【何をするか】
#   工程をまたぐ依頼（コードベース・docs への変更指示、Issue/PR への着手指示）が
#   main エージェントへ直接届くことを検知し、プロンプトをブロックする。
#   受け入れ条件・検知パターンの単一情報源は docs/specs/orchestrator-entry-hook.md
#   （AC-1〜AC-8）。本ファイルへ条件を書き写すのではなく、同仕様を実装したもの。
#
# 【バージョン依存の前提】
#   本スクリプトが前提とする stdin payload のキー構成・終了コードの意味・
#   サブエージェント起動時に発火しないという性質は、Claude Code 2.1.214
#   （2026-07-18 実測）に基づく（docs/specs/orchestrator-entry-hook.md P-1〜P-3）。
#   将来のバージョンでこれらが変わると、本スクリプトは無症状で壊れうる
#   （fixture では検知できず、実環境でしか気づけない。同仕様 AC-6-8）。
#
# 【この検知の限界 ― 偽陰性は残る（トレードオフの選択）】
#   検知は正規表現ヒューリスティックであり、自然言語の言い換え（「例のやつ
#   いい感じにしといて」等）を網羅的に検知することはできない。これは欠陥ではなく
#   仕様どおりの限界（docs/specs/orchestrator-entry-hook.md AC-6-1、
#   docs/adr/0011-orchestrator-required-entry.md 決定5）。受け皿は
#   docs/rules/responsibility.md が定める規律層であり、本スクリプトはそれを
#   代替しない。
#
#   加えて、B-1/B-2 は「動作語 + 指示語尾の連接」のみを一致対象とする
#   （名詞単独では一致させない。AC-2 B-1-d）。これは初回実装が名詞を裸で
#   語彙登録していたために生じた偽陽性（「この実装どうなってる?」等が一律
#   ブロックされる）を是正した結果であり、**偽陽性を下げた分だけ偽陰性が
#   増える関係にある**。助詞を挟む依頼（「実装をお願いします」）や
#   五段動詞の命令形（「直せ」）は意図的に通す（AC-6-6）。連接規則を緩めて
#   これらを拾いにいくと、名詞形の質問への偽陽性が再び波及するため、
#   緩めないこと。
#
#   英語は `implement` のみを拾い、`fix` は語彙から外している（AC-2 B-1-e）。
#   `fix` は名詞（the fix）と動詞（fix this）が同綴で語境界一致では区別できず、
#   残すと `What is the fix?` のような質問を誤爆するため。結果として
#   `fix this bug` は偽陰性へ回る（AC-6-7）。本リポジトリの運用言語は日本語
#   （ADR 0010 §G）であり、英語の網羅より判定規則の一貫性を優先した。
#
# 【終了コードの意味（P-1）】
#   0 = 通す（stdout はコンテキストへ注入されるため無出力を守る）
#   2 = ブロック（stderr のみ人間に表示される。理由・回避手段・ADR 参照を出す）
#   1 = 判定不能（stdin 空・JSON 破損・prompt キー欠落）。ブロックせず警告して通す
#       （fail-closed は人間の復旧指示すら通せないデッドロックになるため。
#       docs/adr/0011-orchestrator-required-entry.md 決定6）
#
# 【プロンプト文字列をシェルに展開しないこと（AC-8-4）】
#   prompt の値は jq -r でファイルへ書き出し、以降はファイル経由 / 引用符付き
#   変数経由でのみ扱う。コマンドラインへ展開したり eval したりしない。
#   .github/scripts/check-review-trail.sh が TRAIL_FILE 経由で外部入力を渡すのと
#   同じ理由。
#
# 【多バイト文字の扱い】
#   日本語の検知語・区切り記号（。、．，等）は正規表現の文字クラス `[...]` に
#   入れず、常に選択（グループ化 + `|` によるアルタネーション）で扱う。
#   Makefile 178行目付近に記録のとおり、`#{1,6}` のような区間指定を mawk が
#   解釈しなかった事故があり、多バイト文字を character class に混ぜることは
#   同種の環境依存の破綻源になりうる。本スクリプトが使うのは grep -E のみで
#   awk は使わないが、同じ理由でリスクを避けるため踏襲する。
#   区切り記号（T-1）の「文字列終端」は、jq -r が値の末尾に改行を1つ付与する
#   ことと、既定（-z を付けない）の grep が入力を行単位で処理し `$` が
#   「その行の終端（＝直後の改行の手前）」に一致することを利用して、
#   `て` + ERE の `$` アンカーで表す。改行という多バイトでない1バイト文字を
#   文字クラスへ混ぜる必要がないため、この設計でも character class 経路は
#   一切生じない。
#
# 【なぜ prompt キーだけを見るか（フォールバックしない）】
#   P-3（実測）で人間の入力は prompt キーにのみ入ることが確定している。
#   user_input 等へフォールバックすると、payload 形式が変わったときに
#   「たまたま動いてしまい」壊れたことに気づけなくなる（AC-8-6）。
set -uo pipefail

# レグレックス・固定文字列マッチングを locale 非依存にする。UTF-8 ロケール下では
# \b（単語境界）が多バイト文字の前後で不安定になりうることを実測で確認したため、
# 判定用の grep はすべて C ロケールに固定する。
export LC_ALL=C

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

PAYLOAD_FILE="${WORK}/payload.json"
PROMPT_FILE="${WORK}/prompt.txt"

# stdin を丸ごとファイルへ書き出す（バイナリセーフ）。
cat > "$PAYLOAD_FILE"

warn_and_pass() {
  # AC-7: 判定不能。ブロックせず、警告して通す。stdout は無出力を維持する。
  echo "[check-prompt-entry] 警告: $1 判定不能のためブロックせず通す（docs/specs/orchestrator-entry-hook.md AC-7）。" >&2
  exit 1
}

# AC-7-1: stdin が空。
if [ ! -s "$PAYLOAD_FILE" ]; then
  warn_and_pass "stdin が空だった。"
fi

# AC-7-2 / AC-7-3 / AC-8-6: JSON 破損・prompt キー欠落・prompt が文字列でない
# （user_input のみ等）をまとめて1回の jq 呼び出しで判定する。
# select が真を返した場合のみ .prompt を出力するので、-e と組み合わせると
# 「出力が無かった＝false 扱い」で非0 終了になる。JSON 自体が壊れている場合も
# jq がエラー終了するため、いずれも同じ if 分岐でまとめて拾える。
if ! jq -e -r \
  'select(type == "object" and has("prompt") and (.prompt | type) == "string") | .prompt' \
  "$PAYLOAD_FILE" > "$PROMPT_FILE" 2>/dev/null; then
  warn_and_pass "JSON が壊れているか、prompt キー（文字列）が無い。"
fi

# ==============================================================================
# AC-1: [direct] で始まるプロンプトは常に通す（許可はこれが唯一の条件）
# ==============================================================================
# 先頭の空白文字（スペース／タブ／改行）を除いた直後が、半角8文字の "[direct]"
# に完全一致するかを見る。sed -z でファイル全体を1つのバッファとして扱い、
# 行頭アンカーではなく「ファイル冒頭からの空白」を一括で取り除く
# （複数行にまたがる先頭空白・改行を無視するため。AC-1-2）。
TRIMMED_FILE="${WORK}/trimmed.txt"
sed -z 's/^[ \t\n]*//' "$PROMPT_FILE" > "$TRIMMED_FILE"

# "[direct]" は半角8バイトの ASCII トークンなので、先頭8バイトの切り出しは
# マルチバイト文字の境界を壊さない（8バイト目の "]" の直後にどんな文字が
# 続いても、切り出し自体はバイト単位で安全）。
DIRECT_TOKEN='[direct]'
LEADING="$(head -c 8 "$TRIMMED_FILE" 2>/dev/null || true)"

if [ "$LEADING" = "$DIRECT_TOKEN" ]; then
  # AC-5: 通過時は stdout・stderr とも無出力。
  exit 0
fi

# ==============================================================================
# AC-2: 工程をまたぐ依頼はブロックする（B-1 変更指示 / B-2 Issue・PR 着手指示）
# ==============================================================================

# B-1/B-2 は「動作語 + 指示語尾の連接」でのみ一致させる（名詞単独では一致させ
# ない。docs/specs/orchestrator-entry-hook.md AC-2 B-1-a〜B-1-e / B-2）。
# 以下の各断片は ERE の選択（グループ化 + `|`）で組み立てる。多バイト文字は
# 一切文字クラス `[...]` に入れず、常に選択の1要素（1つの選択肢＝1つの完全な
# リテラル文字列）として扱う（上記コメント「多バイト文字の扱い」参照）。

# --- T: 指示語尾（B-1-c） ------------------------------------------------------
# 区切り（区切り記号 + 空白 + 文字列終端）。終端は ERE の `$` を使う。
# `$` は既定（-z なし）の grep が行単位で処理するときの「行末（＝直後の改行の
# 手前）」に一致するので、jq -r が付与する末尾の改行と組み合わせて
# 「文字列の終端 / 改行」の両方をこの1つのアンカーで表せる（多バイトの改行
# バイト自体をパターンへ書く必要がない）。
SEP='( |	|　|。|、|．|，|\.|,|!|！|\?|？)'
# 依頼接尾（T-2）。
REQ='(くれ|ください|下さい|ほしい|欲しい|おいて|もらえ|もらい|もらっ|いただけ|いただき)'
T1='て('"${SEP}"'|$)'
T2='て'"${REQ}"
T3='といて'
# T4（ろ）は B-1-a（サ変名詞型）にのみ適用する。B-1-b の動詞連用形には
# 適用しない（`直しろ` は非文。仕様 B-1-b の注記）。
TAIL_NO_T4='('"${T1}"'|'"${T2}"'|'"${T3}"')'

# --- B-1-a: サ変名詞型 S -------------------------------------------------------
# 一致条件: S+し+T（T4含む） / S+せよ / S+お願い / S+おねがい。
B1A_WORDS='(実装|修正|対応|作成|追加|削除|リファクタリング|リファクタ|コミット|起票)'
TAIL_A='(し('"${TAIL_NO_T4}"'|ろ)|せよ|お願い|おねがい)'
B1A_PATTERN="${B1A_WORDS}${TAIL_A}"

# --- B-1-b: 動詞連用形型 V ------------------------------------------------------
# 一致条件: V+T（T4は適用しない）。
B1B_WORDS='(直し|作っ|消し|書き換え|置き換え|書い)'
B1B_PATTERN="${B1B_WORDS}${TAIL_NO_T4}"

B1_JA_PATTERN="(${B1A_PATTERN}|${B1B_PATTERN})"

# B-1 の日本語側（連接規則）。grep -Eo で「一致した実文字列」を取り出す
# （AC-4-6。動作語だけでなく指示語尾を含めた文字列を理由に出すため）。
B1_JA_MATCHED=()
while IFS= read -r m; do
  [ -n "$m" ] || continue
  B1_JA_MATCHED+=("$m")
done < <(grep -Eo -- "${B1_JA_PATTERN}" "$PROMPT_FILE" 2>/dev/null || true)

# --- B-1-e: 英語 --------------------------------------------------------------
# `implement` のみを、大文字小文字を区別せず語境界で一致させる。
# `fix` は語彙から外す（名詞と動詞が同綴で語境界一致では区別できず、
# `What is the fix?` のような質問を誤爆するため。仕様 B-1-e）。
EN_MATCHED=()
for w in implement; do
  if grep -Eiq -- "\\b${w}\\b" "$PROMPT_FILE"; then
    m="$(grep -Eio -- "\\b${w}\\b" "$PROMPT_FILE" | head -n1)"
    EN_MATCHED+=("$m")
  fi
done

if [ "${#B1_JA_MATCHED[@]}" -gt 0 ] || [ "${#EN_MATCHED[@]}" -gt 0 ]; then
  HAS_B1=1
else
  HAS_B1=0
fi

# B-2: Issue #<数字> または #<数字> の言及。
ISSUE_REF=""
if grep -Eq '#[0-9]+' "$PROMPT_FILE"; then
  ISSUE_REF="$(grep -Eo '#[0-9]+' "$PROMPT_FILE" | head -n1)"
fi

# B-2 の追加動作語（B-1 と同じ指示形の規則を適用。名詞単独では一致させない）。
#   動詞連用形（やっ/進め/片付け）+ T（T4は適用しない）
#   サ変名詞（着手）+ B-1-a と同じ規則（TAIL_A）
#   依頼単独（お願い/おねがい）はそのまま一致
B2_TAIL_WORDS='(やっ|進め|片付け)'
B2_EXTRA_PATTERN="(${B2_TAIL_WORDS}${TAIL_NO_T4}|着手${TAIL_A}|お願い|おねがい)"

B2_EXTRA_MATCHED=()
while IFS= read -r m; do
  [ -n "$m" ] || continue
  B2_EXTRA_MATCHED+=("$m")
done < <(grep -Eo -- "${B2_EXTRA_PATTERN}" "$PROMPT_FILE" 2>/dev/null || true)

HAS_B2=0
if [ -n "$ISSUE_REF" ]; then
  if [ "$HAS_B1" -eq 1 ] || [ "${#B2_EXTRA_MATCHED[@]}" -gt 0 ]; then
    HAS_B2=1
  fi
fi

if [ "$HAS_B1" -eq 0 ] && [ "$HAS_B2" -eq 0 ]; then
  # AC-3: 工程をまたがない依頼は通す。AC-5: 無出力・終了コード0。
  exit 0
fi

# ==============================================================================
# AC-4: ブロック時の出力（stderr のみ。理由・回避手段2種・ADR 参照・一致パターン）
# ==============================================================================

MATCHED=("${B1_JA_MATCHED[@]}" "${EN_MATCHED[@]}")
if [ "$HAS_B2" -eq 1 ]; then
  [ -n "$ISSUE_REF" ] && MATCHED+=("$ISSUE_REF")
  MATCHED+=("${B2_EXTRA_MATCHED[@]}")
fi

# 重複を除きつつ ", " で連結する（表示用。判定には使わない）。
MATCHED_STR=""
declare -A SEEN=()
for m in "${MATCHED[@]}"; do
  [ -n "${SEEN[$m]:-}" ] && continue
  SEEN["$m"]=1
  if [ -z "$MATCHED_STR" ]; then
    MATCHED_STR="$m"
  else
    MATCHED_STR="${MATCHED_STR}, ${m}"
  fi
done

{
  echo "[check-prompt-entry] このプロンプトをブロックしました。"
  echo ""
  echo "理由: 工程をまたぐ依頼（仕様→テスト→実装の受け渡しを伴う変更指示、"
  echo "      または Issue/PR への着手指示）は、main エージェントが直接処理せず"
  echo "      オーケストレーターを入口とする（docs/adr/0011-orchestrator-required-entry.md）。"
  echo ""
  echo "一致した検知パターン: ${MATCHED_STR}"
  echo ""
  echo "回避手段1: オーケストレーター（オーケストレーターエージェント）にこの依頼を渡すこと。"
  echo "回避手段2: 直接処理させたい場合は、プロンプトの冒頭に \"[direct]\" を付けて再送すること。"
  echo ""
  echo "根拠: docs/adr/0011-orchestrator-required-entry.md"
} >&2

exit 2
