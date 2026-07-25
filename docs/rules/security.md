# セキュリティ

- `.env` / `terraform.tfvars` / `*.tfstate` は `.gitignore` 済み。新しい機密ファイル形式を導入するときは**コミット前に** `.gitignore` を更新する
- シークレットをコードやドキュメントに書かない。例示は必ずダミー値（`docs/` 内の AWS アカウントID・接続文字列・URL はすべて架空のもの）
- `gitleaks` を GitHub Actions に組み込む
- **長期認証情報を使わない（CI は GitHub OIDC、ランタイムは実行ロール）**。静的アクセスキーをどこにも置かない（GitHub Secrets、Lambda の環境変数、Terraform 変数ファイル等）
  - CI（GitHub Actions）→ AWS: GitHub OIDC
  - ランタイム: BFF / SSR Lambda（Next.js サーバー）が自身の実行ロールで SigV4 署名し、ドメイン API の Lambda Function URL を直接呼ぶ（`docs/adr/0014`、前提として `docs/adr/0013`）
  - **ランタイムの AWS 認証に外部 IdP との federation を挟まない。** AWS 側の IAM OIDC ID プロバイダ・その向け先の信頼ポリシー・呼び出し元サービスへ渡す `AWS_ROLE_ARN` 等の環境変数を、いずれも設けない
  - **信頼ポリシーで `sub` 条件を省かない（GitHub OIDC の信頼ポリシー）。** GitHub Actions の OIDC プロバイダは GitHub 利用者全体で共有されており、`sub` を省くと任意のリポジトリのワークフローが当該 IAM ロールを引き受けられる。最低でも `repo:[OWNER]/[REPO]` まで固定する
