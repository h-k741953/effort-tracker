# effort-tracker

SES/受託向けの勤怠・工数管理 SaaS。

このリポジトリはポートフォリオを兼ねる。したがって **`docs/` の質はコードの完成度と同格の成果物**であり、「動くから良い」は評価基準にならない。判断の経緯がリポジトリに残っていないものは、存在しないものとして扱う。

デモURLを README で公開するため、**コストとセキュリティのガードレールは機能要件と同格**とする。これらを「後で入れる」ことは許容しない。

---

## 規約の実体は `docs/rules/` にある

規約の本文は `docs/rules/` に置き、この CLAUDE.md からは下記の `@import` で参照する。正解の実体を `docs/` 内に留め（ADR 0004）、かつツールが規約本文を確実に読むための構成である。根拠は `docs/adr/0010-harness-engineering.md` §B。

- 規約を書き換えるときは `docs/rules/` の該当ファイルを直接編集する。**CLAUDE.md にコピーを作らない**
- 肥大化は「毎回読まれるトークン量」で測る（バイト数や見出し数では測らない）

@docs/rules/scope.md
@docs/rules/responsibility.md
@docs/rules/development-process.md
@docs/rules/commands.md
@docs/rules/commit-convention.md
@docs/rules/security.md
@docs/rules/cost-guardrails.md
@docs/rules/architecture.md
@docs/rules/notation.md
