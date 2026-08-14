#!/usr/bin/env bash
# .devcontainer/Dockerfile の Go モジュール事前取得 pin（ARG）と
# services/api/go.mod の direct require の乖離を検査する（Issue #66）。
#
# 仕様の単一情報源: docs/specs/devcontainer-go-module-policy.md AC-5〜AC-7。
# 背景（なぜこの検査が要るか）は同仕様 P-3 の 3（ARG は go.mod の追随側の
# 二重管理であり、放置すれば必ずずれる）。
#
# 【純粋なフィルタ（AC-5-2）】
#   stdin と引数以外の入力を持たない。ネットワーク・git・go コマンドの
#   いずれも呼ばない。go を使わないのは AC-5-3（CI の scripts ジョブは
#   checkout のみで Go をセットアップしないため）。両ファイルをテキストとして
#   読むだけで判定する。
#
# 【入力をシェルへ展開しない（AC-5-4）】
#   ファイル内容は [[ =~ ]] によるパターン一致と文字列切り出しのみに使う。
#   eval・command substitution・バッククォート経由での実行はいずれも行わない。
#
# 入力: $1 = Dockerfile のパス, $2 = go.mod のパス（いずれも必須。既定値は
#       .devcontainer/Dockerfile / services/api/go.mod だが、fixture から
#       差し替えられることが要件である＝AC-5-1）。
#
# 出力契約（AC-7）: 1行目 `VERDICT: <名前>`、2行目以降 `<キー>: <値>` を stdout
#       へ。人間向けの説明は stderr。終了コードは 0 / 1 / 3 のみ（2 は
#       UserPromptSubmit hook のブロックに予約されているため使わない）。
set -uo pipefail

DEFAULT_DOCKERFILE=".devcontainer/Dockerfile"
DEFAULT_GOMOD="services/api/go.mod"

# --- 7-12: 引数が足りない（0個 / 1個） -----------------------------------------
if [ "$#" -lt 2 ]; then
  echo "VERDICT: INDETERMINATE"
  echo "第1引数（Dockerfile パス）と第2引数（go.mod パス）の両方が要る。" >&2
  echo "既定値は ${DEFAULT_DOCKERFILE} / ${DEFAULT_GOMOD}。" >&2
  exit 1
fi

DOCKERFILE="$1"
GOMOD="$2"

# --- 7-11: 引数のパスが存在しない・読めない ------------------------------------
if [ ! -f "$DOCKERFILE" ] || [ ! -r "$DOCKERFILE" ]; then
  echo "VERDICT: INDETERMINATE"
  echo "PATH: $DOCKERFILE"
  echo "Dockerfile を読めない: $DOCKERFILE" >&2
  exit 1
fi
if [ ! -f "$GOMOD" ] || [ ! -r "$GOMOD" ]; then
  echo "VERDICT: INDETERMINATE"
  echo "PATH: $GOMOD"
  echo "go.mod を読めない: $GOMOD" >&2
  exit 1
fi

# ==============================================================================
# Dockerfile を読む — ARG 定義と go get 行を抽出する（AC-6-1〜AC-6-3）。
# ==============================================================================

declare -A ARG_VALUE=()  # ARG名 -> 値（最後に見た値。件数チェックには使わない）
declare -A ARG_COUNT=()  # ARG名 -> 定義された回数

# pin 候補（形式に一致した go get 行のみ）。ARG 解決前の生データ。
declare -a PIN_MODULE=()
declare -a PIN_ARGNAME=()

BAD_LINE=""

lineno=0
while IFS= read -r raw_line || [ -n "$raw_line" ]; do
  lineno=$((lineno + 1))
  # CRLF 対策（AC-7-14）: \r をどの文字列にも混入させない。
  line="${raw_line%$'\r'}"

  # 先頭・末尾の空白を落とす（AC-6-2「行頭・行末の空白…は許す」）。
  trimmed="${line#"${line%%[![:space:]]*}"}"
  trimmed="${trimmed%"${trimmed##*[![:space:]]}"}"

  # コメント行（# で始まる）は対象にしない（AC-6-2）。7-13 の危険な文字混入
  # コメント行もここで弾かれ、以降の一致判定に一切かからない。
  if [[ "$trimmed" == \#* ]]; then
    continue
  fi

  # --- ARG 定義行 ---------------------------------------------------------
  if [[ "$trimmed" =~ ^ARG[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
    name="${BASH_REMATCH[1]}"
    val="${BASH_REMATCH[2]}"
    ARG_COUNT["$name"]=$(( ${ARG_COUNT["$name"]:-0} + 1 ))
    ARG_VALUE["$name"]="$val"
    continue
  fi

  # --- go get 行 -----------------------------------------------------------
  if [[ "$trimmed" != *"go get"* ]]; then
    continue
  fi

  # 末尾の空白・`\`・`;` を繰り返し落とす（継続行・複文の区切りを許す）。
  core="$trimmed"
  for _ in 1 2 3 4; do
    prev="$core"
    core="${core%"${core##*[![:space:]]}"}"   # 末尾空白
    core="${core%\\}"                          # 末尾 backslash
    core="${core%"${core##*[![:space:]]}"}"
    core="${core%;}"                           # 末尾 semicolon
    core="${core%"${core##*[![:space:]]}"}"
    [ "$core" = "$prev" ] && break
  done

  if [[ "$core" =~ ^go[[:space:]]get[[:space:]]\"([^\"[:space:]@]+)@\$\{([A-Za-z_][A-Za-z0-9_]*)\}\"$ ]]; then
    PIN_MODULE+=("${BASH_REMATCH[1]}")
    PIN_ARGNAME+=("${BASH_REMATCH[2]}")
  else
    # AC-6-2: コメント以外に go get を含み形式に一致しない行が1つでもあれば
    # INDETERMINATE。黙って無視しない。最初の1件の行番号を記録する。
    if [ -z "$BAD_LINE" ]; then
      BAD_LINE="$lineno"
    fi
  fi
done < "$DOCKERFILE"

# --- 7-9: 形式に一致しない go get 行 ------------------------------------------
if [ -n "$BAD_LINE" ]; then
  echo "VERDICT: INDETERMINATE"
  echo "LINE: $BAD_LINE"
  echo "Dockerfile の $BAD_LINE 行目が \`go get \"<module>@\${<ARG名>}\"\` 形式に一致しない。" >&2
  exit 1
fi

# --- 7-10: ${<ARG名>} を解決する（AC-6-3） ------------------------------------
declare -A PIN_VERSION=()  # module -> version（Dockerfile 側）

i=0
while [ "$i" -lt "${#PIN_MODULE[@]}" ]; do
  mod="${PIN_MODULE[$i]}"
  arg="${PIN_ARGNAME[$i]}"
  cnt="${ARG_COUNT["$arg"]:-0}"
  if [ "$cnt" -ne 1 ]; then
    echo "VERDICT: INDETERMINATE"
    echo "ARG: $arg"
    if [ "$cnt" -eq 0 ]; then
      echo "\${$arg} に対応する ARG 行が Dockerfile に無い。" >&2
    else
      echo "\${$arg} に対応する ARG 行が $cnt 個ある（後勝ち・先勝ちを推測しない）。" >&2
    fi
    exit 1
  fi
  PIN_VERSION["$mod"]="${ARG_VALUE["$arg"]}"
  i=$((i + 1))
done

# --- 7-7: pin の組が1つも取れない → NO_PIN（成功に倒さない） -------------------
if [ "${#PIN_VERSION[@]}" -eq 0 ]; then
  echo "VERDICT: NO_PIN"
  echo "Dockerfile から go get のモジュール pin が1つも取れなかった: $DOCKERFILE" >&2
  exit 1
fi

# ==============================================================================
# go.mod を読む — direct require のみを対象にする（AC-6-4）。
# ブロック形式 `require ( ... )` と単一行形式 `require <path> <version>` の
# 両方を読む。`// indirect` を持つ require は対象外。
# ==============================================================================

declare -A REQ_VERSION=()  # module -> version（go.mod 側、direct のみ）
in_block=0

while IFS= read -r raw_line || [ -n "$raw_line" ]; do
  line="${raw_line%$'\r'}"

  trimmed="${line#"${line%%[![:space:]]*}"}"
  trimmed="${trimmed%"${trimmed##*[![:space:]]}"}"

  if [ "$in_block" -eq 1 ]; then
    if [ "$trimmed" = ")" ]; then
      in_block=0
      continue
    fi
    body="$trimmed"
  elif [[ "$trimmed" =~ ^require[[:space:]]*\($ ]]; then
    in_block=1
    continue
  elif [[ "$trimmed" =~ ^require[[:space:]](.*)$ ]]; then
    body="${BASH_REMATCH[1]}"
    body="${body#"${body%%[![:space:]]*}"}"
  else
    continue
  fi

  # 行内コメント（// ...）を分離し、indirect 判定に使ってから core を切り出す。
  comment=""
  core="$body"
  if [[ "$body" == *"//"* ]]; then
    core="${body%%//*}"
    comment="${body#*//}"
  fi
  core="${core%"${core##*[![:space:]]}"}"
  [ -z "$core" ] && continue

  is_indirect=0
  if [[ "$comment" == *indirect* ]]; then
    is_indirect=1
  fi

  # 先頭トークン = モジュールパス、2番目のトークン = バージョン。
  read -r mod ver _rest <<< "$core"
  [ -z "$mod" ] && continue
  [ -z "$ver" ] && continue

  if [ "$is_indirect" -eq 0 ]; then
    REQ_VERSION["$mod"]="$ver"
  fi
done < "$GOMOD"

# --- 7-8: direct require が1つも無い → NO_REQUIRE（成功に倒さない） -----------
if [ "${#REQ_VERSION[@]}" -eq 0 ]; then
  echo "VERDICT: NO_REQUIRE"
  echo "go.mod に direct require が1つも無かった: $GOMOD" >&2
  exit 1
fi

# ==============================================================================
# 双方向で比較する（AC-6-6）。行順・コメント位置・ブロックの並びは使わない。
# ==============================================================================

declare -a VIOLATIONS=()

for mod in "${!PIN_VERSION[@]}"; do
  dver="${PIN_VERSION["$mod"]}"
  if [ -z "${REQ_VERSION["$mod"]+set}" ]; then
    VIOLATIONS+=("MODULE_NOT_REQUIRED: $mod $dver")
  else
    gver="${REQ_VERSION["$mod"]}"
    if [ "$dver" != "$gver" ]; then
      VIOLATIONS+=("VERSION_MISMATCH: $mod dockerfile=$dver gomod=$gver")
    fi
  fi
done

for mod in "${!REQ_VERSION[@]}"; do
  if [ -z "${PIN_VERSION["$mod"]+set}" ]; then
    VIOLATIONS+=("MISSING_PIN: $mod ${REQ_VERSION["$mod"]}")
  fi
done

if [ "${#VIOLATIONS[@]}" -gt 0 ]; then
  echo "VERDICT: PIN_DRIFT"
  for v in "${VIOLATIONS[@]}"; do
    echo "$v"
  done
  {
    echo "Dockerfile の ARG pin と go.mod の direct require が食い違っている。"
    echo "docs/specs/devcontainer-go-module-policy.md P-3 のとおり go.mod が正解の実体であり、"
    echo "Dockerfile の ARG をそちらへ追随させること（逆はしない）。"
  } >&2
  exit 3
fi

echo "VERDICT: OK"
echo "PAIRS: ${#PIN_VERSION[@]}"
exit 0
