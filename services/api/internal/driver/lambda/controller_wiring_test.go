package lambda_test

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-15③
// （AC-10-8②「リクエストごとの結線」の境界）のうち、**その http.Handler が
// 自分のエンドポイントの controller を実際に呼ぶこと**だけを固定する薄い対を、
// E-4（削除）・E-5（締め）・E-6（承認）・E-7（差戻し）について置く。
//
// 往復2 のレビューで、この4本は「controller を呼ばない」実装へ変異させても
// どのテストも Red にならない（偽 Green）ことが実測された。router_test は
// 経路の spy 止まりで controller まで届かず、assembly_test はこの4本を
// noopHandler で埋めているためである。
//
// 共通部（リクエストごとの presenter 生成＝AC-9-13-a、結果なし →
// INTERNAL_ERROR の委譲＝AC-9-13-c）は newWorkMonthHandler に一本化されており、
// E-1・E-3 の対（request_wiring_test.go）が既に固定しているため、ここでは
// 再検査しない。ここで固定するのは「controller を経由して invoker が
// 呼ばれ、要求から取り出した値が入力 DTO に載ること」だけである。
//
// なお「別のエンドポイントの controller を呼ぶ」取り違えは、invoker の
// interface（Execute が取る入力 DTO の型）がエンドポイントごとに異なるため
// コンパイルが通らず、テストを待たずに落ちる。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// recordingWorkMonthInvoker は Execute が受け取った入力 DTO を記録し、必ず
// output.Present を呼ぶ手書きの invoker（AC-12-2）。入力 DTO の型を型引数に
// 取ることで、勤務月1件を返す4エンドポイント（E-4〜E-7）で共有する。
type recordingWorkMonthInvoker[Input any] struct {
	output   port.WorkMonthOutputPort
	recorded *[]Input
}

func (s recordingWorkMonthInvoker[Input]) Execute(_ context.Context, input Input) {
	*s.recorded = append(*s.recorded, input)
	s.output.Present(port.WorkMonthOutput{
		ContractID:   "ctr-0001",
		YearMonth:    "2026-07",
		State:        "Draft",
		DailyRecords: []port.DailyRecordOutput{},
	})
}

// newActionRequest は E-4〜E-7 相当のリクエストを組み立てる。ルーティング（①）は
// 経由しないため、パス変数は r.SetPathValue で直接与える（AC-10-8②は①から
// 独立して呼び出せることを要求する）。
func newActionRequest(method, path, role string, pathValues map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("X-Actor-Id", "act-0001")
	r.Header.Set("X-Actor-Role", role)
	for name, value := range pathValues {
		r.SetPathValue(name, value)
	}
	return r
}

// assertControllerInvokedOnce は「controller を経由して invoker がちょうど
// 1回呼ばれ、応答が 200 になった」ことを検査する。handler が controller を
// 呼ばなければ invoker も呼ばれず、presenter が「結果なし」のままとなって
// 応答は INTERNAL_ERROR になる（AC-9-13-c）ため、両方が Red になる。
func assertControllerInvokedOnce(t *testing.T, rec *httptest.ResponseRecorder, calls int) {
	t.Helper()
	if calls != 1 {
		t.Fatalf("invoker.Execute の呼び出し回数 = %d, want 1（handler が controller を呼んでいない）: body=%s", calls, rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}
}

// TestDeleteDailyRecordHandler_CallsController は E-4 の handler が
// controller.HandleDeleteDailyRecord を呼び、パス変数が入力 DTO へ載ることを
// 固定する（W-2）。
func TestDeleteDailyRecordHandler_CallsController(t *testing.T) {
	var got []port.DeleteDailyRecordInput
	handler := lambda.NewDeleteDailyRecordHandler(func(output port.WorkMonthOutputPort) lambda.DeleteDailyRecordInvoker {
		return recordingWorkMonthInvoker[port.DeleteDailyRecordInput]{output: output, recorded: &got}
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newActionRequest(
		http.MethodDelete,
		"/work-months/ctr-e4/2026-07/daily-records/2026-07-03",
		"Engineer",
		map[string]string{"contractId": "ctr-e4", "yearMonth": "2026-07", "date": "2026-07-03"},
	))

	assertControllerInvokedOnce(t, rec, len(got))
	input := got[0]
	if input.ContractID.String() != "ctr-e4" {
		t.Errorf("入力 DTO の ContractID = %q, want %q", input.ContractID.String(), "ctr-e4")
	}
	if input.YearMonth.Year() != 2026 || input.YearMonth.Month() != 7 {
		t.Errorf("入力 DTO の YearMonth = %d-%d, want 2026-7", input.YearMonth.Year(), input.YearMonth.Month())
	}
	if input.Date.Day() != 3 {
		t.Errorf("入力 DTO の Date の日 = %d, want 3", input.Date.Day())
	}
	if input.Actor.ID != "act-0001" {
		t.Errorf("入力 DTO の Actor.ID = %q, want %q", input.Actor.ID, "act-0001")
	}
}

// TestCloseWorkMonthHandler_CallsController は E-5 の handler が
// controller.HandleCloseWorkMonth を呼ぶことを固定する（W-2）。
func TestCloseWorkMonthHandler_CallsController(t *testing.T) {
	var got []port.CloseWorkMonthInput
	handler := lambda.NewCloseWorkMonthHandler(func(output port.WorkMonthOutputPort) lambda.CloseWorkMonthInvoker {
		return recordingWorkMonthInvoker[port.CloseWorkMonthInput]{output: output, recorded: &got}
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newActionRequest(
		http.MethodPost,
		"/work-months/ctr-e5/2026-08/close",
		"Engineer",
		map[string]string{"contractId": "ctr-e5", "yearMonth": "2026-08"},
	))

	assertControllerInvokedOnce(t, rec, len(got))
	input := got[0]
	if input.ContractID.String() != "ctr-e5" {
		t.Errorf("入力 DTO の ContractID = %q, want %q", input.ContractID.String(), "ctr-e5")
	}
	if input.YearMonth.Year() != 2026 || input.YearMonth.Month() != 8 {
		t.Errorf("入力 DTO の YearMonth = %d-%d, want 2026-8", input.YearMonth.Year(), input.YearMonth.Month())
	}
	if input.Actor.ID != "act-0001" {
		t.Errorf("入力 DTO の Actor.ID = %q, want %q", input.Actor.ID, "act-0001")
	}
}

// TestApproveWorkMonthHandler_CallsController は E-6 の handler が
// controller.HandleApproveWorkMonth を呼ぶことを固定する（W-2）。
func TestApproveWorkMonthHandler_CallsController(t *testing.T) {
	var got []port.ApproveWorkMonthInput
	handler := lambda.NewApproveWorkMonthHandler(func(output port.WorkMonthOutputPort) lambda.ApproveWorkMonthInvoker {
		return recordingWorkMonthInvoker[port.ApproveWorkMonthInput]{output: output, recorded: &got}
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newActionRequest(
		http.MethodPost,
		"/work-months/ctr-e6/2026-09/approve",
		"Approver",
		map[string]string{"contractId": "ctr-e6", "yearMonth": "2026-09"},
	))

	assertControllerInvokedOnce(t, rec, len(got))
	input := got[0]
	if input.ContractID.String() != "ctr-e6" {
		t.Errorf("入力 DTO の ContractID = %q, want %q", input.ContractID.String(), "ctr-e6")
	}
	if input.YearMonth.Year() != 2026 || input.YearMonth.Month() != 9 {
		t.Errorf("入力 DTO の YearMonth = %d-%d, want 2026-9", input.YearMonth.Year(), input.YearMonth.Month())
	}
	if input.Actor.ID != "act-0001" {
		t.Errorf("入力 DTO の Actor.ID = %q, want %q", input.Actor.ID, "act-0001")
	}
}

// TestRejectWorkMonthHandler_CallsController は E-7 の handler が
// controller.HandleRejectWorkMonth を呼ぶことを固定する（W-2）。
func TestRejectWorkMonthHandler_CallsController(t *testing.T) {
	var got []port.RejectWorkMonthInput
	handler := lambda.NewRejectWorkMonthHandler(func(output port.WorkMonthOutputPort) lambda.RejectWorkMonthInvoker {
		return recordingWorkMonthInvoker[port.RejectWorkMonthInput]{output: output, recorded: &got}
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newActionRequest(
		http.MethodPost,
		"/work-months/ctr-e7/2026-10/reject",
		"Approver",
		map[string]string{"contractId": "ctr-e7", "yearMonth": "2026-10"},
	))

	assertControllerInvokedOnce(t, rec, len(got))
	input := got[0]
	if input.ContractID.String() != "ctr-e7" {
		t.Errorf("入力 DTO の ContractID = %q, want %q", input.ContractID.String(), "ctr-e7")
	}
	if input.YearMonth.Year() != 2026 || input.YearMonth.Month() != 10 {
		t.Errorf("入力 DTO の YearMonth = %d-%d, want 2026-10", input.YearMonth.Year(), input.YearMonth.Month())
	}
	if input.Actor.ID != "act-0001" {
		t.Errorf("入力 DTO の Actor.ID = %q, want %q", input.Actor.ID, "act-0001")
	}
}
