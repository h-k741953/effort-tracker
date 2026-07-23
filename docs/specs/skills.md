# Skills の作成 — 補助手順を `.claude/skills/` に切り出す

CLAUDE.md / `docs/rules/` に散文で埋まっている手順のうち、**「走らなくても工程・ゲート・停止条件が素通りしない補助手順」だけ**を Skill として `.claude/skills/` に切り出す。根拠は `docs/adr/0010-harness-engineering.md`（5構成要素の Skills）と、置き場所の判断基準 `docs/harness/commands-and-skills.md`。

**適格性の判定条件そのものはこのファイルに書き写さない。** 正解は `docs/harness/commands-and-skills.md` の判断木にあり、本仕様はそれを参照して各候補を分類する（ADR 0004）。手順の本文（ADR の書式・TDD の回し方）も同様に docs / ADR 側を正解とし、Skill・本仕様へコピーしない。

---

## 前提（P）

### P-1. Skill の実体は Claude Code のバージョンに依存する前提

| 項目 | 値 |
|---|---|
| 実体 | `.claude/skills/<name>/SKILL.md`（ディレクトリ名 = Skill 名、ファイル名は大文字 `SKILL.md`） |
| frontmatter | `name`（kebab-case slug）と `description`（1行） |
| `description` の役割 | **モデルが文脈から関連性を判断して自律起動するための唯一の手掛かり** |

**これは Claude Code のバージョンに依存する前提であり、本リポジトリが決められる事柄ではない**（`docs/specs/issue-command.md` P-2 の `gh` スキーマ、`docs/specs/orchestrator-entry-hook.md` P-1〜P-3 と同型）。形式が変われば本仕様の AC-3 は追随して更新する。

## スコープ（人間の決定 2026-07-23）

Skill 化の対象は**補助手順のみ**とする（`docs/harness/commands-and-skills.md` の判断木に厳密準拠）。TDD の回し方は工程エージェント（tester / implementer）が担うため Skill にしない。この決定の経緯は AC-2 の分類表に残す。

---

## 受け入れ条件（AC）

| AC | 主題 | 主に効く工程 |
|---|---|---|
| AC-1 | Skill 適格性は判断木に委ね、条件を書き写さない | specifier・reviewer |
| AC-2 | 候補手順のインベントリと分類（TDD 却下の経緯を残す） | specifier・reviewer |
| AC-3 | 適格な候補だけを Skill として作る（現時点は ADR 起票の1つ） | implementer |
| AC-4 | Skill 本文は docs を参照し、決定・仕様・手順本文をコピーしない | implementer・reviewer |
| AC-5 | 機械検査は「参照が張られているか」まで。範囲と限界を固定する | tester・implementer |

### AC-1. 適格性は `commands-and-skills.md` の判断木に委ねる

**Skill 化してよいのは「走らなくても工程・ゲート・停止条件が素通りしない補助手順」だけである。** 判定条件そのもの（軸1〜3・選び方の判断木）は `docs/harness/commands-and-skills.md` にあり、本仕様・Skill 本文へ書き写さない（ADR 0004）。

- 作成する各 Skill について、判断木の**「いいえ枝（走らなくても損害が無い補助手順）」に該当する根拠**が、本仕様 AC-2 の表に1行で記録されていること
- **却下した候補も、なぜ Skill でないか（どの枝で「はい」に落ちたか）を同表に残す。** 「Skill にしなかったこと」は記録がなければ観測できない（`commands-and-skills.md` 軸1）

### AC-2. 候補手順のインベントリと分類

CLAUDE.md / `docs/rules/` / ADR に散文で存在する手順を洗い出し、判断木のどの枝で確定するかを記録する。**判断木の「走らないと何かが素通りするか」に対する答えだけで分類し、本文の自然言語の勢いで決めない。**

| 候補手順 | 出所 | 走らないと素通りするか | 確定先 |
|---|---|---|---|
| ADR の起こし方（書式・粒度・不可変ルール） | `docs/adr/0001` の運用 | **いいえ**。書式が不揃いになるだけ。ADR を起こすか否かの判断・AI 非自己承認・既存 ADR 不可変は Rules 層（`docs/rules/responsibility.md` / ADR 0001）で担保される | **Skill**（`.claude/skills/adr-authoring/`） |
| TDD Red→Green→Refactor | `docs/rules/development-process.md` | **はい**。走らないと Red の確認と工程間の受け渡しが素通りする | サブエージェント（tester / implementer）。**Skill 却下** |
| 仕様書の起こし方（SDD） | `docs/rules/development-process.md` | **はい**。走らないと仕様なし実装が素通りする | サブエージェント（specifier）。**Skill 却下** |
| コミットの順序（`docs:`→`test:`→`feat:`） | `docs/rules/commit-convention.md` | **はい**。走らないと履歴に順序が残らない | 工程 / orchestrator が担う規約。**Skill 却下** |
| `make verify` によるコミット前検査 | `docs/rules/commands.md` | **はい**。走らないと lint / test / ドメイン依存検査が素通りする | Rules + CI（機械強制）。**Skill 却下** |

- **結論として、現時点で新設する Skill は `adr-authoring` の1つのみ**である。他の手順は工程エージェント・Rules・CI がすでに担っており、Skill にするとモデルの自律選択に委ねる分だけ「走らなかったこと」が観測できなくなる（`commands-and-skills.md` 軸1「必須の入口をモデルの選択に委ねない」）
- 「Skill が1つしかない」ことは不足ではなく、判断木が候補を正しく弾いた結果である。将来、工程にも Rules にも属さない補助手順が現れたら、本表に追記して Skill を増やす

### AC-3. 適格な候補だけを Skill として作る

AC-2 で確定先が **Skill** の候補についてのみ、以下を満たす `SKILL.md` を作る。

**`adr-authoring`**:

| 項目 | 要件 |
|---|---|
| パス | `.claude/skills/adr-authoring/SKILL.md` |
| `name` | `adr-authoring` |
| `description` | ADR を起草するとき（書式・構成・不可変ルール・粒度の目安）に選ばれる1行。**「起草の手順」であって「起票の可否判断」ではない**ことが読み取れること |
| 本文が持つもの | (a) ADR の構成（ステータス / 日付 / 決定者 / コンテキスト / 決定 / 影響 / 検討した代替案）を **`docs/adr/0001` への参照として**案内する手順、(b) 起草時の停止点（技術選定の変更・ADR の要否は人間の決定 → `docs/rules/responsibility.md`）、(c) AI が単独で「承認済み」にしない・既存 ADR を書き換えない（`docs/adr/0001` 運用ルール3・5）への参照 |
| 本文が持たないもの | ADR 0001 の運用ルール本文・責務分界表の**コピー**（AC-4） |

### AC-4. Skill 本文は docs を参照し、決定・仕様・手順本文をコピーしない

`SKILL.md` は**正解の置き場所（`docs/` / ADR）へのリンクを張り、そこを参照・実行する形**にする（ADR 0004、Issue #15 の「手順の実体は docs 側を正解とし、Skills はそれを参照・実行する」）。

- 決定した仕様・手順の本文を Skill へ転写して二重管理を作らない。散文を `docs/rules/` から Skill へ「引っ越す」のではなく、**Skill から `docs/` を参照させる**
- 各 `SKILL.md` は、その手順の正解が置かれている `docs/` 配下のパスへの参照を**本文に1つ以上持つ**

### AC-5. 機械検査の範囲と限界を固定する

プロンプト資産（Skill）について機械的に検査できるのは**「参照が張られているか」までである**（`docs/harness/commands-and-skills.md` 軸3）。記述内容が仕様どおりか・モデルが実際に Skill を選んだかは検査できない。

- **作る**: `make check-skills`。`.claude/skills/*/SKILL.md` の各ファイルについて、(1) frontmatter に `name` と `description` が存在し、(2) 本文に `docs/` への参照が1つ以上あることを **grep で機械検査**する。純粋なフィルタとして書き、fixture でローカル実行でき、`make verify` の依存に入り、CI（`.github/workflows/ci.yml`）から呼ばれること（`docs/harness/verification-loop.md` の「3点すべてを満たす」要求。Issue #30 の穴を踏まない）
- **検査できないこと（限界として明記）**: Skill 本文の手順が ADR 0001 と整合しているか / モデルがその Skill を選んだか。これは `commands-and-skills.md` の「プロンプト側の資産が仕様どおりに書かれているかは検査できない」と同型の限界であり、受け皿は規律層（`docs/rules/responsibility.md`）とレビュー工程である

---

## 非スコープ

- **TDD / SDD / コミット順序 / `make verify` の Skill 化**（AC-2 で却下済み。工程エージェント・Rules・CI が担う）
- **サブエージェント構成の変更**（分割軸・エージェント数）。人間の承認事項（ADR 0010 §A）
- **Hook の導入**（`PreToolUse` 等）。Issue #16 / #5 の範囲
- **ADR を起こすか否かの判断**。`adr-authoring` は起草の手順のみを持ち、起票の可否は人間が決める（`docs/rules/responsibility.md`）
- Skill の使用率（モデルが選んだか）の計測。軸1 の限界により機械では測れない

## 関連

- `docs/adr/0010-harness-engineering.md`: 5構成要素の Skills（実装 Issue 3）／自己改善の境界 §A（Skill の手順は改変してよいが、サブエージェント構成は承認事項）
- `docs/adr/0004-issue-docs-reference-model.md`: 正解は docs、参照で張る（AC-1 / AC-4）
- `docs/adr/0001-record-architecture-decisions.md`: ADR の構成・不可変・粒度・AI 非自己承認（AC-3 が参照させる正解）
- `docs/harness/commands-and-skills.md`: Skill / コマンド / サブエージェント / hook の使い分けの判断木（AC-1 / AC-2 / AC-5 の正解）
- `docs/harness/verification-loop.md`: 切り出したスクリプトへの make + CI + test の3点要求（AC-5）
- `docs/rules/responsibility.md`: プロンプト層が取りこぼしたものの受け皿（AC-5 の限界）
