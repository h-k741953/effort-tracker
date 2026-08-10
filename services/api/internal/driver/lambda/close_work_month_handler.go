package lambda

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8②「リクエストごとの結線」の境界を、CloseWorkMonth
// （E-5）について実装する（get_work_month_handler.go と同じ形）。

// CloseWorkMonthInvoker は CloseWorkMonth（E-5）の呼び出し先が満たす最小の
// interface（AC-9-8-a と同じ形を driver/lambda 側で宣言）。
type CloseWorkMonthInvoker interface {
	Execute(ctx context.Context, input port.CloseWorkMonthInput)
}

// NewCloseWorkMonthHandler は CloseWorkMonth（E-5）の http.Handler を返す
// （AC-10-8②）。共通の結線は newWorkMonthHandler が担う（W-2）。
func NewCloseWorkMonthHandler(buildInvoker func(port.WorkMonthOutputPort) CloseWorkMonthInvoker) http.Handler {
	return newWorkMonthHandler(buildInvoker, func(r *http.Request, invoker CloseWorkMonthInvoker, output *presenter.WorkMonthPresenter) {
		controller.HandleCloseWorkMonth(r, invoker, output)
	})
}
