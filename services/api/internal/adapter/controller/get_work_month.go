package controller

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// getWorkMonthInvoker は GetWorkMonth（E-1）の呼び出し先が満たす最小の
// interface（AC-9-8-a）。usecase/interactor.GetWorkMonth は Go の interface
// 充足により、この宣言を知らないまま満たす。
type getWorkMonthInvoker interface {
	Execute(ctx context.Context, input port.GetWorkMonthInput)
}

// HandleGetWorkMonth は E-1（GET /work-months/{contractId}/{yearMonth}）を
// 入力 DTO へ変換し invoker を呼ぶ（AC-9-5-a）。
//
// 参照系は操作者ヘッダの有無で応答を変えない（AC-9-7-d）。両ヘッダ不在でも
// 弾かず、未認証の Actor を組み立てて渡す（AC-9-7-a②）。
func HandleGetWorkMonth(r *http.Request, invoker getWorkMonthInvoker, output errorPresenter) {
	actor, err := buildActorAllowGuest(r)
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

	invoker.Execute(r.Context(), port.GetWorkMonthInput{
		Actor:      actor,
		ContractID: contractID,
		YearMonth:  yearMonth,
	})
}
