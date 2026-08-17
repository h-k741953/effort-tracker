[`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app) で作成した [Next.js](https://nextjs.org) プロジェクト。

## はじめかた

まず開発サーバーを起動する。

```bash
npm run dev
```

ブラウザで [http://localhost:3000](http://localhost:3000) を開くと結果を確認できる。

`src/app/page.tsx` を編集するとページが自動で更新される。

## 参考資料

Next.js について詳しく知るには以下を参照する。

- [Next.js Documentation](https://nextjs.org/docs) — Next.js の機能と API
- [Learn Next.js](https://nextjs.org/learn) — 対話形式のチュートリアル
- [Next.js GitHub リポジトリ](https://github.com/vercel/next.js)

## デプロイ

このアプリは **Vercel ではなく AWS ネイティブなワークロード**（Lambda 上の OpenNext + CloudFront）としてデプロイする。決定と根拠は `docs/adr/0013-aws-native-hosting-over-vercel.md` を参照。

Terraform / OpenNext の構成自体は後続の Issue であり、**このスキャフォールドには含まれない**。
