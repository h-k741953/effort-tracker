package presenter

import (
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// ListWorkMonthsPresenter は port.ListWorkMonthsOutputPort を実装する
// （AC-9-2・AC-9-13-d）。リクエストごとに生成し、プロセス内で共有しない
// （AC-9-13-a と同じ形）。WorkMonthPresenter とは型を分けるが、エラー →
// code・ステータスの対応表（errorMapping）は共有し、二重に持たない
// （AC-9-13-d・AC-11-10）。
var _ port.ListWorkMonthsOutputPort = (*ListWorkMonthsPresenter)(nil)

// NewListWorkMonthsPresenter は ListWorkMonthsPresenter を生成する
// （AC-9-13-a・AC-9-13-d。リクエストごとに生成する）。
func NewListWorkMonthsPresenter() *ListWorkMonthsPresenter {
	return &ListWorkMonthsPresenter{}
}

// ListWorkMonthsPresenter は成功・失敗いずれか1回の呼び出しの結果を保持する
// （AC-9-13-b・AC-9-13-d）。
type ListWorkMonthsPresenter struct {
	result *Result
}

// WorkMonthListItemViewModel は契約 AC-10-2 の `items[]` の1行。
type WorkMonthListItemViewModel struct {
	ContractID          string `json:"contractId"`
	ContractDisplayName string `json:"contractDisplayName"`
	YearMonth           string `json:"yearMonth"`
	State               string `json:"state"`
}

// WorkMonthListViewModel は契約 AC-10-2 の一覧の表現（成功応答のボディ）。
type WorkMonthListViewModel struct {
	// Items は常に非 nil のスライスとし、空でも [] として直列化する
	// （AC-9-11-f）。
	Items  []WorkMonthListItemViewModel `json:"items"`
	Total  int                          `json:"total"`
	Limit  int                          `json:"limit"`
	Offset int                          `json:"offset"`
}

// Present は一覧の出力 DTO を ViewModel へ変換し保持する（AC-9-10-c・
// AC-9-11-f）。
//
// テスト工程時点では未実装（TDD の Red を確認するための宣言のみ）。実装は
// 次工程（implementer）が行う。
func (p *ListWorkMonthsPresenter) Present(output port.ListWorkMonthsOutput) {
}

// PresentError はエラーをステータス・code へ写して保持する（AC-9-12・
// AC-11-12・AC-11-13。errorMapping を共有する＝AC-9-13-d）。
//
// テスト工程時点では未実装。実装は次工程（implementer）が行う。
func (p *ListWorkMonthsPresenter) PresentError(err error) {
}

// Result は直近の Present / PresentError の呼び出し結果を返す。一度も
// 呼ばれていなければ ok は false（AC-9-13-c）。
func (p *ListWorkMonthsPresenter) Result() (Result, bool) {
	if p.result == nil {
		return Result{}, false
	}
	return *p.result, true
}
