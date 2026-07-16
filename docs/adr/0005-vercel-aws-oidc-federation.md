# 0005. Vercel から AWS への認証に OIDC Federation を使う

- **ステータス**: 承認済み
- **日付**: 2026-07-16
- **決定者**: 人間（プロジェクトオーナー）

## コンテキスト

ADR 0003 で Lambda Function URL を `authorization_type = "AWS_IAM"` とした。これにより呼び出しには SigV4 署名が必須になり、ブラウザから直接到達できなくなった。

その結果、**Vercel のランタイム（BFF の Route Handler）が AWS 認証情報を持つ必要が生じた**。ADR 0003 はこの点を未決事項として残していた。

ここには方針上の衝突があった。`CLAUDE.md` は「CI の AWS 認証は OIDC。静的アクセスキーを GitHub Secrets に置かない」と定めているが、これは **CI/CD の文脈**で書かれたものである。Lambda を実際に呼ぶ主体は CI ではなく Vercel のランタイムであり、別途認証情報が要る。ここで静的キーを使うと、ポートフォリオとして「静的キーを使わない」と掲げながらランタイムで使うことになり、説明を要する状態になる。

未決の核心は「**Vercel OIDC federation が Hobby プランで使えるか**」の一点だった。使えなければ静的キー以外の選択肢が実質なく、方針との衝突を受け入れるしかなかった。

### 調査結果

公式ドキュメントおよび変更履歴を確認した（2026-07-16 時点）。

- **OIDC Federation は Hobby を含む全プランで利用可能**であり、追加費用はない。GA 済み
- Vercel が発行する短命の OIDC トークンを AWS が信頼し、`sts:AssumeRoleWithWebIdentity` で短命の認証情報に交換する
- Functions では `x-vercel-oidc-token` ヘッダにトークンが載る。トークンの TTL は60分、キャッシュは45分で、実行中の失効を避ける余裕が取られている
- プラン制限があるのは **Secure Compute**（AWS のプライベート網への接続、Enterprise 限定）である。本プロジェクトは Function URL を公開エンドポイントとして IAM 認証で保護する構成（ADR 0003）なので、**Secure Compute を必要としない**

したがって未決の前提は解消し、案A（OIDC）が成立する。

## 決定

**Vercel から AWS への認証に OIDC Federation を採用する。静的アクセスキーは使わない。**

これにより「静的アクセスキーを使わない」という方針が、CI/CD だけでなく**ランタイムまで一貫**する。方針との衝突は消滅した。

### 構成

| 場所 | 設定 |
|---|---|
| AWS | OIDC ID プロバイダ（`oidc.vercel.com`）を Terraform で作成 |
| AWS | IAM ロール + 信頼ポリシー（`aud` と `sub` を条件に必須） |
| AWS | ロールの権限は `lambda:InvokeFunctionUrl` を**当該関数のみ**に限定 |
| Vercel | 環境変数 `AWS_ROLE_ARN` と `AWS_REGION` |
| コード | `@vercel/oidc-aws-credentials-provider` の `awsCredentialsProvider()` を AWS SDK の `credentials` に渡す。呼び出しは `apps/web/src/lib/lambda-client.ts` に集約する |

### 信頼ポリシーで `sub` 条件を省かない

**これは最重要の実装制約である。** 信頼ポリシーの条件を書かない、あるいは `aud` だけで `sub` を省くと、**Vercel にサインアップした任意の第三者が当該 IAM ロールを引き受けられる**。OIDC プロバイダは Vercel 利用者全体で共有されているため、「Vercel が発行したトークンであること」は本人性を何ら保証しない。

`sub` は `owner:[TEAM]:project:[PROJECT]:environment:[ENV]` の形を取る。ここまで固定して初めて「自分のプロジェクトの、その環境から」に限定される。

現在は AWS 側も、既知の共有 OIDC プロバイダに対して条件の明示的な評価を強制している。欠けている場合はロールの作成・更新が `MalformedPolicyDocument` で失敗する。**この検査に依存せず、意図して書く。**

### 多層防御は変更しない

ADR 0003 の多層防御はそのまま維持する。OIDC により認証情報の漏洩リスクは大きく下がるが、**ゼロにはならない**。

**予約同時実行数 = 5 は引き続きコスト面の最終防衛線**である。OIDC の短命トークンであっても、信頼ポリシーの設定を誤れば第三者がロールを引き受けうる。「認証は破られない」という前提を置かない方針は ADR 0003 から変えない。

## 影響

### 良い影響

- **静的な長期認証情報がどこにも存在しなくなる。** 漏洩させる対象がないので、鍵のローテーション運用も不要になる
- 認証情報が短命になり、漏洩時の有効期間が構造的に限られる
- 「静的アクセスキーを使わない」という方針が CI とランタイムで一貫する。ポートフォリオとして説明に穴がない

### 悪い影響 / コスト

- **信頼ポリシーの設定ミスが単一障害点**になる。`sub` 条件を省くと第三者にロールを引き受けられる。静的キーより安全とは限らず、**正しく設定した場合にのみ安全**である
- **Vercel のチーム名・プロジェクト名を変更すると `sub` が変わり、信頼ポリシーが壊れる**。リネームは AWS 側の更新とセットで行う必要がある
- `AWS_REGION` は既定では安定しないため、明示指定が必須
- ローカル開発で `vercel env pull` によるトークン取得が要り、手数が増える（ADR 0003 で述べた「ローカルで curl できない」の延長）
- **Vercel への依存が1つ増える。** ホスティングを変更する場合、この認証機構ごと作り直しになる

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| IAM ユーザーの静的アクセスキーを Vercel の環境変数に置く | OIDC が全プランで使える以上、これを選ぶ理由がない。長期認証情報が存在し、方針とも衝突する |
| Lambda の呼び出しを Vercel 以外（GitHub Actions 等）から行う | BFF 経由の一方通行というアーキテクチャ制約（ADR 0003）に反する |
| Function URL を `NONE` にして認証情報自体を不要にする | ADR 0003 で却下済み。認証と到達可能性の混同 |

## 未解決のリスク（本 ADR の範囲外）

**Vercel Hobby プランの商用利用制限**が、本プロジェクトの前提と抵触する可能性がある。

Fair Use Guidelines と利用規約は Hobby を非商用・個人利用に限定しており、商用の定義は「プロジェクトの制作に関わった誰かの金銭的利益を目的とするあらゆるデプロイ」と広い。本プロジェクトは公開デモを伴うポートフォリオであり、その位置づけが実績紹介にとどまるか、営業導線と解釈されるかで結論が変わりうる。Vercel は Hobby のデプロイを予告なく削除する権利を留保している。

これは ADR 0002（Vercel Hobby の採用）の前提に関わる独立した判断であり、代替ホスティングの調査を要する。**本 ADR では扱わず、別途判断する。**

## 関連

- ADR 0003: Function URL を IAM 認証で保護する（本 ADR の前提。未決事項の出所）
- ADR 0002: サーバーレス構成の採用（Vercel Hobby を選んだ判断）

## 参考

- [Vercel: OpenID Connect (OIDC) Federation](https://vercel.com/docs/oidc)
- [Vercel: Connect to Amazon Web Services (AWS)](https://vercel.com/docs/oidc/aws)
- [Vercel: OIDC Federation Reference](https://vercel.com/docs/oidc/reference)
- [AWS: Identity-provider controls for shared OIDC providers](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_oidc_secure-by-default.html)
