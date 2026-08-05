package presenter

import (
	"errors"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// WorkMonthPresenter は port.WorkMonthOutputPort を実装する（AC-9-2）。
// リクエストごとに生成し、プロセス内で共有しない（AC-9-13-a）。
var _ port.WorkMonthOutputPort = (*WorkMonthPresenter)(nil)

// ErrRouteNotFound は未定義のパス・メソッドへのリクエストを表す（AC-1-11）。
// ルーティングは usecase を経由しない（`driver/lambda` がリクエストを
// どのハンドラにも振り分けられなかった場合の番兵）ため port ではなく
// presenter に置く。`code`/ステータスの対応表を presenter 以外に持たせない
// （AC-11-10）ため、`driver/lambda` はこの番兵を渡して PresentError を呼ぶ
// 側に倒し、404/`NOT_FOUND` を自前で組み立てない。置き場所・識別子名は
// 仕様が固定していない（AC-13-17）。
var ErrRouteNotFound = errors.New("presenter: route not found")

// WorkMonthPresenter は成功・失敗いずれか1回の呼び出しの結果を保持する
// （AC-9-13-b）。
type WorkMonthPresenter struct {
	result *Result
}

// NewWorkMonthPresenter は WorkMonthPresenter を生成する（AC-9-13-a。
// リクエストごとに生成する）。
func NewWorkMonthPresenter() *WorkMonthPresenter {
	return &WorkMonthPresenter{}
}

// HoursViewModel は契約 AC-10-1 の時分オブジェクト（AC-9-11-a）。
type HoursViewModel struct {
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
}

// SettlementRangeViewModel は契約 AC-10-1 の `settlementRange`。
type SettlementRangeViewModel struct {
	LowerBound HoursViewModel `json:"lowerBound"`
	UpperBound HoursViewModel `json:"upperBound"`
}

// DailyRecordViewModel は契約 AC-10-1 の `dailyRecords[]`。
type DailyRecordViewModel struct {
	Date                string         `json:"date"`
	WorkingHours        HoursViewModel `json:"workingHours"`
	RoundedWorkingHours HoursViewModel `json:"roundedWorkingHours"`
}

// WorkMonthViewModel は契約 AC-10-1 の勤務月の表現（成功応答のボディ）。
type WorkMonthViewModel struct {
	ContractID          string                   `json:"contractId"`
	ContractDisplayName string                   `json:"contractDisplayName"`
	YearMonth           string                   `json:"yearMonth"`
	State               string                   `json:"state"`
	Generated           bool                     `json:"generated"`
	SettlementRange     SettlementRangeViewModel `json:"settlementRange"`
	TotalHours          HoursViewModel           `json:"totalHours"`
	// Excess・Shortfall はポインタとし、未確定を null として直列化する
	// （AC-9-11-b。0 に置き換えない）。
	Excess    *HoursViewModel `json:"excess"`
	Shortfall *HoursViewModel `json:"shortfall"`
	// DailyRecords は常に非 nil のスライスとし、空でも [] として直列化する
	// （AC-9-11-c）。
	DailyRecords []DailyRecordViewModel `json:"dailyRecords"`
}

// toHoursViewModel は port.Hours を ViewModel へ写す（AC-9-11-a）。
func toHoursViewModel(h port.Hours) HoursViewModel {
	return HoursViewModel{Hours: h.Hours, Minutes: h.Minutes}
}

// toOptionalHoursViewModel は未確定（nil）を保ったまま写す（AC-9-11-b）。
func toOptionalHoursViewModel(h *port.Hours) *HoursViewModel {
	if h == nil {
		return nil
	}
	v := toHoursViewModel(*h)
	return &v
}

// Present は勤務月1件の出力 DTO を ViewModel へ変換し保持する（AC-9-10・AC-9-11）。
func (p *WorkMonthPresenter) Present(output port.WorkMonthOutput) {
	records := make([]DailyRecordViewModel, 0, len(output.DailyRecords))
	for _, r := range output.DailyRecords {
		records = append(records, DailyRecordViewModel{
			Date:                r.Date,
			WorkingHours:        toHoursViewModel(r.WorkingHours),
			RoundedWorkingHours: toHoursViewModel(r.RoundedWorkingHours),
		})
	}

	body := WorkMonthViewModel{
		ContractID:          output.ContractID,
		ContractDisplayName: output.ContractDisplayName,
		YearMonth:           output.YearMonth,
		State:               output.State,
		Generated:           output.Generated,
		SettlementRange: SettlementRangeViewModel{
			LowerBound: toHoursViewModel(output.SettlementRange.LowerBound),
			UpperBound: toHoursViewModel(output.SettlementRange.UpperBound),
		},
		TotalHours:   toHoursViewModel(output.TotalHours),
		Excess:       toOptionalHoursViewModel(output.Excess),
		Shortfall:    toOptionalHoursViewModel(output.Shortfall),
		DailyRecords: records,
	}

	p.result = &Result{StatusCode: http.StatusOK, Body: body}
}

// errorMapping は AC-11-12・AC-11-13 が固定する Go のエラー識別子 → `code` の
// 対応（ステータスは契約 AC-9 の表）。**対応表に無いエラーは INTERNAL_ERROR**
// （AC-11-11）であり、この一覧に workmonth.ErrInvalidValue を含めない
// （AC-9-12-a・AC-11-13。Reconstruct 由来の破損・gateway の構築失敗は400として
// 外へ出さない＝決定9）。
var errorMapping = []struct {
	sentinel error
	code     string
	status   int
	message  string
}{
	{controller.ErrInvalidRequest, "INVALID_REQUEST", http.StatusBadRequest, "the request is invalid"},
	{workmonth.ErrWorkingHoursOutOfRange, "WORKING_HOURS_OUT_OF_RANGE", http.StatusBadRequest, "working hours out of range"},
	{workmonth.ErrFutureDate, "FUTURE_DATE_NOT_ALLOWED", http.StatusBadRequest, "date must not be in the future"},
	{workmonth.ErrDateOutOfMonth, "DATE_OUT_OF_WORK_MONTH", http.StatusBadRequest, "date is out of the work month"},
	{port.ErrUnauthenticated, "UNAUTHENTICATED", http.StatusUnauthorized, "authentication is required"},
	{port.ErrNotOwner, "FORBIDDEN_NOT_OWNER", http.StatusForbidden, "actor is not the owner"},
	{port.ErrNotApprover, "FORBIDDEN_NOT_APPROVER", http.StatusForbidden, "actor is not an approver"},
	{port.ErrSelfApproval, "FORBIDDEN_SELF_APPROVAL", http.StatusForbidden, "self approval is not allowed"},
	{port.ErrContractNotFound, "CONTRACT_NOT_FOUND", http.StatusNotFound, "contract not found"},
	{port.ErrWorkMonthNotFound, "WORK_MONTH_NOT_FOUND", http.StatusNotFound, "work month not found"},
	{workmonth.ErrNotEditable, "WORK_MONTH_NOT_EDITABLE", http.StatusConflict, "work month is not editable"},
	{workmonth.ErrNotClosable, "INVALID_STATE_FOR_CLOSE", http.StatusConflict, "work month is not in Draft"},
	{workmonth.ErrNotApprovable, "INVALID_STATE_FOR_APPROVE", http.StatusConflict, "work month is not in PendingApproval"},
	{workmonth.ErrNotRejectable, "INVALID_STATE_FOR_REJECT", http.StatusConflict, "work month is not in PendingApproval"},
	{ErrRouteNotFound, "NOT_FOUND", http.StatusNotFound, "the requested path or method does not exist"},
}

// PresentError はエラーをステータス・code へ写して保持する（AC-9-12・AC-11-12・
// AC-11-13）。対応表に無いエラー（workmonth.ErrInvalidValue を含む）は
// INTERNAL_ERROR とする（AC-11-11）。message には受け取ったエラーの文字列を
// そのまま入れない（AC-9-12-b・docs/rules/security.md）。
func (p *WorkMonthPresenter) PresentError(err error) {
	for _, m := range errorMapping {
		if errors.Is(err, m.sentinel) {
			p.result = &Result{StatusCode: m.status, Body: ErrorResponse{Error: ErrorBody{
				Code:    m.code,
				Message: m.message,
			}}}
			return
		}
	}
	p.result = &Result{StatusCode: http.StatusInternalServerError, Body: ErrorResponse{Error: ErrorBody{
		Code:    "INTERNAL_ERROR",
		Message: "internal error",
	}}}
}

// Result は直近の Present / PresentError の呼び出し結果を返す。一度も
// 呼ばれていなければ ok は false（AC-9-13-c。`driver/lambda` はこれを
// INTERNAL_ERROR として応答する）。
func (p *WorkMonthPresenter) Result() (Result, bool) {
	if p.result == nil {
		return Result{}, false
	}
	return *p.result, true
}
