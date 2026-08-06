package presenter_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-10-c・AC-9-11-f・AC-9-13-d・AC-12-14。
//
// work_month_presenter_test.go と同じ形（HTTP サーバも interactor も起動せず、
// 出力ポートを直接呼び、保持された結果を検査する）。mustErrorBody は同ファイル
// （presenter_test パッケージ内）のものを共有する。

func presentListAndMarshal(t *testing.T, output port.ListWorkMonthsOutput) (presenter.Result, []byte) {
	t.Helper()
	p := presenter.NewListWorkMonthsPresenter()
	p.Present(output)

	result, ok := p.Result()
	if !ok {
		t.Fatalf("Present の呼び出し後に Result が保持されていない（AC-9-13-b・AC-9-13-d）")
	}
	raw, err := json.Marshal(result.Body)
	if err != nil {
		t.Fatalf("json.Marshal(result.Body) failed: %v", err)
	}
	return result, raw
}

func presentListAndUnmarshalTop(t *testing.T, output port.ListWorkMonthsOutput) map[string]json.RawMessage {
	t.Helper()
	_, raw := presentListAndMarshal(t, output)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("json.Unmarshal(raw) failed: %v", err)
	}
	return obj
}

// sampleListOutput は契約 AC-10-2 の JSON 例に対応する一覧の出力 DTO。
func sampleListOutput() port.ListWorkMonthsOutput {
	return port.ListWorkMonthsOutput{
		Items: []port.ListWorkMonthsOutputRow{
			{ContractID: "ctr-0001", ContractDisplayName: "サンプル株式会社 / 基幹システム保守", YearMonth: "2026-07", State: "PendingApproval"},
		},
		Total:  42,
		Limit:  20,
		Offset: 0,
	}
}

// TestListWorkMonthsPresenter_Present_StatusCodeIsAlwaysOK は成功応答が常に
// 200 であることを検証する（AC-9-10-a）。
func TestListWorkMonthsPresenter_Present_StatusCodeIsAlwaysOK(t *testing.T) {
	result, _ := presentListAndMarshal(t, sampleListOutput())
	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d（AC-9-10-a）", result.StatusCode, http.StatusOK)
	}
}

// TestListWorkMonthsPresenter_Present_MapsItemsToContractShape は items[] の
// フィールド名・値が契約 AC-10-2 と一致することを検証する（AC-9-10-c・
// AC-12-14①）。
func TestListWorkMonthsPresenter_Present_MapsItemsToContractShape(t *testing.T) {
	obj := presentListAndUnmarshalTop(t, sampleListOutput())

	var items []map[string]any
	if err := json.Unmarshal(obj["items"], &items); err != nil {
		t.Fatalf("json.Unmarshal(items) failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items の件数 = %d, want 1", len(items))
	}
	want := map[string]any{
		"contractId":          "ctr-0001",
		"contractDisplayName": "サンプル株式会社 / 基幹システム保守",
		"yearMonth":           "2026-07",
		"state":               "PendingApproval",
	}
	if diff := cmp.Diff(want, items[0]); diff != "" {
		t.Errorf("items[0] が契約 AC-10-2 と不一致 (-want +got):\n%s", diff)
	}
}

// TestListWorkMonthsPresenter_Present_PassesThroughTotalLimitOffset は
// total・limit・offset が出力 DTO の値のまま直列化されることを検証する
// （AC-12-14③）。
func TestListWorkMonthsPresenter_Present_PassesThroughTotalLimitOffset(t *testing.T) {
	output := port.ListWorkMonthsOutput{
		Items:  []port.ListWorkMonthsOutputRow{},
		Total:  42,
		Limit:  10,
		Offset: 5,
	}
	obj := presentListAndUnmarshalTop(t, output)

	var total, limit, offset int
	if err := json.Unmarshal(obj["total"], &total); err != nil {
		t.Fatalf("json.Unmarshal(total) failed: %v", err)
	}
	if err := json.Unmarshal(obj["limit"], &limit); err != nil {
		t.Fatalf("json.Unmarshal(limit) failed: %v", err)
	}
	if err := json.Unmarshal(obj["offset"], &offset); err != nil {
		t.Fatalf("json.Unmarshal(offset) failed: %v", err)
	}
	if total != 42 || limit != 10 || offset != 5 {
		t.Errorf("total/limit/offset = %d/%d/%d, want 42/10/5（AC-12-14③）", total, limit, offset)
	}
}

// TestListWorkMonthsPresenter_Present_EmptyItemsSerializeToEmptyArrayNotNull は
// 該当0件で items が [] として直列化され、null にならないことを検証する
// （AC-9-11-f・AC-12-14②）。
func TestListWorkMonthsPresenter_Present_EmptyItemsSerializeToEmptyArrayNotNull(t *testing.T) {
	output := port.ListWorkMonthsOutput{
		Items:  []port.ListWorkMonthsOutputRow{},
		Total:  0,
		Limit:  20,
		Offset: 0,
	}
	obj := presentListAndUnmarshalTop(t, output)

	got := strings.TrimSpace(string(obj["items"]))
	if got != "[]" {
		t.Errorf("items の直列化 = %s, want []（null にしない。AC-9-11-f）", got)
	}
	var total int
	if err := json.Unmarshal(obj["total"], &total); err != nil {
		t.Fatalf("json.Unmarshal(total) failed: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0（AC-9-11-f・AC-12-14②）", total)
	}
}

// TestListWorkMonthsPresenter_PresentError_MapsToCodeAndStatus は AC-11-12・
// AC-11-13 の写像を承認待ち一覧のエラー（契約 AC-3-2）について固定する
// （AC-12-14④）。
func TestListWorkMonthsPresenter_PresentError_MapsToCodeAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{
			name:       "port.ErrUnauthenticated は UNAUTHENTICATED へ写る（承認待ち一覧の401。契約AC-3-2）",
			err:        port.ErrUnauthenticated,
			wantCode:   "UNAUTHENTICATED",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "port.ErrNotApprover は FORBIDDEN_NOT_APPROVER へ写る（承認待ち一覧の403。契約AC-3-2）",
			err:        port.ErrNotApprover,
			wantCode:   "FORBIDDEN_NOT_APPROVER",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "対応表のいずれにも該当しないエラーは INTERNAL_ERROR へ写る（AC-11-11）",
			err:        errors.New("some unmapped driver failure"),
			wantCode:   "INTERNAL_ERROR",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := presenter.NewListWorkMonthsPresenter()
			p.PresentError(tt.err)

			result, ok := p.Result()
			if !ok {
				t.Fatalf("PresentError の呼び出し後に Result が保持されていない（AC-9-13-b・AC-9-13-d）")
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

// TestListWorkMonthsPresenter_PresentError_MapsWrappedErrorToSameCode は、
// ラップされたエラーでも errors.Is で同じ code へ写ることを固定する
// （AC-12-14④・AC-9-12-a）。
func TestListWorkMonthsPresenter_PresentError_MapsWrappedErrorToSameCode(t *testing.T) {
	direct := presenter.NewListWorkMonthsPresenter()
	direct.PresentError(port.ErrNotApprover)
	directResult, ok := direct.Result()
	if !ok {
		t.Fatalf("PresentError の呼び出し後に Result が保持されていない（AC-9-13-b・AC-9-13-d）")
	}
	directBody := mustErrorBody(t, directResult.Body)

	wrapped := fmt.Errorf("interactor: list failed: %w", port.ErrNotApprover)
	wp := presenter.NewListWorkMonthsPresenter()
	wp.PresentError(wrapped)
	wrappedResult, ok := wp.Result()
	if !ok {
		t.Fatalf("PresentError の呼び出し後に Result が保持されていない（AC-9-13-b・AC-9-13-d）")
	}
	wrappedBody := mustErrorBody(t, wrappedResult.Body)

	if wrappedBody.Code != directBody.Code {
		t.Errorf("ラップされたエラーの code = %q, want %q（errors.Is で判定すること。AC-9-12-a・AC-11-9）", wrappedBody.Code, directBody.Code)
	}
	if wrappedResult.StatusCode != directResult.StatusCode {
		t.Errorf("ラップされたエラーの StatusCode = %d, want %d（AC-9-12-a・AC-11-9）", wrappedResult.StatusCode, directResult.StatusCode)
	}
}

// TestListWorkMonthsPresenter_Result_ReportsNoResultUntilPresented は、
// Present / PresentError のいずれも呼ばれていない presenter が「結果なし」を
// 表現できることを検証する（AC-9-13-c・AC-9-13-d）。
func TestListWorkMonthsPresenter_Result_ReportsNoResultUntilPresented(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(p *presenter.ListWorkMonthsPresenter)
		wantOK bool
	}{
		{
			name:   "Present も PresentError も呼ばれていない → ok は偽（AC-9-13-c）",
			invoke: func(*presenter.ListWorkMonthsPresenter) {},
			wantOK: false,
		},
		{
			name:   "Present を呼んだ後 → ok は真（AC-9-13-b）",
			invoke: func(p *presenter.ListWorkMonthsPresenter) { p.Present(sampleListOutput()) },
			wantOK: true,
		},
		{
			name:   "PresentError を呼んだ後 → ok は真（AC-9-13-b）",
			invoke: func(p *presenter.ListWorkMonthsPresenter) { p.PresentError(port.ErrNotApprover) },
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := presenter.NewListWorkMonthsPresenter()
			tt.invoke(p)

			result, ok := p.Result()
			if ok != tt.wantOK {
				t.Fatalf("Result() の ok = %t, want %t（AC-9-13-c）", ok, tt.wantOK)
			}
			if !ok {
				if diff := cmp.Diff(presenter.Result{}, result); diff != "" {
					t.Errorf("結果なしのときの Result が非ゼロ値 (-want +got):\n%s（AC-9-13-c）", diff)
				}
			}
		})
	}
}
