// Command cloudfront-killswitch は CloudFront 従量遮断回路の実行主体である
// （docs/specs/infra-terraform.md D-17・D-18・AC-9-7〜AC-9-13-1・AC-5-4）。
//
// この package は業務ロジックではない（P-5）。internal/domain・
// internal/usecase・internal/adapter のいずれも import しない（AC-9-10）。
//
// 起動の手続きは main() の外へ Run として取り出してある（AC-9-13-1 ①）。
// Run は遮断対象の解決と AWS 設定の解決を済ませてからハンドラを組み立て、
// ランタイムへの登録をちょうど1回行う（同 ③）。登録の手段は引数で受け取るため、
// 実際の Lambda ランタイムを起動せずに手続きを呼べる（同 ②）。
//
// 遮断対象のディストリビューションは Terraform が本 Lambda の環境変数へ注入し
// （docs/specs/infra-terraform.md AC-5-4）、この `cmd` はその環境変数を読む側
// である（AC-9-12）。SNS メッセージの本文をパースして対象を決めることはしない
// （AC-5-4・11-33）。SNS は「閾値に触れた」という契機としてのみ使う。
//
// main() に残るのは、取り出した手続きを実際の解決・実際の受け口の組み立て・
// 実際のランタイムへの登録に結びつけて1回だけ呼び、失敗したら異常終了する
// ことだけである（AC-9-13-1 ①。分岐と計算を残さない。ここに残った結線は
// どのテストも観測しない＝12-34）。
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	awslambda "github.com/aws/aws-lambda-go/lambda"
)

// SNSEventHandler は、SNS イベントを受け取るランタイムへ登録するハンドラの形
// である（AC-9-13-1 ②。イベントを受け取る手段は既存の aws-lambda-go で足りる
// ＝AC-9-14）。
type SNSEventHandler func(ctx context.Context, event events.SNSEvent) error

// Run は AC-9-13-1 の①〜⑥を、main() から呼ばれる1つの手続きとして取り出した
// ものである（形は docs/specs/workmonth-implementation-design.md AC-10-15 /
// AC-10-18 と同型。要求本文はそちらから書き写さない＝ADR 0004）。
//
// 受け取るのは3つである:
//
//	resolveDistributionID 遮断対象の解決（AC-9-12。環境変数から受け取る方式の
//	                      持ち主は AC-5-4）。プロセスの環境変数を暗黙に読まない。
//	newDisabler           AWS 設定の解決（AC-9-15 ①）と CloudFront 呼び出しの
//	                      受け口（AC-9-12）の組み立て。手書きのインメモリ Fake
//	                      へ差し替えられる形のまま保つ（実 AWS を呼ばない）。
//	register              ランタイムへの登録（AC-9-13-1 ②）。
//
// 順序と回数を固定する: 遮断対象の解決と AWS 設定の解決（それぞれちょうど1回）
// を済ませてからハンドラを組み立て、登録をちょうど1回行う。同じ手続きの中で
// 2回以上登録しない（AC-9-13-1 ③）。2つの解決どうしの前後は仕様が固定して
// いない。
//
// 起動時の解決に失敗したら、登録を行わずにエラーで終える（AC-9-13-1 ④・
// AC-8-5 と同型。要求を受け付けてから失敗させず、コールドスタートで失敗させる。
// 既定値へ黙って落ちず、対象を推測もしない）。ラップは %w で行い errors.Is が
// 通る形にする。
//
// 返すエラーの文言に、解決で得た値や認証情報を含めない
// （docs/rules/security.md）。
func Run(
	resolveDistributionID func() (string, error),
	newDisabler func(ctx context.Context) (DistributionDisabler, error),
	register func(h SNSEventHandler),
) error {
	// ③ 遮断対象の解決（ちょうど1回。登録より前）。
	distributionID, err := resolveDistributionID()
	if err != nil {
		// ④ 登録を行わずにエラーで終える。
		return fmt.Errorf("cloudfront-killswitch: 遮断対象の解決に失敗した: %w", err)
	}

	// ③ AWS 設定の解決と受け口の組み立て（ちょうど1回。登録より前）。
	disabler, err := newDisabler(context.Background())
	if err != nil {
		// ④ 登録を行わずにエラーで終える。
		return fmt.Errorf("cloudfront-killswitch: AWS 設定の解決に失敗した: %w", err)
	}

	// ⑤⑥ 登録するハンドラ。SNS イベントの本文を一切見ず（引数を捨てている＝
	// 11-33 の禁止が形として現れている）、起動時に解決した遮断対象に対して
	// 無効化を行う。無効化が失敗したら成功として扱わず失敗として返す。
	// 再試行・通知・復旧は持たせない（非スコープ「遮断 Lambda の遮断以外の
	// 振る舞い」）。
	register(func(ctx context.Context, _ events.SNSEvent) error {
		if err := DisableDistribution(ctx, disabler, distributionID); err != nil {
			return fmt.Errorf("cloudfront-killswitch: ディストリビューションの無効化に失敗した: %w", err)
		}
		return nil
	})

	return nil
}

// newDistributionDisabler は CloudFront 呼び出しの受け口の組み立てを Run が
// 要求する形へ合わせる。newCloudFrontDistributionDisabler は具象型を返すため、
// インターフェースの戻り値へ合わせるのにこの1行が要る。分岐も計算も持たない。
func newDistributionDisabler(ctx context.Context) (DistributionDisabler, error) {
	return newCloudFrontDistributionDisabler(ctx)
}

// startLambda は Lambda ランタイムへの登録を Run が要求する形へ合わせる。
// aws-lambda-go の Start は任意の値を受け取るため、型を名指すのにこの1行が
// 要る。分岐も計算も持たない。
func startLambda(h SNSEventHandler) { awslambda.Start(h) }

// main は取り出した手続き（Run）を、実際の遮断対象の解決・実際の受け口の
// 組み立て・実際のランタイムへの登録に結びつけて1回だけ呼び、失敗したら
// 異常終了するだけである（AC-9-13-1 ①。分岐と計算を残さない。ここに残った
// 結線はどのテストも観測しない＝12-34）。
func main() {
	if err := Run(distributionIDFromEnv, newDistributionDisabler, startLambda); err != nil {
		log.Fatalf("cloudfront-killswitch: 起動に失敗した: %v", err)
	}
}
