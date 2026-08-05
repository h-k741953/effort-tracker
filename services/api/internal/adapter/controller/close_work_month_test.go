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

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-9-5-e（入力 DTO への写し）・
//     AC-9-6（弾くもの／弾かないもの）・AC-9-7（操作者ヘッダ → port.Actor）・
//     AC-9-8（interactor の呼び出し）・AC-12-5（httptest・Lambda を起動しない）・
//     AC-12-9（controller のテスト）
//   - docs/specs/domain-api-http-contract.md E-5・AC-1-4〜AC-1-6・AC-6-8
//   - 決定10（操作者ヘッダ不在と構文検査が同時に該当する場合の順序）

// closeTarget は httptest.NewRequest 用の URL を組み立てる。httptest.NewRequest は
// target 文字列を生の HTTP リクエスト行として解釈するため、契約識別子・年月の
// 書式不正値（空白・スラッシュ等）はエスケープする。実際にハンドラへ渡る値は
// r.SetPathValue で別途与える生の値（closePaths）であり、この URL 文字列の
// 中身そのものは検査の対象ではない。
func closeTarget(contractID, yearMonth string) string {
	return "/work-months/" + url.PathEscape(contractID) + "/" + url.PathEscape(yearMonth) + "/close"
}

func closePaths(contractID, yearMonth string) []pathValue {
	return []pathValue{{name: "contractId", value: contractID}, {name: "yearMonth", value: yearMonth}}
}

// TestHandleCloseWorkMonth_MapsInput は妥当な要求から
// port.CloseWorkMonthInput（AC-9-5-e）がちょうど1回 invoker へ渡ることを検証する。
func TestHandleCloseWorkMonth_MapsInput(t *testing.T) {
	invoker := &invokerSpy[port.CloseWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodPost, closeTarget(testContractID, "2026-07"), nil,
		actorHeaders(testActorID, string(port.RoleEngineer)),
		closePaths(testContractID, "2026-07")...,
	)

	controller.HandleCloseWorkMonth(r, invoker, output)

	output.wantNoErr(t)
	want := port.CloseWorkMonthInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		ContractID: mustContractID(t, testContractID),
		YearMonth:  mustYearMonth(t, 2026, 7),
	}
	got := invoker.onlyCall(t)
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{})); diff != "" {
		t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-e）", diff)
	}
}

// TestHandleCloseWorkMonth_RejectsWhenBothHeadersAbsent は更新系（契約 AC-9 順1 の
// 対象）で両ヘッダが不在の要求を、構文が妥当であっても弾くことを検証する
// （決定10・AC-9-7-a①）。
func TestHandleCloseWorkMonth_RejectsWhenBothHeadersAbsent(t *testing.T) {
	invoker := &invokerSpy[port.CloseWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(http.MethodPost, closeTarget(testContractID, "2026-07"), nil, nil, closePaths(testContractID, "2026-07")...)

	controller.HandleCloseWorkMonth(r, invoker, output)

	if err := output.onlyErr(t); !errors.Is(err, port.ErrUnauthenticated) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)（決定10）", err)
	}
	invoker.wantNoCall(t)
}

// TestHandleCloseWorkMonth_RejectsPartialOrInvalidActorHeader は操作者ヘッダが
// 片方だけ、またはロール値が2値以外の要求を要求の構文不正として弾くことを検証する
// （AC-9-7-c）。「両方不在」（決定10）ではないため port.ErrUnauthenticated ではなく
// controller.ErrInvalidRequest が渡る。
func TestHandleCloseWorkMonth_RejectsPartialOrInvalidActorHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{name: "X-Actor-Id のみ（AC-9-7-c）", headers: map[string]string{headerActorID: testActorID}},
		{name: "X-Actor-Role のみ（AC-9-7-c）", headers: map[string]string{headerActorRole: string(port.RoleEngineer)}},
		{name: "ロール値が2値以外（AC-1-5）", headers: actorHeaders(testActorID, "Guest")},
		{name: "ロール値の大文字小文字が異なる（AC-1-5・AC-9-7-c）", headers: actorHeaders(testActorID, "engineer")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.CloseWorkMonthInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(http.MethodPost, closeTarget(testContractID, "2026-07"), nil, tt.headers, closePaths(testContractID, "2026-07")...)

			controller.HandleCloseWorkMonth(r, invoker, output)

			err := output.onlyErr(t)
			if errors.Is(err, port.ErrUnauthenticated) {
				t.Fatalf("片方だけ／不正なロール値は UNAUTHENTICATED ではなく INVALID_REQUEST（AC-9-7-c）: %v", err)
			}
			if !errors.Is(err, controller.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, controller.ErrInvalidRequest)", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleCloseWorkMonth_RejectsInvalidContractIDFormat は契約 AC-1-10 の書式に
// 適合しない契約識別子を弾くことを検証する（AC-9-6-a）。
// workmonth.NewContractID は空文字しか弾かないため、書式の検査は controller が持つ。
func TestHandleCloseWorkMonth_RejectsInvalidContractIDFormat(t *testing.T) {
	tests := []struct {
		name       string
		contractID string
	}{
		{name: "空文字", contractID: ""},
		{name: "許可されない文字（スペース）", contractID: "ctr 0001"},
		{name: "許可されない文字（スラッシュ）", contractID: "ctr/0001"},
		{name: "65文字（上限超過）", contractID: repeatChar('a', 65)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.CloseWorkMonthInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodPost, closeTarget(tt.contractID, "2026-07"), nil,
				actorHeaders(testActorID, string(port.RoleEngineer)),
				closePaths(tt.contractID, "2026-07")...,
			)

			controller.HandleCloseWorkMonth(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, controller.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, controller.ErrInvalidRequest)（AC-9-6-a）", err)
			}
			if errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("controller の識別子は workmonth.ErrInvalidValue を兼ねてはならない（AC-9-9-b・AC-11-13）: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleCloseWorkMonth_RejectsInvalidYearMonthFormat は年月の構築失敗・書式
// 不一致を弾くことを検証する（AC-9-6-b）。domain の workmonth.ErrInvalidValue を
// そのまま出力側へ渡さず、controller.ErrInvalidRequest へ変換することを併せて
// 確認する（AC-9-9-b・AC-9-9-c・AC-11-13）。
func TestHandleCloseWorkMonth_RejectsInvalidYearMonthFormat(t *testing.T) {
	tests := []struct {
		name      string
		yearMonth string
	}{
		{name: "月が範囲外（契約 AC-2-4）", yearMonth: "2026-13"},
		{name: "区切り文字が違う", yearMonth: "2026/07"},
		{name: "数値でない", yearMonth: "abcd-ef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.CloseWorkMonthInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodPost, closeTarget(testContractID, tt.yearMonth), nil,
				actorHeaders(testActorID, string(port.RoleEngineer)),
				closePaths(testContractID, tt.yearMonth)...,
			)

			controller.HandleCloseWorkMonth(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, controller.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, controller.ErrInvalidRequest)（AC-9-6-b）", err)
			}
			if errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("controller の識別子は workmonth.ErrInvalidValue を兼ねてはならない（AC-9-9-b・AC-11-13）: %v", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleCloseWorkMonth_HeaderAbsenceOrderedBeforeSyntaxCheck は決定10の核心
// （操作者ヘッダの有無の判定を構文検査より前に置く）を固定する。
// 「両方不在 かつ 構文不正」「片方（ヘッダ不在）だけ該当」「片方（構文不正）だけ
// 該当」を対にして置く（AC-12-9「対にしないと『ヘッダの有無が構文検査より先』が
// 観測されない」）。
func TestHandleCloseWorkMonth_HeaderAbsenceOrderedBeforeSyntaxCheck(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		yearMonth  string
		wantErr    error
		wantNotErr error // 併せて「これではない」ことも確認する
	}{
		{
			name:       "両方不在 かつ 年月の書式不正 → UNAUTHENTICATED が先（決定10）",
			headers:    nil,
			yearMonth:  "2026-13",
			wantErr:    port.ErrUnauthenticated,
			wantNotErr: controller.ErrInvalidRequest,
		},
		{
			name:       "ヘッダ不在のみ（年月は妥当） → UNAUTHENTICATED",
			headers:    nil,
			yearMonth:  "2026-07",
			wantErr:    port.ErrUnauthenticated,
			wantNotErr: controller.ErrInvalidRequest,
		},
		{
			name:       "構文不正のみ（ヘッダは妥当） → INVALID_REQUEST",
			headers:    actorHeaders(testActorID, string(port.RoleEngineer)),
			yearMonth:  "2026-13",
			wantErr:    controller.ErrInvalidRequest,
			wantNotErr: port.ErrUnauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.CloseWorkMonthInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodPost, closeTarget(testContractID, tt.yearMonth), nil, tt.headers,
				closePaths(testContractID, tt.yearMonth)...,
			)

			controller.HandleCloseWorkMonth(r, invoker, output)

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

// TestHandleCloseWorkMonth_IgnoresBody はボディを取らないエンドポイントで
// ボディを読まない・検査しないことを検証する（AC-9-6-h）。送られた壊れた JSON でも
// 弾かれず、妥当な入力 DTO が渡ることを確認する。
func TestHandleCloseWorkMonth_IgnoresBody(t *testing.T) {
	invoker := &invokerSpy[port.CloseWorkMonthInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodPost, closeTarget(testContractID, "2026-07"), []byte("{not json"),
		actorHeaders(testActorID, string(port.RoleEngineer)),
		closePaths(testContractID, "2026-07")...,
	)

	controller.HandleCloseWorkMonth(r, invoker, output)

	output.wantNoErr(t)
	invoker.onlyCall(t)
}

func repeatChar(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
