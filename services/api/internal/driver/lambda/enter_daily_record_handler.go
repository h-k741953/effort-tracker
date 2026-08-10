package lambda

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8②「リクエストごとの結線」の境界を、EnterDailyRecord
// （E-3）について実装する（get_work_month_handler.go と同じ形）。

// EnterDailyRecordInvoker は EnterDailyRecord（E-3）の呼び出し先が満たす
// 最小の interface（AC-9-8-a と同じ形を driver/lambda 側で宣言）。
type EnterDailyRecordInvoker interface {
	Execute(ctx context.Context, input port.EnterDailyRecordInput)
}

// NewEnterDailyRecordHandler は EnterDailyRecord（E-3）の http.Handler を
// 返す（AC-10-8②）。共通の結線（リクエストごとの presenter 生成・
// INTERNAL_ERROR への委譲）は newWorkMonthHandler が担う（W-2）。
func NewEnterDailyRecordHandler(buildInvoker func(port.WorkMonthOutputPort) EnterDailyRecordInvoker) http.Handler {
	return newWorkMonthHandler(buildInvoker, func(r *http.Request, invoker EnterDailyRecordInvoker, output *presenter.WorkMonthPresenter) {
		controller.HandleEnterDailyRecord(r, invoker, output)
	})
}
