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

// 検証対象の受け入れ条件: AC-9-5-a・AC-9-6-a・AC-9-6-b・AC-9-7（a②・c・d）・
// AC-9-8・AC-12-9。契約 E-1・AC-1-6。

// getTarget は httptest.NewRequest 用の URL を組み立てる（closeTarget と同じ
// 理由でエスケープする）。
func getTarget(contractID, yearMonth string) string {
	return "/work-months/" + url.PathEscape(contractID) + "/" + url.PathEscape(yearMonth)
}

func getPaths(contractID, yearMonth string) []pathValue {
	return []pathValue{{name: "contractId", value: contractID}, {name: "yearMonth", value: yearMonth}}
}

// TestHandleGetWorkMonth_MapsInput は妥当な要求から port.GetWorkMonthInput
// （AC-9-5-a）がちょうど1回 invoker へ渡ることを検証する。
func TestHandleGetWorkMonth_MapsInput(t *testing.T) {
	invoker := &invokerSpy[port.GetWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, getTarget(testContractID, "2026-07"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
		getPaths(testContractID, "2026-07")...,
	)

	controller.HandleGetWorkMonth(r, invoker, output)

	output.wantNoErr(t)
	want := port.GetWorkMonthInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		ContractID: mustContractID(t, testContractID),
		YearMonth:  mustYearMonth(t, 2026, 7),
	}
	got := invoker.onlyCall(t)
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{})); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-a）", diff)
	}
}

// TestHandleGetWorkMonth_AllowsGhostWhenHeadersAbsent は参照系（E-1）で両ヘッダが
// 不在でも弾かず、未認証の Actor を組み立てて渡すことを検証する
// （AC-9-7-a②・AC-9-7-d・契約 AC-1-6・AC-12-9）。
func TestHandleGetWorkMonth_AllowsGhostWhenHeadersAbsent(t *testing.T) {
	invoker := &invokerSpy[port.GetWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(http.MethodGet, getTarget(testContractID, "2026-07"), nil, nil, getPaths(testContractID, "2026-07")...)

	controller.HandleGetWorkMonth(r, invoker, output)

	output.wantNoErr(t)
	got := invoker.onlyCall(t)
	want := port.Actor{ID: "", Role: "", Authenticated: false}
	if diff := cmp.Diff(want, got.Actor); diff != "" {
		t.Errorf("Actor が不一致 (-want +got):\n%s（AC-9-7-a②。Guest というロール値を作らないこと＝D-9）", diff)
	}
}

// TestHandleGetWorkMonth_RejectsPartialActorHeader は操作者ヘッダが片方だけ、
// またはロール値が2値以外の要求を参照系でも構文不正として弾くことを検証する
// （AC-9-7-c。片方欠落と「両方不在」＝AC-9-7-a②は異なる扱い）。
func TestHandleGetWorkMonth_RejectsPartialActorHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{name: "X-Actor-Id のみ", headers: map[string]string{headerActorID: testActorID}},
		{name: "X-Actor-Role のみ", headers: map[string]string{headerActorRole: string(port.RoleEngineer)}},
		{name: "ロール値が2値以外", headers: actorHeaders(testActorID, "Guest")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.GetWorkMonthInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(http.MethodGet, getTarget(testContractID, "2026-07"), nil, tt.headers, getPaths(testContractID, "2026-07")...)

			controller.HandleGetWorkMonth(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-7-c）", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleGetWorkMonth_RejectsInvalidContractIDFormat は契約識別子の書式不正を
// 弾くことを検証する（AC-9-6-a）。
func TestHandleGetWorkMonth_RejectsInvalidContractIDFormat(t *testing.T) {
	tests := []struct {
		name       string
		contractID string
	}{
		{name: "空文字", contractID: ""},
		{name: "許可されない文字（スペース）", contractID: "ctr 0001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.GetWorkMonthInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodGet, getTarget(tt.contractID, "2026-07"), nil,
				actorHeaders(testActorID, string(port.RoleEngineer)),
				getPaths(tt.contractID, "2026-07")...,
			)

			controller.HandleGetWorkMonth(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-6-a）", err)
			}
			if errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("controller の識別子は workmonth.ErrInvalidValue を兼ねてはならない（AC-9-9-b・AC-11-13）: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleGetWorkMonth_RejectsInvalidYearMonthFormat は年月の書式不正を弾くことを
// 検証する（AC-9-6-b）。
func TestHandleGetWorkMonth_RejectsInvalidYearMonthFormat(t *testing.T) {
	tests := []struct {
		name      string
		yearMonth string
	}{
		{name: "月が範囲外", yearMonth: "2026-13"},
		{name: "区切り文字が違う", yearMonth: "2026/07"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.GetWorkMonthInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodGet, getTarget(testContractID, tt.yearMonth), nil,
				actorHeaders(testActorID, string(port.RoleEngineer)),
				getPaths(testContractID, tt.yearMonth)...,
			)

			controller.HandleGetWorkMonth(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-6-b）", err)
			}
			if errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("controller の識別子は workmonth.ErrInvalidValue を兼ねてはならない（AC-9-9-b・AC-11-13）: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}
