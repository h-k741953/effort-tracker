#!/usr/bin/env bash
# check-go-module-pins.sh の fixture テスト（Issue #66）。
#
# 【なぜこれが要るか】
#   docs/specs/devcontainer-go-module-policy.md AC-5〜AC-8 をテーブル駆動で
#   固定する。検査本体は stdin を持たず、第1引数（Dockerfile パス）・第2引数
#   （go.mod パス）だけを入力とする純粋なフィルタである前提なので（AC-5-2）、
#   入力ファイルを一時ディレクトリへ組み立てて渡すだけでローカルで完全に
#   再現・検査できる。.github/scripts/test-check-review-trail.sh と同じ型
#   （ケース定義・実行・期待値比較・失敗時の出力・件数サマリ）に倣う。
#
# 【AC-8-1: 本体が無ければ失敗すること（SKIP しない）】
#   本コミット時点で .github/scripts/check-go-module-pins.sh は存在しない。
#   ここでは「本体が無いのでスキップする」ような早期リターンを一切書かない。
#   bash "$TARGET" は「No such file or directory」で rc=127 を返し、以下の
#   すべてのケースが期待 rc（0/1/3）と不一致で FAIL するため、テスト全体が
#   Red になる。これは実装（次工程）が本体を書いた時点で初めて解消する。
#
# 【入力をシェルへ展開しないこと（AC-5-4 / AC-8-5）】
#   fixture の Dockerfile・go.mod はクォートしたヒアドキュメント区切り
#   （<<'EOF'）で組み立てる。7-13 ケースは `"` / `` ` `` / `$(...)` を含む
#   行を実際に混入させ、それが評価されて副作用ファイルが生成されないことまで
#   確認する（.claude/hooks/test-check-prompt-entry.sh AC-8-4 と同じ理由）。
#
# 【実ファイルを書き換えないこと（AC-8-4）】
#   .devcontainer/Dockerfile / services/api/go.mod は一切参照・変更しない。
#   すべて WORK（mktemp -d）配下に組み立てた fixture のみを渡す。
#
# 【終了コードは 0/1/3 のみ（AC-7 前文）】
#   `2` は UserPromptSubmit hook のブロックに予約されているため、このテストで
#   `2` を期待するケースは作らない。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="${SCRIPT_DIR}/check-go-module-pins.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

echo "==> test-check-go-module-pins"

# --- 実行 ---------------------------------------------------------------------

RC=""
OUT=""
ERR=""

# run_target <args...>: TARGET を WORK をカレントディレクトリとして実行し、
# RC/OUT/ERR を埋める。WORK を cwd にするのは、7-13 で「シェル評価された場合に
# 生成される副作用ファイル」の出現先を固定するため（相対パスで touch されても
# WORK 配下に落ちる）。
run_target() {
  local outf errf
  outf="$(mktemp -p "$WORK")"
  errf="$(mktemp -p "$WORK")"
  ( cd "$WORK" && bash "$TARGET" "$@" > "$outf" 2> "$errf" )
  RC=$?
  OUT="$(cat "$outf")"
  ERR="$(cat "$errf")"
  rm -f "$outf" "$errf"
}

# assert_verdict <verdict名>: stdout 1行目が「VERDICT: <verdict名>」であること。
assert_verdict() {
  local want="$1" first_line
  first_line="$(printf '%s\n' "$OUT" | head -n1)"
  [ "$first_line" = "VERDICT: ${want}" ]
}

# assert_detail_key <部分文字列>: stdout の2行目以降（詳細行）に含まれること。
assert_detail_key() {
  local needle="$1" rest
  rest="$(printf '%s\n' "$OUT" | tail -n +2)"
  grep -qF -- "$needle" <<< "$rest"
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

# --- verdict 別の期待値ヘルパー -------------------------------------------------

# expect_ok <名前> <dockerfile> <gomod>: VERDICT: OK, rc=0。
expect_ok() {
  local name="$1" df="$2" gm="$3"
  run_target "$df" "$gm"
  local ok=1
  [ "$RC" = "0" ] || ok=0
  assert_verdict "OK" || ok=0
  report "$name" "$ok" "0"
}

# expect_pin_drift <名前> <dockerfile> <gomod> [詳細行の部分文字列...]:
# VERDICT: PIN_DRIFT（1行）, rc=3。渡した詳細行の部分文字列すべてが
# stdout の2行目以降に現れること（7-5 では複数種の違反を同時に検査する）。
expect_pin_drift() {
  local name="$1" df="$2" gm="$3"; shift 3
  run_target "$df" "$gm"
  local ok=1
  [ "$RC" = "3" ] || ok=0
  assert_verdict "PIN_DRIFT" || ok=0
  local d
  for d in "$@"; do
    assert_detail_key "$d" || ok=0
  done
  report "$name" "$ok" "3"
}

# expect_no_pin <名前> <dockerfile> <gomod>: VERDICT: NO_PIN, rc=1。
expect_no_pin() {
  local name="$1" df="$2" gm="$3"
  run_target "$df" "$gm"
  local ok=1
  [ "$RC" = "1" ] || ok=0
  assert_verdict "NO_PIN" || ok=0
  report "$name" "$ok" "1"
}

# expect_no_require <名前> <dockerfile> <gomod>: VERDICT: NO_REQUIRE, rc=1。
expect_no_require() {
  local name="$1" df="$2" gm="$3"
  run_target "$df" "$gm"
  local ok=1
  [ "$RC" = "1" ] || ok=0
  assert_verdict "NO_REQUIRE" || ok=0
  report "$name" "$ok" "1"
}

# expect_indeterminate <名前> <dockerfile> <gomod> [詳細行の部分文字列...]:
# VERDICT: INDETERMINATE, rc=1。
expect_indeterminate() {
  local name="$1" df="$2" gm="$3"; shift 3
  run_target "$df" "$gm"
  local ok=1
  [ "$RC" = "1" ] || ok=0
  assert_verdict "INDETERMINATE" || ok=0
  local d
  for d in "$@"; do
    assert_detail_key "$d" || ok=0
  done
  report "$name" "$ok" "1"
}

# ==============================================================================
# 7-1: 現行相当（差分なし） → OK, rc=0
# ==============================================================================
# 詳細行のキー名は AC-7 の表に literal 指定が無い（「検査した組数」という記述の
# みで、7-2/7-3/7-4/7-9/7-10/7-11 のような `<キー>: ` 形式が与えられていない）。
# 未指定のキー名を推測して固定すると、それ自体が仕様に無い業務ルールの埋め込み
# になるため、ここでは終了コードと VERDICT のみを検査する（AC-8-3 は「詳細行の
# キー」を判定軸の1つとするが、キーが定義されている行にのみ適用する）。

mkdir -p "${WORK}/c71"
cat > "${WORK}/c71/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3
ARG BETA_VERSION=v0.9.0

RUN set -eux; \
    go get "example.org/alpha@${ALPHA_VERSION}"; \
    go get "example.org/beta@${BETA_VERSION}"
EOF
cat > "${WORK}/c71/go.mod" <<'EOF'
module example.org/target

go 1.22

require (
	example.org/alpha v1.2.3
	example.org/beta v0.9.0
)
EOF
expect_ok "7-1: pin と direct require が完全一致" \
  "${WORK}/c71/Dockerfile" "${WORK}/c71/go.mod"

# ==============================================================================
# 7-2: 版の不一致 → PIN_DRIFT / VERSION_MISMATCH, rc=3
# ==============================================================================

mkdir -p "${WORK}/c72"
cat > "${WORK}/c72/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c72/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.3.0
EOF
expect_pin_drift "7-2: 版の不一致（VERSION_MISMATCH）" \
  "${WORK}/c72/Dockerfile" "${WORK}/c72/go.mod" \
  "VERSION_MISMATCH: example.org/alpha dockerfile=v1.2.3 gomod=v1.3.0"

# ==============================================================================
# 7-3: direct require に対応する pin が Dockerfile に無い → MISSING_PIN, rc=3
# ==============================================================================

mkdir -p "${WORK}/c73"
cat > "${WORK}/c73/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c73/go.mod" <<'EOF'
module example.org/target

go 1.22

require (
	example.org/alpha v1.2.3
	example.org/beta v0.9.0
)
EOF
expect_pin_drift "7-3: pin し忘れ（MISSING_PIN）" \
  "${WORK}/c73/Dockerfile" "${WORK}/c73/go.mod" \
  "MISSING_PIN: example.org/beta v0.9.0"

# ==============================================================================
# 7-4: Dockerfile が pin するモジュールが direct require に無い
#      → MODULE_NOT_REQUIRED, rc=3
# ==============================================================================

mkdir -p "${WORK}/c74"
cat > "${WORK}/c74/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3
ARG GAMMA_VERSION=v2.0.0

RUN set -eux; \
    go get "example.org/alpha@${ALPHA_VERSION}"; \
    go get "example.org/gamma@${GAMMA_VERSION}"
EOF
cat > "${WORK}/c74/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF
expect_pin_drift "7-4: 余分な pin（MODULE_NOT_REQUIRED）" \
  "${WORK}/c74/Dockerfile" "${WORK}/c74/go.mod" \
  "MODULE_NOT_REQUIRED: example.org/gamma v2.0.0"

# ==============================================================================
# 7-5: 違反が複数件・複数種類 → PIN_DRIFT（1行）＋該当するすべての詳細行, rc=3
# ==============================================================================
# 最初の1件で打ち切らないことを固定するため、種類の異なる違反
# （VERSION_MISMATCH と MODULE_NOT_REQUIRED）を同居させる。

mkdir -p "${WORK}/c75"
cat > "${WORK}/c75/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3
ARG GAMMA_VERSION=v2.0.0

RUN set -eux; \
    go get "example.org/alpha@${ALPHA_VERSION}"; \
    go get "example.org/gamma@${GAMMA_VERSION}"
EOF
cat > "${WORK}/c75/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.3.0
EOF
expect_pin_drift "7-5: 複数種類の違反が両方とも詳細行に出る（打ち切らない）" \
  "${WORK}/c75/Dockerfile" "${WORK}/c75/go.mod" \
  "VERSION_MISMATCH: example.org/alpha dockerfile=v1.2.3 gomod=v1.3.0" \
  "MODULE_NOT_REQUIRED: example.org/gamma v2.0.0"

# ==============================================================================
# 7-6: // indirect の require に対応する pin が無い（他に違反なし） → OK, rc=0
# ==============================================================================

mkdir -p "${WORK}/c76"
cat > "${WORK}/c76/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c76/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3

require example.org/delta v0.1.0 // indirect
EOF
expect_ok "7-6: indirect require の pin 欠落は違反にしない" \
  "${WORK}/c76/Dockerfile" "${WORK}/c76/go.mod"

# ==============================================================================
# 7-6b（回帰・レビュー往復2 C-1）: `// indirect` 以外のコメントを持つ
# direct require は対象に残る → pin が無ければ MISSING_PIN, rc=3
# ==============================================================================
# AC-6-4 は「行末に `// indirect` を持つ require は対象外」であり、コメントに
# `indirect` という語を含むだけでは対象外にならない（部分文字列一致は危険側
# ＝見逃す側に倒れる）。`// used indirectly via helper` のような direct require
# の pin し忘れを、間違って OK にしないことを固定する。

mkdir -p "${WORK}/c76b"
cat > "${WORK}/c76b/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c76b/go.mod" <<'EOF'
module example.org/target

go 1.22

require (
	example.org/alpha v1.2.3
	example.org/beta v2.0.0 // used indirectly via helper
)
EOF
expect_pin_drift "7-6b: // indirect 以外のコメントを持つ direct require は対象に残る（MISSING_PIN）" \
  "${WORK}/c76b/Dockerfile" "${WORK}/c76b/go.mod" \
  "MISSING_PIN: example.org/beta v2.0.0"

# ==============================================================================
# 7-7: Dockerfile から pin の組が1つも取れない → NO_PIN, rc=1（成功に倒さない）
# ==============================================================================
# RUN 行に「go get」という文字列そのものを含めない（AC-6-2 は「コメント以外に
# go get を含みこの形式に一致しない行が1つでもあれば INDETERMINATE」なので、
# 説明文にたまたま go get という語を混ぜると 7-9（INDETERMINATE）と混線する）。

mkdir -p "${WORK}/c77"
cat > "${WORK}/c77/Dockerfile" <<'EOF'
FROM debian:bookworm

ARG SOME_OTHER_TOOL_VERSION=v1.0.0

RUN echo "no module pins configured here"
EOF
cat > "${WORK}/c77/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF
expect_no_pin "7-7: pin の組が0件（NO_PIN。OKに倒さない）" \
  "${WORK}/c77/Dockerfile" "${WORK}/c77/go.mod"

# ==============================================================================
# 7-8: go.mod に direct require が1つも無い → NO_REQUIRE, rc=1（成功に倒さない）
# ==============================================================================

mkdir -p "${WORK}/c78"
cat > "${WORK}/c78/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c78/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/delta v0.1.0 // indirect
EOF
expect_no_require "7-8: direct require が0件（NO_REQUIRE。OKに倒さない）" \
  "${WORK}/c78/Dockerfile" "${WORK}/c78/go.mod"

# ==============================================================================
# 7-9: コメント以外に、形式に一致しない go get 行がある
#      → INDETERMINATE / LINE: <行番号>, rc=1
# ==============================================================================
# 行番号は下の Dockerfile の実際の行に対応させる（1: ARG, 2: 空行,
# 3: RUN \, 4: 正しい go get, 5: 形式不一致の go get）。

mkdir -p "${WORK}/c79"
cat > "${WORK}/c79/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"; \
    go get example.org/beta@v1.0.0
EOF
cat > "${WORK}/c79/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF
expect_indeterminate "7-9: 形式に一致しない go get 行（LINE: 5）" \
  "${WORK}/c79/Dockerfile" "${WORK}/c79/go.mod" \
  "LINE: 5"

# ==============================================================================
# 7-10: ${<ARG名>} に対応する ARG 行が無い／同名が2つ以上ある
#       → INDETERMINATE / ARG: <ARG名>, rc=1
# ==============================================================================

# 7-10a: 対応する ARG 行が無い。
mkdir -p "${WORK}/c710a"
cat > "${WORK}/c710a/Dockerfile" <<'EOF'
RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c710a/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF
expect_indeterminate "7-10a: 対応する ARG 行が無い（ARG: ALPHA_VERSION）" \
  "${WORK}/c710a/Dockerfile" "${WORK}/c710a/go.mod" \
  "ARG: ALPHA_VERSION"

# 7-10b: 同名の ARG 行が2つ以上ある（後勝ち・先勝ちを推測しない）。
mkdir -p "${WORK}/c710b"
cat > "${WORK}/c710b/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3
ARG ALPHA_VERSION=v9.9.9

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c710b/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF
expect_indeterminate "7-10b: 同名 ARG が2つ以上（ARG: ALPHA_VERSION）" \
  "${WORK}/c710b/Dockerfile" "${WORK}/c710b/go.mod" \
  "ARG: ALPHA_VERSION"

# ==============================================================================
# 7-11: 引数のパスが存在しない・読めない → INDETERMINATE / PATH: <パス>, rc=1
# ==============================================================================

mkdir -p "${WORK}/c711"
cat > "${WORK}/c711/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF

# needle は "PATH:" とパス文字列を結合した1本にする（レビュー往復2 I-5）。
# 別々の needle だと「PATH: というキーがどこかにある」ことと「そのパスが
# どこかにある」ことしか固定できず、両者が同じ行で結びついていることまでは
# 検査できない。結合した needle にすると AC-7-11 の出力契約
# （`PATH: <パス>`）を正確に固定できる。

MISSING_DOCKERFILE="${WORK}/c711/does-not-exist-Dockerfile"
expect_indeterminate "7-11a: 第1引数（Dockerfile）のパスが存在しない" \
  "$MISSING_DOCKERFILE" "${WORK}/c711/go.mod" \
  "PATH: $MISSING_DOCKERFILE"

cat > "${WORK}/c711/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
MISSING_GOMOD="${WORK}/c711/does-not-exist-go.mod"
expect_indeterminate "7-11b: 第2引数（go.mod）のパスが存在しない" \
  "${WORK}/c711/Dockerfile" "$MISSING_GOMOD" \
  "PATH: $MISSING_GOMOD"

# ==============================================================================
# 7-12: 引数が足りない（0個 / 1個） → INDETERMINATE, rc=1（詳細行は — なので検査しない）
# ==============================================================================

run_target
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
report "7-12a: 引数0個" "$ok" "1"

run_target "${WORK}/c71/Dockerfile"
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
report "7-12b: 引数1個（go.mod パスが無い）" "$ok" "1"

# ==============================================================================
# 7-13: 入力に `"` / `` ` `` / `$(` を含む行がある → 判定を壊さない
#       （シェルで評価しない。副作用ファイルが生成されないこと）
# ==============================================================================
# 危険な文字はコメント行の中に置く（AC-6-2 によりコメントは対象外なので、
# 判定は「危険な行が無かったのと同じ OK」になるはずである）。ヒアドキュメントは
# クォート区切り（<<'EOF'）なので、このテストスクリプト自身の実行時にも
# `$(touch ...)` は評価されない（AC-8-5）。run_target が WORK を cwd として
# 呼び出すため、万一 TARGET 側が評価してしまえば WORK 直下に副作用ファイルが
# 生成されるはずであり、その不在まで確認する。

mkdir -p "${WORK}/c713"
rm -f "${WORK}/PWNED_SIDE_EFFECT.txt"
cat > "${WORK}/c713/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

# 危険な文字: "quotes" ` backtick $(touch PWNED_SIDE_EFFECT.txt)
RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c713/go.mod" <<'EOF'
module example.org/target

go 1.22

// 危険な文字: "quotes" ` backtick $(touch PWNED_SIDE_EFFECT.txt)
require example.org/alpha v1.2.3
EOF
expect_ok "7-13: 危険な文字を含むコメント行があっても判定を壊さない" \
  "${WORK}/c713/Dockerfile" "${WORK}/c713/go.mod"

# 直前の expect_ok（run_target）の RC を前提条件に含める（レビュー往復2 W-1）。
# 「副作用ファイルが無いこと」だけを見て ok にすると、本体が固まったまま
# 何も実行されなかった場合（RC 不一致）でも空回りで pass してしまう。
# RC=0（かつ VERDICT: OK、上の expect_ok 内で検査済み）であることを土台にし、
# そのうえで副作用ファイルが存在しないことを確認する。
ok=1
[ "$RC" = "0" ] || ok=0
if [ -e "${WORK}/PWNED_SIDE_EFFECT.txt" ] || \
   find "$WORK" -maxdepth 4 -name 'PWNED_SIDE_EFFECT.txt' -print -quit 2>/dev/null | grep -q .; then
  ok=0
fi
if [ "$ok" = 1 ]; then
  pass=$((pass + 1))
  echo "  ok   7-13: 危険な文字が評価されず副作用ファイルが生成されていない（かつ判定は OK のまま）"
else
  fail=$((fail + 1))
  echo "  FAIL 7-13: RC=$RC、または入力中の \$(touch ...) が実際に実行されてしまった（副作用ファイルが存在する）"
fi

# ==============================================================================
# 7-14: 入力が CRLF 改行 → 判定を壊さない（\r を版文字列へ混入させない）
# ==============================================================================
# 片方のファイルだけを CRLF にする（もう片方は LF のまま）。version の比較は
# 文字列の完全一致（AC-6-5）なので、\r を版文字列から取り除かずに比較する実装
# では「同じ版なのに一致しない」という誤った VERSION_MISMATCH が出るはずである
# （両方のファイルを同時に CRLF化すると \r 同士が相殺してこの回帰を検出できない
# ため、片方のみを CRLF にして非対称にする）。

# 7-14a: Dockerfile 側のみ CRLF。
mkdir -p "${WORK}/c714a"
cat > "${WORK}/c714a/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
sed -i 's/$/\r/' "${WORK}/c714a/Dockerfile"
cat > "${WORK}/c714a/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF
expect_ok "7-14a: Dockerfile のみ CRLF（\\r を版文字列へ混入させない）" \
  "${WORK}/c714a/Dockerfile" "${WORK}/c714a/go.mod"

# 7-14b: go.mod 側のみ CRLF。
mkdir -p "${WORK}/c714b"
cat > "${WORK}/c714b/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c714b/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v1.2.3
EOF
sed -i 's/$/\r/' "${WORK}/c714b/go.mod"
expect_ok "7-14b: go.mod のみ CRLF（\\r を版文字列へ混入させない）" \
  "${WORK}/c714b/Dockerfile" "${WORK}/c714b/go.mod"

# ==============================================================================
# 7-15: 危険な文字を主経路（ARG 値・require の版トークン）に置いても評価されない
#       （レビュー往復2 W-2）
# ==============================================================================
# 7-13 はコメント行にしか危険な文字を置いておらず、コメント行は本体の入口
# （# 判定・require 行の対象外判定）で早期に弾かれるため、AC-5-4 が守るべき
# 主経路（ARG の値、require の版トークン、形式不一致の判定を経て詳細行へ
# 出力される値）を1本も通っていなかった。ここでは `:>PWNED_*.txt`（空白を
# 含まない、評価されれば実際にファイルを作るリダイレクト）を ARG 値・
# require の版トークンへ直接埋め込み、(a) 詳細行にリテラルとして現れること、
# (b) 副作用ファイルが生成されていないこと、の両方を固定する。

# 7-15a: ARG 値に危険な文字 → PIN_VERSION の解決を経て VERSION_MISMATCH の
#        詳細行にリテラルのまま現れる（ARG 解決は変数代入のみで eval しない）。
mkdir -p "${WORK}/c715a"
rm -f "${WORK}/PWNED_ARG.txt"
cat > "${WORK}/c715a/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3$(:>PWNED_ARG.txt)

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c715a/go.mod" <<'EOF'
module example.org/target

go 1.22

require example.org/alpha v9.9.9
EOF
run_target "${WORK}/c715a/Dockerfile" "${WORK}/c715a/go.mod"
ok=1
[ "$RC" = "3" ] || ok=0
assert_verdict "PIN_DRIFT" || ok=0
assert_detail_key 'VERSION_MISMATCH: example.org/alpha dockerfile=v1.2.3$(:>PWNED_ARG.txt) gomod=v9.9.9' || ok=0
if [ -e "${WORK}/PWNED_ARG.txt" ] || \
   find "$WORK" -maxdepth 4 -name 'PWNED_ARG.txt' -print -quit 2>/dev/null | grep -q .; then
  ok=0
fi
report "7-15a: ARG 値の危険な文字は評価されずそのまま版文字列として現れる" "$ok" "3"

# 7-15b: require の版トークンに危険な文字 → pin が無いので MISSING_PIN の
#        詳細行にリテラルのまま現れる（go.mod 側は read によるトークン化のみ）。
mkdir -p "${WORK}/c715b"
rm -f "${WORK}/PWNED_MOD.txt"
cat > "${WORK}/c715b/Dockerfile" <<'EOF'
ARG ALPHA_VERSION=v1.2.3

RUN \
    go get "example.org/alpha@${ALPHA_VERSION}"
EOF
cat > "${WORK}/c715b/go.mod" <<'EOF'
module example.org/target

go 1.22

require (
	example.org/alpha v1.2.3
	example.org/beta v1.0.0$(:>PWNED_MOD.txt)
)
EOF
run_target "${WORK}/c715b/Dockerfile" "${WORK}/c715b/go.mod"
ok=1
[ "$RC" = "3" ] || ok=0
assert_verdict "PIN_DRIFT" || ok=0
assert_detail_key 'MISSING_PIN: example.org/beta v1.0.0$(:>PWNED_MOD.txt)' || ok=0
if [ -e "${WORK}/PWNED_MOD.txt" ] || \
   find "$WORK" -maxdepth 4 -name 'PWNED_MOD.txt' -print -quit 2>/dev/null | grep -q .; then
  ok=0
fi
report "7-15b: require の版トークンの危険な文字は評価されずそのまま現れる" "$ok" "3"

# ==============================================================================
# AC-8-6: 違反側が Red になることの固定（通る側だけを見ない）
# ==============================================================================
# 「OK でないこと」だけを見る検査（assert_verdict "OK" の否定）は、検査本体が
# 存在しない・無出力でも満たされてしまい、落ちようのない空回りになる
# （レビュー往復2 W-1）。AC-8-6 の充足根拠は 7-2 / 7-3 / 7-4 が既に持っている
# ことをここに明示する。expect_pin_drift は rc=3 と VERDICT: PIN_DRIFT を
# 肯定形で要求し、かつ VERSION_MISMATCH / MISSING_PIN / MODULE_NOT_REQUIRED の
# 詳細行キーまで固定しているため、このループを重ねて足しても検査力は増えず、
# 削除する（許容されるケース削除。仕様の充足は他ケースが担保している）。

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
