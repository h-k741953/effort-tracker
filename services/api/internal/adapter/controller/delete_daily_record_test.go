package controller_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-5-d・AC-9-6-c・AC-9-6-d・AC-9-6-h・AC-9-7-a①・
// AC-9-8・AC-12-9・決定10。

// deleteTarget は httptest.NewRequest 用の URL を組み立てる（closeTarget と同じ
// 理由でエスケープする）。
func deleteTarget(contractID, yearMonth, date string) string {
	return "/work-months/" + url.PathEscape(contractID) + "/" + url.PathEscape(yearMonth) + "/daily-records/" + url.PathEscape(date)
}

func deletePaths(contractID, yearMonth, date string) []pathValue {
	return []pathValue{
		{name: "contractId", value: contractID},
		{name: "yearMonth", value: yearMonth},
		{name: "date", value: date},
	}
}

// TestHandleDeleteDailyRecord_MapsInput は妥当な要求から
// port.DeleteDailyRecordInput（AC-9-5-d）がちょうど1回 invoker へ渡ることを検証する。
func TestHandleDeleteDailyRecord_MapsInput(t *testing.T) {
	invoker := &invokerSpy[port.DeleteDailyRecordInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodDelete, deleteTarget(testContractID, "2026-07", "2026-07-01"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
		deletePaths(testContractID, "2026-07", "2026-07-01")...,
	)

	controller.HandleDeleteDailyRecord(r, invoker, output)

	output.wantNoErr(t)
	want := port.DeleteDailyRecordInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		ContractID: mustContractID(t, testContractID),
		YearMonth:  mustYearMonth(t, 2026, 7),
		Date:       mustDate(t, 2026, 7, 1),
	}
	got := invoker.onlyCall(t)
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{}, workmonth.Date{})); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-d）", diff)
	}
}

// TestHandleDeleteDailyRecord_PassesThroughDateOutOfMonth は対象日が当該年月に
// 属さない要求を controller が弾かず、そのまま入力 DTO へ渡すことを検証する
// （AC-9-6-d。業務バリデーションは interactor・集約が ErrDateOutOfMonth で判定する）。
func TestHandleDeleteDailyRecord_PassesThroughDateOutOfMonth(t *testing.T) {
	invoker := &invokerSpy[port.DeleteDailyRecordInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodDelete, deleteTarget(testContractID, "2026-07", "2026-08-01"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
		deletePaths(testContractID, "2026-07", "2026-08-01")...,
	)

	controller.HandleDeleteDailyRecord(r, invoker, output)

	output.wantNoErr(t)
	want := port.DeleteDailyRecordInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		ContractID: mustContractID(t, testContractID),
		YearMonth:  mustYearMonth(t, 2026, 7),
		Date:       mustDate(t, 2026, 8, 1),
	}
	got := invoker.onlyCall(t)
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{}, workmonth.Date{})); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-6-d）", diff)
	}
}

// TestHandleDeleteDailyRecord_RejectsInvalidDateFormat は対象日が暦上存在しない・
// 書式不正の要求を弾くことを検証する（AC-9-6-c。契約 AC-4-8）。
func TestHandleDeleteDailyRecord_RejectsInvalidDateFormat(t *testing.T) {
	tests := []struct {
		name string
		date string
	}{
		{name: "暦上存在しない（2月30日）", date: "2026-02-30"},
		{name: "書式不正", date: "2026/07/01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.DeleteDailyRecordInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodDelete, deleteTarget(testContractID, "2026-07", tt.date), nil,
				actorHeaders(testActorID, string(port.RoleEngineer)),
				deletePaths(testContractID, "2026-07", tt.date)...,
			)

			controller.HandleDeleteDailyRecord(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-6-c）", err)
			}
			if errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("controller の識別子は workmonth.ErrInvalidValue を兼ねてはならない（AC-9-9-b・AC-11-13）: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleDeleteDailyRecord_RejectsWhenBothHeadersAbsent は更新系（契約 AC-9 順1
// の対象）で両ヘッダが不在の要求を弾くことを検証する（決定10・AC-9-7-a①）。
func TestHandleDeleteDailyRecord_RejectsWhenBothHeadersAbsent(t *testing.T) {
	invoker := &invokerSpy[port.DeleteDailyRecordInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodDelete, deleteTarget(testContractID, "2026-07", "2026-07-01"), nil, nil,
		deletePaths(testContractID, "2026-07", "2026-07-01")...,
	)

	controller.HandleDeleteDailyRecord(r, invoker, output)

	if err := output.onlyErr(t); !errors.Is(err, port.ErrUnauthenticated) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)（決定10）", err)
	}
	invoker.wantNoCall(t)
}

// TestHandleDeleteDailyRecord_IgnoresBody はボディを取らないエンドポイントで
// ボディを読まない・検査しないことを検証する（AC-9-6-h）。
func TestHandleDeleteDailyRecord_IgnoresBody(t *testing.T) {
	invoker := &invokerSpy[port.DeleteDailyRecordInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodDelete, deleteTarget(testContractID, "2026-07", "2026-07-01"), []byte("{not json"),
		actorHeaders(testActorID, string(port.RoleEngineer)),
		deletePaths(testContractID, "2026-07", "2026-07-01")...,
	)

	controller.HandleDeleteDailyRecord(r, invoker, output)

	output.wantNoErr(t)
	invoker.onlyCall(t)
}
