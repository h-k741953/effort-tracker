package lambda

import (
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8②「リクエストごとの結線」の境界のうち、勤務月1件を
// 出力する6エンドポイント（E-1・E-3〜E-7）に共通する形をまとめる
// （AC-9-13-a〜c）。エンドポイントごとに異なるのは invoker の具体型と
// controller.HandleXxx の呼び出しだけであり、それを handle として受け取る
// （W-2。AC-10-8②の「1つの関数にまとめることは要求しない」は「まとめては
// ならない」という意味ではない）。
//
// 一覧（E-2）は出力ポートの型が異なる（port.ListWorkMonthsOutputPort・
// *presenter.ListWorkMonthsPresenter＝AC-9-13-d）ため、本関数の型引数には
// 含めず、list_work_months_handler.go にもう1本だけ置く。

// newWorkMonthHandler は http.Handler を返す。リクエストごとに出力ポート
// （presenter）を新しく生成し（AC-9-13-a）、buildInvoker でその出力ポートに
// 束ねた invoker を組み立てて handle を呼ぶ。invoker が一度も
// Present/PresentError を呼ばなければ INTERNAL_ERROR とする（AC-9-13-c。
// `code`／ステータスの対応表は driver/lambda に持たない＝AC-11-10・C-2）。
func newWorkMonthHandler[Invoker any](
	buildInvoker func(port.WorkMonthOutputPort) Invoker,
	handle func(*http.Request, Invoker, *presenter.WorkMonthPresenter),
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := presenter.NewWorkMonthPresenter()
		invoker := buildInvoker(output)

		handle(r, invoker, output)

		writeResultOrDelegate(w, output)
	})
}
