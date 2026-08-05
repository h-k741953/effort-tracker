package controller

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// closeWorkMonthInvoker は CloseWorkMonth（E-5）の呼び出し先が満たす
// 最小の interface（AC-9-8-a）。
type closeWorkMonthInvoker interface {
	Execute(ctx context.Context, input port.CloseWorkMonthInput)
}

// HandleCloseWorkMonth は E-5
// （POST /work-months/{contractId}/{yearMonth}/close）を入力 DTO へ変換し
// invoker を呼ぶ（AC-9-5-e）。ボディは読まない・検査しない（AC-9-6-h）。
// 更新系は契約 AC-9 順1 の対象（決定10・AC-9-7-a①）。
func HandleCloseWorkMonth(r *http.Request, invoker closeWorkMonthInvoker, output errorPresenter) {
	actor, err := requireActorHeader(r)
	if err != nil {
		output.PresentError(err)
		return
	}

	contractID, err := parseContractID(r.PathValue("contractId"))
	if err != nil {
		output.PresentError(err)
		return
	}

	yearMonth, err := parseYearMonth(r.PathValue("yearMonth"))
	if err != nil {
		output.PresentError(err)
		return
	}

	invoker.Execute(r.Context(), port.CloseWorkMonthInput{
		Actor:      actor,
		ContractID: contractID,
		YearMonth:  yearMonth,
	})
}
