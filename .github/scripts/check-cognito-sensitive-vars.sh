#!/usr/bin/env bash
# AC-9-g / AC-9-r の宣言側 —— 静的チェッカ（Issue #52・Q-G = (a)）。
#
# 仕様の単一情報源: docs/specs/cognito-auth-infra.md AC-12
# （AC-9 が指定する `.tftest.hcl` + `mock_provider` + `command = plan` では、
#   変数の宣言そのもの＝`sensitive` 属性・既定値の有無を観測できないため、
#   その宣言側だけを本チェッカへ移す。理由は同仕様 AC-9 前文・AC-11-13）。
#
# 【検査すること（AC-12-1）】
#   `infra/terraform` の変数宣言を解析し、シークレットの変数（AC-4-3。名前は
#   google_client_secret）と署名鍵の変数（AC-8-7。名前は role_cookie_signing_key）
#   のそれぞれについて、(i) `sensitive` として宣言されていること、
#   (ii) 既定値を持たないこと、を検査する。(i)(ii) のいずれを欠いても赤くなる。
#   変数が1つも見つからない場合も「満たしている」ではなく「確かめられていない」
#   ため、緑にせず赤にする。
#
# 【値そのものを見ない（AC-12-1・P-4・AC-1-9）】
#   実値もダミー値も期待値として持たない。見るのは宣言の字面（`sensitive` の
#   有無・`default` 属性の有無）だけであり、値が変数経由で流れ込むことは
#   `.tftest.hcl` 側（AC-9-g / AC-9-r）の担当である（AC-12-7）。
#
# 【解析の限界（AC-11-13）】
#   構成の字面を読むのであって plan を評価しない。変数がどのリソースへ
#   流れるか・apply で受理されるかは見ない。
#
# 呼び出し規約（test-check-cognito-sensitive-vars.sh が固定する暫定インター
# フェース）:
#   check-cognito-sensitive-vars.sh [DIR]
#     DIR（第1引数・省略可）: 変数宣言を解析する対象ディレクトリ（*.tf を読む）。
#     省略時の既定値は "infra/terraform"（AC-12-4「既定は実構成」）。
#   終了コード: 0 = 緑（対象の2変数がいずれも要求を満たす）、0 以外 = 赤。
#   赤の場合、標準出力に違反した変数名と句（sensitive / default）を含める
#   （AC-12-6「どの変数のどの句が要求を満たしていないかを出す」）。
#
# 【新しい依存を足さない（AC-12-2・AC-1-7）】
#   bash と標準コマンド（find・sort）のみを使う。jq・hcl2json 等の外部ツール・
#   言語処理系は使わない。
set -uo pipefail

DIR="${1:-infra/terraform}"

# 対象の2変数（AC-9-g / AC-9-r・AC-4-3 / AC-8-7）。名前はこれ以外へ広げない
# （AC-12 前文「対象は AC-9-g / AC-9-r の2条に限る」）。
TARGET_VARS=("google_client_secret" "role_cookie_signing_key")

if [ ! -d "$DIR" ]; then
  echo "NG: 対象ディレクトリが無い: $DIR"
  exit 1
fi

# 対象ディレクトリ配下の *.tf を集める（.terraform/ のダウンロード済みモジュール
# は除く）。順序をファイルシステム依存にしないよう固定順でソートする。
mapfile -t TF_FILES < <(find "$DIR" -type f -name '*.tf' -not -path '*/.terraform/*' 2>/dev/null | LC_ALL=C sort)

# 変数ごとの解析結果。見つからなければ FOUND のまま 0（= 満たしていない扱い）。
declare -A FOUND=()
declare -A SENSITIVE_TRUE=()
declare -A HAS_DEFAULT=()
for v in "${TARGET_VARS[@]}"; do
  FOUND["$v"]=0
  SENSITIVE_TRUE["$v"]=0
  HAS_DEFAULT["$v"]=0
done

is_target() {
  local name="$1" v
  for v in "${TARGET_VARS[@]}"; do
    [ "$name" = "$v" ] && return 0
  done
  return 1
}

for f in "${TF_FILES[@]}"; do
  in_var=0
  cur_name=""
  depth=0

  while IFS= read -r raw_line || [ -n "$raw_line" ]; do
    line="${raw_line%$'\r'}"

    if [ "$in_var" -eq 0 ]; then
      if [[ "$line" =~ ^[[:space:]]*variable[[:space:]]+\"([^\"]+)\"[[:space:]]*\{ ]]; then
        cur_name="${BASH_REMATCH[1]}"
        in_var=1
        depth=1
        if is_target "$cur_name"; then
          FOUND["$cur_name"]=1
        fi
      fi
      continue
    fi

    # in_var=1: ブロック内。depth==1（変数ブロック直下）のときだけ、
    # sensitive / default の属性行を判定する（validation ブロックなど
    # ネストした内側の属性を誤検出しない）。
    if [ "$depth" -eq 1 ] && is_target "$cur_name"; then
      if [[ "$line" =~ ^[[:space:]]*sensitive[[:space:]]*=[[:space:]]*true[[:space:]]*(#.*)?$ ]]; then
        SENSITIVE_TRUE["$cur_name"]=1
      fi
      if [[ "$line" =~ ^[[:space:]]*default[[:space:]]*= ]]; then
        HAS_DEFAULT["$cur_name"]=1
      fi
    fi

    # 波括弧の増減で depth を更新する（同一行の { と } の数を数える）。
    local_open="${line//[^\{]/}"
    local_close="${line//[^\}]/}"
    depth=$(( depth + ${#local_open} - ${#local_close} ))

    if [ "$depth" -le 0 ]; then
      in_var=0
      cur_name=""
      depth=0
    fi
  done < "$f"
done

VIOLATIONS=()
for v in "${TARGET_VARS[@]}"; do
  if [ "${FOUND["$v"]}" -ne 1 ]; then
    VIOLATIONS+=("$v: variable が見つからない（宣言が無いため sensitive / 既定値なしを確かめられない）")
    continue
  fi
  if [ "${SENSITIVE_TRUE["$v"]}" -ne 1 ]; then
    VIOLATIONS+=("$v: sensitive = true が宣言されていない")
  fi
  if [ "${HAS_DEFAULT["$v"]}" -ne 0 ]; then
    VIOLATIONS+=("$v: default（既定値）が宣言されている")
  fi
done

if [ "${#VIOLATIONS[@]}" -gt 0 ]; then
  echo "NG: infra/terraform の変数宣言が AC-4-3 / AC-8-7 を満たしていない（対象: $DIR）"
  for v in "${VIOLATIONS[@]}"; do
    echo "  - $v"
  done
  exit 1
fi

echo "OK: 対象の変数がいずれも sensitive = true かつ既定値なしで宣言されている（対象: $DIR）"
for v in "${TARGET_VARS[@]}"; do
  echo "  - $v"
done
exit 0
