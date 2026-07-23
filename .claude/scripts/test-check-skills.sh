#!/usr/bin/env bash
# check-skills.sh の fixture テスト（Issue #15 / docs/specs/skills.md AC-5）。
#
# 【なぜこれが要るか】
#   AC-5 は check-skills が (1) frontmatter に name/description を持ち
#   (2) 本文に docs/ 参照を1つ以上持つ、を grep で機械検査すると定める。
#   プロンプト資産（Skill）について機械で検査できるのは「参照が張られているか」
#   まで（docs/harness/commands-and-skills.md 軸3）であり、本テストはその境界を
#   fixture で固定する。check-skills.sh は「引数で渡した skills ルートだけを
#   入力とする純粋なフィルタ」である前提なので（同 AC-5）、擬似ディレクトリを
#   差し替えるだけでローカルで完全に再現・検査できる。
#   .github/scripts/test-check-review-trail.sh / .claude/scripts/test-issue-gate.sh
#   と同じ型（ケース定義・実行ヘルパ・期待値比較・失敗時出力・件数サマリ）に倣う。
#
# 【擬似ディレクトリを使う理由】
#   実リポジトリの .claude/skills を対象にすると、将来 Skill を増減したときに
#   本テストが壊れる。各ケースは対象の1条件だけが verdict の理由になるよう
#   入力を単独化する（1行を実装から削ると最低1件 Red になる＝ミューテーション検知）。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="${SCRIPT_DIR}/check-skills.sh"

pass=0
fail=0

# fixture の skills ルートを作る。
#   $1 = 一時ルートのパス（呼び出し側が mktemp -d 済み）
#   残りの引数: "slug|frontmatter|body" の並び。SKILL.md を <slug>/SKILL.md に置く。
#   frontmatter は --- で囲まれる中身、body は --- の後の本文。
make_skill() {
  local root="$1"; shift
  local slug fm body dir spec
  for spec in "$@"; do
    slug="${spec%%|*}"; spec="${spec#*|}"
    fm="${spec%%|*}"; body="${spec#*|}"
    dir="${root}/${slug}"
    mkdir -p "${dir}"
    {
      printf -- '---\n'
      printf '%s\n' "${fm}"
      printf -- '---\n'
      printf '\n%s\n' "${body}"
    } > "${dir}/SKILL.md"
  done
}

# run_case <名前> <期待終了コード: 0=成功 / 1=違反> <root>
run_case() {
  local name="$1" want="$2" root="$3"
  local out code
  out="$(bash "${TARGET}" "${root}" 2>&1)"; code=$?
  local got=0
  [ "${code}" -ne 0 ] && got=1
  if [ "${got}" -eq "${want}" ]; then
    pass=$((pass+1))
  else
    fail=$((fail+1))
    echo "FAIL: ${name}"
    echo "  期待(0=成功/1=違反)=${want} 実際=${got}（exit=${code}）"
    echo "  --- 出力 ---"
    echo "${out}" | sed 's/^/  /'
  fi
}

VALID_FM=$'name: sample-skill\ndescription: サンプル手順を案内する1行'
VALID_BODY='手順の正解は docs/adr/0001-record-architecture-decisions.md を参照する。'

# 1. 正常: name/description あり・本文に docs/ 参照あり → 成功
r="$(mktemp -d)"; make_skill "$r" "sample-skill|${VALID_FM}|${VALID_BODY}"
run_case "valid skill は成功する" 0 "$r"; rm -rf "$r"

# 2. name 欠落 → 違反
r="$(mktemp -d)"; make_skill "$r" "no-name|description: 説明はあるが name が無い|${VALID_BODY}"
run_case "name 欠落は違反" 1 "$r"; rm -rf "$r"

# 3. description 欠落 → 違反
r="$(mktemp -d)"; make_skill "$r" "no-desc|name: no-desc|${VALID_BODY}"
run_case "description 欠落は違反" 1 "$r"; rm -rf "$r"

# 4. 本文に docs/ 参照が無い → 違反
r="$(mktemp -d)"; make_skill "$r" "no-ref|${VALID_FM}|参照が本文に無い散文だけがある。"
run_case "docs/ 参照が無いのは違反" 1 "$r"; rm -rf "$r"

# 5. name の値が空 → 違反（キーだけあって値が無い）
r="$(mktemp -d)"; make_skill "$r" "empty-name|"$'name:\ndescription: 値が空の name'"|${VALID_BODY}"
run_case "name の値が空は違反" 1 "$r"; rm -rf "$r"

# 6. 複数 Skill のうち1つが不正 → 全体が違反
r="$(mktemp -d)"
make_skill "$r" "ok-one|${VALID_FM}|${VALID_BODY}" "bad-two|name: bad-two|参照の無い本文"
run_case "1つでも不正なら違反" 1 "$r"; rm -rf "$r"

# 7. SKILL.md が1つも無い（Skill は任意の補助手順）→ 成功（SKIP 相当）
r="$(mktemp -d)"
run_case "Skill が無いディレクトリは成功（任意）" 0 "$r"; rm -rf "$r"

echo ""
echo "check-skills fixture: pass=${pass} fail=${fail}"
[ "${fail}" -eq 0 ]
