// Command cloudfront-killswitch は CloudFront 従量遮断回路の実行主体である
// （docs/specs/infra-terraform.md D-17・D-18・AC-9-7〜AC-9-13・AC-5-4）。
//
// この package は業務ロジックではない（P-5）。internal/domain・
// internal/usecase・internal/adapter のいずれも import しない（AC-9-10）。
package main

import (
	"context"
	"log"
)

// main は AWS 設定の解決とディストリビューション無効化の実行主体
// （cloudfrontDistributionDisabler）を組み立てるところまでを行う。
//
// 遮断対象のディストリビューションは Terraform が本 Lambda の環境変数へ注入し
// （docs/specs/infra-terraform.md AC-5-4）、この `cmd` はその環境変数を読む側
// である（AC-9-12）。SNS メッセージの本文をパースして対象を決めることはしない
// （AC-5-4・11-33）。ただし、SNS イベントを受け取るランタイムへのハンドラ登録
// を行わない状態で Issue #8 を終えることは既に決まっている（AC-9-13-1。
// 扱いは、取得または復号に失敗したらハンドラを登録せずエラーで終える
// AC-8-5 と同型）。その配線は Issue #8 の外で、対応する Go のテストと
// ともに追加する。
func main() {
	ctx := context.Background()
	// AC-9-13-1 の配線待ち（SNS ハンドラ登録時に無効化の実行主体として使う）。配線が入ったら、受け取った値を DisableDistribution へ渡す。
	if _, err := newCloudFrontDistributionDisabler(ctx); err != nil {
		log.Fatalf("cloudfront-killswitch: 起動に失敗した: %v", err)
	}
	log.Fatal("cloudfront-killswitch: SNS イベントハンドラの配線は未実装（docs/specs/infra-terraform.md AC-9-13-1 参照）")
}
