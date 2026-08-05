package controller_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-5-c・AC-9-6（a〜i）・AC-9-7-a①・AC-9-8・AC-12-9・決定10。
// 契約 AC-1-2（Content-Type）・AC-4-5〜AC-4-10。

// enterTarget は httptest.NewRequest 用の URL を組み立てる（closeTarget と同じ
// 理由でエスケープする）。
func enterTarget(contractID, yearMonth, date string) string {
	return "/work-months/" + url.PathEscape(contractID) + "/" + url.PathEscape(yearMonth) + "/daily-records/" + url.PathEscape(date)
}

func enterPaths(contractID, yearMonth, date string) []pathValue {
	return []pathValue{
		{name: "contractId", value: contractID},
		{name: "yearMonth", value: yearMonth},
		{name: "date", value: date},
	}
}

func jsonHeaders(id, role string) map[string]string {
	h := actorHeaders(id, role)
	h["Content-Type"] = "application/json"
	return h
}

// TestHandleEnterDailyRecord_MapsInput は妥当な要求から
// port.EnterDailyRecordInput（AC-9-5-c）がちょうど1回 invoker へ渡ることを検証する。
// 稼働時間は素の整数のまま写す（AC-9-6-e）。
func TestHandleEnterDailyRecord_MapsInput(t *testing.T) {
	invoker := &invokerSpy[port.EnterDailyRecordInput]{}
	output := &errorPresenterSpy{}
	body := []byte(`{"workingHours":{"hours":8,"minutes":30}}`)
	r := newRequest(
		http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body,
		jsonHeaders(testActorID, string(port.RoleEngineer)),
		enterPaths(testContractID, "2026-07", "2026-07-01")...,
	)

	controller.HandleEnterDailyRecord(r, invoker, output)

	output.wantNoErr(t)
	want := port.EnterDailyRecordInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		ContractID: mustContractID(t, testContractID),
		YearMonth:  mustYearMonth(t, 2026, 7),
		Date:       mustDate(t, 2026, 7, 1),
		Hours:      8,
		Minutes:    30,
	}
	got := invoker.onlyCall(t)
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{}, workmonth.Date{})); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-c）", diff)
	}
}

// TestHandleEnterDailyRecord_PassesThroughOutOfRangeHours は稼働時間が値域外
// （負・24時間超・分が0〜59外）でも controller が弾かず、素の整数のまま渡すことを
// 検証する（AC-9-6-e。値域の判定は集約が行う）。
func TestHandleEnterDailyRecord_PassesThroughOutOfRangeHours(t *testing.T) {
	tests := []struct {
		name    string
		hours   int
		minutes int
	}{
		{name: "時が負", hours: -1, minutes: 0},
		{name: "24時間超", hours: 25, minutes: 0},
		{name: "分が0〜59の範囲外", hours: 1, minutes: 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.EnterDailyRecordInput]{}
			output := &errorPresenterSpy{}
			body := []byte(fmt.Sprintf(`{"workingHours":{"hours":%d,"minutes":%d}}`, tt.hours, tt.minutes))
			r := newRequest(
				http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body,
				jsonHeaders(testActorID, string(port.RoleEngineer)),
				enterPaths(testContractID, "2026-07", "2026-07-01")...,
			)

			controller.HandleEnterDailyRecord(r, invoker, output)

			output.wantNoErr(t)
			got := invoker.onlyCall(t)
			if got.Hours != tt.hours || got.Minutes != tt.minutes {
				t.Errorf("Hours/Minutes = %d/%d, want %d/%d（AC-9-6-e。値域を検査せずそのまま渡すこと）",
					got.Hours, got.Minutes, tt.hours, tt.minutes)
			}
		})
	}
}

// TestHandleEnterDailyRecord_PassesThroughFutureDate は対象日が当日より後でも
// controller が弾かず、そのまま入力 DTO へ渡すことを検証する（AC-9-6-f。
// 未来日の判定は集約が「当日」を引数に受け取って行う）。
func TestHandleEnterDailyRecord_PassesThroughFutureDate(t *testing.T) {
	invoker := &invokerSpy[port.EnterDailyRecordInput]{}
	output := &errorPresenterSpy{}
	body := []byte(`{"workingHours":{"hours":8,"minutes":0}}`)
	r := newRequest(
		http.MethodPut, enterTarget(testContractID, "2099-07", "2099-07-01"), body,
		jsonHeaders(testActorID, string(port.RoleEngineer)),
		enterPaths(testContractID, "2099-07", "2099-07-01")...,
	)

	controller.HandleEnterDailyRecord(r, invoker, output)

	output.wantNoErr(t)
	got := invoker.onlyCall(t)
	wantDate := mustDate(t, 2099, 7, 1)
	if diff := cmp.Diff(wantDate, got.Date, cmp.AllowUnexported(workmonth.Date{})); diff != "" {
		t.Errorf("Date が不一致 (-want +got):\n%s（AC-9-6-f。未来日でも弾かずそのまま渡すこと）", diff)
	}
}

// TestHandleEnterDailyRecord_IgnoresUnknownBodyFields はボディの未知フィールドを
// エラーにせず落とすことを検証する（AC-9-6-g。契約 AC-4-10）。
func TestHandleEnterDailyRecord_IgnoresUnknownBodyFields(t *testing.T) {
	invoker := &invokerSpy[port.EnterDailyRecordInput]{}
	output := &errorPresenterSpy{}
	body := []byte(`{"workingHours":{"hours":8,"minutes":30},"startTime":"09:00","comment":"未知フィールド"}`)
	r := newRequest(
		http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body,
		jsonHeaders(testActorID, string(port.RoleEngineer)),
		enterPaths(testContractID, "2026-07", "2026-07-01")...,
	)

	controller.HandleEnterDailyRecord(r, invoker, output)

	output.wantNoErr(t)
	got := invoker.onlyCall(t)
	if got.Hours != 8 || got.Minutes != 30 {
		t.Errorf("Hours/Minutes = %d/%d, want 8/30（未知フィールドはエラーにせず落とす。AC-9-6-g）", got.Hours, got.Minutes)
	}
}

// TestHandleEnterDailyRecord_RejectsMissingOrMistypedWorkingHours は
// workingHours の欠落・型不一致を弾くことを検証する（AC-9-6-e「弾くのは欠落と
// 型不一致だけ」。契約 AC-4-9）。
func TestHandleEnterDailyRecord_RejectsMissingOrMistypedWorkingHours(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "workingHours が欠落", body: `{}`},
		{name: "hours が文字列（型不一致）", body: `{"workingHours":{"hours":"eight","minutes":30}}`},
		{name: "workingHours が配列（型不一致）", body: `{"workingHours":[8,30]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.EnterDailyRecordInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), []byte(tt.body),
				jsonHeaders(testActorID, string(port.RoleEngineer)),
				enterPaths(testContractID, "2026-07", "2026-07-01")...,
			)

			controller.HandleEnterDailyRecord(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, controller.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, controller.ErrInvalidRequest)（AC-9-6-e）", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleEnterDailyRecord_RejectsContentTypeMismatch はボディを持つ E-3 で
// Content-Type を検査することを検証する（AC-9-6-i。契約 AC-1-2）。
func TestHandleEnterDailyRecord_RejectsContentTypeMismatch(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		setHeader   bool
	}{
		{name: "Content-Type が無い", setHeader: false},
		{name: "Content-Type が application/json でない", contentType: "text/plain", setHeader: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.EnterDailyRecordInput]{}
			output := &errorPresenterSpy{}
			headers := actorHeaders(testActorID, string(port.RoleEngineer))
			if tt.setHeader {
				headers["Content-Type"] = tt.contentType
			}
			body := []byte(`{"workingHours":{"hours":8,"minutes":30}}`)
			r := newRequest(
				http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body, headers,
				enterPaths(testContractID, "2026-07", "2026-07-01")...,
			)

			controller.HandleEnterDailyRecord(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, controller.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, controller.ErrInvalidRequest)（AC-9-6-i）", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleEnterDailyRecord_RejectsWhenBothHeadersAbsent は更新系（契約 AC-9 順1
// の対象）で両ヘッダが不在の要求を弾くことを検証する（決定10・AC-9-7-a①）。
func TestHandleEnterDailyRecord_RejectsWhenBothHeadersAbsent(t *testing.T) {
	invoker := &invokerSpy[port.EnterDailyRecordInput]{}
	output := &errorPresenterSpy{}
	body := []byte(`{"workingHours":{"hours":8,"minutes":30}}`)
	r := newRequest(
		http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body, map[string]string{"Content-Type": "application/json"},
		enterPaths(testContractID, "2026-07", "2026-07-01")...,
	)

	controller.HandleEnterDailyRecord(r, invoker, output)

	if err := output.onlyErr(t); !errors.Is(err, port.ErrUnauthenticated) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)（決定10）", err)
	}
	invoker.wantNoCall(t)
}

// TestHandleEnterDailyRecord_HeaderAbsenceOrderedBeforeSyntaxCheck は決定10
// （操作者ヘッダの有無の判定を構文検査より前に置く）を E-3 でも固定する。
// close_work_month_test.go の同名の検証と対になる（AC-12-9）。
func TestHandleEnterDailyRecord_HeaderAbsenceOrderedBeforeSyntaxCheck(t *testing.T) {
	validBody := []byte(`{"workingHours":{"hours":8,"minutes":30}}`)
	tests := []struct {
		name       string
		headers    map[string]string
		date       string
		wantErr    error
		wantNotErr error
	}{
		{
			name:       "両方不在 かつ 対象日の書式不正 → UNAUTHENTICATED が先（決定10）",
			headers:    map[string]string{"Content-Type": "application/json"},
			date:       "2026-02-30",
			wantErr:    port.ErrUnauthenticated,
			wantNotErr: controller.ErrInvalidRequest,
		},
		{
			name:       "構文不正のみ（ヘッダは妥当） → INVALID_REQUEST",
			headers:    jsonHeaders(testActorID, string(port.RoleEngineer)),
			date:       "2026-02-30",
			wantErr:    controller.ErrInvalidRequest,
			wantNotErr: port.ErrUnauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.EnterDailyRecordInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodPut, enterTarget(testContractID, "2026-07", tt.date), validBody, tt.headers,
				enterPaths(testContractID, "2026-07", tt.date)...,
			)

			controller.HandleEnterDailyRecord(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if errors.Is(err, tt.wantNotErr) {
				t.Fatalf("PresentError に渡されたエラーが %v であってはならない: %v", tt.wantNotErr, err)
			}
			invoker.wantNoCall(t)
		})
	}
}
