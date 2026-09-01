// 検証対象: docs/specs/infra-terraform.md AC-5-4・AC-7-2・AC-9-12。
//
// CloudFront の呼び出しは、この `cmd` が自ら宣言する最小のインターフェース
// （DistributionDisabler）越しに受け取る。SDK のクライアント構造体を公開 API の
// 引数・戻り値へ露出させない（AC-9-12・AC-8-11 ③ と同型）。実装は
// AWS SDK for Go v2 の CloudFront パッケージと、資格情報・リージョンの解決に
// 同 SDK の設定解決パッケージを用いる（AC-9-15）。
//
// 宣言するメソッドは無効化に要る最小（現行設定の取得と更新＝AC-7-2 の2操作）に
// 限る。遮断対象のディストリビューション ID は Terraform が注入する環境変数から
// 受け取り（AC-5-4・AC-9-12。読み取りは distribution_id_env.go）、SNS メッセージ
// の本文からは決めない（11-33）。SNS イベントを受け取るランタイムへのハンドラ
// 登録は main.go の Run が行い（AC-9-13-1）、ここは受け口と無効化の手続きだけを
// 持つ。
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// DistributionConfig はこの `cmd` が扱う最小の設定表現（AC-9-12「無効化に要る
// 最小」）。ETag は更新時の楽観ロックに、Enabled は無効化の対象に使う。
type DistributionConfig struct {
	ETag    string
	Enabled bool
}

// DistributionDisabler is the minimal interface this package declares for
// getting and updating a CloudFront distribution's config (AC-9-12, the two
// operations corresponding to AC-7-2). Declaring it here, rather than naming
// the SDK's CloudFront client type directly, lets tests swap in a
// hand-written in-memory Fake without a mocking library (ADR 0007).
type DistributionDisabler interface {
	GetDistributionConfig(ctx context.Context, distributionID string) (DistributionConfig, error)
	UpdateDistribution(ctx context.Context, distributionID string, cfg DistributionConfig) error
}

// DisableDistribution は現行設定を取得したうえで Enabled = false にして更新する
// （AC-5-4・AC-7-2）。取得に失敗したら更新を試みない。
func DisableDistribution(ctx context.Context, disabler DistributionDisabler, distributionID string) error {
	cfg, err := disabler.GetDistributionConfig(ctx, distributionID)
	if err != nil {
		return fmt.Errorf("cloudfront-killswitch: 現行設定の取得に失敗した: %w", err)
	}

	cfg.Enabled = false

	if err := disabler.UpdateDistribution(ctx, distributionID, cfg); err != nil {
		return fmt.Errorf("cloudfront-killswitch: 更新に失敗した: %w", err)
	}
	return nil
}

// cloudfrontDistributionDisabler は DistributionDisabler を AWS SDK for Go v2 の
// CloudFront クライアントで満たす実装。SDK の型はこの構造体の内側に閉じ、
// DistributionDisabler のシグネチャには現れない（AC-9-12）。
//
// DistributionConfig（この `cmd` が扱う最小表現）には ETag と Enabled しか
// 無いが、CloudFront の UpdateDistribution はディストリビューションの完全な
// 設定を要求する（他のフィールドを欠くと上書きされてしまう）。そのため、
// GetDistributionConfig で取得した完全な設定を内部にキャッシュしておき、
// UpdateDistribution では Enabled だけを書き換えて送り返す。
type cloudfrontDistributionDisabler struct {
	client *cloudfront.Client
	cache  map[string]*types.DistributionConfig
}

// newCloudFrontDistributionDisabler は実行ロールの資格情報とリージョンを
// AWS SDK の設定解決パッケージで解決し、CloudFront クライアントを組み立てる。
func newCloudFrontDistributionDisabler(ctx context.Context) (*cloudfrontDistributionDisabler, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("cloudfront-killswitch: AWS 設定の解決に失敗した: %w", err)
	}
	return &cloudfrontDistributionDisabler{
		client: cloudfront.NewFromConfig(cfg),
		cache:  make(map[string]*types.DistributionConfig),
	}, nil
}

func (d *cloudfrontDistributionDisabler) GetDistributionConfig(ctx context.Context, distributionID string) (DistributionConfig, error) {
	out, err := d.client.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{
		Id: aws.String(distributionID),
	})
	if err != nil {
		return DistributionConfig{}, fmt.Errorf("cloudfront-killswitch: CloudFront からの現行設定の取得に失敗した: %w", err)
	}
	if out.DistributionConfig == nil || out.ETag == nil {
		return DistributionConfig{}, errors.New("cloudfront-killswitch: CloudFront から取得した現行設定が不完全だった")
	}

	d.cache[distributionID] = out.DistributionConfig

	return DistributionConfig{
		ETag:    *out.ETag,
		Enabled: aws.ToBool(out.DistributionConfig.Enabled),
	}, nil
}

func (d *cloudfrontDistributionDisabler) UpdateDistribution(ctx context.Context, distributionID string, cfg DistributionConfig) error {
	full, ok := d.cache[distributionID]
	if !ok {
		return errors.New("cloudfront-killswitch: 現行設定が取得されていない（先に GetDistributionConfig を呼ぶ必要がある）")
	}
	full.Enabled = aws.Bool(cfg.Enabled)

	if _, err := d.client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{
		Id:                 aws.String(distributionID),
		IfMatch:            aws.String(cfg.ETag),
		DistributionConfig: full,
	}); err != nil {
		return fmt.Errorf("cloudfront-killswitch: CloudFront への更新に失敗した: %w", err)
	}
	return nil
}

var _ DistributionDisabler = (*cloudfrontDistributionDisabler)(nil)
