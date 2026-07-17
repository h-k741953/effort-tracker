# 0007. テストは標準 testing と go-cmp のみで書く

- **ステータス**: 承認済み
- **日付**: 2026-07-17
- **決定者**: 人間（プロジェクトオーナー）

## コンテキスト

TDD を開発プロセスの中核に置いている（CLAUDE.md）。Red → Green → Refactor を回す以上、**テストの失敗を読むことが日常的な作業**になる。テストツールの選定は、この読解コストに直接効く。

同時に、ハーネスに穴が見つかった。`make check-domain-deps` が使う `go list -deps` は、**テストファイルの import を出力しない**。実測で確認している（`domain` パッケージに `net/http` を import する `_test.go` を置いても検出は0件。`-test` を付けると検出される）。

つまり「ドメイン層は Go の標準ライブラリのみに依存する」という規約は、**テストコードには一切効いていなかった**。ドメインのテストが任意の外部ライブラリを import しても CI は通る。

この2つは同じ決定に属する。テストツールを選ぶことは、この穴を塞ぐか否かを決めることでもある。

ADR 0006 で採用したオニオンアーキテクチャは、repository interface を `domain` に置く。アプリケーション層のテストはこの interface のテストダブルを必要とするため、モックの方針も併せて決める。

## 決定

**Go のテストは標準 `testing` と `github.com/google/go-cmp` のみで書く。モックライブラリを導入せず、テストダブルは手書きのインメモリ Fake とする。**

### 1. アサーションライブラリを導入しない

`testify` を採らない。

`assert` と `require` の使い分けが実際の落とし穴になる。`assert` は失敗しても実行を継続するため、直後の行で nil ポインタ参照が起き、**本当の失敗原因がスタックトレースに埋もれる**。TDD では Red を読むことが仕事であり、このコストは軽くない。

依存も `go-spew` / `go-difflib` / `objx` へ広がる。依存の規律を主題とするリポジトリで、ドメインのテストにアサーション DSL を持ち込むのは筋が通らない。

テストは**テーブル駆動**で書く。Go の標準的な書き方であり、追加の語彙を要さない。

### 2. go-cmp のみを例外として許す

標準 `testing` が唯一苦手とするのは**構造体とスライスの比較**である。`==` は使えず、`reflect.DeepEqual` は真偽しか返さず差分を示さない。ここだけを go-cmp で埋める。

go-cmp を例外とする根拠は3つ。

- Go チームが提供している
- **テスト専用として設計されている。** パッケージドキュメントが "It is intended to only be used in tests, as performance is not a goal and it may panic if it cannot compare the values" と明示している
- アサーション DSL ではなく、**差分を返す関数**である。テストの語彙を増やさない

Red の可読性はハーネスの前提そのものである。「信用できない Red は、無視される Red になる」（`docs/harness/verification-loop.md`）。

### 3. モックライブラリを導入しない

`gomock` / `moq` を採らない。テストダブルは手書きのインメモリ Fake とする。

- interface は repository の1つだけである。**コード生成の仕組みごと持ち込む価値がない**
- **インメモリ Fake のほうが Repository の意味に合う。** DDD の Repository は「集約をメモリ上のコレクションのように見せる契約」であり、これは ADR 0006 でオニオンを採用した理由そのものである。Fake はその意味を実演する
- 呼び出し回数や順序の検証が必要になった時点で再考する

### 4. ハーネスを `-test` 込みに強化する

`make check-domain-deps` を `go list -deps` から `go list -deps -test` に変え、go-cmp のみを明示的に許可する。

```make
violations="$(go list -deps -test -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/domain/... \
  | grep -v '^$' \
  | grep -v '^$(MODULE)/internal/domain/' \
  | grep -v '^github.com/google/go-cmp/' || true)"
```

テストバリアント（`workmonth.test` および `workmonth [workmonth.test]`）の ImportPath はいずれも `$(MODULE)/internal/domain/` で始まるため、**既存のフィルタがそのまま効く**。追加の除外は要らない（実測で確認）。

これにより検査の主張が精密になる。

| 対象 | 許される依存 |
|---|---|
| `domain` の本体 | 標準ライブラリのみ（例外なし） |
| `domain` のテスト | 標準ライブラリ + go-cmp のみ |

**両方が CI で強制される。** 「テストは素通り」という曖昧な状態が消える。

### 5. Web は Vitest

`apps/web` のテストは Vitest とする。導入はスキャフォールド（#9）の時点で行う。**本 ADR は選定のみを決め、設定は #9 の範囲とする。**

## 影響

### 良い影響

- **「ドメイン層は標準ライブラリのみ」がテストまで含めて事実になる。** 現在は本体にしか効いておらず、主張と実態がずれている
- 失敗の読解コストが下がる。`cmp.Diff` は `-want +got` の差分を示す
- Go の標準的な書き方に寄るため、Go を知る読み手が前提知識なしにテストを読める

### 悪い影響 / コスト

- **テストが冗長になる。** `require.NoError(t, err)` の1行が `if err != nil { t.Fatalf(...) }` の3行になる。これは受け入れる
- **失敗メッセージを自分で書く必要がある。** テーブル駆動の `name` と `t.Errorf` の書式に規律が要る
- **Fake の保守が手作業になる。** interface が増えれば Fake も増える。**repository が1つである前提に依存した判断**であり、集約が増えれば覆りうる
- **許可リストを手で維持する。** 「テスト専用ライブラリなら許す」という一般則ではなく、go-cmp という具体名を許している。2つ目を許すときは本 ADR を置換する判断が要る
- **`infrastructure` 層の永続化テストは本 ADR で決めていない。** testcontainers は Docker Hub が allowlist になく、devcontainer に Docker も無いため動かない。実 DB を使うテストは「CI と同一」の原則と正面から衝突する。`docs/specs/` が固まってから単独で判断する

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| testify（assert / require / mock） | `assert` の継続実行が失敗原因を隠す。依存が3つ以上に広がる。ドメインのテストにアサーション DSL が入る |
| 標準 testing のみ（go-cmp も入れない） | 構造体比較が `reflect.DeepEqual` になり、差分が出ない。**Red が読めなければハーネスの前提が崩れる** |
| gomock / moq | interface が1つの段階でコード生成の仕組みを持ち込む価値がない。Fake のほうが Repository の意味を表す |
| ハーネスを現状維持（テストを検査対象外とする） | 「ドメイン層は標準ライブラリのみ」の意味が曖昧なまま残る。**1語（`-test`）で塞げる穴を残す理由がない** |
| Jest（Web） | Next.js の新規プロジェクトでは Vitest が既定的。設定量が少ない |

## 関連

- ADR 0006: オニオンアーキテクチャ（repository interface が `domain` にある。Fake を選んだ根拠）
- `docs/harness/verification-loop.md`: `make check-domain-deps` の設計
- CLAUDE.md: TDD の運用

## 参考

- [google/go-cmp](https://pkg.go.dev/github.com/google/go-cmp/cmp) — "It is intended to only be used in tests"
