#!/usr/bin/env bash
# Skill 資産（.claude/skills/*/SKILL.md）の形式検査。
#
# docs/specs/skills.md AC-5 / docs/harness/commands-and-skills.md 軸3。
# 機械で検査できるのは「参照が張られているか」まで。記述内容が仕様どおりか・
# モデルが実際に Skill を選んだかは検査できない（受け皿は規律層と review 工程）。
#
# 純粋なフィルタ: 第1引数で渡した skills ルートだけを入力とする。既定は
# .claude/skills。入力を差し替えれば fixture でローカル完全再現できる
# （.claude/scripts/test-check-skills.sh）。
#
# 検査項目（各 SKILL.md について）:
#   (1) frontmatter（先頭 --- から次の --- まで）に name と description があり、
#       いずれも値が空でないこと
#   (2) 本文（frontmatter の後）に docs/ への参照が1つ以上あること
set -uo pipefail

ROOT="${1:-.claude/skills}"

if [ ! -d "${ROOT}" ]; then
  echo "SKIP: ${ROOT} が無い（Skill 未導入。Skill は任意の補助手順）"
  exit 0
fi

# <slug>/SKILL.md のみを対象にする（ルート直下のスクリプト等は拾わない）。
mapfile -t files < <(find "${ROOT}" -mindepth 2 -maxdepth 2 -type f -name 'SKILL.md' | sort)

if [ "${#files[@]}" -eq 0 ]; then
  echo "SKIP: ${ROOT} 配下に SKILL.md が無い（Skill は任意の補助手順）"
  exit 0
fi

violations=0
checked=0
for f in "${files[@]}"; do
  checked=$((checked+1))

  # frontmatter（先頭 --- から次の --- までの中身）を取り出す。
  fm="$(awk 'NR==1 && $0=="---"{infm=1; next} infm && $0=="---"{exit} infm{print}' "${f}")"
  # 本文（2つ目の --- 以降）を取り出す。
  body="$(awk 'body{print} /^---$/{c++; if(c==2) body=1}' "${f}")"

  msgs=()
  if ! printf '%s\n' "${fm}" | grep -Eq '^name:[[:space:]]*[^[:space:]]'; then
    msgs+=("frontmatter に name（値つき）が無い")
  fi
  if ! printf '%s\n' "${fm}" | grep -Eq '^description:[[:space:]]*[^[:space:]]'; then
    msgs+=("frontmatter に description（値つき）が無い")
  fi
  if ! printf '%s\n' "${body}" | grep -q 'docs/'; then
    msgs+=("本文に docs/ への参照が無い（正解は docs/ に置き参照で張る。ADR 0004）")
  fi

  if [ "${#msgs[@]}" -gt 0 ]; then
    violations=$((violations+1))
    echo "  NG: ${f}"
    for m in "${msgs[@]}"; do echo "    - ${m}"; done
  fi
done

if [ "${violations}" -gt 0 ]; then
  echo "  → docs/specs/skills.md AC-3 / AC-4 と docs/harness/commands-and-skills.md を確認すること。"
  exit 1
fi

echo "  OK: ${checked} 件の SKILL.md（frontmatter name/description + docs/ 参照）"
exit 0
