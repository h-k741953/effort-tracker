# 0022. デモデータの日次リセットを、EventBridge Scheduler から呼ぶ専用の Lambda で行う

- **ステータス**: 提案中
- **日付**: 2026-10-27
- **決定者**: 人間（2026-10-27 に Q-1・Q-7・Q-8 を明示選択）。本文の起草は仕様工程

> 決定の中身は、人間が 2026-10-27 に AskUserQuestion で明示選択したものである（`docs/specs/demo-data-reset.md` Q-1・Q-7・Q-8）。この ADR の文面は **AI（仕様工程）が起草した**。AI は ADR を「承認済み」で起票しないため（ADR 0001 運用ルール5）、ステータスは「提案中」とする。

## コンテキスト

`docs/rules/cost-guardrails.md` は、デモ公開の前提条件として**デモデータの日次リセット**を求めている。既存の条文は、リセットの実行基盤をデモデータを扱う Issue（#56）へ委ねてきた。

1. **移行の手段を兼ねる** — 稼働実績の入力を時刻から算出する方式へ改めたとき、時刻を持たない旧データは移行せずに破棄すると決まった（`docs/specs/daily-record-entry.md` D-15）。破棄の手段は「契約以外の行をすべて破棄し、デモデータを再投入する」ことであり、日次リセットを前倒しして行うのと同じ操作とされた（`docs/specs/workmonth-implementation-design.md` AC-10-20 ③・決定17）。この手段が入るまで本番デプロイしない（同 AC-13-27 ⑤）
2. **既存の Lambda は3本である** — `docs/specs/infra-terraform.md` は、BFF/SSR・ドメイン API・CloudFront 遮断の3本を定義した。リセット用の実行基盤は、そのいずれにも含まれていない（同 非スコープ）
3. **DDL の適用手段も決まっていない** — DDL の全文とマイグレーションの手段は、実装 PR に委ねられていた（`docs/specs/workmonth-implementation-design.md` AC-10-6）。契約に契約期間の列を足す変更（ADR 0021 決定2）も、実環境へ適用する手段を持っていない
4. **制約** — 長期認証情報を使わない（`docs/rules/security.md`）。月 $5 の予算に収める（`docs/rules/cost-guardrails.md`）。Go のテストは標準 `testing` + go-cmp で書く（ADR 0007）。依存を黙って増やさない（ADR 0010 §A）

## 決定

### 1. EventBridge Scheduler から、4本目の Lambda（リセット専用）を呼ぶ

- リセット用の Lambda を、既存の3本とは**別の関数**として1本足す
- 起動は EventBridge Scheduler のスケジュールから行う
- この Lambda は Function URL を持たない
- 成果物は、ほかの Lambda とは別の zip とし、別の変数で受け取る
- 実行ロールに許すのは、自身のロググループへの書き込みと、Neon の接続文字列を持つ SSM パラメータの読み取りに限る
- Scheduler が引き受けるロールに許すのは、この Lambda の呼び出しだけである

構成の受け入れ条件は `docs/specs/demo-data-reset.md` AC-12 が持つ。

### 2. DDL は SQL を標準の `embed` でバイナリに埋め込み、リセット用 Lambda が冪等に適用する

- マイグレーション用のライブラリを導入しない。**依存を追加しない**
- 契約の行は残す決定（`docs/specs/demo-data-reset.md` Q-2）であるため、契約のテーブルを作り直さず、列を足す変更を冪等に書く

受け入れ条件は同 AC-10 が持つ。

### 3. 実行の形

- **時刻**: 日本時間（`Asia/Tokyo`）の毎日午前4時に実行する
- **整合**: 破棄と投入を**1つのトランザクション**で行う
- **失敗時**: ロールバックして前日のデータを残し、CloudWatch Logs に記録するだけにする。**通知の仕組みは足さない**

受け入れ条件は同 AC-11 が持つ。

## 影響

### 良い影響

- **破棄の処理を公開経路から切り離せる。** 全行を破棄する処理が、HTTP の要求を受ける関数に同居しない。CloudFront 遮断 Lambda を独立させた先例（`docs/specs/infra-terraform.md` AC-4-8）と同じ形である
- **認証を実行ロールに閉じられる。** 接続文字列は既存の SSM パラメータから実行時に取得し、AWS の外に長期認証情報を置かない
- **依存が増えない。** devcontainer のリビルドも、Dockerfile の pin の追加も要らない
- **リセットの途中の状態が読み取り側に見えない。** 1つのトランザクションのため、リセット前かリセット後の集合のどちらかしか見えない
- **組み立てを Go の標準 `testing` で検証できる**（ADR 0007）

### 悪い影響 / トレードオフ

- **Lambda が4本になる。** 予約同時実行数・ログ保持・実行ロールの要求が4本目にも掛かる（`docs/specs/infra-terraform.md` の「すべての Lambda」に掛かる条文）
- **AWS のサービスが1つ増える**（EventBridge Scheduler）。料金は無料枠の範囲に収まる見込みだが、実測していない
- **失敗に気づきにくい。** 通知を足さないため、失敗はログを読まない限り分からない。失敗が続く間、前日のデータが残り、リセット日を基準にした相対の年月がずれていく
- **DDL の冪等性は SQL の書き方に依存する。** ライブラリによる適用履歴の管理を持たないため、冪等でない SQL を混ぜると2回目の適用で失敗する

### 未決のまま残るもの

次の事項は、本 ADR の決定に含まれない。人間の判断を待つ（`docs/specs/demo-data-reset.md` U 節）。

- **リセット用 Lambda の予約同時実行数の値**（U-5）。全 Lambda に明示する要求は既にあるが、値を持つ `docs/rules/cost-guardrails.md` にリセット用 Lambda の行が無い
- **デプロイロールの拡張**（U-4）。Scheduler は、デプロイロールの限定列挙（`docs/specs/infra-terraform.md` D-15）に無い。列挙を広げる判断は人間の承認事項である
- **`docs/rules/architecture.md` との関係**（U-3）。リセット用 Lambda は Route Handler 以外（Scheduler）から起動される。規約「Route Handler 以外から Lambda を呼ばない」をどう読むかは決まっていない
- **README のコストの節の同期**（U-7）。Lambda の本数と予約同時実行数の記述を、どの PR で揃えるかは決まっていない
- そのほか、同 U-1（相対の契約期間と残す契約の行）・U-2（1つのトランザクションと、集約をまたぐトランザクションの禁止）・U-6（`services/api` に非業務の `cmd` を足すこと）が残る

## 検討した代替案

### スケジュール実行の手段（Q-1）

| 案 | 却下理由 |
|---|---|
| (b) EventBridge Scheduler から、既存のドメイン API Lambda に別のイベント源を足して呼ぶ | `docs/rules/architecture.md`「Route Handler 以外から Lambda を呼ばない」に字面で反する。公開経路を担う関数に全行を破棄する処理が同居し、イベントの種別の分岐を誤ると、HTTP の経路から破棄に到達する余地が生じる。予約同時実行数を API と分け合う |
| (c) GitHub Actions の `schedule` から DB へ直接つなぐ | 接続文字列を GitHub Secrets に置くと、長期認証情報を置くことになる（`docs/rules/security.md` の趣旨に反する）。回避するにはデプロイロールの拡張か新しいロールが要る。`schedule` は実行時刻を保証せず、公開リポジトリでは活動が無いと無効化される。AWS の外に運用経路が増える |
| (d) Neon の `pg_cron` で DB 内で実行する | 利用中のプランで使えるか、スケール・トゥ・ゼロで停止している間にジョブが走るかが未実測である。デモデータの組み立てが SQL・PL/pgSQL 側に移り、Go の標準 `testing` で検証する方針（ADR 0007）の外に出る |
| (e) リクエスト時の遅延リセット（その日の最初の要求で入れ直す） | 実行基盤は要らないが、利用者の要求の経路で全行の破棄が走り、同時に来た要求と競合する。デモデータの投入とリセットを HTTP 契約の対象外とした趣旨（`docs/specs/domain-api-http-contract.md` AC-11-8）から外れる |

### DDL の適用手段（Q-7）

| 案 | 却下理由 |
|---|---|
| (b) マイグレーション用のライブラリを導入する | `services/api/go.mod` と `.devcontainer/Dockerfile` の pin の変更と、devcontainer のリビルドが要る（`docs/rules/commands.md`）。依存の追加は承認事項であり、得られる適用履歴の管理は、日次で入れ直すデモの用途に対して過剰である |
| (c) 人間が Neon のコンソールや CLI で SQL を手で流し、手順を docs に残す | 実行基盤は要らないが、適用の有無が人の手順に依存する。本番デプロイの判断に手順の確認が加わる |

## 関連

- `docs/specs/demo-data-reset.md`（本 ADR の決定を受け入れ条件に落とした仕様）
- ADR 0013（AWS ネイティブホスティング）／ADR 0003（Function URL の IAM 認証）／ADR 0014（実行ロールで呼ぶ）／ADR 0007（テスト）／ADR 0021（契約期間）
