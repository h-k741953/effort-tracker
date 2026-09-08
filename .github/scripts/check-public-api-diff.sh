#!/usr/bin/env bash
# 公開 API 差分の比較器（Issue #93）。
#
# 仕様の単一情報源: docs/specs/public-api-diff-check.md AC-1〜AC-5。
#
# 【純粋なフィルタ（AC-1-1〜AC-1-3）】
#   第1引数 = 旧側のエクスポート一覧のパス、第2引数 = 新側のエクスポート
#   一覧のパス。既定値は持たない。引数以外の入力を持たない。ネットワーク・
#   git・go のいずれも呼ばない（抽出は services/api/cmd/exportlist が担う。
#   比較器と fixture が Go を要求すると、fixture が抽出器の置き場所
#   〈D-1〉に巻き込まれる）。
#
# 【入力をシェルへ展開しない（AC-1-4）】
#   1行は必ず「素の文字列」として扱う。command substitution（`` ` ``・
#   `$(...)`）にも eval にも通さない。1行をタブちょうど3個で4分割する
#   処理は、bash の word splitting（IFS=$'\t' read -a）を使わない ――
#   tab は bash の「IFS 空白文字」として連続する区切りを1つに畳んでしまい
#   （実測済み）、空フィールドが消える。代わりにパラメータ展開の
#   `%%`/`#*` だけで切り出す（split_fields）。これは評価ではなく文字列の
#   前後だけを見る操作であり、内容がどんな文字列でも安全である。
#
# 【行順に依存しない（AC-1-5）】
#   キー（pkg+kind+name）ごとに連想配列へ積み、集合として比較する。
#
# 【CRLF（AC-1-6 / AC-5-12）】
#   各行の末尾の \r を読み取り直後に落とす。どちら側の入力が CRLF でも
#   判定・シグネチャ文字列を汚染しない。
#
# 出力契約（AC-5）: 1行目に `VERDICT: <名前>`、2行目以降に `<キー>: <値>`
# を stdout へ。人間向けの説明は stderr。終了コードは 0 / 1 / 3 のみ
# （2 は UserPromptSubmit hook のブロックに予約されているため使わない）。
set -uo pipefail

# --- AC-5-6: 引数が足りない ----------------------------------------------------
if [ "$#" -lt 2 ]; then
  echo "VERDICT: INDETERMINATE"
  echo "ARGS: $#"
  echo "第1引数（旧側の一覧パス）と第2引数（新側の一覧パス）の両方が要る。" >&2
  exit 1
fi

OLD="$1"
NEW="$2"

# --- AC-5-7: 引数のパスが存在しない・読めない ----------------------------------
if [ ! -f "$OLD" ] || [ ! -r "$OLD" ]; then
  echo "VERDICT: INDETERMINATE"
  echo "PATH: $OLD"
  echo "旧側の一覧を読めない: $OLD" >&2
  exit 1
fi
if [ ! -f "$NEW" ] || [ ! -r "$NEW" ]; then
  echo "VERDICT: INDETERMINATE"
  echo "PATH: $NEW"
  echo "新側の一覧を読めない: $NEW" >&2
  exit 1
fi

# --- AC-5-8: どちらかの一覧が0行（0バイト） ------------------------------------
if [ ! -s "$OLD" ]; then
  echo "VERDICT: INDETERMINATE"
  echo "EMPTY: old"
  echo "旧側の一覧が0行: $OLD" >&2
  exit 1
fi
if [ ! -s "$NEW" ]; then
  echo "VERDICT: INDETERMINATE"
  echo "EMPTY: new"
  echo "新側の一覧が0行: $NEW" >&2
  exit 1
fi

# --- フィールド分割 -------------------------------------------------------------
# タブちょうど3個で4分割する。多すぎ・少なすぎのどちらも不正形式として
# 扱う（AC-5-9）。IFS を使った word splitting は tab を「IFS 空白文字」として
# 畳んでしまうため使わない（上のコメント参照）。グローバル F1〜F4 へ書く。
split_fields() {
  local rest="$1"
  if [[ "$rest" != *$'\t'* ]]; then
    return 1
  fi
  F1="${rest%%$'\t'*}"
  rest="${rest#*$'\t'}"
  if [[ "$rest" != *$'\t'* ]]; then
    return 1
  fi
  F2="${rest%%$'\t'*}"
  rest="${rest#*$'\t'}"
  if [[ "$rest" != *$'\t'* ]]; then
    return 1
  fi
  F3="${rest%%$'\t'*}"
  rest="${rest#*$'\t'}"
  if [[ "$rest" == *$'\t'* ]]; then
    # 5個目以降のタブがある = フィールドが多すぎる。
    return 1
  fi
  F4="$rest"
  return 0
}

declare -A OLD_SIG=()
declare -A NEW_SIG=()
declare -a OLD_KEYS=()
declare -a NEW_KEYS=()

# parse_list <パス> <old|new>: AC-5-9（不正形式）・AC-5-10（重複キー）を
# 検出しながら、連想配列 <label>_SIG[key] = signature を埋める。
# 不正形式・重複を見つけた時点で VERDICT を出して即終了する。
parse_list() {
  local path="$1" label="$2"
  local lineno=0
  local line key

  while IFS= read -r line || [ -n "$line" ]; do
    lineno=$((lineno + 1))
    line="${line%$'\r'}"

    if ! split_fields "$line"; then
      echo "VERDICT: INDETERMINATE"
      echo "MALFORMED: $label $lineno"
      echo "タブ区切り4フィールドでない行がある: $label 側 $lineno 行目。" >&2
      exit 1
    fi

    key="$F1"$'\t'"$F2"$'\t'"$F3"

    if [ "$label" = "old" ]; then
      if [ -n "${OLD_SIG[$key]+set}" ]; then
        echo "VERDICT: INDETERMINATE"
        echo "DUPLICATE: old $F1"$'\t'"$F2"$'\t'"$F3"
        echo "同一キー（pkg+kind+name）の行が旧側に複数ある。" >&2
        exit 1
      fi
      OLD_SIG["$key"]="$F4"
      OLD_KEYS+=("$key")
    else
      if [ -n "${NEW_SIG[$key]+set}" ]; then
        echo "VERDICT: INDETERMINATE"
        echo "DUPLICATE: new $F1"$'\t'"$F2"$'\t'"$F3"
        echo "同一キー（pkg+kind+name）の行が新側に複数ある。" >&2
        exit 1
      fi
      NEW_SIG["$key"]="$F4"
      NEW_KEYS+=("$key")
    fi
  done < "$path"
}

parse_list "$OLD" "old"
parse_list "$NEW" "new"

# --- AC-4: 比較規則 --------------------------------------------------------------
# キーは pkg+kind+name の3つ組。signature はキーに含めない（AC-4-1）。
declare -a ADDED_LINES=()
declare -a CHANGED_LINES=()
declare -a REMOVED_LINES=()

for key in "${NEW_KEYS[@]}"; do
  if [ -z "${OLD_SIG[$key]+set}" ]; then
    ADDED_LINES+=("ADDED: ${key}"$'\t'"${NEW_SIG[$key]}")
  elif [ "${OLD_SIG[$key]}" != "${NEW_SIG[$key]}" ]; then
    CHANGED_LINES+=("SIGNATURE_CHANGED: ${key}"$'\t'"old=${OLD_SIG[$key]} new=${NEW_SIG[$key]}")
  fi
done

for key in "${OLD_KEYS[@]}"; do
  if [ -z "${NEW_SIG[$key]+set}" ]; then
    REMOVED_LINES+=("REMOVED: ${key}"$'\t'"${OLD_SIG[$key]}")
  fi
done

added_count="${#ADDED_LINES[@]}"
changed_count="${#CHANGED_LINES[@]}"
removed_count="${#REMOVED_LINES[@]}"

# 出力順は集合比較の判定に影響しない（AC-1-5）が、人間が読みやすいよう
# キーでソートする。
if [ "$added_count" -gt 0 ]; then
  mapfile -t ADDED_LINES < <(printf '%s\n' "${ADDED_LINES[@]}" | sort)
fi
if [ "$changed_count" -gt 0 ]; then
  mapfile -t CHANGED_LINES < <(printf '%s\n' "${CHANGED_LINES[@]}" | sort)
fi
if [ "$removed_count" -gt 0 ]; then
  mapfile -t REMOVED_LINES < <(printf '%s\n' "${REMOVED_LINES[@]}" | sort)
fi

# --- AC-4-6: verdict ------------------------------------------------------------
if [ "$added_count" -gt 0 ] || [ "$changed_count" -gt 0 ]; then
  echo "VERDICT: WARN"
else
  echo "VERDICT: OK"
fi

for l in "${ADDED_LINES[@]:-}"; do
  [ -n "$l" ] && printf '%s\n' "$l"
done
for l in "${CHANGED_LINES[@]:-}"; do
  [ -n "$l" ] && printf '%s\n' "$l"
done
for l in "${REMOVED_LINES[@]:-}"; do
  [ -n "$l" ] && printf '%s\n' "$l"
done

echo "COUNT: added=${added_count} signature_changed=${changed_count} removed=${removed_count}"

if [ "$added_count" -gt 0 ] || [ "$changed_count" -gt 0 ]; then
  {
    echo "公開 API に差分がある（ADDED または SIGNATURE_CHANGED）。"
    echo "見るべき差分の候補であり、判定ではない（docs/specs/public-api-diff-check.md AC-9-1）。"
  } >&2
  exit 3
fi

exit 0
