# 実行必須コマンド

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
