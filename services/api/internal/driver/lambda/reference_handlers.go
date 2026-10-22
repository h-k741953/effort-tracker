package lambda

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-22④「リクエストごとの結線」を ListContracts（E-8）・
// ListWorkTrends（E-9）について実装する。出力ポートの型が異なるため、
// list_work_months_handler.go と同じく単独で置く（AC-10-8②）。

// ListContractsInvoker は ListContracts（E-8）の呼び出し先が満たす最小の interface。
type ListContractsInvoker interface {
	Execute(ctx context.Context, input port.ListContractsInput)
}

// ListWorkTrendsInvoker は ListWorkTrends（E-9）の呼び出し先が満たす最小の interface。
type ListWorkTrendsInvoker interface {
	Execute(ctx context.Context, input port.ListWorkTrendsInput)
}

// NewListContractsHandler は ListContracts（E-8）の http.Handler を返す。
// リクエストごとに presenter を生成し（AC-9-13-a）、invoker が一度も
// Present/PresentError を呼ばなければ INTERNAL_ERROR とする（AC-9-13-c）。
func NewListContractsHandler(buildInvoker func(port.ListContractsOutputPort) ListContractsInvoker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := presenter.NewListContractsPresenter()
		invoker := buildInvoker(output)

		controller.HandleListContracts(r, invoker, output)

		writeResultOrDelegate(w, output)
	})
}

// NewListWorkTrendsHandler は ListWorkTrends（E-9）の http.Handler を返す。
func NewListWorkTrendsHandler(buildInvoker func(port.ListWorkTrendsOutputPort) ListWorkTrendsInvoker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := presenter.NewListWorkTrendsPresenter()
		invoker := buildInvoker(output)

		controller.HandleListWorkTrends(r, invoker, output)

		writeResultOrDelegate(w, output)
	})
}
