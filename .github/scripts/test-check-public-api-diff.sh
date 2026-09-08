#!/usr/bin/env bash
# check-public-api-diff.sh の fixture テスト（Issue #93）。
#
# 【なぜこれが要るか】
#   docs/specs/public-api-diff-check.md AC-1 / AC-4 / AC-5 / AC-6 をテーブル
#   駆動で固定する。検査本体は引数以外の入力を持たない純粋なフィルタである
#   前提なので（AC-1-1 / AC-1-2）、入力（エクスポート一覧2本）を一時ディレクトリへ
#   組み立てて渡すだけでローカルで完全に再現・検査できる。
#   .github/scripts/test-check-go-module-pins.sh と同じ型（ケース定義・実行・
#   期待値比較・失敗時の出力・件数サマリ）に倣う。
#
# 【AC-6-1: 本体が無ければ失敗すること（SKIP しない）】
#   本コミット時点で .github/scripts/check-public-api-diff.sh は存在しない。
#   ここでは「本体が無いのでスキップする」ような早期リターンを一切書かない。
#   bash "$TARGET" は「No such file or directory」で rc=127 を返し、以下の
#   すべてのケースが期待 rc（0/1/3）と不一致で FAIL するため、テスト全体が
#   Red になる。これは実装（次工程）が本体を書いた時点で初めて解消する。
#
# 【入力をシェルへ展開しないこと（AC-1-4 / AC-6-7）】
#   fixture のエクスポート一覧はクォートしたヒアドキュメント区切り（<<'EOF'）で
#   組み立てる。5-11 / AC-1-4 ケースは `"` / `` ` `` / `$(...)` を含む行を
#   実際に混入させ、それが評価されて副作用ファイルが生成されないことまで
#   確認する（.claude/hooks/test-check-prompt-entry.sh AC-8-4 と同じ理由）。
#
# 【実ファイルを書き換えないこと（AC-6-6）】
#   このテストが読み書きするのは WORK（mktemp -d）配下のファイルのみ。
#   リポジトリ内の実ファイルは一切参照・変更しない。
#
# 【抽出器を持ち込まないこと（AC-6-9）】
#   本 fixture は go を一切呼ばない。AC-3（抽出規則）・AC-3-15 / 3-16（自己言及）
#   の検査は services/api/cmd/exportlist の Go テストが持つ。
#
# 【終了コードは 0/1/3 のみ（AC-5 前文）】
#   `2` は UserPromptSubmit hook のブロックに予約されているため、このテストで
#   `2` を期待するケースは作らない。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="${SCRIPT_DIR}/check-public-api-diff.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

echo "==> test-check-public-api-diff"

# --- 実行 ---------------------------------------------------------------------

RC=""
OUT=""
ERR=""

# run_target [--locale <名前>] <args...>: TARGET を WORK をカレントディレクトリ
# として実行し、RC/OUT/ERR を埋める。WORK を cwd にするのは、5-11 で「シェル
# 評価された場合に生成される副作用ファイル」の出現先を固定するため（相対パスで
# touch されても WORK 配下に落ちる）。--locale を渡すと LC_ALL をその値に
# して実行する（AC-6-8: ロケール非依存の検査）。既定は C。
run_target() {
  local locale="C"
  if [ "${1:-}" = "--locale" ]; then
    locale="$2"
    shift 2
  fi
  local outf errf
  outf="$(mktemp -p "$WORK")"
  errf="$(mktemp -p "$WORK")"
  ( cd "$WORK" && LC_ALL="$locale" bash "$TARGET" "$@" > "$outf" 2> "$errf" )
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

# assert_no_detail_key <部分文字列>: stdout の2行目以降に含まれないこと。
assert_no_detail_key() {
  local needle="$1" rest
  rest="$(printf '%s\n' "$OUT" | tail -n +2)"
  ! grep -qF -- "$needle" <<< "$rest"
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

# write_list <パス>: stdin をそのままファイルへ書く。呼び出し側は必ず
# クォートしたヒアドキュメント（<<'EOF'）で渡し、シェル展開させない。
write_list() {
  cat > "$1"
}

# to_crlf <入力パス> <出力パス>: 各行末に \r を付けて CRLF 化する（AC-5-12）。
to_crlf() {
  sed 's/$/\r/' "$1" > "$2"
}

# --- verdict 別の期待値ヘルパー -------------------------------------------------

# expect_ok <名前> <old> <new> [詳細行の部分文字列...]:
# VERDICT: OK, rc=0。渡した詳細行の部分文字列すべてが stdout の2行目以降に
# 現れること（5-4 の REMOVED 表示・COUNT: 行を確認するために使う）。
expect_ok() {
  local name="$1" old="$2" new="$3"; shift 3
  run_target "$old" "$new"
  local ok=1
  [ "$RC" = "0" ] || ok=0
  assert_verdict "OK" || ok=0
  local d
  for d in "$@"; do
    assert_detail_key "$d" || ok=0
  done
  report "$name" "$ok" "0"
}

# expect_warn <名前> <old> <new> [詳細行の部分文字列...]:
# VERDICT: WARN, rc=3。渡した詳細行の部分文字列すべてが stdout の2行目以降に
# 現れること（AC-6-5: 通る側だけでなく、期待した詳細行キーで落ちることを固定する）。
expect_warn() {
  local name="$1" old="$2" new="$3"; shift 3
  run_target "$old" "$new"
  local ok=1
  [ "$RC" = "3" ] || ok=0
  assert_verdict "WARN" || ok=0
  local d
  for d in "$@"; do
    assert_detail_key "$d" || ok=0
  done
  report "$name" "$ok" "3"
}

# expect_indeterminate <名前> <verdict詳細行の部分文字列> <args...>:
# VERDICT: INDETERMINATE, rc=1。$3 以降が TARGET へ渡す引数。
expect_indeterminate() {
  local name="$1" detail="$2"; shift 2
  run_target "$@"
  local ok=1
  [ "$RC" = "1" ] || ok=0
  assert_verdict "INDETERMINATE" || ok=0
  assert_detail_key "$detail" || ok=0
  report "$name" "$ok" "1"
}

# ==============================================================================
# AC-5-1 / AC-6-4(c): 両側完全一致 → OK, COUNT: added=0 signature_changed=0
# removed=0
# ==============================================================================

write_list "$WORK/5-1-old.tsv" <<'EOF'
.	func	Foo	() ()
internal/domain/workmonth	type	WorkMonth	struct { ... }
EOF
write_list "$WORK/5-1-new.tsv" <<'EOF'
.	func	Foo	() ()
internal/domain/workmonth	type	WorkMonth	struct { ... }
EOF
expect_ok "5-1 / 6-4c: 両側完全一致 → OK" \
  "$WORK/5-1-old.tsv" "$WORK/5-1-new.tsv" \
  "COUNT: added=0 signature_changed=0 removed=0"

# ==============================================================================
# AC-5-2 / AC-6-4(a): 新側にのみ存在するキー → WARN / ADDED
# ==============================================================================

write_list "$WORK/5-2-old.tsv" <<'EOF'
.	func	Foo	() ()
EOF
write_list "$WORK/5-2-new.tsv" <<'EOF'
.	func	Foo	() ()
.	func	Bar	(int) (error)
EOF
expect_warn "5-2 / 6-4a: 新側にのみ存在 → WARN / ADDED" \
  "$WORK/5-2-old.tsv" "$WORK/5-2-new.tsv" \
  "ADDED: .	func	Bar	(int) (error)"

# ==============================================================================
# AC-5-3 / AC-6-4(b): 両側にあり signature が違う → WARN / SIGNATURE_CHANGED
# ==============================================================================

write_list "$WORK/5-3-old.tsv" <<'EOF'
.	func	Foo	() ()
EOF
write_list "$WORK/5-3-new.tsv" <<'EOF'
.	func	Foo	(int) ()
EOF
expect_warn "5-3 / 6-4b: シグネチャ変更 → WARN / SIGNATURE_CHANGED" \
  "$WORK/5-3-old.tsv" "$WORK/5-3-new.tsv" \
  "SIGNATURE_CHANGED: .	func	Foo	old=() () new=(int) ()"

# ==============================================================================
# AC-5-4: 旧側にのみ存在するキー（他に差分なし） → OK / REMOVED + COUNT
# ==============================================================================

write_list "$WORK/5-4-old.tsv" <<'EOF'
.	func	Foo	() ()
.	func	Gone	() ()
EOF
write_list "$WORK/5-4-new.tsv" <<'EOF'
.	func	Foo	() ()
EOF
expect_ok "5-4: 旧側にのみ存在（他に差分なし） → OK / REMOVED" \
  "$WORK/5-4-old.tsv" "$WORK/5-4-new.tsv" \
  "REMOVED: .	func	Gone	() ()" \
  "COUNT: added=0 signature_changed=0 removed=1"

# ==============================================================================
# AC-5-5: ADDED / SIGNATURE_CHANGED / REMOVED が混在・複数件 → WARN、該当する
# すべての詳細行 + COUNT（AC-4-7: 最初の1件で打ち切らない）
# ==============================================================================

write_list "$WORK/5-5-old.tsv" <<'EOF'
.	func	Foo	() ()
.	func	Changed	() ()
.	func	Gone	() ()
EOF
write_list "$WORK/5-5-new.tsv" <<'EOF'
.	func	Foo	() ()
.	func	Changed	(int) ()
.	func	NewOne	() ()
.	func	AnotherNew	() (error)
EOF
run_target "$WORK/5-5-old.tsv" "$WORK/5-5-new.tsv"
ok=1
[ "$RC" = "3" ] || ok=0
assert_verdict "WARN" || ok=0
assert_detail_key "ADDED: .	func	NewOne	() ()" || ok=0
assert_detail_key "ADDED: .	func	AnotherNew	() (error)" || ok=0
assert_detail_key "SIGNATURE_CHANGED: .	func	Changed	old=() () new=(int) ()" || ok=0
assert_detail_key "REMOVED: .	func	Gone	() ()" || ok=0
assert_detail_key "COUNT: added=2 signature_changed=1 removed=1" || ok=0
report "5-5 / 4-7: 複数件混在 → 該当するすべての詳細行 + COUNT" "$ok" "3"

# ==============================================================================
# AC-4-8: kind が変わったシンボルは REMOVED + ADDED の2行として現れる（まとめない）
# ==============================================================================

write_list "$WORK/4-8-old.tsv" <<'EOF'
.	func	F	() ()
EOF
write_list "$WORK/4-8-new.tsv" <<'EOF'
.	var	F	int
EOF
run_target "$WORK/4-8-old.tsv" "$WORK/4-8-new.tsv"
ok=1
[ "$RC" = "3" ] || ok=0
assert_verdict "WARN" || ok=0
assert_detail_key "REMOVED: .	func	F	() ()" || ok=0
assert_detail_key "ADDED: .	var	F	int" || ok=0
report "4-8: kind 変更は REMOVED + ADDED の2行（1件へまとめない）" "$ok" "3"

# ==============================================================================
# AC-5-6 / AC-6-4(d): 引数が足りない（0個 / 1個） → INDETERMINATE / ARGS
# ==============================================================================

run_target
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
assert_detail_key "ARGS: 0" || ok=0
report "5-6 / 6-4d: 引数0個 → INDETERMINATE / ARGS: 0" "$ok" "1"

run_target "$WORK/5-1-old.tsv"
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
assert_detail_key "ARGS: 1" || ok=0
report "5-6: 引数1個 → INDETERMINATE / ARGS: 1" "$ok" "1"

# ==============================================================================
# AC-5-7: 引数のパスが存在しない・読めない → INDETERMINATE / PATH
# ==============================================================================

expect_indeterminate "5-7a: 旧側パスが存在しない → INDETERMINATE / PATH" \
  "PATH: $WORK/does-not-exist-old.tsv" \
  "$WORK/does-not-exist-old.tsv" "$WORK/5-1-new.tsv"

expect_indeterminate "5-7b: 新側パスが存在しない → INDETERMINATE / PATH" \
  "PATH: $WORK/does-not-exist-new.tsv" \
  "$WORK/5-1-old.tsv" "$WORK/does-not-exist-new.tsv"

# ==============================================================================
# AC-5-8: どちらかの一覧が0行 → INDETERMINATE / EMPTY: old|new
# ==============================================================================

: > "$WORK/5-8-empty.tsv"

expect_indeterminate "5-8a: 旧側が0行 → INDETERMINATE / EMPTY: old" \
  "EMPTY: old" \
  "$WORK/5-8-empty.tsv" "$WORK/5-1-new.tsv"

expect_indeterminate "5-8b: 新側が0行 → INDETERMINATE / EMPTY: new" \
  "EMPTY: new" \
  "$WORK/5-1-old.tsv" "$WORK/5-8-empty.tsv"

# ==============================================================================
# AC-5-9: タブ区切り4フィールドでない行がある（空行を含む） → INDETERMINATE /
# MALFORMED: <old|new> <行番号>
# ==============================================================================

write_list "$WORK/5-9-old-badfields.tsv" <<'EOF'
.	func	Foo	() ()
.	func	OnlyThree
EOF
run_target "$WORK/5-9-old-badfields.tsv" "$WORK/5-1-new.tsv"
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
assert_detail_key "MALFORMED: old 2" || ok=0
report "5-9a: フィールド数不足の行 → INDETERMINATE / MALFORMED: old 2" "$ok" "1"

write_list "$WORK/5-9-new-blankline.tsv" <<'EOF'
.	func	Foo	() ()

.	func	Bar	() ()
EOF
run_target "$WORK/5-1-old.tsv" "$WORK/5-9-new-blankline.tsv"
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
assert_detail_key "MALFORMED: new 2" || ok=0
report "5-9b: 空行を含む → INDETERMINATE / MALFORMED: new 2" "$ok" "1"

# 5-9c: タブが4個以上（フィールドが5個以上、多すぎる） → INDETERMINATE / MALFORMED
# split_fields はタブちょうど3個で4分割し、4個目以降のタブが残っていれば
# 不正形式として扱う（check-public-api-diff.sh:94-97）。「少なすぎ」側は
# 5-9a が固定しているが「多すぎ」側は未検査だった（Issue #93 reviewer W-1:
# この分岐を削っても fixture 21件が全通過していた実測）。
# トリガ行 `.	func	Extra	() ()	EXTRA` の実際のタブ数は4個（5フィールド）
# （Issue #93 reviewer W-1r: 旧コメント・旧 report ラベルの「5個」は
# off-by-one。判定基準（rc / verdict / detail key）は変更していない）。
write_list "$WORK/5-9-new-toomany.tsv" <<'EOF'
.	func	Foo	() ()
.	func	Extra	() ()	EXTRA
EOF
run_target "$WORK/5-1-old.tsv" "$WORK/5-9-new-toomany.tsv"
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
assert_detail_key "MALFORMED: new 2" || ok=0
report "5-9c: タブ4個以上（フィールドが多すぎる） → INDETERMINATE / MALFORMED: new 2" "$ok" "1"

# ==============================================================================
# AC-5-10: 同一キーの行が同じ一覧に2つ以上 → INDETERMINATE / DUPLICATE
# ==============================================================================

write_list "$WORK/5-10-old-dup.tsv" <<'EOF'
.	func	Foo	() ()
.	func	Foo	(int) ()
EOF
run_target "$WORK/5-10-old-dup.tsv" "$WORK/5-1-new.tsv"
ok=1
[ "$RC" = "1" ] || ok=0
assert_verdict "INDETERMINATE" || ok=0
assert_detail_key "DUPLICATE: old .	func	Foo" || ok=0
report "5-10: 同一キーの重複行 → INDETERMINATE / DUPLICATE" "$ok" "1"

# ==============================================================================
# AC-1-4 / AC-5-11 / AC-6-7: 入力に " / ` / $( を含む行があっても評価しない
# （シェルで評価されない＝副作用ファイルが生成されない）
# ==============================================================================

rm -f "$WORK/INJECTED_MARKER"
write_list "$WORK/5-11-new.tsv" <<'EOF'
.	func	Foo	() ()
.	func	Danger	(string) () // `id` "x" $(touch INJECTED_MARKER)
EOF
run_target "$WORK/5-1-old.tsv" "$WORK/5-11-new.tsv"
ok=1
[ "$RC" = "3" ] || ok=0
assert_verdict "WARN" || ok=0
assert_detail_key 'ADDED: .	func	Danger	(string) () // `id` "x" $(touch INJECTED_MARKER)' || ok=0
if [ -e "$WORK/INJECTED_MARKER" ] || \
   find "$WORK" -maxdepth 4 -name 'INJECTED_MARKER' -print -quit 2>/dev/null | grep -q .; then
  ok=0
fi
report "1-4 / 5-11: 危険な文字を含む行は評価されずそのまま現れる（副作用なし）" "$ok" "3"

# ==============================================================================
# AC-5-12: 入力が CRLF 改行でも判定を壊さない（\r をシグネチャへ混入させない）
# ==============================================================================

to_crlf "$WORK/5-1-old.tsv" "$WORK/5-12-old-crlf.tsv"
to_crlf "$WORK/5-1-new.tsv" "$WORK/5-12-new-crlf.tsv"
expect_ok "5-12a: 両側 CRLF で内容一致 → OK（\\r が判定を壊さない）" \
  "$WORK/5-12-old-crlf.tsv" "$WORK/5-12-new-crlf.tsv"

# 片側だけ CRLF、もう片側は LF で論理的に同じ内容 → \r がシグネチャへ混入すれば
# SIGNATURE_CHANGED という偽陽性が出る。それが起きないことを確認する。
expect_ok "5-12b: 旧側 CRLF・新側 LF で内容一致 → OK（\\r 混入なし）" \
  "$WORK/5-12-old-crlf.tsv" "$WORK/5-1-new.tsv"

# ==============================================================================
# AC-1-5: 入力の行順に依存しない（集合として比較する）
# ==============================================================================

write_list "$WORK/1-5-old-a.tsv" <<'EOF'
.	func	Alpha	() ()
.	func	Beta	() ()
.	func	Gamma	() ()
EOF
write_list "$WORK/1-5-old-b.tsv" <<'EOF'
.	func	Gamma	() ()
.	func	Alpha	() ()
.	func	Beta	() ()
EOF
write_list "$WORK/1-5-new.tsv" <<'EOF'
.	func	Alpha	() ()
.	func	Beta	() ()
.	func	Gamma	() ()
.	func	Delta	() ()
EOF
run_target "$WORK/1-5-old-a.tsv" "$WORK/1-5-new.tsv"
RC_A="$RC"; OUT_A="$OUT"
run_target "$WORK/1-5-old-b.tsv" "$WORK/1-5-new.tsv"
RC_B="$RC"; OUT_B="$OUT"
ok=1
[ "$RC_A" = "$RC_B" ] || ok=0
[ "$RC_A" = "3" ] || ok=0
[ "$(printf '%s\n' "$OUT_A" | sort)" = "$(printf '%s\n' "$OUT_B" | sort)" ] || ok=0
RC="$RC_B"; OUT="$OUT_B"
assert_verdict "WARN" || ok=0
assert_detail_key "ADDED: .	func	Delta	() ()" || ok=0
report "1-5: 旧側の行順を変えても判定が変わらない（集合比較）" "$ok" "3"

# ==============================================================================
# AC-6-8: ロケール設定で結果が変わらない（LC_ALL に依存する比較をロケール
# 非依存に検査する）。
# 「2回の実行が互いに一致する」だけでは、両方とも rc=127（本体が無い）で
# 一致してしまい空振りで緑になる。AC-5-3（両側にあり signature が違う →
# WARN / SIGNATURE_CHANGED, rc=3）の期待値を絶対値としても固定する。
# ==============================================================================

run_target --locale "C" "$WORK/5-3-old.tsv" "$WORK/5-3-new.tsv"
RC_C="$RC"; OUT_C="$OUT"
run_target --locale "C.utf8" "$WORK/5-3-old.tsv" "$WORK/5-3-new.tsv"
RC_U="$RC"; OUT_U="$OUT"
ok=1
[ "$RC_C" = "$RC_U" ] || ok=0
[ "$OUT_C" = "$OUT_U" ] || ok=0
[ "$RC_C" = "3" ] || ok=0
RC="$RC_C"; OUT="$OUT_C"
assert_verdict "WARN" || ok=0
assert_detail_key "SIGNATURE_CHANGED: .	func	Foo	old=() () new=(int) ()" || ok=0
report "6-8 / 5-3: LC_ALL=C と LC_ALL=C.utf8 で結果が変わらず、かつ AC-5-3 どおり WARN / SIGNATURE_CHANGED, rc=3" "$ok" "3"

# ==============================================================================
# AC-6-6: fixture はリポジトリ内の実ファイルを書き換えない
# （このテスト自身が check-public-api-diff.sh 以外へ書き込んでいないことは
# 上記のとおりすべて $WORK 配下への書き込みであることで担保される。
# ここでは検査対象スクリプトのハッシュが不変であることを明示的に固定する。）
#
# 対象（$TARGET）の有無で report を出す/出さないを分けない（AC-6-1: 本体が
# 無ければ失敗する。SKIP しない）。対象が無い間は sha256sum が失敗して
# BEFORE_HASH = AFTER_HASH = "" となり一致してしまうため、ハッシュ一致だけでは
# 空振りで緑になる。5-1 の入力（両側完全一致）で run_target した結果が
# AC-5-1 どおり VERDICT: OK / rc=0 であることも合わせて固定し、対象が無い間
# （rc=127）は確実に FAIL させる。
# ==============================================================================

BEFORE_HASH="$(sha256sum "$TARGET" 2>/dev/null || true)"
run_target "$WORK/5-1-old.tsv" "$WORK/5-1-new.tsv"
AFTER_HASH="$(sha256sum "$TARGET" 2>/dev/null || true)"
ok=1
[ "$RC" = "0" ] || ok=0
assert_verdict "OK" || ok=0
[ "$BEFORE_HASH" = "$AFTER_HASH" ] || ok=0
report "6-6 / 6-1: 検査対象スクリプト自身を書き換えず、対象不在でも黙って通らない" "$ok" "0"

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
