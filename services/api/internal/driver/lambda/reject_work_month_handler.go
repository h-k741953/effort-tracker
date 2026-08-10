package lambda

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8②「リクエストごとの結線」の境界を、RejectWorkMonth
// （E-7）について実装する（get_work_month_handler.go と同じ形）。

// RejectWorkMonthInvoker は RejectWorkMonth（E-7）の呼び出し先が満たす最小の
// interface（AC-9-8-a と同じ形を driver/lambda 側で宣言）。
type RejectWorkMonthInvoker interface {
	Execute(ctx context.Context, input port.RejectWorkMonthInput)
}

// NewRejectWorkMonthHandler は RejectWorkMonth（E-7）の http.Handler を返す
// （AC-10-8②）。共通の結線は newWorkMonthHandler が担う（W-2）。
func NewRejectWorkMonthHandler(buildInvoker func(port.WorkMonthOutputPort) RejectWorkMonthInvoker) http.Handler {
	return newWorkMonthHandler(buildInvoker, func(r *http.Request, invoker RejectWorkMonthInvoker, output *presenter.WorkMonthPresenter) {
		controller.HandleRejectWorkMonth(r, invoker, output)
	})
}
