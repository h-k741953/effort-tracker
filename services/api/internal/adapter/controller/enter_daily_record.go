package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// enterDailyRecordInvoker は EnterDailyRecord（E-3）の呼び出し先が満たす
// 最小の interface（AC-9-8-a）。
type enterDailyRecordInvoker interface {
	Execute(ctx context.Context, input port.EnterDailyRecordInput)
}

// HandleEnterDailyRecord は E-3
// （PUT /work-months/{contractId}/{yearMonth}/daily-records/{date}）を
// 入力 DTO（port.EnterDailyRecordInput）へ変換し invoker を呼ぶ（AC-9-5-c）。
//
// 更新系は契約 AC-9 順1 の対象。両ヘッダ不在なら入力 DTO を組み立てずに
// port.ErrUnauthenticated を errorPresenter へ渡す（決定10・AC-9-7-a①）。
// 稼働時間は値域を検査せず素の整数のまま写す（AC-9-6-e）。未来日・当該月外・
// 状態・認可はいずれも検査しない（AC-9-6-d・AC-9-6-f）。
func HandleEnterDailyRecord(r *http.Request, invoker enterDailyRecordInvoker, output errorPresenter) {
	actor, err := requireActorHeader(r)
	if err != nil {
		output.PresentError(err)
		return
	}

	// 契約 AC-1-2 が固定するのはメディアタイプであって charset 等のパラメータ
	// ではないため、mime.ParseMediaType でメディアタイプだけを比較する
	// （`application/json; charset=utf-8` のようなパラメータ付きの値を
	// 誤って弾かないため）。
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		output.PresentError(fmt.Errorf("%w: Content-Type must be application/json", port.ErrInvalidRequest))
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

	var body struct {
		WorkingHours *struct {
			Hours   int `json:"hours"`
			Minutes int `json:"minutes"`
		} `json:"workingHours"`
	}
	// 未知フィールドは落とす（エラーにしない。AC-9-6-g）。
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.WorkingHours == nil {
		output.PresentError(fmt.Errorf("%w: workingHours is missing or malformed", port.ErrInvalidRequest))
		return
	}

	invoker.Execute(r.Context(), port.EnterDailyRecordInput{
		Actor:      actor,
		ContractID: contractID,
		YearMonth:  yearMonth,
		Date:       date,
		Hours:      body.WorkingHours.Hours,
		Minutes:    body.WorkingHours.Minutes,
	})
}
