package presenter_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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
			err:        port.ErrInvalidRequest,
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
		// driver/lambda が検出する未定義のパス・メソッドの識別子（AC-9-12-d・AC-11-13）。
		// 名前も置き場所も仕様は固定しない（AC-9-9-d・AC-13-17）ため、テストは
		// 実装が置いた識別子を参照するだけで、その名前・置き場所を主張しない。
		// 固定するのは写像（→ NOT_FOUND / 404）のみ（AC-12-10。契約 AC-1-11）。
		{
			name:       "未定義のパス・メソッドを表す識別子は NOT_FOUND へ写る（AC-11-13。契約 AC-1-11）",
			err:        presenter.ErrRouteNotFound,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
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

// ---- Present（成功系）の直列化を検証する（AC-9-10-a〜d・AC-9-11-a〜e） -----------
//
// 構造体の比較（cmp.Diff）だけでは「未確定 → null」と「未確定 →
// &HoursViewModel{}（0時間0分）」を区別できない（どちらも Go の値としては
// ゼロ値と非ゼロ値で違って見えるが、比較先を間違えると検出できない事故が
// 起きやすい）。そのため encoding/json で実際に直列化し、生の JSON バイト列を
// 検査する（契約 domain-api-http-contract.md AC-10-1）。

// sampleClosedOutput は契約 AC-10-1 の JSON 例に対応する、超過／不足が確定済み・
// 稼働実績1件を持つ出力 DTO を組み立てる。
func sampleClosedOutput() port.WorkMonthOutput {
	return port.WorkMonthOutput{
		ContractID:          "ctr-0001",
		ContractDisplayName: "サンプル株式会社 / 基幹システム保守",
		YearMonth:           "2026-07",
		State:               "PendingApproval",
		Generated:           true,
		SettlementRange: port.SettlementRangeOutput{
			LowerBound: port.Hours{Hours: 140, Minutes: 0},
			UpperBound: port.Hours{Hours: 180, Minutes: 0},
		},
		TotalHours: port.Hours{Hours: 180, Minutes: 15},
		Excess:     &port.Hours{Hours: 0, Minutes: 15},
		Shortfall:  &port.Hours{Hours: 0, Minutes: 0},
		DailyRecords: []port.DailyRecordOutput{
			{
				Date:                "2026-07-01",
				WorkingHours:        port.Hours{Hours: 8, Minutes: 50},
				RoundedWorkingHours: port.Hours{Hours: 8, Minutes: 45},
			},
		},
	}
}

// sampleDraftOutputWithNoRecords は超過／不足が未確定（Draft）・稼働実績0件の
// 出力 DTO を組み立てる（AC-9-11-b・AC-9-11-c の境界を突く形）。
func sampleDraftOutputWithNoRecords() port.WorkMonthOutput {
	return port.WorkMonthOutput{
		ContractID:          "ctr-0002",
		ContractDisplayName: "別の契約株式会社",
		YearMonth:           "2026-08",
		State:               "Draft",
		Generated:           true,
		SettlementRange: port.SettlementRangeOutput{
			LowerBound: port.Hours{Hours: 140, Minutes: 0},
			UpperBound: port.Hours{Hours: 180, Minutes: 0},
		},
		TotalHours:   port.Hours{Hours: 0, Minutes: 0},
		Excess:       nil,
		Shortfall:    nil,
		DailyRecords: nil,
	}
}

// presentAndMarshal は Present を呼び、保持された Result と、その Body を
// 実際に json.Marshal した生バイト列の両方を返す。
func presentAndMarshal(t *testing.T, output port.WorkMonthOutput) (presenter.Result, []byte) {
	t.Helper()
	p := presenter.NewWorkMonthPresenter()
	p.Present(output)

	result, ok := p.Result()
	if !ok {
		t.Fatalf("Present の呼び出し後に Result が保持されていない（AC-9-13-b）")
	}
	raw, err := json.Marshal(result.Body)
	if err != nil {
		t.Fatalf("json.Marshal(result.Body) failed: %v", err)
	}
	return result, raw
}

// presentAndUnmarshalTop は presentAndMarshal に加え、トップレベルのフィールドを
// map[string]json.RawMessage へ写して返す。
func presentAndUnmarshalTop(t *testing.T, output port.WorkMonthOutput) map[string]json.RawMessage {
	t.Helper()
	_, raw := presentAndMarshal(t, output)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("json.Unmarshal(raw) failed: %v", err)
	}
	return obj
}

// TestWorkMonthPresenter_Present_StatusCodeIsAlwaysOK は、Present が保持する
// ステータスが常に 200 であることを検証する（AC-9-10-a: 成功応答はすべて 200）。
func TestWorkMonthPresenter_Present_StatusCodeIsAlwaysOK(t *testing.T) {
	tests := []struct {
		name   string
		output port.WorkMonthOutput
	}{
		{name: "確定済み・稼働実績あり", output: sampleClosedOutput()},
		{name: "未確定・稼働実績なし（Draft）", output: sampleDraftOutputWithNoRecords()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := presentAndMarshal(t, tt.output)
			if result.StatusCode != http.StatusOK {
				t.Errorf("StatusCode = %d, want %d（AC-9-10-a: 成功応答はすべて 200）", result.StatusCode, http.StatusOK)
			}
		})
	}
}

// TestWorkMonthPresenter_Present_UndeterminedExcessShortfallSerializeToNull は、
// 未確定の超過／不足が JSON 上 null として直列化されることを検証する
// （AC-9-11-b・契約 AC-2-6。0 に置き換えない。構造体比較では null と
// {"hours":0,"minutes":0} を区別できないため、実際に直列化して確認する）。
func TestWorkMonthPresenter_Present_UndeterminedExcessShortfallSerializeToNull(t *testing.T) {
	obj := presentAndUnmarshalTop(t, sampleDraftOutputWithNoRecords())

	for _, field := range []string{"excess", "shortfall"} {
		got := strings.TrimSpace(string(obj[field]))
		if got != "null" {
			t.Errorf("%s の直列化 = %s, want null（未確定は null。AC-9-11-b・契約 AC-2-6）", field, got)
		}
	}
}

// TestWorkMonthPresenter_Present_DeterminedExcessShortfallSerializeToObject は、
// 確定済みの超過／不足が時分オブジェクトとして直列化されることを検証する
// （契約 AC-10-1。上の null 検証と対にして、null と非 null の両境界を固定する）。
func TestWorkMonthPresenter_Present_DeterminedExcessShortfallSerializeToObject(t *testing.T) {
	obj := presentAndUnmarshalTop(t, sampleClosedOutput())

	wantExcess := `{"hours":0,"minutes":15}`
	if got := strings.TrimSpace(string(obj["excess"])); got != wantExcess {
		t.Errorf("excess の直列化 = %s, want %s（契約 AC-10-1）", got, wantExcess)
	}
	wantShortfall := `{"hours":0,"minutes":0}`
	if got := strings.TrimSpace(string(obj["shortfall"])); got != wantShortfall {
		t.Errorf("shortfall の直列化 = %s, want %s（契約 AC-10-1）", got, wantShortfall)
	}
}

// TestWorkMonthPresenter_Present_EmptyDailyRecordsSerializeToEmptyArrayNotNull は、
// 稼働実績が0件のとき dailyRecords が空配列として直列化され、null にならないことを
// 検証する（AC-9-11-c・契約 AC-2-2。構造体比較では nil スライスと空スライスを
// 区別できないため、実際に直列化して確認する）。
func TestWorkMonthPresenter_Present_EmptyDailyRecordsSerializeToEmptyArrayNotNull(t *testing.T) {
	obj := presentAndUnmarshalTop(t, sampleDraftOutputWithNoRecords())

	got := strings.TrimSpace(string(obj["dailyRecords"]))
	if got != "[]" {
		t.Errorf("dailyRecords の直列化 = %s, want []（空でも配列であり null にしない。AC-9-11-c・契約 AC-2-2）", got)
	}
}

// TestWorkMonthPresenter_Present_PreservesContractDisplayName は、契約の
// 表示名を書き換えず（空文字にせず）そのまま直列化することを検証する
// （契約 AC-10-1: contractDisplayName）。
func TestWorkMonthPresenter_Present_PreservesContractDisplayName(t *testing.T) {
	output := sampleClosedOutput()
	obj := presentAndUnmarshalTop(t, output)

	var got string
	if err := json.Unmarshal(obj["contractDisplayName"], &got); err != nil {
		t.Fatalf("json.Unmarshal(contractDisplayName) failed: %v", err)
	}
	if got != output.ContractDisplayName {
		t.Errorf("contractDisplayName = %q, want %q（契約 AC-10-1）", got, output.ContractDisplayName)
	}
	if got == "" {
		t.Errorf("contractDisplayName が空文字（契約 AC-10-1 違反）")
	}
}

// TestWorkMonthPresenter_Present_FieldSetMatchesContractExactly は、トップレベルの
// フィールド集合が契約 AC-10-1 と過不足なく一致することを検証する（AC-9-11-e:
// 契約 AC-10 に無いフィールドを足さない。契約 AC-10-3 が挙げる「含めないもの」
// （技術者・承認者の氏名、差戻し理由、日時・履歴、精算金額等）が紛れ込んでいないことも
// このキー集合の突き合わせで併せて固定する）。
func TestWorkMonthPresenter_Present_FieldSetMatchesContractExactly(t *testing.T) {
	obj := presentAndUnmarshalTop(t, sampleClosedOutput())

	gotKeys := make([]string, 0, len(obj))
	for k := range obj {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)

	wantKeys := []string{
		"contractDisplayName", "contractId", "dailyRecords", "excess", "generated",
		"settlementRange", "shortfall", "state", "totalHours", "yearMonth",
	}
	sort.Strings(wantKeys)

	if diff := cmp.Diff(wantKeys, gotKeys); diff != "" {
		t.Errorf("トップレベルのフィールド集合が契約と不一致 (-want +got):\n%s（AC-9-11-e・契約 AC-10-1・AC-10-3）", diff)
	}
}

// TestWorkMonthPresenter_Result_ReportsNoResultUntilPresented は、Present /
// PresentError のいずれも呼ばれていない presenter が「結果なし」を表現できる
// ことを検証する（AC-9-13-c）。`driver/lambda` はこれを INTERNAL_ERROR として
// 応答し、配線の誤りを 200 で返さない。
//
// 生成直後（未呼び出し）で ok が偽になることと、いずれか一方を呼んだ後に
// ok が真になることを対にして置く（対にしないと「常に偽を返す」実装でも
// 通ってしまい、「一度も呼ばれていないこと」を観測できない）。
func TestWorkMonthPresenter_Result_ReportsNoResultUntilPresented(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(p *presenter.WorkMonthPresenter)
		wantOK bool
	}{
		{
			name:   "Present も PresentError も呼ばれていない → ok は偽（AC-9-13-c）",
			invoke: func(*presenter.WorkMonthPresenter) {},
			wantOK: false,
		},
		{
			name:   "Present を呼んだ後 → ok は真（AC-9-13-b）",
			invoke: func(p *presenter.WorkMonthPresenter) { p.Present(sampleClosedOutput()) },
			wantOK: true,
		},
		{
			name:   "PresentError を呼んだ後 → ok は真（AC-9-13-b）",
			invoke: func(p *presenter.WorkMonthPresenter) { p.PresentError(workmonth.ErrNotClosable) },
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := presenter.NewWorkMonthPresenter()
			tt.invoke(p)

			result, ok := p.Result()
			if ok != tt.wantOK {
				t.Fatalf("Result() の ok = %t, want %t（AC-9-13-c）", ok, tt.wantOK)
			}
			if !ok {
				// 「結果なし」のときは、呼び出し元が誤って直列化しないよう
				// ゼロ値のままであること（ステータス 0・ボディ nil）。
				if diff := cmp.Diff(presenter.Result{}, result); diff != "" {
					t.Errorf("結果なしのときの Result が非ゼロ値 (-want +got):\n%s（AC-9-13-c）", diff)
				}
			}
		})
	}
}
