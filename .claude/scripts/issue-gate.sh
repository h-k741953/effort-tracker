#!/usr/bin/env bash
# `/issue` の機械判定部（Issue #31 / ADR 0012）。
#
# 【この対象・fixture について】
#   単一情報源は docs/specs/issue-command.md。判定条件（AC-1〜AC-3, AC-6-b, AC-7,
#   AC-10）はそちらに書いてあり、このファイルはそれを実装したものであって
#   条件を書き写さない。fixture は .claude/scripts/test-issue-gate.sh。
#
# 【純粋なフィルタであること（仕様「対象」節）】
#   このスクリプトは stdin と引数以外の入力を持たない。gh を呼ばない
#   （ネットワークアクセスを内包しない）。gate モードが受け取る JSON は
#   呼び出し側（.claude/commands/issue.md、AI）が `gh issue view` の出力を
#   そのまま渡す想定（P-2）。入力を差し替えるだけで完全にローカル実行・検査
#   できることを保証する（.github/scripts/check-review-trail.sh と同じ設計）。
#
# 【出力形式（AC-4）】
#   stdout 1行目は "VERDICT: <名前>"、2行目以降は "<キー>: <値>"（機械可読）。
#   説明文は stderr のみに出す。終了コードは 0 / 1 / 3 / 4 のみを使う
#   （2 は .claude/hooks/check-prompt-entry.sh が UserPromptSubmit の
#   「ブロック」に使う値であり、同じ数字に別の意味を持たせない。AC-4-3）。
#
# 【外部入力をシェルへ展開しないこと（AC-7-1 / AC-10-1 / AC-10-2 / AC-8-5）】
#   引数は "$1" 等クオート付きの変数参照でのみ扱い、eval や `bash -c "$x"`
#   は使わない。gate モードの body は stdin → ファイル → jq 抽出 → ファイルの
#   経路のみを通り、コマンドラインへ展開しない（大きくても切り捨てない）。
set -uo pipefail

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# --- 出力ヘルパ ------------------------------------------------------------
#
# finish は「VERDICT 行 → 付随する値の行 → （必要なら）stderr 説明」を出して
# そのままプロセスを終了する。全 mode がここを通ることで AC-4 の形式を
# 一箇所に閉じ込める（mode ごとに printf 形式がずれることを防ぐ）。
finish() {
  local verdict="$1" rc="$2" msg="$3"
  shift 3
  printf 'VERDICT: %s\n' "$verdict"
  local line
  for line in "$@"; do
    [ -n "$line" ] && printf '%s\n' "$line"
  done
  if [ -n "$msg" ]; then
    printf '[issue-gate] %s\n' "$msg" >&2
  fi
  exit "$rc"
}

# ==============================================================================
# mode arg: issue-gate.sh arg <引数文字列>  ― AC-1
# ==============================================================================
#
# 【設計判断: MULTIPLE_ARG と INVALID_ARG の切り分け】
#   `31 32`（AC-1-8）は MULTIPLE_ARG、`31; touch ...`（AC-1-12）は INVALID_ARG。
#   どちらも空白区切りで複数トークンに割れるが、期待する verdict が異なる。
#   したがって「複数トークンかどうか」だけでは弁別できず、「複数トークンの
#   *すべてが* 単独で有効な Issue 番号か」で切り分ける。すべて有効なら
#   「複数の Issue 番号が渡された」＝ MULTIPLE_ARG、1つでも無効なトークンが
#   混ざれば「番号ではない何か（ゴミ・注入の試み）」＝ INVALID_ARG とする。
#
# 【trim と分割を read -a に任せる理由】
#   `read -r -a tokens <<< "$argstr"` は IFS（空白・タブ・改行）で分割しつつ
#   前後の空白を自動的に無視する。空文字列・空白のみの入力は tokens が
#   0個になるため、AC-1-4 / AC-1-5（NO_ARG）を分岐なしで拾える。
do_arg() {
  local argstr="${1:-}"
  local tokens=()
  read -r -a tokens <<< "$argstr"
  local n="${#tokens[@]}"

  if [ "$n" -eq 0 ]; then
    finish NO_ARG 3 "引数が無い。番号を推測しない（仕様 AC-1-4/1-5, AC-6-1）。"
  fi

  if [ "$n" -eq 1 ]; then
    local norm
    if norm="$(normalize_issue_token "${tokens[0]}")"; then
      finish OK 0 "" "Issue番号: ${norm}"
    fi
    finish INVALID_ARG 3 "引数「${tokens[0]}」は Issue 番号として解釈できない。"
  fi

  # n > 1: 全トークンが有効な番号なら MULTIPLE_ARG、そうでなければ INVALID_ARG。
  local all_valid=1 t
  for t in "${tokens[@]}"; do
    if ! normalize_issue_token "$t" > /dev/null; then
      all_valid=0
      break
    fi
  done
  if [ "$all_valid" -eq 1 ]; then
    finish MULTIPLE_ARG 3 "複数の Issue 番号が渡された。1コマンド1 Issue（仕様 AC-1-8, AC-6-2）。"
  fi
  finish INVALID_ARG 3 "引数「${argstr}」は Issue 番号として解釈できない。"
}

# 単一トークンを Issue 番号として正規化する。
# 先頭の "#" は許すが（AC-1-2）、それ以外は「1以上・先頭ゼロなし・数字のみ」
# （AC-1-9 / AC-1-10 / AC-1-11 / AC-1-6 / AC-1-7）のみを有効とする。
# 有効なら正規化した番号を stdout に出して 0 を返す。無効なら何も出さず 1。
normalize_issue_token() {
  local t="$1"
  case "$t" in
    '#'*) t="${t#\#}" ;;
  esac
  if [[ "$t" =~ ^[1-9][0-9]*$ ]]; then
    printf '%s' "$t"
    return 0
  fi
  return 1
}

# ==============================================================================
# mode gate: issue-gate.sh gate <リポジトリルート>  ― AC-2 / AC-3 / AC-10
# ==============================================================================
#
# stdin から P-2 のスキーマの JSON を読み、AC-2 の優先順位（順1〜順8）で
# 判定する。上位で確定したら下位を評価しない（早期 finish で表現する）。
do_gate() {
  local root="${1:-}"
  local payload="${WORK}/gate-payload.json"
  cat > "$payload"

  # --- 順1: stdin が空 / JSON 破損 / P-2 の必須キーを欠く -----------------
  if [ ! -s "$payload" ]; then
    finish INDETERMINATE 1 "stdin が空。ゲート判定不能（仕様 AC-2 順1）。"
  fi
  if ! jq -e 'type == "object"' "$payload" > /dev/null 2>&1; then
    finish INDETERMINATE 1 "stdin が JSON として壊れている（仕様 AC-2 順1）。"
  fi

  # P-2 の必須キー（number, title, body, labels, state）。
  # 【キーが無い場合と値が null の場合を同一視する理由（AC-10-4 / AC-10-5）】
  #   「キーごと欠落」も「キーはあるが null」も、このスクリプトから見れば
  #   同じ「判定に必要な値が無い」状態であり、区別しても後続の判定は
  #   どのみち行えない。空配列・空文字列へフォールバックしない
  #   （フォールバックすると payload 形式が変わったときに壊れたことへ
  #   気づけなくなる。AC-10-4 の注記）。
  local key
  for key in number title body labels state; do
    if ! jq -e "has(\"${key}\") and (.${key} != null)" "$payload" > /dev/null 2>&1; then
      finish INDETERMINATE 1 "必須キー '${key}' が無い、または null（仕様 P-2 / AC-2 順1 / AC-10-4 / AC-10-5）。"
    fi
  done

  # --- 値の取り出し（body はファイル経由。シェルへ展開しない。AC-10-1/10-2）---
  local body_file="${WORK}/body.txt"
  jq -r '.body' "$payload" > "$body_file"
  local state_val
  state_val="$(jq -r '.state' "$payload")"
  local labels_file="${WORK}/labels.txt"
  jq -r '.labels[]?.name // empty' "$payload" > "$labels_file"

  # --- 順2: state が OPEN でない ------------------------------------------
  # 大文字小文字を吸収しない（P-2 は "OPEN" と定める。AC-10-6）。
  if [ "$state_val" != "OPEN" ]; then
    finish NOT_OPEN 3 "state='${state_val}' は OPEN でない（仕様 AC-2 順2, AC-10-6, AC-9-7）。" \
      "state: ${state_val}"
  fi

  # --- 順3〜5: ラベル分岐 -------------------------------------------------
  # 完全一致・大文字小文字を吸収しない（AC-3）。grep -x -F で行全体一致。
  local has_task=0 has_discussion=0
  grep -qxF 'task' "$labels_file" && has_task=1
  grep -qxF 'discussion' "$labels_file" && has_discussion=1

  local labels_joined
  labels_joined="$(paste -sd ',' "$labels_file" 2> /dev/null || true)"

  if [ "$has_task" -eq 1 ] && [ "$has_discussion" -eq 1 ]; then
    finish LABEL_CONFLICT 3 "labels に task と discussion の両方がある（仕様 AC-2 順3, AC-3-4）。" \
      "ラベル一覧: ${labels_joined}"
  fi
  if [ "$has_discussion" -eq 1 ]; then
    # 順4: task は無い時点でここに来る。DISCUSSION は工程を回さない
    # （仕様 AC-3-3, AC-5-5）。
    finish DISCUSSION 4 "labels に discussion がある（task は無い）。工程を回さない（仕様 AC-2 順4, AC-3-3/3-8）。" \
      "ラベル一覧: ${labels_joined}"
  fi
  if [ "$has_task" -eq 0 ]; then
    finish LABEL_UNKNOWN 3 "labels に task が無い。ラベルを AI が推測して付けない（仕様 AC-2 順5, AC-3-5/3-6/3-7）。" \
      "ラベル一覧: ${labels_joined}"
  fi

  # --- 順6/順7: SDD ゲート（AC-2-a） --------------------------------------
  # パス抽出は body 全体に正規表現を適用し重複を除いた集合とする（AC-2-a）。
  # body の自然言語（指示・命令・依頼）は一切見ない（AC-2-b）。抽出後は
  # 「実在するかどうか」という機械的な事実だけを見る。
  local paths_file="${WORK}/paths.txt"
  grep -oE 'docs/specs/[A-Za-z0-9._/-]+\.md' "$body_file" 2> /dev/null \
    | sort -u > "$paths_file" || true

  if [ ! -s "$paths_file" ]; then
    finish SPEC_MISSING_LINK 3 "body に docs/specs/ のパスが1つも無い（仕様 AC-2 順6, AC-2-4/2-7/2-9/2-10）。"
  fi

  # 【実在確認をリポジトリルート配下に閉じる（AC-10-8）】
  #   `realpath -m` でシンボリックな `..` を解決し正規化する（ファイルが
  #   実在しなくても解決できる）。正規化後の絶対パスがリポジトリルートの
  #   配下に収まっていなければ、実在確認の対象にせず「無いもの」として扱う
  #   （SPEC_MISSING_FILE 側に倒す。ADR 0010 §A「弱体化の疑いがある側に倒す」）。
  local root_abs
  root_abs="$(realpath -m -- "$root" 2> /dev/null || true)"
  if [ -z "$root_abs" ]; then
    finish INDETERMINATE 1 "リポジトリルート '${root}' を解決できない。"
  fi

  local missing_file="${WORK}/missing.txt"
  local present_file="${WORK}/present.txt"
  : > "$missing_file"
  : > "$present_file"
  local p full_abs
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    full_abs="$(realpath -m -- "${root_abs}/${p}" 2> /dev/null || true)"
    if [ -n "$full_abs" ] && [[ "$full_abs" == "${root_abs}/"* ]] && [ -f "$full_abs" ]; then
      printf '%s\n' "$p" >> "$present_file"
    else
      printf '%s\n' "$p" >> "$missing_file"
    fi
  done < "$paths_file"

  if [ -s "$missing_file" ]; then
    local extra=()
    while IFS= read -r p; do
      extra+=("不在仕様書パス: ${p}")
    done < "$missing_file"
    finish SPEC_MISSING_FILE 3 "抽出したパスのうち少なくとも1つがリポジトリ内に実在しない（仕様 AC-2 順7, AC-2-5/2-6/2-14, AC-10-8）。" \
      "${extra[@]}"
  fi

  # --- 順8: PROCEED --------------------------------------------------------
  local extra=()
  while IFS= read -r p; do
    extra+=("実在仕様書パス: ${p}")
  done < "$present_file"
  finish PROCEED 0 "" "${extra[@]}"
}

# ==============================================================================
# mode branch: issue-gate.sh branch <Issue番号> <ブランチ名>  ― AC-6-b
# ==============================================================================
#
# 名前の生成は機械化しない（仕様 AC-6-b「名前の生成は機械化しない」）。
# ここで検査するのは形式だけ。
do_branch() {
  local num="${1:-}" name="${2:-}"

  # 構造・文字種の検査（type・番号・スラグの区切り）。長さの上限はここでは
  # 見ない（上限超過は INVALID_FORMAT ではなく TOO_LONG。AC-6-16 の
  # 「スラグ3文字未満は INVALID_FORMAT」との非対称は仕様どおり）。
  #
  # type は docs/rules/commit-convention.md の主な type から test を除いた
  # ものを、AC-6-b が定めた集合として直接使う（このスクリプトが唯一の
  # 実装点であり、.claude/commands/issue.md へは書き写さない。AC-8-11）。
  local re='^(feat|fix|docs|refactor|chore|ci)/([0-9]+)-([a-z0-9]+(-[a-z0-9]+)*)$'
  if [[ ! "$name" =~ $re ]]; then
    finish INVALID_FORMAT 3 "'${name}' はブランチ名の形式に一致しない（仕様 AC-6-b）。"
  fi
  local name_num="${BASH_REMATCH[2]}"
  local slug="${BASH_REMATCH[3]}"

  if [ "${#slug}" -lt 3 ]; then
    finish INVALID_FORMAT 3 "スラグ '${slug}' が3文字未満（仕様 AC-6-16）。"
  fi
  if [ "${#slug}" -gt 40 ]; then
    finish TOO_LONG 3 "スラグ '${slug}' が40文字を超えている（仕様 AC-6-17）。"
  fi
  if [ "${#name}" -gt 60 ]; then
    finish TOO_LONG 3 "ブランチ名全体が60文字を超えている（仕様 AC-6-18）。"
  fi
  if [ "$name_num" != "$num" ]; then
    finish NUMBER_MISMATCH 3 "ブランチ名の番号 '${name_num}' が対象 '${num}' と違う（仕様 AC-6-11）。"
  fi
  finish OK 0 "" "ブランチ名: ${name}"
}

# ==============================================================================
# mode progress: issue-gate.sh progress <Issue番号> <progress.md のパス>  ― AC-7
# ==============================================================================
#
# 対象 Issue 番号の読み取りは「## 対象タスク 見出しから次の見出しまでの
# 範囲に現れる最初の #<数字>」（仕様 AC-7）。
#
# 【awk ではなく bash の while read で書く理由】
#   docs/harness/verification-loop.md「mawk の事故」「同名コマンドの別実装」の
#   注記のとおり、CI の既定 awk（mawk）はローカルの gawk と ERE の解釈が
#   食い違うことがある（区間指定 `#{1,6}` を解釈しなかった実例）。このスクリプト
#   は Makefile 経由でローカル・CI 双方から同じ bash で実行されるため、
#   bash 組み込みの while/=~ を使い、awk 実装差の影響を最初から受けない設計にする。
#
# 【heading（見出し）の判定を「##」固定にせず「#+ 一般」にする理由】
#   .github/scripts/check-review-trail.sh の往復ブロック検出（`^[ \t]*#+`）と
#   同じ判断。見出しレベルをちょうど2に限定すると、レベルの打ち間違い
#   （`### 対象タスク` 等）で「次の見出し」判定が効かなくなる事故を招く
#   （同ファイルの mawk コメントと同型のリスク）。
do_progress() {
  local num="${1:-}" path="${2:-}"

  if [ ! -e "$path" ]; then
    finish ABSENT 0 "" "対象Issue番号: ${num}"
  fi
  if [ ! -s "$path" ]; then
    finish INDETERMINATE 1 "progress.md が空ファイル（仕様 AC-7-6）。"
  fi

  # CRLF でも判定を壊さない（AC-10-7）。全体を正規化してから走査する。
  local norm="${WORK}/progress-normalized.txt"
  tr -d '\r' < "$path" > "$norm"

  local in_section=0 found_section=0 found_number=""
  local line
  # 数字抽出パターンは変数経由で埋め込む。`=~` の直後にリテラルで `#...` を
  # 書くと、シェルが行頭以外でも「空白の直後の # で始まる語」をコメント開始と
  # 解釈しうる（bash の既知の落とし穴）。変数参照 `$NUM_PATTERN` はリテラルの
  # `#` で始まらないため、この事故を構造的に避けられる。
  local NUM_PATTERN='#([0-9]+)'
  while IFS= read -r line || [ -n "$line" ]; do
    if [[ "$line" =~ ^#+[[:space:]] ]]; then
      if [ "$in_section" -eq 1 ]; then
        # 次の見出しに到達した＝対象タスク節の終わり。
        break
      fi
      if [[ "$line" =~ ^##[[:space:]]対象タスク[[:space:]]*$ ]]; then
        in_section=1
        found_section=1
      fi
      continue
    fi
    if [ "$in_section" -eq 1 ] && [ -z "$found_number" ]; then
      if [[ "$line" =~ $NUM_PATTERN ]]; then
        found_number="${BASH_REMATCH[1]}"
        break
      fi
    fi
  done < "$norm"

  if [ "$found_section" -eq 0 ]; then
    finish INDETERMINATE 1 "'## 対象タスク' 節が無い（仕様 AC-7-4）。"
  fi
  if [ -z "$found_number" ]; then
    finish INDETERMINATE 1 "'## 対象タスク' 節内に #<数字> が無い（仕様 AC-7-5）。"
  fi
  if [ "$found_number" = "$num" ]; then
    finish RESUME 0 "" "対象Issue番号: ${found_number}"
  fi
  finish CONFLICT 3 "既存の progress.md は Issue #${found_number} のもので、対象 #${num} と違う（仕様 AC-7-3）。前タスクの作業記憶を握り潰さない。" \
    "既存対象Issue番号: ${found_number}" "指定Issue番号: ${num}"
}

# ==============================================================================
# エントリポイント
# ==============================================================================

MODE="${1:-}"
shift 2> /dev/null || true

case "$MODE" in
  arg) do_arg "$@" ;;
  gate) do_gate "$@" ;;
  branch) do_branch "$@" ;;
  progress) do_progress "$@" ;;
  *)
    echo "[issue-gate] 未知の mode '${MODE}'（arg / gate / branch / progress のいずれかを指定すること）" >&2
    exit 1
    ;;
esac
