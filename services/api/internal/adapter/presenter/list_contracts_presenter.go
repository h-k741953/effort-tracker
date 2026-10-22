package presenter

import (
	"errors"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは契約一覧（E-8）・稼働の推移（E-9）の presenter を置く
// （実装設計 AC-9-10-e・AC-9-11-h・AC-9-13-e）。エラー → code・ステータスの
// 対応表（errorMapping）は work_month_presenter.go の1つを共有する。

var (
	_ port.ListContractsOutputPort  = (*ListContractsPresenter)(nil)
	_ port.ListWorkTrendsOutputPort = (*ListWorkTrendsPresenter)(nil)
)

// errorResult はエラーを対応表（errorMapping）で Result へ写す。表に無いエラーは
// INTERNAL_ERROR（AC-9-12-a）。
func errorResult(err error) *Result {
	for _, m := range errorMapping {
		if errors.Is(err, m.sentinel) {
			return &Result{StatusCode: m.status, Body: ErrorResponse{Error: ErrorBody{
				Code:    m.code,
				Message: m.message,
			}}}
		}
	}
	return &Result{StatusCode: http.StatusInternalServerError, Body: ErrorResponse{Error: ErrorBody{
		Code:    "INTERNAL_ERROR",
		Message: "internal error",
	}}}
}

// ContractPeriodViewModel は契約 AC-10-4 の `contractPeriod`。End は null を取りうる。
type ContractPeriodViewModel struct {
	Start string  `json:"start"`
	End   *string `json:"end"`
}

// ContractWorkMonthViewModel は契約 AC-10-4 の `workMonth`。
type ContractWorkMonthViewModel struct {
	YearMonth string `json:"yearMonth"`
	State     string `json:"state"`
	Generated bool   `json:"generated"`
}

// ContractListItemViewModel は契約 AC-10-4 の `items[]` の1行。
type ContractListItemViewModel struct {
	ContractID          string                     `json:"contractId"`
	ContractDisplayName string                     `json:"contractDisplayName"`
	SettlementRange     SettlementRangeViewModel   `json:"settlementRange"`
	ContractPeriod      ContractPeriodViewModel    `json:"contractPeriod"`
	WorkMonth           ContractWorkMonthViewModel `json:"workMonth"`
}

// ContractListViewModel は契約 AC-10-4 の成功応答のボディ。
type ContractListViewModel struct {
	// Items は常に非 nil のスライスとし、空でも [] として直列化する。
	Items []ContractListItemViewModel `json:"items"`
}

// ListContractsPresenter は port.ListContractsOutputPort を実装する
// （リクエストごとに生成する）。
type ListContractsPresenter struct {
	result *Result
}

// NewListContractsPresenter は ListContractsPresenter を生成する。
func NewListContractsPresenter() *ListContractsPresenter {
	return &ListContractsPresenter{}
}

// Present は出力 DTO を ViewModel へ変換し保持する。
func (p *ListContractsPresenter) Present(output port.ListContractsOutput) {
	items := make([]ContractListItemViewModel, 0, len(output.Items))
	for _, row := range output.Items {
		items = append(items, ContractListItemViewModel{
			ContractID:          row.ContractID,
			ContractDisplayName: row.ContractDisplayName,
			SettlementRange: SettlementRangeViewModel{
				LowerBound: toHoursViewModel(row.SettlementRange.LowerBound),
				UpperBound: toHoursViewModel(row.SettlementRange.UpperBound),
			},
			ContractPeriod: ContractPeriodViewModel{
				Start: row.ContractPeriod.Start,
				End:   row.ContractPeriod.End,
			},
			WorkMonth: ContractWorkMonthViewModel{
				YearMonth: row.WorkMonth.YearMonth,
				State:     row.WorkMonth.State,
				Generated: row.WorkMonth.Generated,
			},
		})
	}
	p.result = &Result{StatusCode: http.StatusOK, Body: ContractListViewModel{Items: items}}
}

// PresentError はエラーをステータス・code へ写して保持する。
func (p *ListContractsPresenter) PresentError(err error) {
	p.result = errorResult(err)
}

// Result は直近の結果を返す。一度も呼ばれていなければ ok は false。
func (p *ListContractsPresenter) Result() (Result, bool) {
	if p.result == nil {
		return Result{}, false
	}
	return *p.result, true
}

// WorkTrendItemViewModel は契約 AC-10-5 の `items[]` の1行。
type WorkTrendItemViewModel struct {
	ContractID      string                   `json:"contractId"`
	YearMonth       string                   `json:"yearMonth"`
	State           string                   `json:"state"`
	SettlementRange SettlementRangeViewModel `json:"settlementRange"`
	TotalHours      HoursViewModel           `json:"totalHours"`
	// Excess・Shortfall は未確定なら null（キーを省略しない）。
	Excess    *HoursViewModel `json:"excess"`
	Shortfall *HoursViewModel `json:"shortfall"`
}

// WorkTrendListViewModel は契約 AC-10-5 の成功応答のボディ。
type WorkTrendListViewModel struct {
	// Items は常に非 nil のスライスとし、空でも [] として直列化する。
	Items []WorkTrendItemViewModel `json:"items"`
}

// ListWorkTrendsPresenter は port.ListWorkTrendsOutputPort を実装する
// （リクエストごとに生成する）。
type ListWorkTrendsPresenter struct {
	result *Result
}

// NewListWorkTrendsPresenter は ListWorkTrendsPresenter を生成する。
func NewListWorkTrendsPresenter() *ListWorkTrendsPresenter {
	return &ListWorkTrendsPresenter{}
}

// Present は出力 DTO を ViewModel へ変換し保持する。
func (p *ListWorkTrendsPresenter) Present(output port.ListWorkTrendsOutput) {
	items := make([]WorkTrendItemViewModel, 0, len(output.Items))
	for _, row := range output.Items {
		items = append(items, WorkTrendItemViewModel{
			ContractID: row.ContractID,
			YearMonth:  row.YearMonth,
			State:      row.State,
			SettlementRange: SettlementRangeViewModel{
				LowerBound: toHoursViewModel(row.SettlementRange.LowerBound),
				UpperBound: toHoursViewModel(row.SettlementRange.UpperBound),
			},
			TotalHours: toHoursViewModel(row.TotalHours),
			Excess:     toOptionalHoursViewModel(row.Excess),
			Shortfall:  toOptionalHoursViewModel(row.Shortfall),
		})
	}
	p.result = &Result{StatusCode: http.StatusOK, Body: WorkTrendListViewModel{Items: items}}
}

// PresentError はエラーをステータス・code へ写して保持する。
func (p *ListWorkTrendsPresenter) PresentError(err error) {
	p.result = errorResult(err)
}

// Result は直近の結果を返す。一度も呼ばれていなければ ok は false。
func (p *ListWorkTrendsPresenter) Result() (Result, bool) {
	if p.result == nil {
		return Result{}, false
	}
	return *p.result, true
}
