# 0006. 内部アーキテクチャにオニオンアーキテクチャを採用する

- **ステータス**: 承認済み
- **日付**: 2026-07-17
- **決定者**: 人間（プロジェクトオーナー）

## コンテキスト

`services/api` の内部構造は、これまでリポジトリに記録されていなかった。ディレクトリ構成案（`domain` / `usecase` / `adapter` / `platform`）は会話の中で承認されていたが、`docs/` にも ADR にも根拠が無い。CLAUDE.md の「判断の経緯がリポジトリに残っていないものは、存在しないものとして扱う」に従えば、**内部アーキテクチャは決まっていない状態**だった。実際、`usecase` / `adapter` / `platform` はディレクトリとして存在すらしていない。

一方で、**境界は既に1本引かれ、機械的に強制されている**。CLAUDE.md は「`services/api/internal/domain/` は Go の標準ライブラリのみに依存する」と定め、`make check-domain-deps` がこれを検査している（`docs/harness/verification-loop.md`）。内部アーキテクチャの選定は、この既存の境界と整合する必要がある。

集約は WorkMonth 1つに限定されており（`docs/domain/ubiquitous-language.md`）、層を増やす余地は小さい。

## 決定

**オニオンアーキテクチャ（Jeffrey Palermo, 2008）を採用する。**

### 採用の理由

主たる理由は、**オニオンが規定する境界と、本プロジェクトが既に CI で強制している境界が、同一の線であること**である。

DDD において Repository は**ドメイン層のパターン**である。集約をメモリ上のコレクションのように見せる契約であり、ユビキタス言語の一部を成す。インフラに属するのは実装だけである。

検討した3つのパターンのうち、**この配置を構造として規定するのはオニオンだけ**である。

| パターン | repository interface の所在 |
|---|---|
| **オニオン** | Domain Model の外周（Domain Services リング）。**オニオンの内側＝ドメインの一部**として明示的に規定される |
| ポート&アダプタ | 規定しない。ポートは「アプリケーション」が定義するとしか言わず、ドメインとアプリケーションを区別しない |
| クリーン | ユースケース層に置く読み方が一般的。DDD の配置とは一致しない（ここは解釈に幅がある） |

本プロジェクトは既に `domain/` を「標準ライブラリのみ」として CI で強制している。オニオンを採ると、**このパターンの中心的な主張——すべての依存が、何にも依存しないドメインへ内向きに向かう——が、既存の `make check-domain-deps` によってそのまま機械検査される**。

ポート&アダプタでは、パターンの中心的な主張はアプリケーション境界の対称性にある。`check-domain-deps` はそれを検証しない。**同じ検査を持っていても、証明できる範囲がオニオンのほうが広い。**

これは本プロジェクトの主題に直結する。「ドメイン層を分離しました」は誰でも言える。CI が落ちる状態にしてあることが差になる（`docs/harness/verification-loop.md`）。オニオンは、**その検査でアーキテクチャの中核まで証明できる**。

従たる理由は以下の2点である。

- 集約が WorkMonth 1つなので、クリーンの presenter リングは儀式になる。層が少ないほうが規模に合う
- オニオンは層飛ばしを許すため、`infrastructure → domain` を直接書ける。間接層を挟まず、Go の素直なパッケージ構成に落ちる

### 層とディレクトリの対応

```
services/api/internal/
├── domain/              ← 中心。Go 標準ライブラリのみ（CI で検査）
│   └── workmonth/
│       ├── workmonth.go      集約ルート・値オブジェクト
│       ├── settlement.go     精算幅・丸め
│       └── repository.go     リポジトリ interface
├── application/         ← アプリケーションサービス（ユースケース）
└── infrastructure/      ← 最外周。interface の実装
    ├── persistence/          Neon 実装
    └── lambda/               ハンドラ・DI 配線
```

**依存はすべて内向き。** 外側の層は任意の内側の層に依存してよい（層飛ばしを許す）。内側は外側を一切知らない。

ファイル名は例示であり、この ADR が固定するのは**層とその依存の向き**のみである。

### Domain Services のリングを独立させない

Palermo の原典は Domain Model / Domain Services / Application Services の3リングを持つ。**本プロジェクトでは Domain Services を `domain/` に同居させ、ディレクトリとして独立させない。**

Domain Services は複数の集約にまたがるロジックの置き場だが、本プロジェクトの集約は WorkMonth 1つであり、当面空になる。**空のディレクトリは層の理解ではなく儀式になる。** 必要が生じた時点で分ける。

**これはリングの概念を否定するものではない。** repository interface は Palermo の分類では Domain Services リングに属し、それを `domain/` に置くこと自体が、この同居の一例である。

### Lambda ハンドラは infrastructure に置く

オニオンでは UI は Infrastructure と同じ最外周にある。Lambda ハンドラは HTTP という配送手段への適応にすぎないため、`infrastructure/lambda/` に置き、`presentation/` を独立させない。理由は Domain Services と同じで、層の数を規模に合わせる。

## 影響

### 良い影響

- **アーキテクチャの中核が機械検査される。** `make check-domain-deps` が既にこれを担保しており、追加のハーネスは要らない。「オニオンで作りました」が主張ではなく事実になる
- repository interface がドメインに属するため、**ドメインが自分の永続化契約を所有する**。interface 定義は `context` 程度しか import しないので、標準ライブラリのみの制約と両立する
- 依存の向きが一方向に定まる。レビューで「どちらを向くべきか」を議論しなくてよくなる

### 悪い影響 / コスト

- **以前の構成案を捨てる。** `usecase` → `application`、`adapter` → `infrastructure`、`platform` は `infrastructure/lambda` に吸収されて消える。実装が無い段階での変更なので、移行コストはドキュメントの更新のみで済む
- **層飛ばしを許すため、規律で守る部分が残る。** CI が検査するのは「`domain` が外を向いていないこと」だけである。`infrastructure` が `application` を飛ばして `domain` を直接触ることの是非は検査されない
- **リポジトリ実装が肥大化しやすい。** 集約の再構築（精算幅のスナップショットを含む）が `infrastructure/persistence` に集中する
- **ハーネスの検査はテストコードに及ばない。** `go list -deps` はテストファイルの import を出力しないため、`domain` のテストが外部ライブラリを import しても検出されない。**この扱いは本 ADR では決めない**（テストツールの選定と併せて別途判断する）

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| ポート&アダプタ（ヘキサゴナル） | 内部の層構造を規定せず、repository interface の所在が曖昧なまま残る。パターンの中心的な主張がアプリケーション境界にあるため、既存の `check-domain-deps` がそれを検証しない。**当初のディレクトリ構成案はこれだった** |
| クリーンアーキテクチャ | 層と儀式が増える。presenter リングは集約1つの規模に対して過剰。repository interface をユースケース層に置く読み方が一般的で、DDD の配置と一致しない |
| レイヤードアーキテクチャ | 依存が上から下へ流れ、ドメインがインフラに依存する。`check-domain-deps` と両立しない |
| 決めずに保留する | README に書いた「ポート&アダプタ」の根拠が無いままになる。CLAUDE.md に照らせば、記述を消すか決めるかの二択だった |

## 関連

- ADR 0001: ADR の運用ルール
- `docs/harness/verification-loop.md`: `make check-domain-deps` の設計。**本 ADR の中核を担保する**
- `docs/domain/ubiquitous-language.md`: 集約が WorkMonth 1つであること

## 参考

- Jeffrey Palermo, "The Onion Architecture: part 1" (2008)
- Eric Evans, "Domain-Driven Design" (2003) — Repository をドメイン層のパターンとして位置づけている
