// 検証対象: docs/specs/infra-terraform.md AC-8-1〜AC-8-11。
//
// Neon への接続文字列は SSM Parameter Store（SecureString）から取得する
// （AC-8-1・AC-8-10・D-4・D-12）。呼ぶ側であるこの `cmd` が自ら宣言する最小の
// インターフェース（SecretFetcher）越しに受け取り、SDK のクライアント構造体を
// 公開 API の引数・戻り値へ露出させない（AC-8-11 ①③）。実装は
// AWS SDK for Go v2 の SSM パッケージと、資格情報・リージョンの解決に同 SDK の
// 設定解決パッケージを用いる（AC-8-10・AC-9-15）。
//
// 置き場所はエントリポイント（`cmd/bootstrap`）の内側に閉じる（AC-8-7）。
// `domain` / `usecase` / `adapter` は import しない。
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SecretFetcher is the minimal interface this package declares for fetching a
// decrypted secret value by parameter name (AC-8-11 ①②). Declaring it here,
// rather than naming the SDK's SSM client type directly, lets tests swap in a
// hand-written in-memory Fake without a mocking library (ADR 0007).
type SecretFetcher interface {
	FetchSecret(ctx context.Context, name string) (string, error)
}

// ResolveConnectionString はパラメータ名を受け取り、SecretFetcher 越しに
// 復号済みの値を返す（AC-8-11 ②）。取得または復号に失敗したら、既定値へ
// 黙って落ちずにエラーで終える（AC-8-5）。エラーの文言にパラメータ名・
// 取得した値を含めない（AC-8-9・docs/rules/security.md）。
func ResolveConnectionString(ctx context.Context, fetcher SecretFetcher, parameterName string) (string, error) {
	value, err := fetcher.FetchSecret(ctx, parameterName)
	if err != nil {
		return "", fmt.Errorf("secret_resolver: パラメータの取得または復号に失敗した: %w", err)
	}
	return value, nil
}

// ssmSecretFetcher は SecretFetcher を AWS SDK for Go v2 の SSM クライアントで
// 満たす実装（AC-8-10・AC-8-11 ④の対になる本番実装）。SDK の型はこの構造体の
// 内側に閉じ、SecretFetcher のシグネチャには現れない（AC-8-11 ③）。
type ssmSecretFetcher struct {
	client *ssm.Client
}

// newSSMSecretFetcher は実行ロールの資格情報とリージョンを AWS SDK の設定解決
// パッケージで解決し、SSM クライアントを組み立てる（AC-8-10）。
func newSSMSecretFetcher(ctx context.Context) (*ssmSecretFetcher, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("secret_resolver: AWS 設定の解決に失敗した: %w", err)
	}
	return &ssmSecretFetcher{client: ssm.NewFromConfig(cfg)}, nil
}

// FetchSecret は SecureString パラメータを復号付きで取得する。
func (f *ssmSecretFetcher) FetchSecret(ctx context.Context, name string) (string, error) {
	out, err := f.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("secret_resolver: SSM からの取得に失敗した: %w", err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", errors.New("secret_resolver: SSM から値を取得できなかった")
	}
	return *out.Parameter.Value, nil
}

var _ SecretFetcher = (*ssmSecretFetcher)(nil)
