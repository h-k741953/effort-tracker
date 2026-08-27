#!/usr/bin/env bash
# Lambda 成果物（ドメイン API / CloudFront 遮断）を作る Makefile ターゲットの
# fixture テスト（Issue #8, C-2）。
#
# 仕様の単一情報源: docs/specs/infra-terraform.md
#   AC-9-2 zip の中身は `services/api/cmd/bootstrap` をビルドした単一の実行
#          ファイル `bootstrap`
#   AC-9-3 Linux 向けクロスコンパイルを Makefile のターゲットとして持つ。
#          ビルドと zip 作成を Terraform の中で行わない
#   AC-9-4 Terraform は zip のパスを変数で受け取る。生成物をリポジトリに
#          コミットしない（.gitignore に入れる）
#   AC-9-5 ターゲットは引数なしで実行でき、make help に説明が出る
#   AC-9-8 遮断 Lambda も同型（provided.al2023 + zip、単一の実行ファイル
#          `bootstrap`）。ドメイン API の zip とは別ファイルとして出力し、
#          同じ zip に2つの実行ファイルを同梱しない
#   AC-9-9 遮断 Lambda 側も Linux クロスコンパイル＋zip 作成を Makefile の
#          ターゲットとして持つ（AC-9-3 と同型）。引数なしで実行でき、
#          make help に説明が出る。生成物はコミットしない。Terraform の中で
#          ビルドしない
#
# 【実測済みの欠陥（C-2）】
#   本テスト作成時点で Makefile にも .gitignore にも zip / GOOS / GOARCH は
#   1件も無い。services/api/cmd/bootstrap・
#   services/api/cmd/cloudfront-killswitch は既に存在し、モジュール取得済み
#   で offline のまま `GOOS=linux GOARCH=amd64 go build` が通ることは
#   本テスト作成時に実測済みだが、それを zip にまとめて Lambda へ渡す手段が
#   リポジトリに無い。
#
# 【ターゲット名を本テストが決め打ちしない理由】
#   本仕様はドメイン API 側・遮断 Lambda 側いずれのターゲット名も固定して
#   いない。したがって本テストは Makefile の *内容*（レシピが GOOS=linux で
#   cmd/bootstrap または cmd/cloudfront-killswitch をビルドし、zip を作る
#   ことが読み取れるか）からターゲットを検出し、検出した名前で
#   `make <検出名>` を実引数なしで実行する。これにより実装工程が選んだ
#   ターゲット名に追随する（test-ci-terraform-test-target.sh のように
#   AC が名前を要求しない場合と同じ扱い）。
#
# 【検査する範囲・しない範囲】
#   する: (a) 該当レシピが Makefile 上に存在し make help に説明が出ること、
#         (b) 実引数なしで `make <target>` が成功すること、
#         (c) 生成された zip の中身が単一のエントリで、名前が厳密に
#             "bootstrap"（ディレクトリ接頭辞なし）であること、
#         (d) そのエントリが Linux の ELF 実行ファイルで実行権限を持つこと、
#         (e) ドメイン API 側と遮断 Lambda 側の zip パスが異なること、
#         (f) 生成された zip のパスが .gitignore で無視されること
#             （`git check-ignore` で実測。パターン文字列の一致では見ない）。
#   しない: infra/terraform 側の *.tf がビルド・zip 作成を内部で行っていない
#         ことの検査（AC-9-3 後段・AC-4-5）。infra/ は本 Issue の別工程が
#         並行して編集中であり、そちら側の内容に本テストの成否を結び付ける
#         と、C-2（Makefile 側の欠落）とは無関係な理由で Red/Green が揺れる。
#         この点は実装完了後、レビュー工程で infra 側の .tf を目視確認する
#         ことを申し送る。
#         GOARCH の値（amd64 か等）も検査しない（仕様が明示していない）。
#
# 単体実行: bash .github/scripts/test-makefile-lambda-package.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MAKEFILE="${REPO_ROOT}/Makefile"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

ok() { pass=$((pass + 1)); printf '  ok   %s\n' "$1"; }
ng() {
  fail=$((fail + 1))
  printf '  FAIL %s\n' "$1"
  shift
  local l
  for l in "$@"; do printf '       | %s\n' "$l"; done
}

echo "==> test-makefile-lambda-package"

# --- Makefile のターゲットをブロック単位で読み出す --------------------------
# ターゲット定義行（行頭・タブなし・"name: ..." の形）を見出しとし、
# 以降タブで始まる行をそのレシピとして name をキーに蓄積する。
declare -A recipe
declare -A header
current=""
while IFS= read -r line || [ -n "$line" ]; do
  if [[ "$line" == $'\t'* ]]; then
    if [ -n "$current" ]; then
      recipe["$current"]+="${line}"$'\n'
    fi
  else
    current=""
    if [[ "$line" =~ ^([A-Za-z0-9_.%-]+):(.*)$ ]]; then
      name="${BASH_REMATCH[1]}"
      current="$name"
      header["$name"]="${BASH_REMATCH[2]}"
    fi
  fi
done < "$MAKEFILE"

# find_target <must-contain...> -- <must-not-contain...>
# 標準出力に見つかったターゲット名を1つ返す（複数該当したら最初の1つ）。
find_target() {
  local -a must=() mustnot=()
  local seen_sep=0
  local a
  for a in "$@"; do
    if [ "$a" = "--" ]; then seen_sep=1; continue; fi
    if [ "$seen_sep" = 0 ]; then must+=("$a"); else mustnot+=("$a"); fi
  done
  local name body okflag
  for name in "${!recipe[@]}"; do
    body="${recipe[$name]}"
    okflag=1
    for a in "${must[@]}"; do
      command grep -qF -- "$a" <<< "$body" || { okflag=0; break; }
    done
    if [ "$okflag" = 1 ]; then
      for a in "${mustnot[@]}"; do
        command grep -qF -- "$a" <<< "$body" && { okflag=0; break; }
      done
    fi
    if [ "$okflag" = 1 ]; then
      echo "$name"
      return 0
    fi
  done
  return 1
}

domain_target="$(find_target "GOOS=linux" "cmd/bootstrap" -- "cloudfront-killswitch" || true)"
killswitch_target="$(find_target "GOOS=linux" "cmd/cloudfront-killswitch" -- || true)"
# zip 作成の痕跡（コマンド名の綴りに依存しないよう "zip" という語幹の有無で見る。
# python3 -m zipfile / zip -j のいずれでも "zip" という部分文字列を含む）。
if [ -n "$domain_target" ] && ! command grep -qi "zip" <<< "${recipe[$domain_target]}"; then
  domain_target=""
fi
if [ -n "$killswitch_target" ] && ! command grep -qi "zip" <<< "${recipe[$killswitch_target]}"; then
  killswitch_target=""
fi

if [ -n "$domain_target" ]; then
  ok "ドメイン API 用のビルド+zip ターゲットが見つかる（AC-9-2・AC-9-3）: $domain_target"
else
  ng "ドメイン API 用のビルド+zip ターゲットが見つかる（AC-9-2・AC-9-3）" \
    "expected: a Makefile target whose recipe builds cmd/bootstrap with GOOS=linux and creates a zip" \
    "actual: no matching target found in $MAKEFILE"
fi

if [ -n "$killswitch_target" ]; then
  ok "遮断 Lambda 用のビルド+zip ターゲットが見つかる（AC-9-8・AC-9-9）: $killswitch_target"
else
  ng "遮断 Lambda 用のビルド+zip ターゲットが見つかる（AC-9-8・AC-9-9）" \
    "expected: a Makefile target whose recipe builds cmd/cloudfront-killswitch with GOOS=linux and creates a zip" \
    "actual: no matching target found in $MAKEFILE"
fi

if [ -n "$domain_target" ] && [ -n "$killswitch_target" ] && [ "$domain_target" = "$killswitch_target" ]; then
  ng "ドメイン API と遮断 Lambda は別ターゲット（AC-9-8: 同じ zip に同梱しない）" \
    "expected: distinct targets" "actual: both resolved to '$domain_target'"
elif [ -n "$domain_target" ] && [ -n "$killswitch_target" ]; then
  ok "ドメイン API と遮断 Lambda は別ターゲット（AC-9-8）"
fi

# --- make help に説明が出ること（AC-9-5・AC-9-9） ----------------------------
help_out="$(cd "$REPO_ROOT" && make help 2>&1)"
for t in "$domain_target" "$killswitch_target"; do
  [ -n "$t" ] || continue
  if command grep -qF -- "$t" <<< "$help_out"; then
    ok "'make help' に $t の説明が出る（AC-9-5・AC-9-9）"
  else
    ng "'make help' に $t の説明が出る（AC-9-5・AC-9-9）" \
      "expected: '$t' listed in 'make help' output" "actual output:" "$help_out"
  fi
done

# --- 実際にビルドして zip の中身を検査する -----------------------------------
# `make -n <target>` は @ 付きレシピでも make 変数を展開して表示するため、
# 実行せずに出力先の .zip パスを特定できる（実行前に判定できる分、
# AC-9-8「別ファイル」を実行結果に頼らず先に確認できる）。
resolve_zip_path() {
  local target="$1" out
  out="$(cd "$REPO_ROOT" && make -n "$target" 2>/dev/null)" || return 1
  local line token dir
  while IFS= read -r line; do
    token="$(command grep -oE '[A-Za-z0-9._/+-]+\.zip' <<< "$line" | head -n1)"
    [ -n "$token" ] || continue
    dir=""
    if [[ "$line" =~ ^[[:space:]]*cd[[:space:]]+([^\&]+)\&\& ]]; then
      dir="$(command sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' <<< "${BASH_REMATCH[1]}")"
    fi
    if [ -n "$dir" ]; then
      echo "${dir%/}/${token}"
    else
      echo "$token"
    fi
    return 0
  done <<< "$out"
  return 1
}

domain_zip=""
killswitch_zip=""
[ -n "$domain_target" ] && domain_zip="$(resolve_zip_path "$domain_target")"
[ -n "$killswitch_target" ] && killswitch_zip="$(resolve_zip_path "$killswitch_target")"

if [ -n "$domain_target" ] && [ -z "$domain_zip" ]; then
  ng "ドメイン API ターゲットの出力 .zip パスを特定できる" \
    "expected: a *.zip path in 'make -n $domain_target' output" \
    "actual: none found"
fi
if [ -n "$killswitch_target" ] && [ -z "$killswitch_zip" ]; then
  ng "遮断 Lambda ターゲットの出力 .zip パスを特定できる" \
    "expected: a *.zip path in 'make -n $killswitch_target' output" \
    "actual: none found"
fi

if [ -n "$domain_zip" ] && [ -n "$killswitch_zip" ]; then
  if [ "$domain_zip" = "$killswitch_zip" ]; then
    ng "ドメイン API と遮断 Lambda の zip 出力先が異なる（AC-9-8）" \
      "expected: different output paths" "actual: both '$domain_zip'"
  else
    ok "ドメイン API と遮断 Lambda の zip 出力先が異なる（AC-9-8）: $domain_zip / $killswitch_zip"
  fi
fi

# check_zip_artifact <ラベル> <target> <zip-path>
# 実際にビルドを実行し、生成された zip の中身が単一の実行ファイル
# "bootstrap" であること・.gitignore に無視されることを検査する。
check_zip_artifact() {
  local label="$1" target="$2" zip_path="$3"
  [ -n "$target" ] && [ -n "$zip_path" ] || return 0

  local abs_zip="${REPO_ROOT}/${zip_path}"
  local build_out build_rc
  build_out="$(cd "$REPO_ROOT" && make "$target" 2>&1)"
  build_rc=$?

  if [ "$build_rc" -ne 0 ]; then
    ng "$label: 'make $target' が引数なしで成功する（AC-9-5・AC-9-9）" \
      "expected: exit code 0" "actual: $build_rc" "output:" "$build_out"
    return 0
  fi
  ok "$label: 'make $target' が引数なしで成功する（AC-9-5・AC-9-9）"

  if [ ! -f "$abs_zip" ]; then
    ng "$label: 期待した zip が生成される" \
      "expected: file exists at $zip_path" "actual: not found"
    return 0
  fi

  local entries entry_count
  entries="$(zipinfo -1 "$abs_zip" 2>/dev/null)"
  entry_count="$(command grep -c . <<< "$entries")"
  if [ "$entry_count" = 1 ] && [ "$entries" = "bootstrap" ]; then
    ok "$label: zip の中身は単一の実行ファイル 'bootstrap'（AC-9-2・AC-9-8）"
  else
    ng "$label: zip の中身は単一の実行ファイル 'bootstrap'（AC-9-2・AC-9-8）" \
      "expected: exactly one entry named 'bootstrap'" \
      "actual entries ($entry_count): $(tr '\n' ',' <<< "$entries")"
  fi

  local extract_dir="${WORK}/${label// /_}"
  mkdir -p "$extract_dir"
  if unzip -o -q "$abs_zip" -d "$extract_dir" 2>/dev/null && [ -f "${extract_dir}/bootstrap" ]; then
    if [ -x "${extract_dir}/bootstrap" ] && file "${extract_dir}/bootstrap" | command grep -qi "ELF"; then
      ok "$label: 'bootstrap' は実行権限を持つ Linux の ELF 実行ファイル（AC-9-3・AC-9-9）"
    else
      ng "$label: 'bootstrap' は実行権限を持つ Linux の ELF 実行ファイル（AC-9-3・AC-9-9）" \
        "expected: executable ELF binary" \
        "actual: $(file "${extract_dir}/bootstrap" 2>&1); perm=$(stat -c '%A' "${extract_dir}/bootstrap" 2>/dev/null)"
    fi
  else
    ng "$label: zip から 'bootstrap' を取り出せる" \
      "expected: 'bootstrap' extracted at zip root" "actual: extraction failed or file missing"
  fi

  if (cd "$REPO_ROOT" && git check-ignore -q -- "$zip_path"); then
    ok "$label: 生成物 $zip_path は .gitignore で無視される（AC-9-4・AC-9-9）"
  else
    ng "$label: 生成物 $zip_path は .gitignore で無視される（AC-9-4・AC-9-9）" \
      "expected: 'git check-ignore' succeeds for $zip_path" "actual: not ignored"
  fi

  # 後始末（zip は git 管理対象外である前提だが、作業木を汚さないよう消す。
  # 出力先ディレクトリが空になれば併せて畳む。中間生成物（ビルド途中の
  # 実行ファイル等）まではパスを特定していないため掃除しない。ターゲット
  # 自身のレシピが後始末する前提＝AC-9-4/AC-9-9 の「生成物をコミットしない」
  # の範囲は .gitignore 側の担保に委ねる）。
  rm -f "$abs_zip"
  rmdir "$(dirname "$abs_zip")" 2>/dev/null || true
}

check_zip_artifact "ドメイン API" "$domain_target" "$domain_zip"
check_zip_artifact "遮断 Lambda" "$killswitch_target" "$killswitch_zip"

echo ""
if [ "$fail" -ne 0 ]; then
  echo "  NG: $fail 件失敗 / $((pass + fail)) 件中"
  exit 1
fi
echo "  OK: $pass 件すべて通過"
