package controller

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// enterDailyRecordInvoker は EnterDailyRecord（E-3）の呼び出し先が満たす
// 最小の interface（AC-9-8-a）。
type enterDailyRecordInvoker interface {
	Execute(ctx context.Context, input port.EnterDailyRecordInput)
}

// HandleEnterDailyRecord は E-3
// （PUT /work-months/{contractId}/{yearMonth}/daily-records/{date}）を
// 入力 DTO（port.EnterDailyRecordInput）へ変換し invoker を呼ぶ（AC-9-5-c）。
//
// 更新系は契約 AC-9 順1 の対象。両ヘッダ不在なら入力 DTO を組み立てずに
// port.ErrUnauthenticated を errorPresenter へ渡す（決定10・AC-9-7-a①）。
// 稼働時間は値域を検査せず素の整数のまま写す（AC-9-6-e）。未来日・当該月外・
// 状態・認可はいずれも検査しない（AC-9-6-d・AC-9-6-f）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない）。
func HandleEnterDailyRecord(_ *http.Request, _ enterDailyRecordInvoker, _ errorPresenter) {
	// TODO(implementer): AC-9-5-c・AC-9-6・AC-9-7・決定10 を実装する。
}
