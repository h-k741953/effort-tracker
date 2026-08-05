package controller

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// deleteDailyRecordInvoker は DeleteDailyRecord（E-4）の呼び出し先が満たす
// 最小の interface（AC-9-8-a）。
type deleteDailyRecordInvoker interface {
	Execute(ctx context.Context, input port.DeleteDailyRecordInput)
}

// HandleDeleteDailyRecord は E-4
// （DELETE /work-months/{contractId}/{yearMonth}/daily-records/{date}）を
// 入力 DTO へ変換し invoker を呼ぶ（AC-9-5-d）。ボディは読まない・検査しない
// （AC-9-6-h）。更新系は契約 AC-9 順1 の対象（決定10・AC-9-7-a①）。
func HandleDeleteDailyRecord(r *http.Request, invoker deleteDailyRecordInvoker, output errorPresenter) {
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

	date, err := parseDate(r.PathValue("date"))
	if err != nil {
		output.PresentError(err)
		return
	}

	invoker.Execute(r.Context(), port.DeleteDailyRecordInput{
		Actor:      actor,
		ContractID: contractID,
		YearMonth:  yearMonth,
		Date:       date,
	})
}
