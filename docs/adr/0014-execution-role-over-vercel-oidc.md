# 0014. Vercel OIDC Federation を廃止し、BFF Lambda の実行ロールで AWS を呼ぶ

- **ステータス**: 承認済み
- **日付**: 2026-07-23
- **決定者**: 人間（プロジェクトオーナー）

> 本 ADR は ADR 0013（ホスティングを AWS ネイティブへ移す）の帰結であり、同一 PR で ADR 0005 を「廃止（0014 により置換）」とする。認証機構の置換をホスティングの決定から切り出して独立に記録する（ADR 0001 運用ルール3。0005 の本文は書き換えない）。

## コンテキスト

ADR 0005 は、BFF（Next.js Route Handler）が **Vercel のランタイム**で動くことを前提に、Vercel → AWS の OIDC Federation を採用した。Lambda Function URL が IAM 認証（ADR 0003）である以上、BFF は AWS 認証情報を持つ必要があり、静的キーを避けるために federation が要る、という構図だった。

ADR 0013 で **BFF を AWS Lambda 上へ移した**。これにより前提が消える。BFF が AWS 内の Lambda で動くなら、**実行ロール（IAM）で直接 AWS を呼べる**。federation という中間機構を挟む理由がなくなった。

ADR 0005 は「未解決のリスク」として Vercel Hobby の商用制限を、また「悪い影響」として **信頼ポリシーの `sub` 条件の設定ミスが単一障害点になる**ことを記録していた。BFF が AWS を離れることで、後者のリスクは機構ごと消える。

## 決定

**Vercel OIDC Federation を廃止し、BFF（Next.js サーバーの Lambda）は自身の実行ロールでドメイン API を呼ぶ。**

- ドメイン API の Lambda Function URL は `authorization_type = "AWS_IAM"` のまま（ADR 0003 を維持）
- BFF Lambda は実行ロールで SigV4 署名して Function URL を呼ぶ
- 実行ロールの権限は `lambda:InvokeFunctionUrl` を**当該関数のみ**に限定する（ADR 0005 の権限最小化を実行ロールへ引き継ぐ）
- AWS 側の OIDC ID プロバイダ（`oidc.vercel.com`）・Vercel 向け信頼ポリシー・Vercel 環境変数（`AWS_ROLE_ARN` 等）は不要になり、Terraform から削除する

## 影響

### 良い影響

- **「静的キーをどこにも置かない」方針が、federation という追加機構なしに満たされる。** 実行ロールは AWS が管理する短命認証情報であり、鍵の保管・ローテーションが存在しない
- **ADR 0005 の単一障害点が消える。** `sub` 条件の設定ミスで第三者にロールを引き受けられるという経路が、共有 OIDC プロバイダごと無くなる
- **Vercel への依存が減る。** ホスティング（ADR 0013）と認証（本 ADR）の両方が AWS に寄り、ポートフォリオの説明が一貫する
- OIDC プロバイダ・信頼ポリシーの Terraform が不要になり、構成が小さくなる

### 悪い影響 / トレードオフ

- **実行ロールの権限最小化は引き続き人手で守る。** `lambda:InvokeFunctionUrl` を当該関数のみに絞る規律は、federation の有無に関わらず必要
- **ADR 0003 の多層防御は変更しない。** 予約同時実行数 = 5 は引き続きコスト面の最終防衛線とする（「認証は破られない」という前提を置かない方針は 0003・0005 から不変）
- **ローカル開発で BFF を Lambda として動かす/エミュレートする手数が要る。** 0005 の `vercel env pull` に相当する手間が、AWS ローカル実行の手間に置き換わる

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| ADR 0005 のまま Vercel OIDC を残す | BFF が AWS 外にあるという 0005 の前提が ADR 0013 で消えた。AWS 内の Lambda に対して外部 federation を挟むのは不要な複雑さ |
| 静的アクセスキーを実行環境に置く | 長期認証情報が存在し方針と衝突する。ADR 0005 で却下済みの理由がそのまま当てはまる |
| BFF から `InvokeFunction`（Function URL を使わない）で直接呼ぶ | ADR 0003 が Function URL + IAM 認証を採用した判断を覆す。呼び出し経路の変更は本 ADR の範囲外 |

## 関連

- ADR 0013: ホスティングを AWS ネイティブへ移す（**本 ADR の前提**）
- ADR 0005: Vercel → AWS の OIDC Federation（**本 ADR が置換する**）
- ADR 0003: Lambda Function URL の IAM 認証（**維持**）
- ADR 0001: ADR の運用ルール（廃止・置換の手順）
