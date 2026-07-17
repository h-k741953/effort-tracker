# effort-tracker

SES/受託向けの勤怠・工数管理 SaaS。

このリポジトリはポートフォリオを兼ねる。したがって **`docs/` の質はコードの完成度と同格の成果物**であり、「動くから良い」は評価基準にならない。判断の経緯がリポジトリに残っていないものは、存在しないものとして扱う。

デモURLを README で公開するため、**コストとセキュリティのガードレールは機能要件と同格**とする。これらを「後で入れる」ことは許容しない。

---

## スコープ

MVP のユースケースは以下の3つに限定する。これ以外を実装しない。

1. 稼働実績の入力・編集
2. 月次締め
3. 承認申請・承認

集約は **勤務月 (WorkMonth) の1つだけ**を厳密にモデリングする。
請求・支払・ファイル添付はスコープ外（README の「今後の展望」に記載する）。

---

## 責務分界

このプロジェクトの主題そのものなので、最も厳格に守る。詳細は `docs/ai-collaboration.md`。

### 必ず人間に確認を取る（AIが勝手に決定しない）

- ドメイン境界、集約の切り方、ユビキタス言語の用語
- 業務ルール（精算幅の丸め、締め後の扱いなど）
- 技術選定の変更（ADR が必要な判断）
- 本番デプロイ

判断に迷ったら実装を進めず、**質問して止まる**。推測で埋めた業務ルールは、間違っていた場合にテストごと嘘になり、Green が信用できなくなる。

### AIが主導してよい

- 仕様書からのテストケース導出
- テストを満たす実装
- リファクタリング提案
- CI 設定・Lint エラーの修正

---

## 開発プロセス

### SDD — 仕様を書いてから実装する

`docs/specs/` に仕様書がない機能は実装しない。仕様書には受け入れ条件を、テストに落とせる粒度で書く。

### Issue と docs の責務分界 — レファレンス型運用

**正解のデータは常に `docs/` にある。** 根拠と代替案は `docs/adr/0004`。

| 置き場所 | 持つもの | 持たないもの |
|---|---|---|
| GitHub Issue | 実装の**タスク**、仕様を決めるための**議論** | 決定した仕様そのもの |
| `docs/` | 決定した**最新の**設計・仕様のみ | 議論の過程、タスクの進捗 |

- **Issue や PR に仕様を書き写さない。** `docs/` へのリンクを貼って参照させる
- 議論は Issue でしてよい。ただし**結論が出た時点で `docs/` に反映**し、Issue にはリンクを残す。コメントに結論を書いて終わらせない
- `docs/design/` は作らない。仕様・受け入れ条件・実装設計はすべて `docs/specs/` に集約する
- `services/` `apps/` を変更する PR は、本文に `docs/` への参照が必須（CI の `spec-link` が検査）。仕様を要さない変更は `no-spec` ラベルで例外扱いにする

Issue のコメントは `docs/` より優先されない。古い docs を読んだAIは、古い仕様を正しく実装する。テストごと古くなるため Green が無意味になる。

### TDD — Red → Green → Refactor

テストを先に書く。**Red を確認してから**実装に入る。Red を踏まずに書かれたテストは、何も検証していない可能性を排除できない。

テストは**標準 `testing` + `google/go-cmp` のみ**で書く。詳細は `docs/adr/0007`。

- **アサーションライブラリを入れない**（testify を採らない）。テーブル駆動で書く
- **モックライブラリを入れない**（gomock / moq を採らない）。テストダブルは手書きのインメモリ Fake
- go-cmp は構造体・スライスの比較にのみ使う
- Web は Vitest（導入は `apps/web` スキャフォールド時）

`domain` はテストコードも検査対象で、標準ライブラリ + go-cmp 以外を import すると CI が落ちる。**許可リストにライブラリを黙って追加しない。** 増やす判断には ADR の置換が要る。

### DDD — ドメイン層の隔離

`services/api/internal/domain/` は **Go の標準ライブラリのみに依存**する。フレームワーク、ORM、AWS SDK、ログライブラリを import しない。

この規約はレビューでは腐るため、CI で import を機械的に検査する（`docs/harness/verification-loop.md`）。

内部アーキテクチャは**オニオンアーキテクチャ**。詳細は `docs/adr/0006`。

```
domain/          ← 中心。標準ライブラリのみ。リポジトリ interface もここに置く
application/     ← アプリケーションサービス（ユースケース）
infrastructure/  ← 最外周。persistence（Neon 実装）と lambda（ハンドラ・DI 配線）
```

**依存はすべて内向き。** 内側は外側を一切知らない。層飛ばし（`infrastructure → domain`）は許す。

Domain Services のリングは `domain/` に同居させ、独立させない（集約が WorkMonth 1つのため当面空になる）。`presentation/` も作らない。**層を増やす判断には人間の承認が要る。**

---

## 実行必須コマンド

作業の区切りで必ず実行する。CI と同一のものがローカルで動くことをハーネスの前提とする。

| コマンド | 内容 |
|---|---|
| `make help` | ターゲット一覧 |
| `make verify` | `lint` + `test` + `check-domain-deps` + `scan-secrets`（コミット前・PR前） |
| `make test` | 全レイヤーのテスト |
| `make lint` | 全レイヤーの Lint / 型チェック |
| `make check-domain-deps` | ドメイン層が標準ライブラリのみに依存しているか検査 |
| `make scan-secrets` | gitleaks によるシークレット検出 |

CI（`.github/workflows/ci.yml`）は**これと同一のターゲットを呼ぶ**。CI 側にだけ存在する検査を作らない。詳細は `docs/harness/verification-loop.md`。

> **ツールチェーンは devcontainer のビルド時に導入される。** `init-firewall.sh` が実行時の外向き通信を制限しているため、コンテナ起動後に Go や Terraform を入れることはできない。追加が必要なら `.devcontainer/Dockerfile` を変更してリビルドすること。
>
> **バージョンを上げるときは `.devcontainer/Dockerfile` と `.github/workflows/ci.yml` を同時に更新する。** 片方だけ上げると「ローカルで通るが CI で落ちる」が発生し、ハーネスの前提が壊れる。

---

## コミット規約

Conventional Commits。**「仕様 → テスト → 実装」の順序が履歴に残る**ようにする。これはこのプロジェクトの成果物の一部であり、後からまとめてコミットすることは許容しない。

```
docs: 月次締めの仕様追加
test: 月次締めのテスト追加      ← この時点で Red
feat: 月次締め実装              ← この時点で Green
```

**1コミットに仕様とテストと実装を混在させない。**

主な type: `docs` / `test` / `feat` / `fix` / `refactor` / `chore` / `ci`

---

## セキュリティ

- `.env` / `terraform.tfvars` / `*.tfstate` は `.gitignore` 済み。新しい機密ファイル形式を導入するときは**コミット前に** `.gitignore` を更新する
- シークレットをコードやドキュメントに書かない。例示は必ずダミー値（`docs/` 内の AWS アカウントID・接続文字列・URL はすべて架空のもの）
- `gitleaks` を GitHub Actions に組み込む
- **AWS 認証は CI・ランタイムとも OIDC**。静的アクセスキーをどこにも置かない（GitHub Secrets にも Vercel の環境変数にも）
  - CI（GitHub Actions）→ AWS: GitHub OIDC
  - ランタイム（Vercel）→ AWS: Vercel OIDC Federation（`docs/adr/0005`）
  - **信頼ポリシーで `sub` 条件を省かない。** 省くと Vercel にサインアップした任意の第三者が IAM ロールを引き受けられる

---

## コストガードレール

デモ公開の前提条件。実装時に省略しない。根拠は `docs/adr/0002` `docs/adr/0003`。

| 項目 | 設定 |
|---|---|
| Lambda 予約同時実行数 | `reserved_concurrent_executions = 5` |
| Lambda Function URL | `authorization_type = "AWS_IAM"`（`NONE` は禁止） |
| AWS Budgets | 月 $5 で通知（実績80% / 予測100% の2本） |
| 使用禁止 | NAT Gateway / ALB / RDS / ECS |
| デモデータ | 日次リセット |
| Route Handler | レート制限 |

---

## アーキテクチャの制約

- **BFF 経由の一方通行**: ブラウザ → Next.js Route Handler → Lambda。Route Handler 以外から Lambda を呼ばない。Function URL が IAM 認証なので、この制約は構造的に担保される（SigV4 署名できるのはサーバー側のみ）
- Lambda 呼び出しは `apps/web/src/lib/lambda-client.ts` に集約する
