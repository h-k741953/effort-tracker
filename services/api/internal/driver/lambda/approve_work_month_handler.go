package lambda

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8②「リクエストごとの結線」の境界を、ApproveWorkMonth
// （E-6）について実装する（get_work_month_handler.go と同じ形）。

// ApproveWorkMonthInvoker は ApproveWorkMonth（E-6）の呼び出し先が満たす
// 最小の interface（AC-9-8-a と同じ形を driver/lambda 側で宣言）。
type ApproveWorkMonthInvoker interface {
	Execute(ctx context.Context, input port.ApproveWorkMonthInput)
}

// NewApproveWorkMonthHandler は ApproveWorkMonth（E-6）の http.Handler を
// 返す（AC-10-8②）。共通の結線は newWorkMonthHandler が担う（W-2）。
func NewApproveWorkMonthHandler(buildInvoker func(port.WorkMonthOutputPort) ApproveWorkMonthInvoker) http.Handler {
	return newWorkMonthHandler(buildInvoker, func(r *http.Request, invoker ApproveWorkMonthInvoker, output *presenter.WorkMonthPresenter) {
		controller.HandleApproveWorkMonth(r, invoker, output)
	})
}
