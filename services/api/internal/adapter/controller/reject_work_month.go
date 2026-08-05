package controller

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// rejectWorkMonthInvoker は RejectWorkMonth（E-7）の呼び出し先が満たす
// 最小の interface（AC-9-8-a）。
type rejectWorkMonthInvoker interface {
	Execute(ctx context.Context, input port.RejectWorkMonthInput)
}

// HandleRejectWorkMonth は E-7
// （POST /work-months/{contractId}/{yearMonth}/reject）を入力 DTO へ変換し
// invoker を呼ぶ（AC-9-5-g）。ボディは読まない・検査しない（AC-9-6-h）。
// 更新系は契約 AC-9 順1 の対象（決定10・AC-9-7-a①）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない）。
func HandleRejectWorkMonth(_ *http.Request, _ rejectWorkMonthInvoker, _ errorPresenter) {
	// TODO(implementer): AC-9-5-g・AC-9-6・AC-9-7・決定10 を実装する。
}
