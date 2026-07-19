#!/usr/bin/env bash
# check-prompt-entry.sh の fixture テスト（Issue #30 / ADR 0011）。
#
# 【なぜこれが要るか】
#   docs/specs/orchestrator-entry-hook.md の AC-1〜AC-8 をテーブル駆動で固定する。
#   検知本体は stdin 以外の入力を持たない純粋なフィルタとして書かれる前提なので
#   （同 spec「対象」節）、入力（stdin の JSON payload）を差し替えるだけで
#   ローカルで完全に再現・検査できる。.github/scripts/test-check-review-trail.sh
#   と同じ型（ケース定義・実行・期待値比較・失敗時の出力・件数サマリ）に倣う。
#
# 【プロンプト文字列をシェルに展開しないこと】
#   payload の組み立ては jq -n --arg を使う。プロンプト本文をコマンドラインへ
#   直接展開すると、AC-8-4（引用符・バッククォート・$(...)）を含む入力で
#   このテストスクリプト自身が壊れる／意図しないコマンドを実行しかねない。
#   check-review-trail.sh が TRAIL_FILE 経由でファイルを渡しているのと同じ理由。
#
# 【判定は終了コード + stdout/stderr の出力有無の3点】
#   exit code だけでは AC-5-2（通過時 stdout 無出力）の要求を検査できない。
#   通過（0・無出力）／ブロック（2・stderr のみ）／判定不能（1・stderr に警告）を
#   すべて弁別できる形にする。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# 被テスト対象のパスは docs/specs/orchestrator-entry-hook.md の「対象」節が単一
# 情報源（ADR 0004）。このテスト工程への指示書は check-orchestrator-entry.sh と
# 書いていたが、仕様書は check-prompt-entry.sh / test-check-prompt-entry.sh と
# 定めているため、仕様側に合わせた（本ファイルの報告を参照）。
TARGET="${SCRIPT_DIR}/check-prompt-entry.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

echo "==> test-check-prompt-entry"

# --- 入力の組み立て ----------------------------------------------------------

# 標準の7キーを持つ payload を組み立てる（docs/specs/orchestrator-entry-hook.md P-3）。
# jq -n --arg でプロンプト本文をシェルに展開せず JSON へ埋め込む。
mkjson() {
  local prompt="$1"
  jq -n --arg prompt "$prompt" '
    {
      cwd: "/workspaces/effort-tracker",
      hook_event_name: "UserPromptSubmit",
      permission_mode: "default",
      prompt: $prompt,
      prompt_id: "test-prompt-id",
      session_id: "test-session-id",
      transcript_path: "/tmp/test-transcript.jsonl"
    }'
}

# <名前> <本文> の組で payload ファイルを1つ作り、パスを echo する。
write_input() {
  local name="$1" prompt="$2"
  local file="${WORK}/${name}.json"
  mkjson "$prompt" > "$file"
  echo "$file"
}

# --- 実行 ---------------------------------------------------------------------

RC=""
OUT=""
ERR=""

# run_hook <input_file>: TARGET を stdin=input_file で実行し、RC/OUT/ERR を埋める。
# input_file が存在しない/空でも /dev/null 相当として扱えるよう、常にファイル経由で渡す。
run_hook() {
  local input_file="$1"
  local outf errf
  outf="$(mktemp -p "$WORK")"
  errf="$(mktemp -p "$WORK")"
  bash "$TARGET" < "$input_file" > "$outf" 2> "$errf"
  RC=$?
  OUT="$(cat "$outf")"
  ERR="$(cat "$errf")"
  rm -f "$outf" "$errf"
}

report() {
  local name="$1" ok="$2" want_rc="$3"
  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s (rc=%s)\n' "$name" "$RC"
  else
    fail=$((fail + 1))
    printf '  FAIL %s (rc=%s, 期待 rc=%s)\n' "$name" "$RC" "$want_rc"
    printf '       stdout | %s\n' "$OUT" | sed 's/^/       | /'
    printf '       stderr | %s\n' "$ERR" | sed 's/^/       | /'
  fi
}

# AC-1(1-1..1-4) / AC-3(3-1..3-9) / AC-5: 通す。stdout・stderr とも無出力・rc=0。
expect_pass() {
  local name="$1" input_file="$2"
  run_hook "$input_file"
  local ok=1
  [ "$RC" = "0" ] || ok=0
  [ -z "$OUT" ] || ok=0
  [ -z "$ERR" ] || ok=0
  report "$name" "$ok" "0"
}

# AC-1(1-5..1-11) / AC-2 / AC-4: ブロックする。rc=2、stdout 無出力、stderr に
# 理由・回避手段2種・ADR 参照・一致した検知パターンのいずれかを含む。
# 一致する語が複数候補ありうるケースは、候補のうち最低1つが出力に現れることを見る
# （実装がどの語を代表として報告するかまでは仕様が一意に定めていないため）。
#
# 【W-2 対応】候補の照合は「一致した検知パターン: 」の行に限定する。
#   ブロック理由の静的文章（「工程をまたぐ依頼（仕様→テスト→実装の受け渡しを
#   伴う変更指示、…」）自体に「実装」という文字列が含まれるため、$ERR 全体を
#   対象に grep -qF -- "$c" すると、候補が "実装" 単体のケースでは
#   AC-4-6（一致した実文字列を示す）を検査せずに常に真になってしまう
#   （往復1 レビュー実測。$ERR 全体を対象にした場合、「一致した検知パターン」の
#   出力そのものを "(省略)" に潰しても 16 件が偽陽性で通過した）。
#   AC-4-6 が検査したいのは「一致した検知パターン: 」の専用行であって、
#   ブロック理由の文章ではないため、そこだけを切り出して照合する。
expect_block() {
  local name="$1" input_file="$2"; shift 2
  run_hook "$input_file"
  local ok=1
  [ "$RC" = "2" ] || ok=0
  [ -z "$OUT" ] || ok=0
  [ -n "$ERR" ] || ok=0
  grep -qF -- "docs/adr/0011-orchestrator-required-entry.md" <<< "$ERR" || ok=0
  grep -qF -- "[direct]" <<< "$ERR" || ok=0
  grep -qF -- "オーケストレーター" <<< "$ERR" || ok=0
  local matched_line
  matched_line="$(grep -F -- '一致した検知パターン:' <<< "$ERR" || true)"
  [ -n "$matched_line" ] || ok=0
  local any=0 c
  for c in "$@"; do
    grep -qF -- "$c" <<< "$matched_line" && any=1
  done
  [ "$any" = 1 ] || ok=0
  report "$name" "$ok" "2"
}

# AC-7: 判定不能。rc=1、stdout 無出力、stderr に警告（非空）。ブロック側(rc=2)に
# 倒れていないことも弁別する（AC-8-6 の user_input フォールバック禁止はここで拾う）。
expect_warn() {
  local name="$1" input_file="$2"
  run_hook "$input_file"
  local ok=1
  [ "$RC" = "1" ] || ok=0
  [ -z "$OUT" ] || ok=0
  [ -n "$ERR" ] || ok=0
  report "$name" "$ok" "1"
}

# ==============================================================================
# AC-1: [direct] で始まるプロンプトは常に通す（完全一致条件）
# ==============================================================================

expect_pass "AC-1-1: [direct] 冒頭・本文あり" \
  "$(write_input ac1-1 '[direct] 一覧APIを実装して')"

expect_pass "AC-1-2: 先頭の空白・改行を無視する" \
  "$(write_input ac1-2 $'\n\n  [direct] 実装して')"

expect_pass "AC-1-3: トークン直後の空白は不要" \
  "$(write_input ac1-3 '[direct]実装して')"

expect_pass "AC-1-4: [direct] のみ（本文なし）" \
  "$(write_input ac1-4 '[direct]')"

expect_block "AC-1-5: 冒頭でない（前置あり）" \
  "$(write_input ac1-5 'よろしく [direct] 実装して')" "実装"

expect_block "AC-1-6: 冒頭でない（末尾）" \
  "$(write_input ac1-6 '実装して [direct]')" "実装"

expect_block "AC-1-7: 大文字小文字を区別する（[Direct]）" \
  "$(write_input ac1-7 '[Direct] 実装して')" "実装"

expect_block "AC-1-8: 大文字小文字を区別する（[DIRECT]）" \
  "$(write_input ac1-8 '[DIRECT] 実装して')" "実装"

expect_block "AC-1-9: 全角角括弧は一致しない" \
  "$(write_input ac1-9 '［direct］ 実装して')" "実装"

expect_block "AC-1-10: 内側の空白は一致しない" \
  "$(write_input ac1-10 '[ direct ] 実装して')" "実装"

expect_block "AC-1-11: 括弧の種類が違う" \
  "$(write_input ac1-11 '(direct) 実装して')" "実装"

# --- 1-12〜1-16: 先頭空白文字の各要素（タブ/\r/半角スペース）と全角スペースの
# 負例。AC-9-11 が要求する「4つは互いに独立して検査される」ことの固定値
# （docs/specs/orchestrator-entry-hook.md AC-1 の表・注記）。
# 1-13/1-14（\r）は往復2 W-3 で仕様側に寄せた（実装は当初これをブロックして
# いた）。現行実装の先頭空白除去は `[ \t\n]*` で \r を含まないため、
# 1-13/1-14 は本コミット時点では Red になる想定（Red の確認対象）。

expect_pass "AC-1-12: 先頭空白 タブ" \
  "$(write_input ac1-12 $'\t[direct] 実装して')"

expect_pass "AC-1-13: 先頭空白 \\r\\n（CRLF。仕様側に寄せた／往復2 W-3）" \
  "$(write_input ac1-13 $'\r\n[direct] 実装して')"

expect_pass "AC-1-14: 先頭空白 \\r 単独" \
  "$(write_input ac1-14 $'\r[direct] 実装して')"

expect_block "AC-1-15: 全角スペースは先頭空白に含めない" \
  "$(write_input ac1-15 '　[direct] 実装して')" "実装"

expect_pass "AC-1-16: 先頭空白 半角スペース1個" \
  "$(write_input ac1-16 ' [direct] 実装して')"

# --- ここまで 1-12〜1-16 ------------------------------------------------------

# ==============================================================================
# AC-2: 工程をまたぐ依頼はブロックする（B-1 変更指示 / B-2 Issue・PR 着手指示）
# ==============================================================================

expect_block "AC-2-1: B-1 実装" \
  "$(write_input ac2-1 '稼働実績の一覧APIを実装して')" "実装"

expect_block "AC-2-2: B-1 対応して / B-2" \
  "$(write_input ac2-2 'Issue #30 を対応して')" "対応して" "#30"

expect_block "AC-2-3: B-2（#番号 + やっといて）" \
  "$(write_input ac2-3 '#30 やっといて')" "やっといて" "#30"

expect_block "AC-2-4: B-1 直して" \
  "$(write_input ac2-4 'このバグを直して')" "直して"

expect_block "AC-2-5: B-1（末尾の実装してのみ一致。冒頭の書いてはからで不一致）" \
  "$(write_input ac2-5 'テストを書いてから実装して')" "実装"

expect_block "AC-2-6: B-1 追加して" \
  "$(write_input ac2-6 'WorkMonth に締め処理を追加して')" "追加して"

expect_block "AC-2-7: B-1 implement（英語・大小区別なし）" \
  "$(write_input ac2-7 'Please implement the close-month usecase')" "implement" "Implement" "IMPLEMENT"

expect_block "AC-2-8: B-1 追加して（偽陽性だが仕様どおり／AC-6-2）" \
  "$(write_input ac2-8 'docs/rules/scope.md に1行追加して')" "追加して"

expect_block "AC-2-9: B-2（#番号 + お願い）" \
  "$(write_input ac2-9 'Issue #30 お願い')" "お願い" "#30"

# --- ここから 2-10〜2-21（境界: 依頼接尾・といて・命令形・区切り記号） ----------
# 3-10〜3-13（名詞形。B-1-d で拾わない）との境界を確認するため、
# 対になる指示形（動作語 + T）はブロックされ続けることを固定する。

expect_block "AC-2-10: B-1-a 修正して（名詞形 3-10 との境界）" \
  "$(write_input ac2-10 'このバグ修正して')" "修正"

expect_block "AC-2-11: B-1-a 実装して（名詞形 3-11 との境界）" \
  "$(write_input ac2-11 'この機能実装して')" "実装"

expect_block "AC-2-12: T-2 依頼接尾（ください）" \
  "$(write_input ac2-12 'テストを修正してください')" "修正" "ください"

expect_block "AC-2-13: T-2 依頼接尾（もらえ。疑問符で終わっても依頼は依頼）" \
  "$(write_input ac2-13 'この関数を直してもらえる?')" "直して" "もらえ"

expect_block "AC-2-14: T-3（といて）" \
  "$(write_input ac2-14 '締め処理を実装しといて')" "実装" "しといて"

expect_block "AC-2-15: T-4（しろ。B-1-a のみ）" \
  "$(write_input ac2-15 'テストを実装しろ')" "実装" "しろ"

expect_block "AC-2-16: B-1-a + せよ" \
  "$(write_input ac2-16 'spec を修正せよ')" "修正" "せよ"

expect_block "AC-2-17: B-1-a リファクタリング + T-2（ください）" \
  "$(write_input ac2-17 'リファクタリングしてください')" "リファクタ" "ください"

expect_block "AC-2-18: B-1-b 書い + T-1（終端）" \
  "$(write_input ac2-18 'テストを書いて')" "書いて" "テストを書"

expect_block "AC-2-19: B-1-b 作っ + T-1" \
  "$(write_input ac2-19 'PR を作って')" "作って"

expect_block "AC-2-20: B-1-a 起票して + T-1（区切りが読点）" \
  "$(write_input ac2-20 'ADR を起票して、実装は後で')" "起票"

expect_block "AC-2-21: T-1（区切りが文字列終端）" \
  "$(write_input ac2-21 '実装して')" "実装"

# --- ここまで 2-10〜2-21 ----------------------------------------------------

# --- ここから 2-22〜2-29（終助詞 + 繰り返し / 閉じ記号 / PRを作ってね / CRLF） ---
# 往復1 C-1（実測）で見つかった穴。初期の区切り列挙は終助詞・閉じ記号・\r を
# 欠いており、以下がすべて rc=0（通過）になっていた。ここが今回の Red の本体
# （docs/specs/orchestrator-entry-hook.md AC-2 の 2-22〜2-29）。

expect_block "AC-2-22: T-1 終助詞 ね + 終端" \
  "$(write_input ac2-22 '実装してね')" "実装して"

expect_block "AC-2-23: T-1 終助詞 よ + 終端" \
  "$(write_input ac2-23 '実装してよ')" "実装して"

expect_block "AC-2-24: T-1 終助詞 な + 終端" \
  "$(write_input ac2-24 '実装してな')" "実装して"

expect_block "AC-2-25: T-1 終助詞の繰り返し（よね）" \
  "$(write_input ac2-25 '実装してよね')" "実装して"

expect_block "AC-2-26: T-1 区切り「」（偽陽性だが仕様どおり／B-1-c）" \
  "$(write_input ac2-26 '「実装して」と伝えた')" "実装して"

expect_block "AC-2-27: T-1 区切り :" \
  "$(write_input ac2-27 'この機能実装して:')" "実装して"

expect_block "AC-2-28: B-1-b 作っ + 終助詞 ね（PRを作ってね）" \
  "$(write_input ac2-28 'PRを作ってね')" "作って"

expect_block "AC-2-29: T-1 区切り \\r（CRLF。AC-8-7と同じ穴）" \
  "$(write_input ac2-29 $'実装して\r\nよろしく')" "実装して"

# --- ここまで 2-22〜2-29 ----------------------------------------------------

# --- ここから 2-30〜2-42（T-2 依頼活用尾。境界規則を挟んでも依頼はブロック維持）---
# docs/specs/orchestrator-entry-hook.md B-1-c「依頼活用尾」9語（る/たい/ます/
# ますか/ませんか/ない/ないか/ていい/てもいい）と、依頼接尾の連接・て連結
# （2-35/2-36）を固定する。AC-9-12（依頼活用尾の各語）・AC-9-13（連接・て連結）
# の要求はここで満たす。候補文字列は依頼接尾の語（現行実装・境界規則実装後の
# いずれでも一致部分文字列として現れる部分）に絞り、活用尾側の文字列そのものを
# 候補にしない（現行実装は活用尾を要求せず接尾直後で一致が閉じるため）。

expect_block "AC-2-30: T-2 依頼活用尾 る + 区切り ？" \
  "$(write_input ac2-30 '実装してくれる？')" "実装して" "くれ"

expect_block "AC-2-31: T-2 もらえ + 依頼活用尾 る + ？" \
  "$(write_input ac2-31 '実装してもらえる？')" "実装して" "もらえ"

expect_block "AC-2-32: T-2 ください + 終端" \
  "$(write_input ac2-32 '対応してください')" "対応して" "ください"

expect_block "AC-2-33: T-2 依頼活用尾 ない + ?（不満の報告との同綴は承知のうえ）" \
  "$(write_input ac2-33 '直してくれない?')" "直して" "くれ"

expect_block "AC-2-34: T-2 依頼活用尾 ませんか + 終端" \
  "$(write_input ac2-34 '直していただけませんか')" "直して" "いただけ"

expect_block "AC-2-35: T-2 依頼接尾の連接（おいて+ください）" \
  "$(write_input ac2-35 '対応しておいてください')" "対応して" "おいて" "ください"

expect_block "AC-2-36: T-2 連接 + て連結（もらっ+て+ください）" \
  "$(write_input ac2-36 '直してもらってください')" "直して" "もらっ" "ください"

expect_block "AC-2-37: T-2 依頼活用尾 ますか + 終端" \
  "$(write_input ac2-37 '実装してくれますか')" "実装して" "くれ"

expect_block "AC-2-38: T-2 依頼活用尾 ていい + ?" \
  "$(write_input ac2-38 '直してもらっていい?')" "直して" "もらっ"

expect_block "AC-2-39: T-2 依頼活用尾 てもいい + ?" \
  "$(write_input ac2-39 '直してもらってもいい?')" "直して" "もらっ"

expect_block "AC-2-40: T-2 依頼活用尾 ないか + 終端" \
  "$(write_input ac2-40 '直してもらえないか')" "直して" "もらえ"

expect_block "AC-2-41: T-2 依頼活用尾 ます + ?" \
  "$(write_input ac2-41 '直していただけます?')" "直して" "いただけ"

expect_block "AC-2-42: T-2 依頼活用尾 たい + 終端" \
  "$(write_input ac2-42 '直してもらいたい')" "直して" "もらい"

# --- ここまで 2-30〜2-42 ------------------------------------------------------

# AC-6-2 の回避手段: 上の偽陽性（AC-2-8）は [direct] を付ければ通る。
expect_pass "AC-6-2: 偽陽性は [direct] で回避できる" \
  "$(write_input ac6-2 '[direct] docs/rules/scope.md に1行追加して')"

# ==============================================================================
# AC-3: 工程をまたがない依頼は通す
# ==============================================================================

expect_pass "AC-3-1: 質問（ADR の内容）" \
  "$(write_input ac3-1 'ADR 0010 の §D は何を決めている?')"

expect_pass "AC-3-2: 質問（make verify）" \
  "$(write_input ac3-2 'make verify は何を呼ぶ?')"

expect_pass "AC-3-3: 要約は B-1 に無い" \
  "$(write_input ac3-3 'docs/rules/ を読んで要約して')"

expect_pass "AC-3-4: 調査は B-1 に無い" \
  "$(write_input ac3-4 'CI が落ちた原因を調査して')"

expect_pass "AC-3-5: 説明は B-1 に無い" \
  "$(write_input ac3-5 'check-domain-deps が -test を付ける理由を説明して')"

expect_pass "AC-3-6: 番号の言及のみ（B-2 着手表現が無い）" \
  "$(write_input ac3-6 '#30 の背景を教えて')"

expect_pass "AC-3-7: 空文字列（判定対象が無い）" \
  "$(write_input ac3-7 '')"

expect_pass "AC-3-8: 空白のみ" \
  "$(write_input ac3-8 '   ')"

expect_pass "AC-3-9: fix が fixture / prefix に誤一致しない" \
  "$(write_input ac3-9 'fixture の prefix を確認したい')"

# --- ここから 3-10〜3-22（実測の偽陽性4件 + 進行形/仮定/可能形/語境界/意図的な偽陰性） ---
# 3-10〜3-13 は実測で確認された偽陽性そのもの。旧実装（名詞の裸一致）ではブロック
# されてしまう。ここが今回の Red の本体（docs/specs/orchestrator-entry-hook.md AC-3）。

expect_pass "AC-3-10: 名詞形。実装の直後がど（実測の偽陽性）" \
  "$(write_input ac3-10 'この実装どうなってる?')"

expect_pass "AC-3-11: 名詞形。実装の直後が助詞を。説明はB-1に無い（実測の偽陽性）" \
  "$(write_input ac3-11 '今の実装を説明して')"

expect_pass "AC-3-12: 名詞形。修正の直後が助詞の（実測の偽陽性）" \
  "$(write_input ac3-12 'この修正の意図を教えて')"

expect_pass "AC-3-13: 名詞形。修正の直後が箇（実測の偽陽性）" \
  "$(write_input ac3-13 'テストの修正箇所はどこ?')"

expect_pass "AC-3-14: T-1/T-2に無い形（していの直後がい）" \
  "$(write_input ac3-14 'もう実装している?')"

expect_pass "AC-3-15: T-1/T-2に無い形（してあるの直後があ）" \
  "$(write_input ac3-15 'この機能は実装してある?')"

expect_pass "AC-3-16: T-1/T-2に無い形（してもの直後がも）" \
  "$(write_input ac3-16 'ここを修正しても大丈夫?')"

expect_pass "AC-3-17: 五段動詞の可能形（命令形と同綴のため拾わない帰結）" \
  "$(write_input ac3-17 'この関数は直せる?')"

expect_pass "AC-3-18: 書い+ての直後があ（してあると同型）" \
  "$(write_input ac3-18 'spec にそう書いてある')"

expect_pass "AC-3-19: 着手の直後が条。B-2も指示形に絞る帰結" \
  "$(write_input ac3-19 '#30 の着手条件は?')"

expect_pass "AC-3-20: implement の語境界に誤一致しない（implementation）" \
  "$(write_input ac3-20 'implementation の方針を教えて')"

expect_pass "AC-3-21: 偽陰性。仕様どおり（助詞を挟む依頼はB-1-dで拾わない）" \
  "$(write_input ac3-21 '実装をお願いします')"

expect_pass "AC-3-22: fix を語彙から外した帰結" \
  "$(write_input ac3-22 'Where is the fix?')"

# --- 3-23〜3-25: 終助詞の非依頼用法（B-1-c の境界値） ------------------------
# 終助詞を「区切りそのもの」ではなく「区切りの前に置ける文字」として定義した
# ことの境界値。ここを取り違える（終助詞を区切り集合に直接入れる）と
# 一律ブロックされ、AC-6-2 が禁じる偽陽性になる（docs/specs/
# orchestrator-entry-hook.md AC-3 3-23〜3-25 の解説）。

expect_pass "AC-3-23: 終助詞よの直後がか。区切りでない（修正してよかった）" \
  "$(write_input ac3-23 '修正してよかった')"

expect_pass "AC-3-24: 終助詞なの直後がい。否定であって依頼ではない" \
  "$(write_input ac3-24 'まだ実装してないよね')"

expect_pass "AC-3-25: 終助詞ねの直後がぇ。口語の否定形" \
  "$(write_input ac3-25 '対応してねぇの?')"

# --- ここまで 3-10〜3-25 ----------------------------------------------------

# --- 3-26〜3-33: T-2 境界規則の境界値（往復2 W-1。実測ですべてブロックされて
#     いた偽陽性）。仕様は T-2 を T-1 と同じ二段構え（依頼接尾(1+) + 依頼活用尾
#     (0..1) + 終助詞*(0+) + 区切り）へ改めた。当初の T-2 は「て+依頼接尾」で
#     閉じており、接尾の直後が何であっても一致していたため、過去形の報告
#     （〜てくれた）・否定過去（〜てもらえなかった）・引用伝聞（〜てほしいと
#     言われた）・連体修飾（〜てもらった件）・過去+疑問（〜てくれたのは誰？）・
#     複合の報告・感謝（〜てくれて、ありがとう）がすべてブロックされていた。
#     この8件は AC-9-14（T-2 境界規則そのものの負例）を満たす。この負例が無いと
#     境界規則を実装から落としても Green のままになる（同仕様 AC-9-14）。
#     現行実装の T2 パターンは境界規則を持たない（て+REQ で閉じる）ため、
#     本コミット時点では以下すべてが Red になる想定（今回の Red の本体）。

expect_pass "AC-3-26: T-2 くれの直後がた。過去形の報告" \
  "$(write_input ac3-26 '実装してくれた')"

expect_pass "AC-3-27: T-2 過去+疑問。？は区切りだが接尾の直後ではない" \
  "$(write_input ac3-27 '修正してくれたのは誰？')"

expect_pass "AC-3-28: T-2 もらえの直後がなかっ。依頼活用尾ないと不一致" \
  "$(write_input ac3-28 '対応してもらえなかった')"

expect_pass "AC-3-29: T-2 もらいの直後がました。依頼活用尾ますと不一致" \
  "$(write_input ac3-29 '実装してもらいました')"

expect_pass "AC-3-30: T-2 もらっの直後がた。連体修飾" \
  "$(write_input ac3-30 '対応してもらった件について')"

expect_pass "AC-3-31: T-2 ほしいの直後がと。引用・伝聞" \
  "$(write_input ac3-31 '実装してほしいと言われた')"

expect_pass "AC-3-32: T-2 複合の報告。おいて+くれと連接するが直後がな" \
  "$(write_input ac3-32 'なぜ対応しておいてくれなかったのか')"

expect_pass "AC-3-33: T-2 末尾接尾直後にてを許さない帰結。感謝・報告" \
  "$(write_input ac3-33 '実装してくれて、ありがとう')"

# --- ここまで 3-26〜3-33 ------------------------------------------------------

# --- AC-6-6 / AC-6-7: 意図的な偽陰性を「通る」こととして固定する ----------------
# ここは検知漏れの放置ではなく仕様が決めた挙動（docs/specs/orchestrator-entry-hook.md
# AC-6-6 / AC-6-7）。将来これを塞ぐ変更が入ったときに気づけるよう、
# 「通ること」自体をテストで固定する。

expect_pass "AC-6-6: 助詞を挟む依頼は偽陰性へ回る（修正のほうお願い）" \
  "$(write_input ac6-6 '修正のほうお願い')"

expect_pass "AC-6-7: 英語は fix を拾わない（fix this bug）" \
  "$(write_input ac6-7 'fix this bug')"

# ==============================================================================
# AC-4 / AC-5 は AC-1・AC-2・AC-3 の expect_block / expect_pass に埋め込み済み
# （終了コード・stdout/stderr の出力有無を毎ケースで検査している）。
# ==============================================================================

# ==============================================================================
# AC-7: 判定不能な入力は警告して通す（rc=1、ブロック側に倒さない）
# ==============================================================================

: > "${WORK}/ac7-1-empty.json"
expect_warn "AC-7-1: stdin が空" \
  "${WORK}/ac7-1-empty.json"

printf '{ this is not valid json' > "${WORK}/ac7-2-broken.json"
expect_warn "AC-7-2: stdin が JSON として壊れている" \
  "${WORK}/ac7-2-broken.json"

jq -n '{
  cwd: "/workspaces/effort-tracker",
  hook_event_name: "UserPromptSubmit",
  permission_mode: "default",
  prompt_id: "test-prompt-id",
  session_id: "test-session-id",
  transcript_path: "/tmp/test-transcript.jsonl"
}' > "${WORK}/ac7-3-noprompt.json"
expect_warn "AC-7-3: JSON に prompt キーが無い" \
  "${WORK}/ac7-3-noprompt.json"

# ==============================================================================
# AC-8: 入力の頑健性
# ==============================================================================

# 8-1: 複数行。冒頭判定は最初の非空白文字。B-1/B-2 は全体を対象に判定する。
expect_block "AC-8-1: 複数行プロンプトの全体を判定する" \
  "$(write_input ac8-1 $'1行目は質問です\n2行目\n実装して')" "実装"

# 8-7: 改行が CRLF の場合。\r を T-1 の区切りとして扱う（B-1-c）。
# 往復1 W-3（実測）で見つかった穴。$ は \n の直前には一致するが、CRLF では
# て と行末の間に \r が入るため、\r を区切りに含めない実装では T-1 が
# 成立しない。AC-2-29 と同じ入力を AC-8 側の受け入れ条件としても固定する。
expect_block "AC-8-7: 改行が CRLF（\\r を区切りとして扱う）" \
  "$(write_input ac8-7 $'実装して\r\nよろしく')" "実装して"

# 8-8: 8-7 と対。CRLF で貼り付けた [direct] は通る（AC-1、1-13 と同じ入力を
# AC-8 側の受け入れ条件としても固定する）。\r を区切りに足してもブロック側
# だけに倒すと、CRLF 入力に対して唯一の逃げ道が塞がってしまう（往復2 W-3）。
expect_pass "AC-8-8: CRLF で貼り付けた [direct]（8-7 と対）" \
  "$(write_input ac8-8 $'\r\n[direct] 実装して')"

# 8-2: JSON エスケープ（\n / \" / \\）を解いたうえで判定する。
# jq が組み立てる JSON ではなく、生の JSON テキストを手書きして
# \n \" \\ という *3文字表現のエスケープ* をそのままファイルに置く
# （シングルクオート heredoc なのでシェルはこれらを一切解釈しない）。
cat > "${WORK}/ac8-2-escaped.json" <<'EOF'
{"cwd":"/workspaces/effort-tracker","hook_event_name":"UserPromptSubmit","permission_mode":"default","prompt":"\n\n[direct] \"引用符\" 実装して\\経由","prompt_id":"p","session_id":"s","transcript_path":"/tmp/t.jsonl"}
EOF
expect_pass "AC-8-2: JSON エスケープを解いたうえで [direct] 冒頭判定する" \
  "${WORK}/ac8-2-escaped.json"

# 8-3: 数千文字のプロンプトを切り捨てずに判定する（末尾に検知語を置く）。
LONG_FILLER="$(printf 'a%.0s' {1..5000})"
expect_block "AC-8-3: 数千文字のプロンプトを切り捨てずに判定する" \
  "$(write_input ac8-3 "${LONG_FILLER} 実装して")" "実装"

# 8-4: シェルメタ文字（引用符・バッククォート・\$(...)）を含んでもシェルへ
# 展開しない。副作用ファイルが実際に作られていないことまで確認する
# （展開されていれば touch が実行され、SIDE_EFFECT ファイルが生成されてしまう）。
SIDE_EFFECT_BLOCK="${WORK}/side-effect-block.txt"
SIDE_EFFECT_PASS="${WORK}/side-effect-pass.txt"
rm -f "$SIDE_EFFECT_BLOCK" "$SIDE_EFFECT_PASS"

PROMPT_METACHAR_BLOCK="\`id\`; \$(touch ${SIDE_EFFECT_BLOCK}); \"二重引用符\"; 実装して"
expect_block "AC-8-4: シェルメタ文字を含んでもブロック判定できる（展開しない）" \
  "$(write_input ac8-4-block "$PROMPT_METACHAR_BLOCK")" "実装"
if [ -e "$SIDE_EFFECT_BLOCK" ]; then
  fail=$((fail + 1))
  echo "  FAIL AC-8-4: プロンプト中の \$(touch ...) が実際に実行されてしまった（block 経路）"
else
  pass=$((pass + 1))
  echo "  ok   AC-8-4: block 経路でシェルメタ文字が実行されていない"
fi

PROMPT_METACHAR_PASS="[direct] \`id\`; \$(touch ${SIDE_EFFECT_PASS}); \"二重引用符\"; 実装して"
expect_pass "AC-8-4: [direct] 側でもシェルメタ文字を展開しない" \
  "$(write_input ac8-4-pass "$PROMPT_METACHAR_PASS")"
if [ -e "$SIDE_EFFECT_PASS" ]; then
  fail=$((fail + 1))
  echo "  FAIL AC-8-4: プロンプト中の \$(touch ...) が実際に実行されてしまった（pass 経路）"
else
  pass=$((pass + 1))
  echo "  ok   AC-8-4: pass 経路でシェルメタ文字が実行されていない"
fi

# 8-5: prompt 以外のキー（cwd / session_id）に B-1 の語が入っていても判定に
# 影響しない。取り出すのは prompt の値のみ（P-3）。
jq -n '{
  cwd: "/workspaces/effort-tracker/実装して/追加して",
  hook_event_name: "UserPromptSubmit",
  permission_mode: "default",
  prompt: "ADR 0010 の内容を教えて",
  prompt_id: "test-prompt-id/修正して",
  session_id: "session/削除して",
  transcript_path: "/tmp/実装して.jsonl"
}' > "${WORK}/ac8-5-otherkeys.json"
expect_pass "AC-8-5: prompt 以外のキーの検知語は判定に影響しない" \
  "${WORK}/ac8-5-otherkeys.json"

# 8-6: user_input キーを持ち prompt を持たない JSON は AC-7-3 として扱う
# （通す + 警告）。user_input へフォールバックしない。
# user_input の中身にわざと B-1 の語（実装して）を入れておき、フォールバックする
# 実装であれば rc=2（ブロック）になってしまうことで弁別できるようにする。
jq -n '{
  cwd: "/workspaces/effort-tracker",
  hook_event_name: "UserPromptSubmit",
  permission_mode: "default",
  user_input: "実装して",
  prompt_id: "test-prompt-id",
  session_id: "test-session-id",
  transcript_path: "/tmp/test-transcript.jsonl"
}' > "${WORK}/ac8-6-userinput.json"
expect_warn "AC-8-6: user_input のみの JSON は prompt へフォールバックせず AC-7-3 扱い" \
  "${WORK}/ac8-6-userinput.json"

# ==============================================================================
# AC-9: 語彙表の各行に fixture を対応させる
# ==============================================================================
# docs/specs/orchestrator-entry-hook.md AC-9。判定基準は「語彙表の1行を削ると
# fixture が最低1件 Red になる」こと。したがって各ケースは、対象の語1つだけが
# ブロック理由になるよう単独の入力にする（他の語と同居させると、対象語を
# 消しても別の語でブロックが成立してしまい、ミューテーションを検知できない）。
# 往復1 のレビューはミューテーション13件のうち11件の生存を確認しており
# （下記コメントで対応箇所を明記）、本節はその実測に対する応答である。
#
# 9-4（終助詞 ね/よ/な/繰り返し）は AC-2-22〜2-25 で、9-3 の 終端/\r/閉じ記号は
# AC-2-21/2-29・AC-8-7・AC-2-26/2-27 で、9-10（implementation の語境界負例）は
# AC-3-20 で、それぞれ既に単独 fixture として固定済みのため重複させない。

# --- 9-1: B-1-a の S。作成/削除/リファクタ(単独)/コミットは既存 fixture に
#     無いため新規に追加する（実装/修正/対応/追加/起票は AC-2-1/2-10/2-11/2-2/
#     2-6/2-20 で既にカバー済み）。「S の 削除 コミット」は往復1 レビューで
#     生存が確認されたミューテーション。
expect_block "AC-9-1a: S 作成" \
  "$(write_input ac9-1a 'ドキュメントを作成して')" "作成して"

expect_block "AC-9-1b: S 削除（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-1b 'このファイルを削除して')" "削除して"

expect_block "AC-9-1c: S リファクタ（リファクタリングを含まない単独形）" \
  "$(write_input ac9-1c 'この関数をリファクタして')" "リファクタして"

expect_block "AC-9-1d: S コミット（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-1d '変更をコミットして')" "コミットして"

# --- 9-2: B-1-b の V。消し/書き換え/置き換えは既存 fixture に無いため追加する
#     （直し/作っ/書い は AC-2-4/2-19/2-18 で既にカバー済み）。
#     「V の 消し 書き換え 置き換え」は往復1 レビューで生存が確認された
#     ミューテーション。
expect_block "AC-9-2a: V 消し（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-2a 'このログを消して')" "消して"

expect_block "AC-9-2b: V 書き換え（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-2b 'この関数を書き換えて')" "書き換えて"

expect_block "AC-9-2c: V 置き換え（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-2c 'この変数を置き換えて')" "置き換えて"

# --- 9-3: T-1 区切りの各要素。終端/\r/閉じ記号は上記のとおり既存 fixture で
#     カバー済み。ここでは \n・空白3種（半角/全角/タブ）・句読点群(。) を追加
#     する。「SEP の 。 と全角スペース」は往復1 レビューで生存が確認された
#     ミューテーション。
expect_block "AC-9-3a: T-1 区切り \\n（文字列途中の改行）" \
  "$(write_input ac9-3a $'実装して\n続きのテキスト')" "実装して"

expect_block "AC-9-3b: T-1 区切り 半角スペース" \
  "$(write_input ac9-3b '実装して 明日には')" "実装して"

expect_block "AC-9-3c: T-1 区切り 全角スペース（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-3c '実装して　続き')" "実装して"

expect_block "AC-9-3d: T-1 区切り タブ" \
  "$(write_input ac9-3d $'実装して\t続き')" "実装して"

expect_block "AC-9-3e: T-1 区切り 句読点（。往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-3e 'このバグを直して。あとでレビューします')" "直して"

# --- 9-3 続き（往復2 W-4）: 9-3 は群粒度（句読点群/閉じ記号群）から26要素へ
#     改められた。往復2 の実測では16要素（．，！？』）”：；.,!?)"; ）が群粒度
#     のまま生存していた。ここではその16要素に加え、上記で未カバーの残り
#     （、/CR は既存 fixture でカバー済みのため対象外）をすべて単独 fixture へ
#     分解する。他語との同居を避けるため、て の直後に対象文字1つだけを置き、
#     直後には非依頼・非検知語の「続き」を続ける（対象を削っても他要素で
#     ブロックが成立しないようにする）。

expect_block "AC-9-3f: T-1 区切り 、（読点）" \
  "$(write_input ac9-3f '実装して、続き')" "実装して"

expect_block "AC-9-3g: T-1 区切り ．（全角ピリオド）" \
  "$(write_input ac9-3g '実装して．続き')" "実装して"

expect_block "AC-9-3h: T-1 区切り ，（全角カンマ）" \
  "$(write_input ac9-3h '実装して，続き')" "実装して"

expect_block "AC-9-3i: T-1 区切り .（半角ピリオド。往復2 W-4 生存要素）" \
  "$(write_input ac9-3i '実装して.続き')" "実装して"

expect_block "AC-9-3j: T-1 区切り ,（半角カンマ。往復2 W-4 生存要素）" \
  "$(write_input ac9-3j '実装して,続き')" "実装して"

expect_block "AC-9-3k: T-1 区切り !（半角。往復2 W-4 生存要素）" \
  "$(write_input ac9-3k '実装して!続き')" "実装して"

expect_block "AC-9-3l: T-1 区切り ！（全角。往復2 W-4 生存要素）" \
  "$(write_input ac9-3l '実装して！続き')" "実装して"

expect_block "AC-9-3m: T-1 区切り ?（半角。往復2 W-4 生存要素）" \
  "$(write_input ac9-3m '実装して?続き')" "実装して"

expect_block "AC-9-3n: T-1 区切り ？（全角。往復2 W-4 生存要素）" \
  "$(write_input ac9-3n '実装して？続き')" "実装して"

expect_block "AC-9-3o: T-1 区切り 』（往復2 W-4 生存要素）" \
  "$(write_input ac9-3o '実装して』続き')" "実装して"

expect_block "AC-9-3p: T-1 区切り ）（全角閉じ括弧。往復2 W-4 生存要素）" \
  "$(write_input ac9-3p '実装して）続き')" "実装して"

expect_block "AC-9-3q: T-1 区切り )（半角閉じ括弧。往復2 W-4 生存要素）" \
  "$(write_input ac9-3q '実装して)続き')" "実装して"

expect_block "AC-9-3r: T-1 区切り ”（全角閉じ引用符。往復2 W-4 生存要素）" \
  "$(write_input ac9-3r '実装して”続き')" "実装して"

expect_block "AC-9-3s: T-1 区切り \"（半角二重引用符。往復2 W-4 生存要素）" \
  "$(write_input ac9-3s '実装して"続き')" "実装して"

expect_block "AC-9-3t: T-1 区切り ：（全角コロン。往復2 W-4 生存要素）" \
  "$(write_input ac9-3t '実装して：続き')" "実装して"

expect_block "AC-9-3u: T-1 区切り ;（半角セミコロン。往復2 W-4 生存要素）" \
  "$(write_input ac9-3u '実装して;続き')" "実装して"

expect_block "AC-9-3v: T-1 区切り ；（全角セミコロン。往復2 W-4 生存要素）" \
  "$(write_input ac9-3v '実装して；続き')" "実装して"

# --- 9-5: T-2 依頼接尾。ください/もらえ は AC-2-12/2-13 で既にカバー済み。
#     残る9語（くれ/下さい/ほしい/欲しい/おいて/もらい/もらっ/いただけ/
#     いただき）を追加する。「依頼接尾の 下さい ほしい」は往復1 レビューで
#     生存が確認されたミューテーション。欲しい は同グループの境界（表記ゆれ）
#     として同様に追加する。
expect_block "AC-9-5a: 依頼接尾 くれ" \
  "$(write_input ac9-5a 'このバグを直してくれ')" "直して" "くれ"

expect_block "AC-9-5b: 依頼接尾 下さい（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-5b 'このテストを修正して下さい')" "修正して" "下さい"

expect_block "AC-9-5c: 依頼接尾 ほしい（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-5c 'この関数を直してほしい')" "直して" "ほしい"

expect_block "AC-9-5d: 依頼接尾 欲しい" \
  "$(write_input ac9-5d 'この関数を直して欲しい')" "直して" "欲しい"

expect_block "AC-9-5e: 依頼接尾 おいて" \
  "$(write_input ac9-5e '資料を作成しておいて')" "作成して" "おいて"

expect_block "AC-9-5f: 依頼接尾 もらい" \
  "$(write_input ac9-5f 'この関数を直してもらいたい')" "直して" "もらい"

expect_block "AC-9-5g: 依頼接尾 もらっ" \
  "$(write_input ac9-5g 'この関数を直してもらっていい?')" "直して" "もらっ"

expect_block "AC-9-5h: 依頼接尾 いただけ" \
  "$(write_input ac9-5h 'この関数を直していただけますか')" "直して" "いただけ"

expect_block "AC-9-5i: 依頼接尾 いただき" \
  "$(write_input ac9-5i 'この関数を直していただきたい')" "直して" "いただき"

# --- 9-6: B-2 の各語。やっ/お願い は AC-2-3/2-9 で既にカバー済み。
#     残る4語（進め/片付け/着手/おねがい）を追加する。「B-2 の 進め 片付け」は
#     往復1 レビューで生存が確認されたミューテーション。
expect_block "AC-9-6a: B-2 進め（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-6a '#30 を進めて')" "#30" "進めて"

expect_block "AC-9-6b: B-2 片付け（往復1 レビュー生存ミューテーション）" \
  "$(write_input ac9-6b '#30 を片付けて')" "#30" "片付けて"

expect_block "AC-9-6c: B-2 着手" \
  "$(write_input ac9-6c '#30 に着手して')" "#30" "着手して"

expect_block "AC-9-6d: B-2 おねがい" \
  "$(write_input ac9-6d 'Issue #30 おねがい')" "#30" "おねがい"

# --- 9-7: B-1-a の接続 せよ/お願い/おねがい、T-3 といて、T-4 ろ。
#     せよ/といて/ろ は AC-2-16/2-14/2-15 で既にカバー済み。
#     B-1-a 直結の お願い/おねがい（B-2 の依頼単独とは別条件）を追加する。
expect_block "AC-9-7a: B-1-a 接続 お願い（S直後、しを介さない）" \
  "$(write_input ac9-7a '実装お願いします')" "実装お願い"

expect_block "AC-9-7b: B-1-a 接続 おねがい（S直後、しを介さない）" \
  "$(write_input ac9-7b '修正おねがいします')" "修正おねがい"

# --- 9-8: B-1-e implement の大文字小文字非区別。小文字は AC-2-7 でカバー
#     済みのため、全角大文字・先頭大文字を追加する。
expect_block "AC-9-8a: implement 大文字小文字非区別（IMPLEMENT）" \
  "$(write_input ac9-8a 'Please IMPLEMENT this feature')" "IMPLEMENT"

expect_block "AC-9-8b: implement 大文字小文字非区別（Implement）" \
  "$(write_input ac9-8b 'Please Implement this feature')" "Implement"

# --- 9-9: T-4（ろ）が B-1-b に適用されないことの負例。「直しろ」は非文であり
#     五段動詞の命令形「直せ」と同型の理由で拾わない（B-1-b 注記）。
#     この fixture が Red になったら、ろ が誤って V にも適用されたことを示す。
expect_pass "AC-9-9: T-4（ろ）は B-1-b に適用されない（直しろは通る）" \
  "$(write_input ac9-9 '直しろ')"

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
