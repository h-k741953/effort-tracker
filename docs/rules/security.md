# セキュリティ

- `.env` / `terraform.tfvars` / `*.tfstate` は `.gitignore` 済み。新しい機密ファイル形式を導入するときは**コミット前に** `.gitignore` を更新する
- シークレットをコードやドキュメントに書かない。例示は必ずダミー値（`docs/` 内の AWS アカウントID・接続文字列・URL はすべて架空のもの）
- `gitleaks` を GitHub Actions に組み込む
- **AWS 認証は CI・ランタイムとも OIDC**。静的アクセスキーをどこにも置かない（GitHub Secrets にも Vercel の環境変数にも）
  - CI（GitHub Actions）→ AWS: GitHub OIDC
  - ランタイム（Vercel）→ AWS: Vercel OIDC Federation（`docs/adr/0005`）
  - **信頼ポリシーで `sub` 条件を省かない。** 省くと Vercel にサインアップした任意の第三者が IAM ロールを引き受けられる
