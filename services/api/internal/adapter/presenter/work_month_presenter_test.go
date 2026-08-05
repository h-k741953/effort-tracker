package presenter_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-12-10（AC-11-13 の写像を presenter のテストで固定する）。
//
// HTTP サーバも interactor も起動せず、PresentError を直接呼び、保持された
// 結果（ステータス + ボディ。AC-9-13-b）を検査する。写像は errors.Is で引く
// ことを固定する（`==` 実装では Red になる形。AC-9-12-a・AC-11-9）。

// mustErrorBody は Result のボディが presenter.ErrorResponse であることを
// 確認して中身を返す（AC-9-12-c。フィールドを増やさない形）。
func mustErrorBody(t *testing.T, body any) presenter.ErrorBody {
	t.Helper()
	res, ok := body.(presenter.ErrorResponse)
	if !ok {
		t.Fatalf("Result.Body の型 = %T, want presenter.ErrorResponse（AC-9-12-c）", body)
	}
	return res.Error
}

// TestWorkMonthPresenter_PresentError_MapsToCodeAndStatus は AC-11-12・AC-11-13 が
// 固定する Go のエラー識別子 → code の対応（ステータスは契約 AC-9 の表）を、
// domain / usecase/port / controller の番兵ごとに固定する。
func TestWorkMonthPresenter_PresentError_MapsToCodeAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		// adapter 層の要求側の識別子（AC-9-9-a・AC-11-13）。
		{
			name:       "controller の要求側の識別子は INVALID_REQUEST へ写る（AC-11-13）",
			err:        controller.ErrInvalidRequest,
			wantCode:   "INVALID_REQUEST",
			wantStatus: http.StatusBadRequest,
		},
		// domain/workmonth の番兵（AC-11-1〜AC-11-6、契約 AC-9 の表）。
		{
			name:       "workmonth.ErrWorkingHoursOutOfRange は WORKING_HOURS_OUT_OF_RANGE へ写る",
			err:        workmonth.ErrWorkingHoursOutOfRange,
			wantCode:   "WORKING_HOURS_OUT_OF_RANGE",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "workmonth.ErrFutureDate は FUTURE_DATE_NOT_ALLOWED へ写る",
			err:        workmonth.ErrFutureDate,
			wantCode:   "FUTURE_DATE_NOT_ALLOWED",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "workmonth.ErrDateOutOfMonth は DATE_OUT_OF_WORK_MONTH へ写る",
			err:        workmonth.ErrDateOutOfMonth,
			wantCode:   "DATE_OUT_OF_WORK_MONTH",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "workmonth.ErrNotEditable は WORK_MONTH_NOT_EDITABLE へ写る",
			err:        workmonth.ErrNotEditable,
			wantCode:   "WORK_MONTH_NOT_EDITABLE",
			wantStatus: http.StatusConflict,
		},
		{
			name:       "workmonth.ErrNotClosable は INVALID_STATE_FOR_CLOSE へ写る",
			err:        workmonth.ErrNotClosable,
			wantCode:   "INVALID_STATE_FOR_CLOSE",
			wantStatus: http.StatusConflict,
		},
		{
			name:       "workmonth.ErrNotApprovable は INVALID_STATE_FOR_APPROVE へ写る（AC-11-12）",
			err:        workmonth.ErrNotApprovable,
			wantCode:   "INVALID_STATE_FOR_APPROVE",
			wantStatus: http.StatusConflict,
		},
		{
			name:       "workmonth.ErrNotRejectable は INVALID_STATE_FOR_REJECT へ写る（AC-11-12）",
			err:        workmonth.ErrNotRejectable,
			wantCode:   "INVALID_STATE_FOR_REJECT",
			wantStatus: http.StatusConflict,
		},
		{
			// Reconstruct 由来の永続化行の破損は 400 として外へ出さない（AC-11-5・決定9）。
			name:       "workmonth.ErrInvalidValue は INTERNAL_ERROR へ写る（AC-11-5・AC-11-13。400 として出さない）",
			err:        workmonth.ErrInvalidValue,
			wantCode:   "INTERNAL_ERROR",
			wantStatus: http.StatusInternalServerError,
		},
		// usecase/port の番兵（AC-11-7・AC-11-8、契約 AC-9 の表）。
		{
			name:       "port.ErrWorkMonthNotFound は WORK_MONTH_NOT_FOUND へ写る",
			err:        port.ErrWorkMonthNotFound,
			wantCode:   "WORK_MONTH_NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "port.ErrContractNotFound は CONTRACT_NOT_FOUND へ写る",
			err:        port.ErrContractNotFound,
			wantCode:   "CONTRACT_NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "port.ErrUnauthenticated は UNAUTHENTICATED へ写る",
			err:        port.ErrUnauthenticated,
			wantCode:   "UNAUTHENTICATED",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "port.ErrNotOwner は FORBIDDEN_NOT_OWNER へ写る",
			err:        port.ErrNotOwner,
			wantCode:   "FORBIDDEN_NOT_OWNER",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "port.ErrNotApprover は FORBIDDEN_NOT_APPROVER へ写る（AC-11-12）",
			err:        port.ErrNotApprover,
			wantCode:   "FORBIDDEN_NOT_APPROVER",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "port.ErrSelfApproval は FORBIDDEN_SELF_APPROVAL へ写る（AC-11-12）",
			err:        port.ErrSelfApproval,
			wantCode:   "FORBIDDEN_SELF_APPROVAL",
			wantStatus: http.StatusForbidden,
		},
		// 対応表のいずれにも該当しないエラー（AC-9-12-a・AC-11-11）。
		{
			name:       "対応表のいずれにも該当しないエラーは INTERNAL_ERROR へ写る（AC-11-11）",
			err:        errors.New("some unmapped driver failure"),
			wantCode:   "INTERNAL_ERROR",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := presenter.NewWorkMonthPresenter()
			p.PresentError(tt.err)

			result, ok := p.Result()
			if !ok {
				t.Fatalf("PresentError の呼び出し後に Result が保持されていない（AC-9-13-b）")
			}
			if result.StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", result.StatusCode, tt.wantStatus)
			}
			body := mustErrorBody(t, result.Body)
			if body.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Code, tt.wantCode)
			}
		})
	}
}

// TestWorkMonthPresenter_PresentError_MapsWrappedErrorToSameCode は、
// fmt.Errorf の %w でラップされたエラーでも errors.Is で同じ code へ写ることを
// 固定する（AC-12-10。`==` 比較の実装では Red になる）。
func TestWorkMonthPresenter_PresentError_MapsWrappedErrorToSameCode(t *testing.T) {
	direct := presenter.NewWorkMonthPresenter()
	direct.PresentError(workmonth.ErrNotClosable)
	directResult, ok := direct.Result()
	if !ok {
		t.Fatalf("PresentError の呼び出し後に Result が保持されていない（AC-9-13-b）")
	}
	directBody := mustErrorBody(t, directResult.Body)

	wrapped := fmt.Errorf("interactor: close failed: %w", workmonth.ErrNotClosable)
	wp := presenter.NewWorkMonthPresenter()
	wp.PresentError(wrapped)
	wrappedResult, ok := wp.Result()
	if !ok {
		t.Fatalf("PresentError の呼び出し後に Result が保持されていない（AC-9-13-b）")
	}
	wrappedBody := mustErrorBody(t, wrappedResult.Body)

	if wrappedBody.Code != directBody.Code {
		t.Errorf("ラップされたエラーの code = %q, want %q（errors.Is で判定すること。AC-9-12-a・AC-11-9）", wrappedBody.Code, directBody.Code)
	}
	if wrappedResult.StatusCode != directResult.StatusCode {
		t.Errorf("ラップされたエラーの StatusCode = %d, want %d（AC-9-12-a・AC-11-9）", wrappedResult.StatusCode, directResult.StatusCode)
	}
}

// TestWorkMonthPresenter_PresentError_MessageDoesNotLeakRawErrorText は、
// message に受け取ったエラーの文字列をそのまま入れないことを固定する
// （AC-9-12-b・契約 AC-9-3・docs/rules/security.md）。
func TestWorkMonthPresenter_PresentError_MessageDoesNotLeakRawErrorText(t *testing.T) {
	rawText := "pgx: connection refused to db-internal.example.internal:5432 user=svc_workmonth"
	err := errors.New(rawText)

	p := presenter.NewWorkMonthPresenter()
	p.PresentError(err)

	result, ok := p.Result()
	if !ok {
		t.Fatalf("PresentError の呼び出し後に Result が保持されていない（AC-9-13-b）")
	}
	body := mustErrorBody(t, result.Body)

	if strings.Contains(body.Message, rawText) {
		t.Errorf("message に受け取ったエラーの文字列がそのまま含まれている: %q（AC-9-12-b・docs/rules/security.md）", body.Message)
	}
}
