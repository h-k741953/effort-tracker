# アーキテクチャの制約

- **BFF 経由の一方通行**: ブラウザ → Next.js Route Handler → Lambda。Route Handler 以外から Lambda を呼ばない。Function URL が IAM 認証なので、この制約は構造的に担保される（SigV4 署名できるのはサーバー側のみ）
- Lambda 呼び出しは `apps/web/src/lib/lambda-client.ts` に集約する
