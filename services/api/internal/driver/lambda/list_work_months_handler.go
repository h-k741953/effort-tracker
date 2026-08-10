package lambda

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8②「リクエストごとの結線」の境界を、ListWorkMonths
// （E-2）について実装する。一覧は出力ポートの型が他の6エンドポイントと異なる
// （port.ListWorkMonthsOutputPort・*presenter.ListWorkMonthsPresenter＝
// AC-9-13-d）ため、handler.go の newWorkMonthHandler とは型引数を共有せず、
// 本ファイルに単独で置く（AC-10-8②の「唯一の分岐」＝レビュー往復1 C-1(a)）。

// ListWorkMonthsInvoker は ListWorkMonths（E-2）の呼び出し先が満たす最小の
// interface（AC-9-8-a と同じ形を driver/lambda 側で宣言）。
type ListWorkMonthsInvoker interface {
	Execute(ctx context.Context, input port.ListWorkMonthsInput)
}

// NewListWorkMonthsHandler は ListWorkMonths（E-2）の http.Handler を返す
// （AC-10-8②）。リクエストごとに出力ポート（ListWorkMonthsPresenter）を
// 新しく生成し（AC-9-13-a）、buildInvoker でその出力ポートに束ねた invoker を
// 組み立てて controller を呼ぶ。invoker が一度も Present/PresentError を
// 呼ばなければ INTERNAL_ERROR とする（AC-9-13-c。`code`／ステータスの対応表は
// driver/lambda に持たない＝AC-11-10）。
func NewListWorkMonthsHandler(buildInvoker func(port.ListWorkMonthsOutputPort) ListWorkMonthsInvoker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := presenter.NewListWorkMonthsPresenter()
		invoker := buildInvoker(output)

		controller.HandleListWorkMonths(r, invoker, output)

		writeResultOrDelegate(w, output)
	})
}
