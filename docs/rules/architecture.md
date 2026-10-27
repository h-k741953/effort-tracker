# アーキテクチャの制約

- **BFF 経由の一方通行**: ブラウザ → Next.js Route Handler → Lambda。Route Handler 以外から Lambda を呼ばない。Function URL が IAM 認証なので、この制約は構造的に担保される（SigV4 署名できるのはサーバー側のみ）
- Lambda 呼び出しは `apps/web/src/lib/lambda-client.ts` に集約する
- **注記（2026-10-27 人間が決定。`docs/specs/demo-data-reset.md` U-3）**: 「Route Handler 以外から Lambda を呼ばない」は、ブラウザからドメイン API への経路を守る制約である。AWS の内部からだけ起動され、Function URL を持たない Lambda（デモデータのリセット用＝ADR 0022、CloudFront 遮断＝`docs/specs/infra-terraform.md`）は対象の外とする
