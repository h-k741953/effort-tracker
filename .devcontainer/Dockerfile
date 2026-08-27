FROM node:24

# 必要なパッケージ（ファイアウォール制御・基本ツール）
# jq / aggregate は init-firewall.sh が GitHub Meta API の
# IP レンジを処理するために必要
# iproute2 は init-firewall.sh がホストネットワークを検出するために必要
# （`ip route` を使用。無いと "ip: command not found" → HOST_IP が空 → exit 1）
RUN apt-get update && apt-get install -y \
    iptables \
    ipset \
    iproute2 \
    dnsutils \
    aggregate \
    jq \
    curl \
    git \
    sudo \
    python3-pip \
    python3-venv \
    pipx \
    && rm -rf /var/lib/apt/lists/*

# --- GitHub CLI（gh）のインストール ---
# 公式apt リポジトリ経由。issue/PR操作をコンテナ内で完結させるために必要。
RUN mkdir -p -m 755 /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
       -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
       > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# node ユーザーに sudo 権限を付与（ファイアウォールスクリプト実行のためのみ）
RUN echo "node ALL=(root) NOPASSWD: /usr/local/bin/init-firewall.sh" \
    > /etc/sudoers.d/node-firewall

# 永続化ディレクトリと npm prefix を node 所有で作成
# ※ USER node の「前」に置くこと。これがないとボリュームが root 所有になり、
#    認証情報の保存に失敗して「Not logged in」が発生する。
# ※ .config/gh は gh CLI（`gh auth login`）の認証情報保存先。再ビルドでも
#    認証状態を保持するため devcontainer.json 側でボリュームマウントする。
RUN mkdir -p /home/node/.claude /commandhistory /home/node/.npm-global /home/node/.config/gh /home/node/go \
    && chown -R node:node /home/node/.claude /commandhistory /home/node/.npm-global /home/node/.config /home/node/go

# =============================================================================
# プロジェクト用ツールチェーン（Go / Terraform / gitleaks / golangci-lint）
#
# ※ すべて「ビルド時」に入れること。init-firewall.sh は実行時に外向き通信を
#    allowlist 方式で遮断しており、go.dev・storage.googleapis.com・
#    releases.hashicorp.com はいずれも許可されていない。ビルド中は
#    ファイアウォール未適用なので、ここで入れておく必要がある。
# =============================================================================

# --- Go ツールチェーン ---
# 公式イメージから COPY する方式。tarball を直接取得する方式と違い、
# amd64/arm64 の出し分けを Docker 側が解決するため case 文が不要になる。
# ベースが node:24（bookworm）なので、glibc を揃えるため -bookworm を使う。
COPY --from=golang:1.26.5-bookworm /usr/local/go /usr/local/go

# --- Terraform ---
# 静的リンクされた単一バイナリなので、alpine ベースの公式イメージから
# COPY しても bookworm 上で動作する。
COPY --from=hashicorp/terraform:1.15.8 /bin/terraform /usr/local/bin/terraform

# --- gitleaks / golangci-lint ---
# GitHub Releases 配布。github.com は allowlist に含まれるため実行時でも取得可能だが、
# 再現性のためビルド時にバージョン固定で入れる。
#
# アセット名の注意（両者で命名が非対称。rtk と同じ罠）:
#   gitleaks       x86_64 → linux_x64    / aarch64 → linux_arm64
#   golangci-lint  x86_64 → linux-amd64  / aarch64 → linux-arm64
ARG GITLEAKS_VERSION=8.30.1
ARG GOLANGCI_LINT_VERSION=2.12.2
RUN set -eux; \
    arch="$(uname -m)"; \
    case "$arch" in \
      x86_64)  gl_arch="x64";   gci_arch="amd64" ;; \
      aarch64) gl_arch="arm64"; gci_arch="arm64" ;; \
      *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/gitleaks.tar.gz \
      "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_${gl_arch}.tar.gz"; \
    tar -xzf /tmp/gitleaks.tar.gz -C /usr/local/bin gitleaks; \
    chmod +x /usr/local/bin/gitleaks; \
    curl -fsSL -o /tmp/golangci-lint.tar.gz \
      "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${gci_arch}.tar.gz"; \
    tar -xzf /tmp/golangci-lint.tar.gz -C /tmp; \
    mv "/tmp/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${gci_arch}/golangci-lint" /usr/local/bin/golangci-lint; \
    chmod +x /usr/local/bin/golangci-lint; \
    rm -rf /tmp/gitleaks.tar.gz /tmp/golangci-lint.tar.gz \
           "/tmp/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${gci_arch}"

# ファイアウォールスクリプトを配置
COPY init-firewall.sh /usr/local/bin/init-firewall.sh
RUN chmod +x /usr/local/bin/init-firewall.sh

USER node

# --- Go の実行時設定 ---
# GOTOOLCHAIN=local: go.mod が新しい Go を要求したときの自動ダウンロードを禁止する。
#   既定値(auto)だと proxy.golang.org からツールチェーンを取りに行き、
#   ファイアウォール環境では原因の分かりにくいハングになる。
#   Go を上げるときは本 Dockerfile の COPY タグを明示的に変更すること。
ENV GOPATH=/home/node/go
ENV GOTOOLCHAIN=local
ENV PATH=/usr/local/go/bin:/home/node/go/bin:$PATH

# --- Go モジュールの事前取得（GOMODCACHE の温め） ---
# 原因・方針・帰結の正解は docs/specs/devcontainer-go-module-policy.md にある（今後の加筆は同仕様側へ）。
# init-firewall.sh は proxy.golang.org / sum.golang.org を allowlist に入れるが、
# 追加するのは `dig` が返した「1つの IP」だけである（init-firewall.sh の該当ループ）。
# 両ホストは大きな IP プールを DNS ラウンドロビンで返すため、実行時の取得は
# 当たりを引いたときだけ通る。実測では 25 回連続で `no route to host` になった。
# GitHub の IP レンジは CIDR で全て許可されているため gh / git は安定するが、
# Go モジュールだけがこの穴に落ちる。
#
# そこで、ファイアウォール未適用のビルド時にモジュールキャッシュを温めておく。
# GOPATH は image のレイヤーに残り（devcontainer.json がボリュームマウントするのは
# .claude / commandhistory / .config/gh だけ）、実行時はネットワーク無しで解決できる。
#
# ※ 使い捨てモジュールで取得するのは、ビルドコンテキストが .devcontainer/ であり
#    services/api/go.mod を COPY できないため。キャッシュ（zip と sumdb のタイル）
#    さえ温まれば、どのモジュールから参照しても再利用される。
# ※ バージョンを上げるときはここも更新すること。実行時に別バージョンを要求すると
#    キャッシュに無く、再びファイアウォールの穴に落ちる。
# ※ それでも実行時に取得が要る場合（未固定のバージョン等）は
#    `sudo /usr/local/bin/init-firewall.sh` で許可リストを引き直す。
#    キャッシュ済みの範囲で完結させたいときは `GOPROXY=off` を付ける。
# ※ 正解の実体は services/api/go.mod 側であり、ここでの ARG はビルド時に
#    キャッシュを温めるためだけの二重管理である。バージョンを上げるときは
#    go.mod を先に変更し、この値を追随させること（逆はしない）。
#    PGX_VERSION も driver/persistence（pgx / Neon 接続）の着手により
#    services/api/go.mod へ require が入ったため、他の ARG と同じく go.mod が
#    正解の実体である。この ARG はキャッシュを温めるための追随側にすぎない。
ARG PGX_VERSION=v5.10.0
ARG AWS_LAMBDA_GO_VERSION=v1.54.0
ARG GO_CMP_VERSION=v0.7.0
ARG AWS_SDK_GO_V2_VERSION=v1.43.7
ARG AWS_SDK_GO_V2_CONFIG_VERSION=v1.32.38
ARG AWS_SDK_GO_V2_SSM_VERSION=v1.73.7
ARG AWS_SDK_GO_V2_CLOUDFRONT_VERSION=v1.68.0
RUN set -eux; \
    mkdir -p /tmp/gomodwarm; \
    cd /tmp/gomodwarm; \
    go mod init warmup; \
    go get "github.com/jackc/pgx/v5@${PGX_VERSION}"; \
    go get "github.com/aws/aws-lambda-go@${AWS_LAMBDA_GO_VERSION}"; \
    go get "github.com/google/go-cmp@${GO_CMP_VERSION}"; \
    go get "github.com/aws/aws-sdk-go-v2@${AWS_SDK_GO_V2_VERSION}"; \
    go get "github.com/aws/aws-sdk-go-v2/config@${AWS_SDK_GO_V2_CONFIG_VERSION}"; \
    go get "github.com/aws/aws-sdk-go-v2/service/ssm@${AWS_SDK_GO_V2_SSM_VERSION}"; \
    go get "github.com/aws/aws-sdk-go-v2/service/cloudfront@${AWS_SDK_GO_V2_CLOUDFRONT_VERSION}"; \
    go mod download all; \
    cd /; \
    rm -rf /tmp/gomodwarm

# --- Terraform provider の事前取得（filesystem mirror の温め） ---
# 選択理由の持ち主: docs/specs/infra-terraform.md AC-1-13 / D-26。
#
# 手順: `terraform providers mirror` で、ビルド時（ファイアウォール未適用）に
# provider（hashicorp/aws。.terraform.lock.hcl と揃えた版）一式をイメージへ焼き、
# CLI 設定（$HOME/.terraformrc）で filesystem_mirror を既定にする。
# `-platform=linux_amd64` を明示する。
# ※ バージョンを上げるときは versions.tf の制約と同一 PR で
#    この ARG も更新すること。
ARG TF_AWS_PROVIDER_VERSION=6.61.0
RUN set -eux; \
    mkdir -p /home/node/.terraform-providers-mirror; \
    mkdir -p /tmp/tfproviderwarm; \
    printf 'terraform {\n  required_providers {\n    aws = {\n      source  = "hashicorp/aws"\n      version = "%s"\n    }\n  }\n}\n' "${TF_AWS_PROVIDER_VERSION}" > /tmp/tfproviderwarm/main.tf; \
    terraform -chdir=/tmp/tfproviderwarm providers mirror -platform=linux_amd64 /home/node/.terraform-providers-mirror; \
    rm -rf /tmp/tfproviderwarm; \
    printf 'provider_installation {\n  filesystem_mirror {\n    path = "/home/node/.terraform-providers-mirror"\n  }\n}\n' > /home/node/.terraformrc

# node ユーザー用の npm prefix を設定
# ※ これがないと自動アップデート時に「no write permission to npm prefix」が発生する。
ENV NPM_CONFIG_PREFIX=/home/node/.npm-global
ENV PATH=/home/node/.npm-global/bin:$PATH

# Claude Code を node ユーザーとしてインストール
# ※ USER node の「後」に置くこと。root でインストールすると書き込み権限がなくなる。
# ※ バージョンは固定せず、自動アップデートで最新に追従する運用（11-3 参照）。
RUN npm install -g @anthropic-ai/claude-code@latest

# --- rtk（Rust Token Killer）のインストール ---
# GitHub Releases からバイナリを取得する方式。ビルド不要でイメージが軽い。
# ※ crates.io の `cargo install rtk` は使わないこと（同名の別プロジェクトが存在する）。
#
# アセット名の注意:
#   x86_64 → rtk-x86_64-unknown-linux-musl.tar.gz   （musl）
#   aarch64 → rtk-aarch64-unknown-linux-gnu.tar.gz  （gnu）
# 非対称なので、アーキテクチャに応じて自動選択している。
ARG RTK_VERSION=v0.43.0
RUN set -eux; \
    arch="$(uname -m)"; \
    case "$arch" in \
      x86_64)  asset="rtk-x86_64-unknown-linux-musl.tar.gz" ;; \
      aarch64) asset="rtk-aarch64-unknown-linux-gnu.tar.gz" ;; \
      *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    mkdir -p /home/node/.local/bin; \
    curl -fsSL -o "/tmp/${asset}" \
      "https://github.com/rtk-ai/rtk/releases/download/${RTK_VERSION}/${asset}"; \
    curl -fsSL -o /tmp/checksums.txt \
      "https://github.com/rtk-ai/rtk/releases/download/${RTK_VERSION}/checksums.txt"; \
    (cd /tmp && grep " ${asset}\$" checksums.txt | sha256sum -c -); \
    tar -xzf "/tmp/${asset}" -C /tmp; \
    mv /tmp/rtk /home/node/.local/bin/rtk; \
    chmod +x /home/node/.local/bin/rtk; \
    rm -f "/tmp/${asset}" /tmp/checksums.txt

ENV PATH=/home/node/.local/bin:$PATH

# --- headroom（コンテキスト圧縮プロキシ）のインストール ---
# rtk と併用する。レイヤーが異なるため二重に効く:
#   rtk      : Claude Code のツール出力を CLI レベルで削る
#   headroom : Claude Code ↔ Anthropic 間の API リクエスト全体をプロキシで圧縮
# 使い方は `headroom wrap claude`（proxy を内部起動し claude を localhost:8787 へ向ける）。
#
# ※ CLI は Python 版のみ提供。npm 版（headroom-ai）は TS SDK のみで
#    `headroom` コマンドが入らないため、ここでは pipx で Python CLI を入れる。
# ※ proxy モードで使うため [proxy] extra を明示する。
# ※ pipx で隔離インストールされ、~/.local/bin/headroom に配置される（PATH は上で通済み）。
# ※ ビルド時インストールなのでファイアウォール未適用。pypi.org 等の実行時許可は不要。
# ※ バージョンは再現性のため固定する（rtk と同じ方針）。
ARG HEADROOM_VERSION=0.6.1
ENV PIPX_HOME=/home/node/.local/pipx
ENV PIPX_BIN_DIR=/home/node/.local/bin
RUN pipx install "headroom-ai[proxy]==${HEADROOM_VERSION}"