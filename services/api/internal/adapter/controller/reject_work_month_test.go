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

// 検証対象の受け入れ条件: AC-9-5-g・AC-9-6-h・AC-9-7-a①・AC-9-8・AC-12-9・決定10。
// 判定順序・書式検査など横断的な検査の全体像は close_work_month_test.go が固定し、
// 本ファイルは E-7 固有の写しと決定10の該当を確認する。

// rejectTarget は httptest.NewRequest 用の URL を組み立てる（closeTarget と同じ
// 理由でエスケープする）。
func rejectTarget(contractID, yearMonth string) string {
	return "/work-months/" + url.PathEscape(contractID) + "/" + url.PathEscape(yearMonth) + "/reject"
}

func rejectPaths(contractID, yearMonth string) []pathValue {
	return []pathValue{{name: "contractId", value: contractID}, {name: "yearMonth", value: yearMonth}}
}

// TestHandleRejectWorkMonth_MapsInput は妥当な要求から
// port.RejectWorkMonthInput（AC-9-5-g）がちょうど1回 invoker へ渡ることを検証する。
func TestHandleRejectWorkMonth_MapsInput(t *testing.T) {
	invoker := &invokerSpy[port.RejectWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodPost, rejectTarget(testContractID, "2026-07"), nil,
		actorHeaders(testActorID, string(port.RoleApprover)),
		rejectPaths(testContractID, "2026-07")...,
	)

	controller.HandleRejectWorkMonth(r, invoker, output)

	output.wantNoErr(t)
	want := port.RejectWorkMonthInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleApprover, Authenticated: true},
		ContractID: mustContractID(t, testContractID),
		YearMonth:  mustYearMonth(t, 2026, 7),
	}
	got := invoker.onlyCall(t)
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{})); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-g）", diff)
	}
}

// TestHandleRejectWorkMonth_RejectsWhenBothHeadersAbsent は更新系（契約 AC-9 順1 の
// 対象）で両ヘッダが不在の要求を弾くことを検証する（決定10・AC-9-7-a①）。
func TestHandleRejectWorkMonth_RejectsWhenBothHeadersAbsent(t *testing.T) {
	invoker := &invokerSpy[port.RejectWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(http.MethodPost, rejectTarget(testContractID, "2026-07"), nil, nil, rejectPaths(testContractID, "2026-07")...)

	controller.HandleRejectWorkMonth(r, invoker, output)

	if err := output.onlyErr(t); !errors.Is(err, port.ErrUnauthenticated) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)（決定10）", err)
	}
	invoker.wantNoCall(t)
}

// TestHandleRejectWorkMonth_IgnoresBody はボディを取らないエンドポイントで
// ボディを読まない・検査しないことを検証する（AC-9-6-h）。
func TestHandleRejectWorkMonth_IgnoresBody(t *testing.T) {
	invoker := &invokerSpy[port.RejectWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodPost, rejectTarget(testContractID, "2026-07"), []byte("{not json"),
		actorHeaders(testActorID, string(port.RoleApprover)),
		rejectPaths(testContractID, "2026-07")...,
	)

	controller.HandleRejectWorkMonth(r, invoker, output)

	output.wantNoErr(t)
	invoker.onlyCall(t)
}
