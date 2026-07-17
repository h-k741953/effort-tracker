# 0008. 内部アーキテクチャにクリーンアーキテクチャを採用する

- **ステータス**: 承認済み
- **日付**: 2026-07-17
- **決定者**: 人間（プロジェクトオーナー）

> 本 ADR は **ADR 0006（オニオンアーキテクチャ）を置換する**。

## コンテキスト

ADR 0006 でオニオンアーキテクチャを採用した。主たる理由は「オニオンが規定する境界と、CI で強制している境界が同一の線であること」だった。DDD において Repository はドメイン層のパターンであり、検討した3案のうちオニオンだけがその配置を構造として規定するためである。

その後、**各層をさらに細かく分けたいという要求**が出た。オニオンはリングを3つしか規定せず、Interface Adapters の内部（controller / presenter / gateway）を規定しない。とりわけ**出力側の依存性反転**の構造を持たない。

ADR 0006 は実装前の決定であり、`services/api` にはドメインパッケージの骨（`doc.go`）しか無い。**コードは1行も書かれていないため、移行コストはドキュメントの更新のみである。**

## 決定

**クリーンアーキテクチャ（Robert C. Martin）を採用する。**

### 採用の理由

**層をより細かく分けるためである。**

クリーンは4つのリングを規定し、さらに Interface Adapters の内部を controller / presenter / gateway に分ける。オニオンが規定しない粒度まで責務が分かれる。

特に**出力側の依存性反転**が入る点が異なる。interactor は具体的な presenter を知らず、output port（interface）を通じて呼ぶ。オニオンにはこの構造が無く、ユースケースの戻り値を呼び出し側が加工する形になる。

### 層とディレクトリの対応

```
services/api/internal/
├── domain/                  ← Entities。Go 標準ライブラリのみ（CI で検査）
│   └── workmonth/
│       ├── workmonth.go         集約ルート・値オブジェクト
│       └── settlement.go        精算幅・丸め
├── usecase/                 ← Use Cases
│   ├── port/
│   │   ├── repository.go        WorkMonthRepository interface
│   │   ├── input.go             入力境界
│   │   └── output.go            出力境界
│   └── interactor/              ユースケースの実装
├── adapter/                 ← Interface Adapters
│   ├── controller/              HTTP → input DTO
│   ├── presenter/               output DTO → ViewModel
│   └── gateway/                 repository の実装
└── driver/                  ← Frameworks & Drivers
    ├── lambda/                  ハンドラ・DI 配線
    └── persistence/             Neon 接続
```

**依存は内側にのみ向かう。** 内側は外側を一切知らない。

```
lambda → controller → interactor
                        ↓ output port（interface）
                     presenter → ViewModel
                        ↓
                     lambda が返す
```

ファイル名は例示であり、本 ADR が固定するのは**リングとその依存の向き**、および Interface Adapters を controller / presenter / gateway に分けることである。

### Entities のディレクトリ名は `domain/` のままとする

クリーンの用語は Entities だが、ディレクトリ名は `domain/` を維持する。

- `make check-domain-deps`、`.github/workflows/ci.yml`、CLAUDE.md の制約文がいずれも `domain` を指しており、**改名は実利のない破壊**になる
- `docs/domain/ubiquitous-language.md` は DDD の語彙で書かれており、`domain` のほうが整合する

**用語のずれは受け入れる。** リング名は Entities、ディレクトリ名は `domain` であることを本 ADR で明示的に記録する。

### repository interface は `usecase/port` に置く

**これは ADR 0006 からの明確な後退である。**

ADR 0006 は「Repository はドメイン層のパターンである」という Evans の配置を根拠にオニオンを選んだ。クリーンでは Gateway の interface をユースケース層に置く読み方が一般的であり、**ドメインは自分の永続化契約を所有しなくなる**。DDD の配置とはずれる。

ADR 0006 の性質を維持する案（層はクリーン、repository interface だけドメインに残すハイブリッド）も検討したが、採らなかった。**「なぜそこだけオニオンなのか」を恒久的に説明し続ける必要が生じ、細かく分けるという本 ADR の目的にも寄与しないため**である。

## 影響

### 良い影響

- 責務がオニオンより細かく分かれる。とりわけ**出力側が依存性反転する**ため、interactor が表示形式を知らずに済む
- Interface Adapters の内部が controller / presenter / gateway に分かれ、「HTTP の都合」「表示の都合」「永続化の都合」が別々の場所に落ちる
- **ハーネスは無傷である。** クリーンの依存性ルールも「依存は内側にのみ向かう」であり、`make check-domain-deps` は最内周（`domain`）が何にも依存しないことを引き続き検査する。`domain/` を改名しないため Makefile と CI の変更も要らない

### 悪い影響 / コスト

- **ADR 0006 の主たる理由を、理由ごと捨てる。** repository interface がドメインを離れ、DDD の Repository 配置との一致は失われる。本 ADR の理由は「細かく分けたい」であり、0006 のような構造的な必然性は無い
- **CI が検査しない境界が増える。** ハーネスが見るのは「`domain` が外を向いていないこと」だけである。`usecase` が `adapter` を import しないこと、`adapter` が `driver` を import しないことは**検査されず、規律で守る**。リングを増やすほど、この未検査領域は広がる。**層が増えることは、機械が保証する範囲が広がることを意味しない**
- **集約が WorkMonth 1つに対して層が多い。** ADR 0006 では presenter リングを「儀式になる」として却下していた。JSON を返すだけの API では presenter が薄くなりやすい。**これは承知のうえでの選択である**
- **リングをまたぐ DTO の変換が増える。** input DTO → ドメイン → output DTO → ViewModel と、同じデータが複数回姿を変える。ボイラープレートが増える
- **ADR 0007 の根拠が一部弱まる。** ADR 0007 はテストダブルに手書き Fake を選んだ根拠として「Repository はコレクションのように見せる契約であり、これは ADR 0006 の採用理由そのもの」と述べている。この参照は本 ADR で無効になる。ただし **Fake の判断自体は覆らない**（interface が1つであることと、Fake がコレクション意味論を表すことは、interface の所在に依存しない）。ADR 0007 は書き換えない

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| オニオンのまま（ADR 0006 を維持） | リングを3つしか規定せず、Interface Adapters の内部を分けない。出力側の依存性反転も無い。**「細かく分けたい」という要求を満たさない** |
| クリーン + repository interface をドメインに残す（ハイブリッド） | ADR 0006 の利点は維持できるが、「なぜそこだけオニオンなのか」を恒久的に説明し続けることになる。細かく分けるという目的にも寄与しない |
| Entities のディレクトリを `entity/` に改名 | Makefile / CI / CLAUDE.md の書き換えを要し、`check-domain-deps` というターゲット名も実態とずれる。ユビキタス言語が DDD の語彙で書かれている点とも衝突する。**得るものは用語の忠実さだけ** |
| presenter を立てない | オニオンより細かくはなるが、クリーンの特徴である出力側の依存性反転を失う。それならオニオンを維持するほうが筋が通る |

## 関連

- **ADR 0006: オニオンアーキテクチャ（本 ADR が置換する）**
- ADR 0007: テストは標準 testing と go-cmp のみで書く（Fake の根拠が本 ADR の影響を受ける）
- `docs/harness/verification-loop.md`: `make check-domain-deps` の設計
- `docs/domain/ubiquitous-language.md`: 集約が WorkMonth 1つであること

## 参考

- Robert C. Martin, "The Clean Architecture" (2012)
- Robert C. Martin, "Clean Architecture: A Craftsman's Guide to Software Structure and Design" (2017)
