#!/bin/bash
set -euo pipefail  # エラー・未定義変数・パイプ失敗で停止
IFS=$'\n\t'

# 1. フラッシュ前に Docker の内部 DNS ルールを退避
DOCKER_DNS_RULES=$(iptables-save -t nat | grep "127\.0\.0\.11" || true)

# 既存ルールと ipset をクリア
# ※ 先にデフォルトポリシーを ACCEPT に戻しておくこと。iptables -F はルールを
#    削除するだけでチェインのデフォルトポリシー（-P OUTPUT DROP 等）は戻さないため、
#    本スクリプトを同一コンテナ内で再実行した場合（postCreateCommand → postStartCommand
#    と連続実行される場合など）、前回実行時に設定した DROP ポリシーが残ったまま
#    許可リスト（ipset）再構築前のネットワーク呼び出し（GitHub IPレンジ取得等）が
#    無応答のままブロックされ、curl がタイムアウト（exit 28）する。
iptables -P INPUT ACCEPT
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT
iptables -F
iptables -X
iptables -t nat -F
iptables -t nat -X
iptables -t mangle -F
iptables -t mangle -X
ipset destroy allowed-domains 2>/dev/null || true

# 2. Docker の内部 DNS 解決のみを復元
if [ -n "$DOCKER_DNS_RULES" ]; then
    echo "Restoring Docker DNS rules..."
    iptables -t nat -N DOCKER_OUTPUT 2>/dev/null || true
    iptables -t nat -N DOCKER_POSTROUTING 2>/dev/null || true
    echo "$DOCKER_DNS_RULES" | xargs -L 1 iptables -t nat
else
    echo "No Docker DNS rules to restore"
fi

# 制限をかける前に DNS とループバックを許可
# ※ SSH(22番)は宛先を限定しないと allowed-domains によるアクセス制限を
#    バイパスしてしまうため、ここでは許可しない。GitHub への SSH が必要な場合は
#    allowed-domains ipset（GitHub IPレンジを含む）経由で全ポートが許可されるため
#    別途ここに追加する必要はない。
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A INPUT -p udp --sport 53 -j ACCEPT
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

# CIDR 対応の ipset を作成
ipset create allowed-domains hash:net

# --- GitHub の IP レンジを Meta API から取得 ---
# これにより github.com / api.github.com / raw.githubusercontent.com 等が一括で許可される
echo "Fetching GitHub IP ranges..."
gh_ranges=$(curl -s https://api.github.com/meta)
if [ -z "$gh_ranges" ]; then
    echo "ERROR: Failed to fetch GitHub IP ranges"
    exit 1
fi
if ! echo "$gh_ranges" | jq -e '.web and .api and .git' >/dev/null; then
    echo "ERROR: GitHub API response missing required fields"
    exit 1
fi

echo "Processing GitHub IPs..."
while read -r cidr; do
    if [[ ! "$cidr" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/[0-9]{1,2}$ ]]; then
        echo "ERROR: Invalid CIDR range from GitHub meta: $cidr"
        exit 1
    fi
    echo "Adding GitHub range $cidr"
    ipset add -exist allowed-domains "$cidr"
done < <(echo "$gh_ranges" | jq -r '(.web + .api + .git)[]' | aggregate -q)

# --- その他の許可ドメインを解決して追加 ---
# 注: sentry.io / statsig.com はテレメトリ用のため意図的に除外している
# 注: ipset add には -exist を付けること。複数のドメインが同一 IP に解決される場合
#     （例: api.anthropic.com と claude.ai）、-exist が無いと重複エラーで停止する。
# 注: proxy.golang.org / sum.golang.org は Go モジュールの取得と
#     チェックサム検証に必要。両方を許可することで GOSUMDB を有効なまま運用できる
#     （GOPROXY=direct + GOSUMDB=off という回避策を採らずに済む）。
#     Go ツールチェーン本体は Dockerfile 側でビルド時に入れているため、
#     storage.googleapis.com / go.dev の許可は不要。
# 注: 上記2ドメインは Google の CDN 上にあり A レコードが流動的。本スクリプトは
#     postStartCommand で毎回再実行されるため、起動のたびに解決し直される。
#     コンテナを起動したまま長時間経過すると IP が変わり得る点は許容する。
for domain in \
    "registry.npmjs.org" \
    "api.anthropic.com" \
    "claude.ai" \
    "proxy.golang.org" \
    "sum.golang.org" \
    "marketplace.visualstudio.com" \
    "vscode.blob.core.windows.net" \
    "update.code.visualstudio.com"; do
    echo "Resolving $domain..."
    ips=$(dig +noall +answer A "$domain" | awk '$4 == "A" {print $5}')
    if [ -z "$ips" ]; then
        echo "ERROR: Failed to resolve $domain"
        exit 1
    fi
    while read -r ip; do
        if [[ ! "$ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
            echo "ERROR: Invalid IP from DNS for $domain: $ip"
            exit 1
        fi
        echo "Adding $ip for $domain"
        ipset add -exist allowed-domains "$ip"
    done < <(echo "$ips")
done

# --- GitHub Actions のログ保管先（ベストエフォート） ---
# CI のログ実体は productionresultssaN.blob.core.windows.net 上にあり、許可しないと
# `gh run view --log` がタイムアウトする。これが読めないと「CI 失敗 → ログを読む →
# ローカルで再現」というループの最初の一歩が踏めない（docs/harness/verification-loop.md）。
#
# 上のループと分けている理由は、失敗時の扱いが正反対だから:
#   - 上のループ: 解決できなければ exit 1。開発に必須なので落として気付かせる
#   - このループ: 解決できなければ黙って skip。ログが読めないだけで開発は続行できる
#
# 注: N の採番は GitHub の内部実装であり公開されていない。実測では sa0〜sa25 が存在し、
#     sa22 のような飛び番もあった。**存在しない番号でエラーにしてはならない。**
#     ここで exit 1 すると、ログが読めないという些細な問題がコンテナ起動不能に化ける。
# 注: 解決される IP はすべて GitHub Meta API の actions レンジ内であることを確認済み
#     （＝GitHub 管理下）。ただし .actions を丸ごと許可する案は約2760万アドレスの
#     Azure 共有空間を開けることになり、egress 制限という本スクリプトの目的に反する。
#     そのため必要なホストのみを名指しで許可している（実測でユニーク IP 15個程度）。
# 注: IP は Azure Storage のフロントエンドであり流動的。postStartCommand で毎回
#     再実行されるため、起動のたびに解決し直される。
for i in $(seq 0 39); do
    blob_domain="productionresultssa${i}.blob.core.windows.net"
    blob_ips=$(dig +noall +answer A "$blob_domain" 2>/dev/null | awk '$4 == "A" {print $5}' || true)
    if [ -n "$blob_ips" ]; then
        while read -r ip; do
            if [[ "$ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
                ipset add -exist allowed-domains "$ip"
            fi
        done < <(echo "$blob_ips")
    fi
done
echo "Added GitHub Actions log storage hosts (best effort)"

# ホストネットワークを検出して許可（ポートフォワード用）
# 注: IPv6（ip6tables）対応は本スクリプトのスコープ外。本コンテナ環境は IPv4 の
#     デフォルトルートのみを前提としており、IPv6 経路が存在する場合でも
#     ip6tables 側のポリシーは変更していないため別途対応が必要（TODO）。
HOST_IP=$(ip route | grep default | cut -d" " -f3)
if [ -z "$HOST_IP" ]; then
    echo "ERROR: Failed to detect host IP"
    exit 1
fi
if [[ ! "$HOST_IP" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
    echo "ERROR: Invalid host IP detected: $HOST_IP"
    exit 1
fi
HOST_NETWORK=$(echo "$HOST_IP" | sed "s/\.[0-9]*$/.0\/24/")
if [[ ! "$HOST_NETWORK" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/[0-9]{1,2}$ ]]; then
    echo "ERROR: Invalid host network computed: $HOST_NETWORK"
    exit 1
fi
echo "Host network detected as: $HOST_NETWORK"
iptables -A INPUT -s "$HOST_NETWORK" -j ACCEPT
iptables -A OUTPUT -d "$HOST_NETWORK" -j ACCEPT

# デフォルトポリシーを DROP に
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

# 確立済み接続の戻りを許可
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# allowlist（ipset）に一致する宛先のみ許可
iptables -A OUTPUT -m set --match-set allowed-domains dst -j ACCEPT

# それ以外は REJECT（即座にエラーを返し、切り分けを容易にする）
iptables -A OUTPUT -j REJECT --reject-with icmp-admin-prohibited

echo "Firewall configuration complete"

# --- verification: 設定ミスに気づかないまま使うことを防ぐ ---
echo "Verifying firewall rules..."
if curl --connect-timeout 5 https://example.com >/dev/null 2>&1; then
    echo "ERROR: Firewall verification failed - was able to reach https://example.com"
    exit 1
else
    echo "Firewall verification passed - unable to reach https://example.com as expected"
fi

if ! curl --connect-timeout 5 https://api.github.com/zen >/dev/null 2>&1; then
    echo "ERROR: Firewall verification failed - unable to reach https://api.github.com"
    exit 1
else
    echo "Firewall verification passed - able to reach https://api.github.com as expected"
fi