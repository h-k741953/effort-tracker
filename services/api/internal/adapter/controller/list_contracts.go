package controller

import (
	"context"
	"fmt"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// listContractsInvoker は ListContracts（E-8）の呼び出し先が満たす最小の
// interface（AC-9-8-a）。
type listContractsInvoker interface {
	Execute(ctx context.Context, input port.ListContractsInput)
}

// listWorkTrendsInvoker は ListWorkTrends（E-9）の呼び出し先が満たす最小の
// interface（AC-9-8-a）。
type listWorkTrendsInvoker interface {
	Execute(ctx context.Context, input port.ListWorkTrendsInput)
}

// HandleListContracts は E-8（GET /contracts）を入力 DTO へ変換し invoker を
// 呼ぶ（AC-9-5-h・AC-9-6-l）。
//
// 順序: ①操作者ヘッダの有無（両ヘッダ不在は port.ErrUnauthenticated）→
// ②片方だけ・ロール値が2値以外 → ③engineerId の欠落・空文字列 → ④yearMonth の
// 欠落・空文字列・書式不正。弾いたら invoker を呼ばない。未知のクエリは読まない。
// 「Engineer かつ本人」は判定しない（AC-8-12。interactor の責務）。
func HandleListContracts(r *http.Request, invoker listContractsInvoker, output errorPresenter) {
	actor, err := requireActorHeader(r)
	if err != nil {
		output.PresentError(err)
		return
	}

	engineerID, err := requireEngineerID(r)
	if err != nil {
		output.PresentError(err)
		return
	}

	yearMonth, err := parseYearMonth(r.URL.Query().Get("yearMonth"))
	if err != nil {
		output.PresentError(err)
		return
	}

	invoker.Execute(r.Context(), port.ListContractsInput{
		Actor:      actor,
		EngineerID: engineerID,
		YearMonth:  yearMonth,
	})
}

// HandleListWorkTrends は E-9（GET /work-trends）を入力 DTO へ変換し invoker を
// 呼ぶ（AC-9-5-i・AC-9-6-l）。順序は HandleListContracts の①〜③と同じ。
func HandleListWorkTrends(r *http.Request, invoker listWorkTrendsInvoker, output errorPresenter) {
	actor, err := requireActorHeader(r)
	if err != nil {
		output.PresentError(err)
		return
	}

	engineerID, err := requireEngineerID(r)
	if err != nil {
		output.PresentError(err)
		return
	}

	invoker.Execute(r.Context(), port.ListWorkTrendsInput{
		Actor:      actor,
		EngineerID: engineerID,
	})
}

// requireEngineerID は engineerId クエリを取り出す。欠落・空文字列は
// port.ErrInvalidRequest。書式は検査しない（E-2 と同じ）。
func requireEngineerID(r *http.Request) (string, error) {
	engineerID := r.URL.Query().Get("engineerId")
	if engineerID == "" {
		return "", fmt.Errorf("%w: engineerId is required", port.ErrInvalidRequest)
	}
	return engineerID, nil
}
