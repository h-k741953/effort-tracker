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