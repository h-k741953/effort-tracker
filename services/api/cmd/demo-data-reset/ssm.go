package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// resolveConnectionString は SSM の SecureString を復号付きで取得する
// （docs/specs/demo-data-reset.md AC-11-7。cmd/bootstrap と同じ SDK のパッケージだけを使う）。
// エラーの文言にパラメータ名・取得した値を含めない（AC-11-6）。
func resolveConnectionString(ctx context.Context, parameterName string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("AWS 設定の解決に失敗した: %w", err)
	}
	out, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("SSM からの取得に失敗した: %w", err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", errors.New("SSM から値を取得できなかった")
	}
	return *out.Parameter.Value, nil
}
