package controller_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-5-b・AC-9-6-j・AC-9-6-k・AC-9-7（a①・a②・c・d）・
// AC-9-8・AC-12-9・決定10。契約 E-2・AC-1-6・AC-3-1〜AC-3-3・AC-3-5・AC-3-6。

func listTarget(query string) string {
	return "/work-months?" + query
}

// TestHandleListWorkMonths_MapsInputWithEngineerID は engineerId を指定した
// 妥当な要求から port.ListWorkMonthsInput（AC-9-5-b）がちょうど1回渡ることを
// 検証する（契約 AC-3-1）。
func TestHandleListWorkMonths_MapsInputWithEngineerID(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkMonthsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, listTarget("engineerId=eng-0001&state=Approved&limit=10&offset=5"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListWorkMonths(r, invoker, output)

	output.wantNoErr(t)
	want := port.ListWorkMonthsInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		EngineerID: "eng-0001",
		State:      "Approved",
		Limit:      10,
		Offset:     5,
	}
	got := invoker.onlyCall(t)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-b）", diff)
	}
}

// TestHandleListWorkMonths_AllowsGhostWhenEngineerIDSpecified は engineerId を
// 指定した要求で両ヘッダが不在でも弾かず、未認証の Actor を渡すことを検証する
// （AC-9-7-a②・AC-9-7-d・契約 AC-1-6・AC-12-9）。
func TestHandleListWorkMonths_AllowsGhostWhenEngineerIDSpecified(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkMonthsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(http.MethodGet, listTarget("engineerId=eng-0001"), nil, nil)

	controller.HandleListWorkMonths(r, invoker, output)

	output.wantNoErr(t)
	got := invoker.onlyCall(t)
	want := port.Actor{ID: "", Role: "", Authenticated: false}
	if diff := cmp.Diff(want, got.Actor); diff != "" {
		t.Errorf("Actor が不一致 (-want +got):\n%s（AC-9-7-a②）", diff)
	}
	if got.EngineerID != "eng-0001" {
		t.Errorf("EngineerID = %q, want %q", got.EngineerID, "eng-0001")
	}
}

// TestHandleListWorkMonths_PassesThroughEngineerRoleForPendingApproval は
// 承認待ち一覧（engineerId 省略・state=PendingApproval）で操作者ヘッダが揃っていれば
// ロールが Engineer でも controller は弾かず invoker へ渡すことを検証する
// （AC-9-6-j「承認待ち一覧のロール要求は判定しない」＝AC-8-10 の責務）。
func TestHandleListWorkMonths_PassesThroughEngineerRoleForPendingApproval(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkMonthsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, listTarget("state=PendingApproval"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListWorkMonths(r, invoker, output)

	output.wantNoErr(t)
	got := invoker.onlyCall(t)
	if got.Actor.Role != port.RoleEngineer {
		t.Errorf("Actor.Role = %q, want %q（ロール要求は interactor の責務。AC-9-6-j・AC-8-10）", got.Actor.Role, port.RoleEngineer)
	}
	if got.State != "PendingApproval" || got.EngineerID != "" {
		t.Errorf("State/EngineerID = %q/%q, want %q/%q", got.State, got.EngineerID, "PendingApproval", "")
	}
}

// TestHandleListWorkMonths_RejectsWhenEngineerIDOmittedAndBothHeadersAbsent は
// 承認待ち一覧（契約 AC-9 順1 の対象）で両ヘッダが不在の要求を弾くことを検証する
// （決定10・契約 AC-3-2）。state の妥当性に関わらず UNAUTHENTICATED が先に立つ。
func TestHandleListWorkMonths_RejectsWhenEngineerIDOmittedAndBothHeadersAbsent(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "state が妥当（PendingApproval）", query: "state=PendingApproval"},
		{name: "state が省略（本来なら AC-3-3 で INVALID_REQUEST）", query: ""},
		{name: "state が不正値（本来なら AC-3-3 で INVALID_REQUEST）", query: "state=Bogus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.ListWorkMonthsInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(http.MethodGet, listTarget(tt.query), nil, nil)

			controller.HandleListWorkMonths(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrUnauthenticated) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)（決定10）", err)
			}
			if errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("UNAUTHENTICATED であるべきところで INVALID_REQUEST も兼ねてはならない: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleListWorkMonths_RejectsInvalidStateCombinationWhenEngineerIDOmitted は
// engineerId を省略し state が PendingApproval 以外（省略含む）の要求を、ヘッダが
// 揃っている場合に弾くことを検証する（契約 AC-3-3・AC-9-6-j）。
func TestHandleListWorkMonths_RejectsInvalidStateCombinationWhenEngineerIDOmitted(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "state が省略", query: ""},
		{name: "state が Draft", query: "state=Draft"},
		{name: "state が Approved", query: "state=Approved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.ListWorkMonthsInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodGet, listTarget(tt.query), nil,
				actorHeaders(testActorID, string(port.RoleApprover)),
			)

			controller.HandleListWorkMonths(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（契約 AC-3-3）", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleListWorkMonths_RejectsInvalidLimitOrOffset は limit・offset が整数でない
// ／範囲外の要求を弾くことを検証する（契約 AC-3-6）。
func TestHandleListWorkMonths_RejectsInvalidLimitOrOffset(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "limit が0", query: "engineerId=eng-0001&limit=0"},
		{name: "limit が負", query: "engineerId=eng-0001&limit=-1"},
		{name: "limit が整数でない", query: "engineerId=eng-0001&limit=abc"},
		{name: "offset が負", query: "engineerId=eng-0001&offset=-1"},
		{name: "offset が整数でない", query: "engineerId=eng-0001&offset=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.ListWorkMonthsInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodGet, listTarget(tt.query), nil,
				actorHeaders(testActorID, string(port.RoleEngineer)),
			)

			controller.HandleListWorkMonths(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（契約 AC-3-6）", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleListWorkMonths_RejectsInvalidStateValue は state が3値
// （Draft/PendingApproval/Approved）以外の要求を弾くことを検証する（AC-9-6-j。
// engineerId 指定時も対象）。
func TestHandleListWorkMonths_RejectsInvalidStateValue(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkMonthsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, listTarget("engineerId=eng-0001&state=Bogus"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListWorkMonths(r, invoker, output)

	err := output.onlyErr(t)
	if !errors.Is(err, port.ErrInvalidRequest) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-6-j）", err)
	}
	invoker.wantNoCall(t)
}

// TestHandleListWorkMonths_AppliesDefaultLimitWhenOmitted は limit 省略時に
// controller.DefaultListLimit を適用し、入力 DTO へ載せることを検証する
// （AC-9-6-k）。既定値そのものはリテラルで期待せず、公開定数を参照する（AC-13-16）。
func TestHandleListWorkMonths_AppliesDefaultLimitWhenOmitted(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkMonthsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, listTarget("engineerId=eng-0001"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListWorkMonths(r, invoker, output)

	output.wantNoErr(t)
	got := invoker.onlyCall(t)
	if got.Limit != controller.DefaultListLimit {
		t.Errorf("Limit = %d, want controller.DefaultListLimit(%d)（AC-9-6-k）", got.Limit, controller.DefaultListLimit)
	}
}

// TestHandleListWorkMonths_RejectsPartialActorHeader は操作者ヘッダが片方だけ、
// またはロール値が2値以外の要求を、engineerId 指定時（参照系）でも構文不正として
// 弾くことを検証する（AC-9-7-c）。
func TestHandleListWorkMonths_RejectsPartialActorHeader(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkMonthsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, listTarget("engineerId=eng-0001"), nil,
		map[string]string{headerActorID: testActorID},
	)

	controller.HandleListWorkMonths(r, invoker, output)

	err := output.onlyErr(t)
	if !errors.Is(err, port.ErrInvalidRequest) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-7-c）", err)
	}
	invoker.wantNoCall(t)
}
