#!/usr/bin/env bash
# check-cognito-sensitive-vars.sh の fixture テスト（Issue #52・Q-G）。
#
# 仕様の単一情報源: docs/specs/cognito-auth-infra.md AC-12
# （AC-9-g / AC-9-r の宣言側 —— 静的チェッカ。Q-G = (a)、2026-09-02 / 人間）。
#
# 【なぜこれが要るか（AC-9 前文・AC-11-13）】
#   `.tftest.hcl` + `mock_provider` + `command = plan` では、変数の宣言そのもの
#   （`sensitive` 属性・既定値の有無）を観測できない（sensitive は plan/state の
#   値に現れず、既定値の不在は `run` 全体を落とすため expect_failures でも
#   捕捉できない）。この2条（AC-4-3・AC-8-7）の宣言側だけを本チェッカへ移す。
#   本ファイルは「チェッカ本体」ではなく「チェッカを fixture で検査する側」
#   （AC-12-5）。チェッカ本体（check-cognito-sensitive-vars.sh）は実装工程が書く。
#
# 【対象の2変数（AC-12-1・AC-9-g・AC-9-r）】
#   変数名は infra/terraform/tests/ac_cognito_9_g_*.tftest.hcl /
#   ac_cognito_9_r_*.tftest.hcl が既に「暫定的に固定するインターフェース」として
#   持つものと同じ2つを使う（google_client_secret / role_cookie_signing_key）。
#   同じ変数を指す句を両側（.tftest.hcl とここ）で重複定義しない、という
#   AC-12-7 の分界を保つため、名前はこの既存の2ファイルへ合わせる。
#
# 【チェッカの呼び出し規約（本ファイルが固定する暫定インターフェース）】
#   check-cognito-sensitive-vars.sh [DIR]
#     DIR（第1引数・省略可）: 変数宣言を解析する対象ディレクトリ（*.tf を読む）。
#     省略時の既定値は "infra/terraform"（相対パス。make 経由でリポジトリ直下を
#     カレントディレクトリとして呼ばれる前提。AC-12-4「既定は実構成」）。
#   fixture を差し替えられるよう DIR を引数で受け取る（AC-12-5。「既存のチェッカが
#   入力を差し替えられる形と同型」＝check-go-module-pins.sh の位置引数と同型）。
#   終了コード: 0 = 緑（2変数とも sensitive かつ既定値なし）、0 以外 = 赤。
#   AC-12 は具体的な終了コード値・出力のキー名までは固定していないため、本テストは
#   「0 か 0 以外か」と「メッセージに変数名・該当する句が現れるか」（AC-12-6）
#   までを検査し、それ以上の出力形式は固定しない。
#
# 【本コミット時点でチェッカが存在しないこと（AC-12-5「Red を先に取る」）】
#   .github/scripts/check-cognito-sensitive-vars.sh はまだ存在しない。ここでは
#   「本体が無いのでスキップする」ような早期リターンを一切書かない。
#   bash "$TARGET" は「No such file or directory」で rc=127 を返し、緑側の
#   ケース（12-a・12-f・12-g）が rc=0 を要求しているため確実に FAIL し、
#   テスト全体が Red になる。赤側のケース（12-b〜12-e）は rc!=0 なら形の上では
#   通ってしまうが、変数名・句の substring 検査（AC-12-6）まで課しているため、
#   チェッカが無出力で落ちる場合はこれも FAIL する。
#
# 【AC-12-4 —— 実構成 infra/terraform に対しても走ること】
#   12-f・12-g は fixture ではなく実際の infra/terraform を対象にする。
#   本コミット時点では google_client_secret / role_cookie_signing_key の変数が
#   infra/terraform 側にまだ存在しない（実装工程の産物）ため、チェッカが実装
#   された後もこの2ケースは赤いままのはずである。これは仕様どおりであり、
#   本 Issue のテスト工程がここまでで担う範囲の限界である。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TARGET="${SCRIPT_DIR}/check-cognito-sensitive-vars.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

echo "==> test-check-cognito-sensitive-vars"

RC=""
OUT=""

# run_target <args...>: TARGET を WORK をカレントディレクトリとして実行し、
# RC/OUT を埋める（stdout・stderr は結合。AC-12 が出力ストリームを分けることを
# 要求していないため、check-review-trail.sh の検査と同じ結合方式にする）。
run_target() {
  local outf
  outf="$(mktemp -p "$WORK")"
  ( cd "$WORK" && bash "$TARGET" "$@" > "$outf" 2>&1 )
  RC=$?
  OUT="$(cat "$outf")"
  rm -f "$outf"
}

report() {
  local name="$1" ok="$2" want="$3"
  if [ "$ok" = 1 ]; then
    pass=$((pass + 1))
    printf '  ok   %s (rc=%s)\n' "$name" "$RC"
  else
    fail=$((fail + 1))
    printf '  FAIL %s (rc=%s, 期待=%s)\n' "$name" "$RC" "$want"
    printf '       out | %s\n' "$OUT" | sed 's/^/       | /'
  fi
}

# expect_green <名前> <dir>: rc=0 であること（両変数とも要求を満たす）。
expect_green() {
  local name="$1" dir="$2"
  run_target "$dir"
  local ok=1
  [ "$RC" = "0" ] || ok=0
  report "$name" "$ok" "rc=0"
}

# expect_red <名前> <dir> <var名> <句のキーワード>: rc!=0 であり、メッセージに
# 変数名と句のキーワード（sensitive / default）の両方が現れること（AC-12-6）。
expect_red() {
  local name="$1" dir="$2" varname="$3" clause="$4"
  run_target "$dir"
  local ok=1
  [ "$RC" != "0" ] || ok=0
  grep -qF -- "$varname" <<< "$OUT" || ok=0
  grep -qF -- "$clause" <<< "$OUT" || ok=0
  report "$name" "$ok" "rc!=0 かつ出力に ${varname} / ${clause} を含む"
}

# ==============================================================================
# 12-a: 両変数とも sensitive = true かつ既定値なし → 緑（AC-12-1 の充足）
# ==============================================================================

mkdir -p "${WORK}/12a"
cat > "${WORK}/12a/variables.tf" <<'EOF'
variable "google_client_secret" {
  description = "Google OAuth のクライアントシークレット（AC-4-3）。fixture のダミー宣言。"
  type        = string
  sensitive   = true
}

variable "role_cookie_signing_key" {
  description = "ロール切替 Cookie の署名鍵（AC-8-7）。fixture のダミー宣言。"
  type        = string
  sensitive   = true
}
EOF
expect_green "12-a: 両変数とも sensitive かつ既定値なし" "${WORK}/12a"

# ==============================================================================
# 12-b: google_client_secret から sensitive を外した宣言 → 赤（AC-12-5 (i)）
# ==============================================================================

mkdir -p "${WORK}/12b"
cat > "${WORK}/12b/variables.tf" <<'EOF'
variable "google_client_secret" {
  description = "sensitive を外した壊れた宣言（AC-12-5 (i)）。"
  type        = string
}

variable "role_cookie_signing_key" {
  description = "ロール切替 Cookie の署名鍵（AC-8-7）。fixture のダミー宣言。"
  type        = string
  sensitive   = true
}
EOF
expect_red "12-b: google_client_secret から sensitive を外した宣言" \
  "${WORK}/12b" "google_client_secret" "sensitive"

# ==============================================================================
# 12-c: role_cookie_signing_key から sensitive を外した宣言 → 赤（AC-12-5 (i)）
# ==============================================================================
# AC-12-1「シークレットの変数と署名鍵の変数のそれぞれについて」検査すること
# ―― 片方の変数だけを見て他方を見落とす実装を許さないため、2変数それぞれで
# (i)(ii) を確かめる。

mkdir -p "${WORK}/12c"
cat > "${WORK}/12c/variables.tf" <<'EOF'
variable "google_client_secret" {
  description = "Google OAuth のクライアントシークレット（AC-4-3）。fixture のダミー宣言。"
  type        = string
  sensitive   = true
}

variable "role_cookie_signing_key" {
  description = "sensitive を外した壊れた宣言（AC-12-5 (i)）。"
  type        = string
}
EOF
expect_red "12-c: role_cookie_signing_key から sensitive を外した宣言" \
  "${WORK}/12c" "role_cookie_signing_key" "sensitive"

# ==============================================================================
# 12-d: google_client_secret に既定値を与えた宣言 → 赤（AC-12-5 (ii)）
# ==============================================================================
# sensitive = true は付けたままにする ―― (i) を満たしていても (ii) 単体の
# 欠落だけで赤くなることを固定する（AC-12-1「(i)(ii)のいずれを欠いても赤くなる
# こと（片方だけを見る形にしない）」）。

mkdir -p "${WORK}/12d"
cat > "${WORK}/12d/variables.tf" <<'EOF'
variable "google_client_secret" {
  description = "既定値を与えた壊れた宣言（AC-12-5 (ii)）。"
  type        = string
  sensitive   = true
  default     = "dummy-default-should-not-exist"
}

variable "role_cookie_signing_key" {
  description = "ロール切替 Cookie の署名鍵（AC-8-7）。fixture のダミー宣言。"
  type        = string
  sensitive   = true
}
EOF
expect_red "12-d: google_client_secret に既定値を与えた宣言" \
  "${WORK}/12d" "google_client_secret" "default"

# ==============================================================================
# 12-e: role_cookie_signing_key に既定値を与えた宣言 → 赤（AC-12-5 (ii)）
# ==============================================================================

mkdir -p "${WORK}/12e"
cat > "${WORK}/12e/variables.tf" <<'EOF'
variable "google_client_secret" {
  description = "Google OAuth のクライアントシークレット（AC-4-3）。fixture のダミー宣言。"
  type        = string
  sensitive   = true
}

variable "role_cookie_signing_key" {
  description = "既定値を与えた壊れた宣言（AC-12-5 (ii)）。"
  type        = string
  sensitive   = true
  default     = "dummy-default-should-not-exist"
}
EOF
expect_red "12-e: role_cookie_signing_key に既定値を与えた宣言" \
  "${WORK}/12e" "role_cookie_signing_key" "default"

# ==============================================================================
# 12-f: 実構成 infra/terraform に対して明示的なパスで走り、緑であること
#       （AC-12-4）。本コミット時点では対象の2変数が infra/terraform に
#       まだ存在しないため、チェッカ実装後もここは赤いままのはずである
#       （実装工程が infra/terraform へ2変数を追加した時点で初めて緑になる）。
# ==============================================================================

expect_green "12-f: 実構成 infra/terraform（明示パス）が緑であること（AC-12-4）" \
  "${REPO_ROOT}/infra/terraform"

# ==============================================================================
# 12-g: 引数省略時は既定で実構成（infra/terraform）を検査すること（AC-12-4・
#       AC-12-5「既定は実構成」）。カレントディレクトリをリポジトリ直下にして
#       引数なしで呼ぶ（make 経由の呼び出しと同じカレントディレクトリ）。
# ==============================================================================

outf="$(mktemp -p "$WORK")"
( cd "$REPO_ROOT" && bash "$TARGET" > "$outf" 2>&1 )
RC=$?
OUT="$(cat "$outf")"
rm -f "$outf"
ok=1
[ "$RC" = "0" ] || ok=0
report "12-g: 引数省略時は既定で実構成を検査し緑であること（AC-12-4・AC-12-5）" "$ok" "rc=0"

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
