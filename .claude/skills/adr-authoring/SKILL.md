---
name: adr-authoring
description: ADR を起草するとき（構成・書式・不可変ルール・粒度の目安）に参照する補助手順。ADR を起こすか否かの可否判断ではなく、起こすと決まった後の書き方を案内する。
---

# ADR を起草する

これは **ADR を起こすと決まった後の起草手順**の案内である。**ADR を起こすか否か・技術選定を変えてよいかの判断は含まない**。可否は人間の決定事項であり、`docs/rules/responsibility.md`（必ず人間に確認を取る）に従って停止・確認する。

手順の正解は本文へ書き写さず、下記の `docs/` を参照して実行する（正解は docs 側に置く。`docs/adr/0004-issue-docs-reference-model.md`）。

## 起草の手順

1. **構成をそろえる。** ADR が持つべき節（ステータス / 日付 / 決定者 / コンテキスト / 決定 / 影響 / 検討した代替案）は `docs/adr/0001-record-architecture-decisions.md` の運用ルールを参照する。既存 ADR（例: `docs/adr/0002-serverless-over-ecs.md`）の並びに倣う。
2. **ファイル名は連番 + ケバブケース。** 採番と命名は `docs/adr/0001-record-architecture-decisions.md` を参照する。
3. **決定者を必ず明記する。** 人間が決めたのか AI が決めたのかの区別はこのプロジェクトの主題である（`docs/adr/0001-record-architecture-decisions.md`）。
4. **粒度を確認する。** すべてを ADR にしない。目安は `docs/adr/0001-record-architecture-decisions.md`「影響 / 悪い影響」を参照する。

## 起草時の停止点（勝手に進めない）

- **技術選定の変更・ADR の要否そのものは人間の決定。** 迷ったら起草を進めず質問して止まる（`docs/rules/responsibility.md`）。
- **AI が単独で「承認済み」で起票しない。** ステータスの扱いは `docs/adr/0001-record-architecture-decisions.md` の運用ルールに従う。
- **既存の ADR を書き換えない・追記しない。** 決定が覆ったら新しい ADR を起こし、古い方のステータスを「廃止（NNNN により置換）」にする（`docs/adr/0001-record-architecture-decisions.md`）。

## 検査できないこと

この手順が ADR 0001 と整合しているか・実際にこの手順で起草されたかは機械では検査できない（`docs/harness/commands-and-skills.md` 軸3）。受け皿は規律層（`docs/rules/responsibility.md`）と review 工程である。
