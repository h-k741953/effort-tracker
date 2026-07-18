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
  local any=0 c
  for c in "$@"; do
    grep -qF -- "$c" <<< "$ERR" && any=1
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

# --- ここまで 3-10〜3-22 ----------------------------------------------------

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

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
