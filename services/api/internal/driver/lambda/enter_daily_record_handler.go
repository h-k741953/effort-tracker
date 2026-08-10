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
// 返す（AC-10-8②）。リクエストごとに出力ポート（presenter）を新しく生成し
// （AC-9-13-a）、buildInvoker でその出力ポートに束ねた invoker を組み立てて
// controller を呼ぶ。invoker が一度も Present/PresentError を呼ばなければ
// INTERNAL_ERROR とする（AC-9-13-c）。
func NewEnterDailyRecordHandler(buildInvoker func(port.WorkMonthOutputPort) EnterDailyRecordInvoker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := presenter.NewWorkMonthPresenter()
		invoker := buildInvoker(output)

		controller.HandleEnterDailyRecord(r, invoker, output)

		result, ok := output.Result()
		if !ok {
			writeInternalErrorResult(w)
			return
		}
		writeResult(w, result)
	})
}
