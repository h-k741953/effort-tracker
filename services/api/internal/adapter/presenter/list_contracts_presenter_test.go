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
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: 実装設計 AC-9-10-e・AC-9-11-h・AC-9-13-e・AC-12-19-d。
// 契約 AC-10-4（契約一覧の表現）・AC-10-5（稼働の推移の表現）・AC-9（エラー表）。
//
// JSON はキー名まで比べる（期待は契約の JSON 例から導き、実装の出力を書き写さない）。
// null を取りうるキーは「キーがあり値が null」を map の比較で固定する
// （キーの省略は map の差分として検出される）。

// decodeJSON は期待の JSON 文字列を汎用の値へ復号する。
func decodeJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("期待の JSON が不正: %v\n%s", err, raw)
	}
	return v
}

// marshalBody は Result.Body を直列化して汎用の値へ復号する。
func marshalBody(t *testing.T, body any) (any, string) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal(result.Body) failed: %v", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	return v, string(raw)
}

func strPtr(s string) *string { return &s }

func hoursPtr(h, m int) *port.Hours { return &port.Hours{Hours: h, Minutes: m} }

// ---- ListContractsPresenter ------------------------------------------------

func sampleListContractsOutput() port.ListContractsOutput {
	return port.ListContractsOutput{Items: []port.ContractOutputRow{
		{
			ContractID:          "ctr-0001",
			ContractDisplayName: "サンプル株式会社 / 基幹システム保守",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 140, Minutes: 0},
				UpperBound: port.Hours{Hours: 180, Minutes: 0},
			},
			ContractPeriod: port.ContractPeriodOutput{Start: "2025-04", End: strPtr("2027-03")},
			WorkMonth:      port.ContractWorkMonthOutput{YearMonth: "2026-10", State: "PendingApproval", Generated: true},
		},
		{
			ContractID:          "ctr-0002",
			ContractDisplayName: "サンプル商事 / 受発注刷新",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 150, Minutes: 30},
				UpperBound: port.Hours{Hours: 190, Minutes: 15},
			},
			ContractPeriod: port.ContractPeriodOutput{Start: "2025-04", End: nil},
			WorkMonth:      port.ContractWorkMonthOutput{YearMonth: "2026-10", State: "Draft", Generated: false},
		},
	}}
}

// TestListContractsPresenter_Present_MatchesContractShape は成功応答が 200 で、
// 契約 AC-10-4 の形（キー名・値・終了未定の null）と完全に一致することを検証する
// （AC-9-10-e・AC-9-11-h ②④⑤⑥）。契約 AC-10-4 に無いフィールド（total / limit /
// offset 等）が増えても Red になる。
func TestListContractsPresenter_Present_MatchesContractShape(t *testing.T) {
	p := presenter.NewListContractsPresenter()
	p.Present(sampleListContractsOutput())

	result, ok := p.Result()
	if !ok {
		t.Fatalf("Present の呼び出し後に Result が保持されていない（AC-9-13-e）")
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d（AC-9-10-a）", result.StatusCode, http.StatusOK)
	}

	want := decodeJSON(t, `{
  "items": [
    {
      "contractId": "ctr-0001",
      "contractDisplayName": "サンプル株式会社 / 基幹システム保守",
      "settlementRange": {
        "lowerBound": { "hours": 140, "minutes": 0 },
        "upperBound": { "hours": 180, "minutes": 0 }
      },
      "contractPeriod": { "start": "2025-04", "end": "2027-03" },
      "workMonth": { "yearMonth": "2026-10", "state": "PendingApproval", "generated": true }
    },
    {
      "contractId": "ctr-0002",
      "contractDisplayName": "サンプル商事 / 受発注刷新",
      "settlementRange": {
        "lowerBound": { "hours": 150, "minutes": 30 },
        "upperBound": { "hours": 190, "minutes": 15 }
      },
      "contractPeriod": { "start": "2025-04", "end": null },
      "workMonth": { "yearMonth": "2026-10", "state": "Draft", "generated": false }
    }
  ]
}`)
	got, raw := marshalBody(t, result.Body)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("契約 AC-10-4 の形と不一致 (-want +got):\n%s\n実際の JSON: %s", diff, raw)
	}
}

// TestListContractsPresenter_Present_EndNullKeepsKey は終了未定の契約で
// contractPeriod.end が「キーがあり値が null」であることを直列化の文字列で
// 固定する（AC-9-11-h ②）。
func TestListContractsPresenter_Present_EndNullKeepsKey(t *testing.T) {
	p := presenter.NewListContractsPresenter()
	p.Present(port.ListContractsOutput{Items: []port.ContractOutputRow{{
		ContractID:      "ctr-0002",
		ContractPeriod:  port.ContractPeriodOutput{Start: "2025-04", End: nil},
		WorkMonth:       port.ContractWorkMonthOutput{YearMonth: "2026-10", State: "Draft"},
		SettlementRange: port.SettlementRangeOutput{},
	}}})

	result, _ := p.Result()
	_, raw := marshalBody(t, result.Body)
	if !strings.Contains(raw, `"end":null`) {
		t.Errorf("contractPeriod.end が null として直列化されていない（キーを省略しない）: %s", raw)
	}
}

// TestListContractsPresenter_Present_EmptyItemsSerializeToEmptyArray は 0件で
// "items":[] になる（null にしない）ことを検証する（AC-9-11-h ①・AC-12-19-d）。
// Items が nil で渡されても [] にする。
func TestListContractsPresenter_Present_EmptyItemsSerializeToEmptyArray(t *testing.T) {
	tests := []struct {
		name  string
		items []port.ContractOutputRow
	}{
		{name: "空のスライス", items: []port.ContractOutputRow{}},
		{name: "nil", items: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := presenter.NewListContractsPresenter()
			p.Present(port.ListContractsOutput{Items: tt.items})

			result, ok := p.Result()
			if !ok {
				t.Fatalf("Result が保持されていない")
			}
			raw, err := json.Marshal(result.Body)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			if got := string(raw); got != `{"items":[]}` {
				t.Errorf("直列化 = %s, want {\"items\":[]}（null にしない。AC-9-11-h ①）", got)
			}
		})
	}
}

// TestListContractsPresenter_Present_KeepsRowOrder は行の順を出力 DTO のまま
// にする（並べ替えない）ことを検証する（AC-9-11-h ⑥）。
func TestListContractsPresenter_Present_KeepsRowOrder(t *testing.T) {
	p := presenter.NewListContractsPresenter()
	p.Present(port.ListContractsOutput{Items: []port.ContractOutputRow{
		{ContractID: "ctr-0009"}, {ContractID: "ctr-0002"}, {ContractID: "ctr-0005"},
	}})

	result, _ := p.Result()
	got, _ := marshalBody(t, result.Body)
	items := got.(map[string]any)["items"].([]any)
	var ids []string
	for _, item := range items {
		ids = append(ids, item.(map[string]any)["contractId"].(string))
	}
	if diff := cmp.Diff([]string{"ctr-0009", "ctr-0002", "ctr-0005"}, ids); diff != "" {
		t.Errorf("行の順が出力 DTO のままでない (-want +got):\n%s（AC-9-11-h ⑥）", diff)
	}
}

// ---- ListWorkTrendsPresenter -----------------------------------------------

func sampleListWorkTrendsOutput() port.ListWorkTrendsOutput {
	return port.ListWorkTrendsOutput{Items: []port.WorkTrendOutputRow{
		{
			ContractID: "ctr-0001",
			YearMonth:  "2026-08",
			State:      "Approved",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 140, Minutes: 0},
				UpperBound: port.Hours{Hours: 180, Minutes: 0},
			},
			TotalHours: port.Hours{Hours: 180, Minutes: 15},
			Excess:     hoursPtr(0, 15),
			Shortfall:  hoursPtr(0, 0),
		},
		{
			ContractID: "ctr-0001",
			YearMonth:  "2026-09",
			State:      "Draft",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 140, Minutes: 0},
				UpperBound: port.Hours{Hours: 180, Minutes: 0},
			},
			TotalHours: port.Hours{Hours: 96, Minutes: 30},
			Excess:     nil,
			Shortfall:  nil,
		},
	}}
}

// TestListWorkTrendsPresenter_Present_MatchesContractShape は成功応答が 200 で、
// 契約 AC-10-5 の形と完全に一致することを検証する（AC-9-10-e・AC-9-11-h
// ③④⑤⑥）。0 の超過／不足（確定済み）と null（未確定）を同じ応答の中で区別する。
func TestListWorkTrendsPresenter_Present_MatchesContractShape(t *testing.T) {
	p := presenter.NewListWorkTrendsPresenter()
	p.Present(sampleListWorkTrendsOutput())

	result, ok := p.Result()
	if !ok {
		t.Fatalf("Present の呼び出し後に Result が保持されていない（AC-9-13-e）")
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d（AC-9-10-a）", result.StatusCode, http.StatusOK)
	}

	want := decodeJSON(t, `{
  "items": [
    {
      "contractId": "ctr-0001",
      "yearMonth": "2026-08",
      "state": "Approved",
      "settlementRange": {
        "lowerBound": { "hours": 140, "minutes": 0 },
        "upperBound": { "hours": 180, "minutes": 0 }
      },
      "totalHours": { "hours": 180, "minutes": 15 },
      "excess": { "hours": 0, "minutes": 15 },
      "shortfall": { "hours": 0, "minutes": 0 }
    },
    {
      "contractId": "ctr-0001",
      "yearMonth": "2026-09",
      "state": "Draft",
      "settlementRange": {
        "lowerBound": { "hours": 140, "minutes": 0 },
        "upperBound": { "hours": 180, "minutes": 0 }
      },
      "totalHours": { "hours": 96, "minutes": 30 },
      "excess": null,
      "shortfall": null
    }
  ]
}`)
	got, raw := marshalBody(t, result.Body)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("契約 AC-10-5 の形と不一致 (-want +got):\n%s\n実際の JSON: %s", diff, raw)
	}
}

// TestListWorkTrendsPresenter_Present_NullKeepsKeys は未確定の excess / shortfall が
// 「キーがあり値が null」であることを直列化の文字列で固定する（AC-9-11-h ③）。
func TestListWorkTrendsPresenter_Present_NullKeepsKeys(t *testing.T) {
	p := presenter.NewListWorkTrendsPresenter()
	p.Present(port.ListWorkTrendsOutput{Items: []port.WorkTrendOutputRow{{
		ContractID: "ctr-0001", YearMonth: "2026-09", State: "Draft",
	}}})

	result, _ := p.Result()
	_, raw := marshalBody(t, result.Body)
	for _, key := range []string{`"excess":null`, `"shortfall":null`} {
		if !strings.Contains(raw, key) {
			t.Errorf("%s が直列化に無い（キーを省略しない。0 に置き換えない）: %s", key, raw)
		}
	}
}

// TestListWorkTrendsPresenter_Present_EmptyItemsSerializeToEmptyArray は 0件で
// "items":[] になることを検証する（AC-9-11-h ①・AC-12-19-d）。
func TestListWorkTrendsPresenter_Present_EmptyItemsSerializeToEmptyArray(t *testing.T) {
	tests := []struct {
		name  string
		items []port.WorkTrendOutputRow
	}{
		{name: "空のスライス", items: []port.WorkTrendOutputRow{}},
		{name: "nil", items: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := presenter.NewListWorkTrendsPresenter()
			p.Present(port.ListWorkTrendsOutput{Items: tt.items})

			result, ok := p.Result()
			if !ok {
				t.Fatalf("Result が保持されていない")
			}
			raw, err := json.Marshal(result.Body)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			if got := string(raw); got != `{"items":[]}` {
				t.Errorf("直列化 = %s, want {\"items\":[]}（null にしない。AC-9-11-h ①）", got)
			}
		})
	}
}

// TestListWorkTrendsPresenter_Present_KeepsRowOrder は行の順を並べ替えない
// ことを検証する（AC-9-11-h ⑥）。
func TestListWorkTrendsPresenter_Present_KeepsRowOrder(t *testing.T) {
	p := presenter.NewListWorkTrendsPresenter()
	p.Present(port.ListWorkTrendsOutput{Items: []port.WorkTrendOutputRow{
		{ContractID: "ctr-0002", YearMonth: "2026-09"},
		{ContractID: "ctr-0001", YearMonth: "2026-10"},
		{ContractID: "ctr-0001", YearMonth: "2026-01"},
	}})

	result, _ := p.Result()
	got, _ := marshalBody(t, result.Body)
	var keys []string
	for _, item := range got.(map[string]any)["items"].([]any) {
		m := item.(map[string]any)
		keys = append(keys, m["contractId"].(string)+"|"+m["yearMonth"].(string))
	}
	want := []string{"ctr-0002|2026-09", "ctr-0001|2026-10", "ctr-0001|2026-01"}
	if diff := cmp.Diff(want, keys); diff != "" {
		t.Errorf("行の順が出力 DTO のままでない (-want +got):\n%s（AC-9-11-h ⑥）", diff)
	}
}

// ---- エラー写像（2つの presenter 共通。AC-9-13-e・AC-12-19-d） -----------------

type errorPresenterUnderTest struct {
	name        string
	presentErr  func(err error)
	result      func() (presenter.Result, bool)
	freshResult func() (presenter.Result, bool) // 何も呼ばない新しい presenter の Result
}

func errorPresentersUnderTest() []errorPresenterUnderTest {
	contracts := presenter.NewListContractsPresenter()
	trends := presenter.NewListWorkTrendsPresenter()
	return []errorPresenterUnderTest{
		{
			name:       "ListContractsPresenter",
			presentErr: contracts.PresentError,
			result:     contracts.Result,
			freshResult: func() (presenter.Result, bool) {
				return presenter.NewListContractsPresenter().Result()
			},
		},
		{
			name:       "ListWorkTrendsPresenter",
			presentErr: trends.PresentError,
			result:     trends.Result,
			freshResult: func() (presenter.Result, bool) {
				return presenter.NewListWorkTrendsPresenter().Result()
			},
		},
	}
}

// TestListContractsAndTrendsPresenter_PresentError_MapsToCodeAndStatus は
// ErrNotOwner → 403 FORBIDDEN_NOT_OWNER、ErrUnauthenticated → 401、
// ErrInvalidValue → 500 INTERNAL_ERROR などの写像を両 presenter について固定する
// （AC-12-19-d・AC-9-13-e・契約 AC-9）。
func TestListContractsAndTrendsPresenter_PresentError_MapsToCodeAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{name: "ErrNotOwner は 403 FORBIDDEN_NOT_OWNER", err: port.ErrNotOwner, wantCode: "FORBIDDEN_NOT_OWNER", wantStatus: http.StatusForbidden},
		{name: "ErrUnauthenticated は 401 UNAUTHENTICATED", err: port.ErrUnauthenticated, wantCode: "UNAUTHENTICATED", wantStatus: http.StatusUnauthorized},
		{name: "ErrInvalidRequest は 400 INVALID_REQUEST", err: port.ErrInvalidRequest, wantCode: "INVALID_REQUEST", wantStatus: http.StatusBadRequest},
		{name: "ErrInvalidValue（永続化行の破損）は 500 INTERNAL_ERROR", err: workmonth.ErrInvalidValue, wantCode: "INTERNAL_ERROR", wantStatus: http.StatusInternalServerError},
		{name: "ErrInvalidValue をラップしても 500 INTERNAL_ERROR", err: fmt.Errorf("gateway: row broken: %w", workmonth.ErrInvalidValue), wantCode: "INTERNAL_ERROR", wantStatus: http.StatusInternalServerError},
		{name: "未対応のエラーは 500 INTERNAL_ERROR", err: errors.New("some unmapped driver failure"), wantCode: "INTERNAL_ERROR", wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		for _, sut := range errorPresentersUnderTest() {
			t.Run(sut.name+"/"+tt.name, func(t *testing.T) {
				sut.presentErr(tt.err)

				result, ok := sut.result()
				if !ok {
					t.Fatalf("PresentError の呼び出し後に Result が保持されていない")
				}
				if result.StatusCode != tt.wantStatus {
					t.Errorf("StatusCode = %d, want %d", result.StatusCode, tt.wantStatus)
				}
				if body := mustErrorBody(t, result.Body); body.Code != tt.wantCode {
					t.Errorf("code = %q, want %q", body.Code, tt.wantCode)
				}
			})
		}
	}
}

// TestListContractsAndTrendsPresenter_PresentError_DoesNotLeakErrorText は
// message に受け取ったエラーの文字列をそのまま入れないことを検証する
// （AC-9-12-b・契約 AC-9-3）。
func TestListContractsAndTrendsPresenter_PresentError_DoesNotLeakErrorText(t *testing.T) {
	secret := "postgres://user:pass@host/db SELECT * FROM contracts"
	for _, sut := range errorPresentersUnderTest() {
		t.Run(sut.name, func(t *testing.T) {
			sut.presentErr(fmt.Errorf("%s: %w", secret, workmonth.ErrInvalidValue))

			result, _ := sut.result()
			if body := mustErrorBody(t, result.Body); strings.Contains(body.Message, "postgres") || strings.Contains(body.Message, "SELECT") {
				t.Errorf("message に内部情報が含まれる: %q", body.Message)
			}
		})
	}
}

// TestListContractsAndTrendsPresenter_Result_ReportsNoResultUntilPresented は
// 一度も呼ばれない presenter が「結果なし」を表現できることを検証する
// （AC-9-13-e が AC-9-13-c と同じ形を要求）。呼ばれた後は ok が真。
func TestListContractsAndTrendsPresenter_Result_ReportsNoResultUntilPresented(t *testing.T) {
	for _, sut := range errorPresentersUnderTest() {
		t.Run(sut.name, func(t *testing.T) {
			result, ok := sut.freshResult()
			if ok {
				t.Fatalf("何も呼ばれていないのに Result() の ok が真")
			}
			if diff := cmp.Diff(presenter.Result{}, result); diff != "" {
				t.Errorf("結果なしのときの Result が非ゼロ値 (-want +got):\n%s", diff)
			}

			sut.presentErr(port.ErrNotOwner)
			if _, ok := sut.result(); !ok {
				t.Errorf("PresentError の後に Result() の ok が偽")
			}
		})
	}
}
