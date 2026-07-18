# docs/rules — 規約の実体

このディレクトリが**規約の正解の実体**である。`CLAUDE.md` からは `@import` で参照され、ツールは CLAUDE.md 経由で本文を確実に読む。根拠は `docs/adr/0010-harness-engineering.md` §B。

- 規約を書き換えるときは、このディレクトリの該当ファイルを直接編集する。**CLAUDE.md にコピーを作らない**（二重管理を避ける・ADR 0004）
- `.claude/rules/` へは置かない。規約の実体が `docs/` の外に出るため
- 肥大化は「毎回読まれるトークン量」で測る。バイト数や見出し数では測らない

## 一覧

| ファイル | 内容 |
|---|---|
| [scope.md](scope.md) | MVP のスコープ（実装する／しない） |
| [responsibility.md](responsibility.md) | 責務分界（人間が決める／AIが主導する） |
| [development-process.md](development-process.md) | SDD / Issue-docs 運用 / TDD / DDD |
| [commands.md](commands.md) | 実行必須コマンド |
| [commit-convention.md](commit-convention.md) | コミット規約 |
| [security.md](security.md) | セキュリティ |
| [cost-guardrails.md](cost-guardrails.md) | コストガードレール |
| [architecture.md](architecture.md) | アーキテクチャの制約 |
