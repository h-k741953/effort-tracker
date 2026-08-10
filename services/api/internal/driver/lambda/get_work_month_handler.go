package lambda

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8②「リクエストごとの結線」の境界を、GetWorkMonth（E-1）
// について実装する（AC-12-15③・AC-9-13-a〜c）。
//
// controller.HandleGetWorkMonth は「呼び出し先が満たす最小の interface」を
// 引数に取る（AC-9-8-a）。GetWorkMonthInvoker はその interface と構造的に
// 一致する形を driver/lambda 側で宣言したものであり、buildInvoker が返す値・
// controller.HandleGetWorkMonth へそのまま渡す値のいずれも、Go の構造的部分
// 型付けにより controller 側の非公開 interface を満たす（driver は
// controller のその interface 自体を import しない）。

// GetWorkMonthInvoker は GetWorkMonth（E-1）の呼び出し先が満たす最小の
// interface（AC-9-8-a と同じ形を driver/lambda 側で宣言）。
type GetWorkMonthInvoker interface {
	Execute(ctx context.Context, input port.GetWorkMonthInput)
}

// NewGetWorkMonthHandler は GetWorkMonth（E-1）の http.Handler を返す
// （AC-10-8②）。リクエストごとに出力ポート（presenter）を新しく生成し
// （AC-9-13-a）、buildInvoker でその出力ポートに束ねた invoker を組み立てて
// controller を呼ぶ。invoker が一度も Present/PresentError を呼ばなければ
// INTERNAL_ERROR とする（AC-9-13-c）。
func NewGetWorkMonthHandler(buildInvoker func(port.WorkMonthOutputPort) GetWorkMonthInvoker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := presenter.NewWorkMonthPresenter()
		invoker := buildInvoker(output)

		controller.HandleGetWorkMonth(r, invoker, output)

		result, ok := output.Result()
		if !ok {
			writeInternalErrorResult(w)
			return
		}
		writeResult(w, result)
	})
}
