package controller_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: 実装設計 AC-9-5-h・AC-9-5-i・AC-9-6-l・AC-9-7-a①・
// AC-9-7-c・AC-12-19-c。契約 E-8・E-9・AC-12-15〜12-17・AC-12-19-a〜d・
// AC-13-4・AC-13-11・AC-13-13。

func contractsTarget(query string) string { return "/contracts?" + query }

func workTrendsTarget(query string) string { return "/work-trends?" + query }

// E-8 の妥当な要求の入力 DTO（AC-9-5-h）。
func wantListContractsInput(t *testing.T) port.ListContractsInput {
	t.Helper()
	return port.ListContractsInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		EngineerID: "eng-0001",
		YearMonth:  mustYearMonth(t, 2026, 10),
	}
}

var unexportedValueObjects = cmp.AllowUnexported(workmonth.YearMonth{})

// TestHandleListContracts_MapsInput は妥当な要求から port.ListContractsInput が
// ちょうど1回渡ることを検証する（AC-9-5-h・AC-12-19-c）。
func TestHandleListContracts_MapsInput(t *testing.T) {
	invoker := &invokerSpy[port.ListContractsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, contractsTarget("engineerId=eng-0001&yearMonth=2026-10"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListContracts(r, invoker, output)

	output.wantNoErr(t)
	if diff := cmp.Diff(wantListContractsInput(t), invoker.onlyCall(t), unexportedValueObjects); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-h）", diff)
	}
}

// TestHandleListContracts_DoesNotJudgeAuthorization は「Engineer かつ本人」を
// controller が判定しないことを検証する（AC-9-6-l 末尾・AC-8-12 は interactor の責務）。
// Approver・他人でも入力 DTO を組み立てて invoker を1回呼ぶ。
func TestHandleListContracts_DoesNotJudgeAuthorization(t *testing.T) {
	tests := []struct {
		name      string
		actorID   string
		actorRole port.Role
	}{
		{name: "Approver で識別子一致", actorID: "eng-0001", actorRole: port.RoleApprover},
		{name: "Engineer で識別子不一致", actorID: "eng-9999", actorRole: port.RoleEngineer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.ListContractsInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodGet, contractsTarget("engineerId=eng-0001&yearMonth=2026-10"), nil,
				actorHeaders(tt.actorID, string(tt.actorRole)),
			)

			controller.HandleListContracts(r, invoker, output)

			output.wantNoErr(t)
			want := wantListContractsInput(t)
			want.Actor = port.Actor{ID: tt.actorID, Role: tt.actorRole, Authenticated: true}
			if diff := cmp.Diff(want, invoker.onlyCall(t), unexportedValueObjects); diff != "" {
				t.Errorf("入力 DTO が不一致 (-want +got):\n%s（AC-9-6-l・AC-8-12）", diff)
			}
		})
	}
}

// TestHandleListContracts_IgnoresUnknownQuery は未知のクエリを無視することを
// 検証する（AC-9-6-l・契約 AC-12-17）。入力 DTO は未知のクエリが無い場合と同じ。
func TestHandleListContracts_IgnoresUnknownQuery(t *testing.T) {
	invoker := &invokerSpy[port.ListContractsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, contractsTarget("engineerId=eng-0001&yearMonth=2026-10&limit=1&offset=abc&state=Draft&contractId=ctr-9"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListContracts(r, invoker, output)

	output.wantNoErr(t)
	if diff := cmp.Diff(wantListContractsInput(t), invoker.onlyCall(t), unexportedValueObjects); diff != "" {
		t.Errorf("未知のクエリで入力 DTO が変わった (-want +got):\n%s（AC-12-17・AC-12-19-c）", diff)
	}
}

// TestHandleListContracts_EngineerIDIsNotFormatChecked は engineerId を文字列の
// まま渡し、書式を検査しないことを検証する（AC-9-5-h。E-2 と同じ）。
func TestHandleListContracts_EngineerIDIsNotFormatChecked(t *testing.T) {
	invoker := &invokerSpy[port.ListContractsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, contractsTarget("engineerId=a%20b%2Fc!&yearMonth=2026-10"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListContracts(r, invoker, output)

	output.wantNoErr(t)
	if got := invoker.onlyCall(t).EngineerID; got != "a b/c!" {
		t.Errorf("EngineerID = %q, want %q（文字列のまま。AC-9-5-h）", got, "a b/c!")
	}
}

// TestHandleListContracts_Rejects は AC-9-6-l の①〜④と AC-12-19-c の組を検証する。
// 弾いた要求では invoker が0回、期待する番兵が出力側へ渡る。
func TestHandleListContracts_Rejects(t *testing.T) {
	engineerHeaders := actorHeaders(testActorID, string(port.RoleEngineer))
	tests := []struct {
		name    string
		query   string
		headers map[string]string
		wantErr error
	}{
		{name: "① 両ヘッダ不在 → ErrUnauthenticated", query: "engineerId=eng-0001&yearMonth=2026-10", headers: nil, wantErr: port.ErrUnauthenticated},
		{name: "① 両ヘッダ不在かつ yearMonth=2026-13 → ErrUnauthenticated（契約 AC-12-19-a）", query: "engineerId=eng-0001&yearMonth=2026-13", headers: nil, wantErr: port.ErrUnauthenticated},
		{name: "① 両ヘッダ不在かつ engineerId なし → ErrUnauthenticated（契約 AC-12-19-b）", query: "yearMonth=2026-10", headers: nil, wantErr: port.ErrUnauthenticated},
		{name: "① 両ヘッダ不在かつ yearMonth なし → ErrUnauthenticated", query: "engineerId=eng-0001", headers: nil, wantErr: port.ErrUnauthenticated},
		{name: "② X-Actor-Id だけ → ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-10", headers: map[string]string{headerActorID: testActorID}, wantErr: port.ErrInvalidRequest},
		{name: "② X-Actor-Role だけ → ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-10", headers: map[string]string{headerActorRole: string(port.RoleEngineer)}, wantErr: port.ErrInvalidRequest},
		{name: "② ロール値が2値以外（Guest）→ ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-10", headers: actorHeaders(testActorID, "Guest"), wantErr: port.ErrInvalidRequest},
		{name: "② ロール値の大文字小文字違い → ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-10", headers: actorHeaders(testActorID, "engineer"), wantErr: port.ErrInvalidRequest},
		{name: "③ engineerId のキーが無い → ErrInvalidRequest", query: "yearMonth=2026-10", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "③ engineerId が空文字 → ErrInvalidRequest", query: "engineerId=&yearMonth=2026-10", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "④ yearMonth のキーが無い → ErrInvalidRequest", query: "engineerId=eng-0001", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "④ yearMonth が空文字 → ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "④ yearMonth=2026-13 → ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-13", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "④ yearMonth=2026-00 → ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-00", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "④ yearMonth=2026-7（0詰めなし）→ ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-7", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "④ yearMonth=2026-10-01（日付）→ ErrInvalidRequest", query: "engineerId=eng-0001&yearMonth=2026-10-01", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "12-19-c Approver かつ yearMonth=2026-13 → ErrInvalidRequest（400 が 403 より先）", query: "engineerId=eng-0001&yearMonth=2026-13", headers: actorHeaders(testActorID, string(port.RoleApprover)), wantErr: port.ErrInvalidRequest},
		{name: "12-19-d Engineer・識別子不一致かつ yearMonth なし → ErrInvalidRequest", query: "engineerId=eng-0001", headers: actorHeaders("eng-9999", string(port.RoleEngineer)), wantErr: port.ErrInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.ListContractsInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(http.MethodGet, contractsTarget(tt.query), nil, tt.headers)

			controller.HandleListContracts(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if tt.wantErr == port.ErrUnauthenticated && errors.Is(err, port.ErrInvalidRequest) {
				t.Errorf("UNAUTHENTICATED が INVALID_REQUEST も兼ねてはならない: %v", err)
			}
			if tt.wantErr == port.ErrInvalidRequest && errors.Is(err, workmonth.ErrInvalidValue) {
				t.Errorf("要求側の識別子が workmonth.ErrInvalidValue を兼ねてはならない（AC-9-9-b）: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleListWorkTrends_MapsInput は妥当な要求から port.ListWorkTrendsInput が
// ちょうど1回渡ることを検証する（AC-9-5-i・AC-12-19-c）。
func TestHandleListWorkTrends_MapsInput(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkTrendsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, workTrendsTarget("engineerId=eng-0001"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListWorkTrends(r, invoker, output)

	output.wantNoErr(t)
	want := port.ListWorkTrendsInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		EngineerID: "eng-0001",
	}
	if diff := cmp.Diff(want, invoker.onlyCall(t)); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-i）", diff)
	}
}

// TestHandleListWorkTrends_DoesNotJudgeAuthorization は controller が認可を
// 判定しないことを検証する（AC-9-6-l 末尾）。
func TestHandleListWorkTrends_DoesNotJudgeAuthorization(t *testing.T) {
	tests := []struct {
		name      string
		actorID   string
		actorRole port.Role
	}{
		{name: "Approver で識別子一致", actorID: "eng-0001", actorRole: port.RoleApprover},
		{name: "Engineer で識別子不一致", actorID: "eng-9999", actorRole: port.RoleEngineer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.ListWorkTrendsInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodGet, workTrendsTarget("engineerId=eng-0001"), nil,
				actorHeaders(tt.actorID, string(tt.actorRole)),
			)

			controller.HandleListWorkTrends(r, invoker, output)

			output.wantNoErr(t)
			want := port.ListWorkTrendsInput{
				Actor:      port.Actor{ID: tt.actorID, Role: tt.actorRole, Authenticated: true},
				EngineerID: "eng-0001",
			}
			if diff := cmp.Diff(want, invoker.onlyCall(t)); diff != "" {
				t.Errorf("入力 DTO が不一致 (-want +got):\n%s（AC-9-6-l・AC-8-12）", diff)
			}
		})
	}
}

// TestHandleListWorkTrends_IgnoresUnknownQuery は未知のクエリ（絞り込み・
// ページング・E-8 の yearMonth の不正値を含む）を読まずに無視することを
// 検証する（AC-9-6-l・契約 AC-13-4）。
func TestHandleListWorkTrends_IgnoresUnknownQuery(t *testing.T) {
	invoker := &invokerSpy[port.ListWorkTrendsInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodGet, workTrendsTarget("engineerId=eng-0001&contractId=ctr-1&from=2026-01&to=2026-13&yearMonth=2026-13&state=Draft&limit=0&offset=-1"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
	)

	controller.HandleListWorkTrends(r, invoker, output)

	output.wantNoErr(t)
	want := port.ListWorkTrendsInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		EngineerID: "eng-0001",
	}
	if diff := cmp.Diff(want, invoker.onlyCall(t)); diff != "" {
		t.Errorf("未知のクエリで入力 DTO が変わった (-want +got):\n%s（AC-13-4・AC-12-19-c）", diff)
	}
}

// TestHandleListWorkTrends_Rejects は AC-9-6-l の①〜③（E-9 に④は無い）と
// 契約 AC-13-13 の組を検証する。
func TestHandleListWorkTrends_Rejects(t *testing.T) {
	engineerHeaders := actorHeaders(testActorID, string(port.RoleEngineer))
	tests := []struct {
		name    string
		query   string
		headers map[string]string
		wantErr error
	}{
		{name: "① 両ヘッダ不在 → ErrUnauthenticated", query: "engineerId=eng-0001", headers: nil, wantErr: port.ErrUnauthenticated},
		{name: "① 両ヘッダ不在かつ engineerId なし → ErrUnauthenticated（契約 AC-13-13）", query: "", headers: nil, wantErr: port.ErrUnauthenticated},
		{name: "② X-Actor-Id だけ → ErrInvalidRequest", query: "engineerId=eng-0001", headers: map[string]string{headerActorID: testActorID}, wantErr: port.ErrInvalidRequest},
		{name: "② X-Actor-Role だけ → ErrInvalidRequest", query: "engineerId=eng-0001", headers: map[string]string{headerActorRole: string(port.RoleEngineer)}, wantErr: port.ErrInvalidRequest},
		{name: "② ロール値が2値以外（Guest）→ ErrInvalidRequest", query: "engineerId=eng-0001", headers: actorHeaders(testActorID, "Guest"), wantErr: port.ErrInvalidRequest},
		{name: "③ engineerId のキーが無い → ErrInvalidRequest", query: "", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "③ engineerId が空文字 → ErrInvalidRequest", query: "engineerId=", headers: engineerHeaders, wantErr: port.ErrInvalidRequest},
		{name: "13-13 Approver かつ engineerId なし → ErrInvalidRequest（400 が 403 より先）", query: "", headers: actorHeaders(testActorID, string(port.RoleApprover)), wantErr: port.ErrInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.ListWorkTrendsInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(http.MethodGet, workTrendsTarget(tt.query), nil, tt.headers)

			controller.HandleListWorkTrends(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if tt.wantErr == port.ErrUnauthenticated && errors.Is(err, port.ErrInvalidRequest) {
				t.Errorf("UNAUTHENTICATED が INVALID_REQUEST も兼ねてはならない: %v", err)
			}
			if tt.wantErr == port.ErrInvalidRequest && errors.Is(err, workmonth.ErrInvalidValue) {
				t.Errorf("要求側の識別子が workmonth.ErrInvalidValue を兼ねてはならない（AC-9-9-b）: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}
