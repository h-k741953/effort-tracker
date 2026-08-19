# 開発プロセス

## SDD — 仕様を書いてから実装する

`docs/specs/` に仕様書がない機能は実装しない。仕様書には受け入れ条件を、テストに落とせる粒度で書く。

## Issue と docs の責務分界 — レファレンス型運用

**正解のデータは常に `docs/` にある。** 根拠と代替案は `docs/adr/0004`。

| 置き場所 | 持つもの | 持たないもの |
|---|---|---|
| GitHub Issue | 実装の**タスク**、仕様を決めるための**議論** | 決定した仕様そのもの |
| `docs/` | 決定した**最新の**設計・仕様のみ | 議論の過程、タスクの進捗 |

- **Issue や PR に仕様を書き写さない。** `docs/` へのリンクを貼って参照させる
- 議論は Issue でしてよい。ただし**結論が出た時点で `docs/` に反映**し、Issue にはリンクを残す。コメントに結論を書いて終わらせない
- `docs/design/` は作らない。仕様・受け入れ条件・実装設計はすべて `docs/specs/` に集約する
- `services/` `apps/` を変更する PR は、本文に `docs/` への参照が必須（CI の `spec-link` が検査）。仕様を要さない変更は `no-spec` ラベルで例外扱いにする
- **完了済み仕様の条文が持つ事実の記述（実測・進捗）が現況と食い違ったら、その記述を現況へ改める。** `docs/specs/` は最新の状態へ書き換える（ADR 0001 運用ルール3「書き換えず、追記もしない」は **ADR にのみ掛かる**）。**要求（不変条件・禁止・限界）を実装の都合で緩めない — 緩める判断は人間の承認事項**
- **特定時点の実測を仕様の条文に埋め込まない。** 実測は対象側が現況として持ち、仕様は趣旨だけを持つ

Issue のコメントは `docs/` より優先されない。古い docs を読んだAIは、古い仕様を正しく実装する。テストごと古くなるため Green が無意味になる。

## TDD — Red → Green → Refactor

テストを先に書く。**Red を確認してから**実装に入る。Red を踏まずに書かれたテストは、何も検証していない可能性を排除できない。

テストは**標準 `testing` + `google/go-cmp` のみ**で書く。詳細は `docs/adr/0007`。

- **アサーションライブラリを入れない**（testify を採らない）。テーブル駆動で書く
- **モックライブラリを入れない**（gomock / moq を採らない）。テストダブルは手書きのインメモリ Fake
- go-cmp は構造体・スライスの比較にのみ使う
- Web は Vitest（導入は `apps/web` スキャフォールド時）

`domain` はテストコードも検査対象で、標準ライブラリ + go-cmp 以外を import すると CI が落ちる。**許可リストにライブラリを黙って追加しない。** 増やす判断には ADR の置換が要る。

## DDD — ドメイン層の隔離

`services/api/internal/domain/` は **Go の標準ライブラリのみに依存**する。フレームワーク、ORM、AWS SDK、ログライブラリを import しない。

この規約はレビューでは腐るため、CI で import を機械的に検査する（`docs/harness/verification-loop.md`）。

内部アーキテクチャは**クリーンアーキテクチャ**。詳細は `docs/adr/0008`（`0006` はオニオンを採用していたが廃止済み）。

```
domain/               ← Entities。標準ライブラリのみ
usecase/
  port/               ← repository interface、入出力の境界
  interactor/         ← ユースケースの実装
adapter/
  controller/         ← HTTP → input DTO
  presenter/          ← output DTO → ViewModel
  gateway/            ← repository の実装
driver/
  lambda/             ← ハンドラ・DI 配線
  persistence/        ← Neon 接続
```

**依存は内側にのみ向かう。** 内側は外側を一切知らない。出力側も `port` の interface を通じて反転させる（interactor は presenter を直接知らない）。

- **repository interface は `usecase/port` に置く。** `domain` には置かない
- ディレクトリ名は `domain` だが、リングとしては Entities。用語のずれは ADR 0008 で承知のうえ
- **CI が検査するのは「`domain` が外を向いていないこと」だけ。** `usecase → adapter` や `adapter → driver` の逆流は検査されず、規律で守る
- **層を増やす・減らす判断には人間の承認が要る**
