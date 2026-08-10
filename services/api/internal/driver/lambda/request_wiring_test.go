package lambda_test

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-15③
// （AC-10-8②「リクエストごとの結線」の境界。AC-9-13-a〜c）。
//
// lambda.NewGetWorkMonthHandler は「出力ポート（presenter）を受け取り、
// controller の呼び出し先（AC-9-8-a の抽象＝lambda.GetWorkMonthInvoker）を
// 返す関数」を引数に取り、http.Handler を返す（AC-10-8②）。ここでは
// controller.HandleGetWorkMonth 自体を経由させず、Execute を呼んでも
// 一度も output.Present/PresentError を呼ばない invoker と、必ず呼ぶ
// invoker を手書きし、「結果なし → INTERNAL_ERROR」（AC-9-13-c）と
// 「結果あり → 200」を対にして検査する。
//
// 3件目は同一の handler（buildInvoker は1回だけ渡す）へ2回リクエストし、
// 1回目は成功・2回目は「結果なし」となるよう invoker を仕込むことで、
// presenter がリクエストごとに新しく生成されること（プロセス内で使い回さ
// ないこと。AC-9-13-a）を検査する。もし実装が presenter をプロセス内で
// 共有していれば、2回目の応答は（本来 INTERNAL_ERROR になるはずが）1回目の
// 結果を引きずって 200 のまま返ってしまい、この対の検査が Red で捉える。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// sequencedGetWorkMonthInvoker は Execute が呼ばれても、あらかじめ指定した
// とおりにしか output.Present を呼ばない、手書きの invoker（AC-12-2）。
type sequencedGetWorkMonthInvoker struct {
	output  port.WorkMonthOutputPort
	present bool
}

func (s sequencedGetWorkMonthInvoker) Execute(_ context.Context, _ port.GetWorkMonthInput) {
	if s.present {
		s.output.Present(port.WorkMonthOutput{
			ContractID:   "ctr-0001",
			YearMonth:    "2026-07",
			State:        "Draft",
			DailyRecords: []port.DailyRecordOutput{},
		})
	}
}

// buildSequencedGetWorkMonthInvoker は呼ばれるたびに（＝リクエストごとに）
// presentOn の対応する要素を見て、その回に Present を呼ぶかどうかを決める
// invoker を生成する関数を返す。1回目の buildInvoker 呼び出しが presentOn[0]、
// 2回目が presentOn[1]、というように対応する。
func buildSequencedGetWorkMonthInvoker(presentOn ...bool) func(port.WorkMonthOutputPort) lambda.GetWorkMonthInvoker {
	call := 0
	return func(output port.WorkMonthOutputPort) lambda.GetWorkMonthInvoker {
		idx := call
		call++
		present := idx < len(presentOn) && presentOn[idx]
		return sequencedGetWorkMonthInvoker{output: output, present: present}
	}
}

// newGetWorkMonthRequest は E-1 相当のリクエストを組み立てる。ルーティング
// （①）は経由しないため、パス変数は r.SetPathValue で直接与える
// （AC-10-8②は①から独立して呼び出せることを要求する）。
func newGetWorkMonthRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/work-months/ctr-0001/2026-07", nil)
	r.SetPathValue("contractId", "ctr-0001")
	r.SetPathValue("yearMonth", "2026-07")
	return r
}

// TestGetWorkMonthHandler_ResultlessInvoker_And_SucceedingInvoker は
// AC-12-15③の対を検査する: 「一度も呼ばれない spy → INTERNAL_ERROR」と
// 「1回で成功する spy → 200」。
func TestGetWorkMonthHandler_ResultlessInvoker_And_SucceedingInvoker(t *testing.T) {
	tests := []struct {
		name       string
		presentOn  []bool
		wantStatus int
		wantCode   string // 空文字なら error.code を検査しない
	}{
		{
			name:       "invokerがpresenterを一度も呼ばない→INTERNAL_ERROR",
			presentOn:  []bool{false},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
		{
			name:       "invokerがpresenterを1回呼ぶ→200",
			presentOn:  []bool{true},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := lambda.NewGetWorkMonthHandler(buildSequencedGetWorkMonthInvoker(tt.presentOn...))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, newGetWorkMonthRequest())

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d（AC-9-13-c）: body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantCode != "" {
				var body presenter.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("応答ボディの JSON デコードに失敗した: %v（body=%s）", err, rec.Body.String())
				}
				if body.Error.Code != tt.wantCode {
					t.Errorf("error.code = %q, want %q", body.Error.Code, tt.wantCode)
				}
			}
		})
	}
}

// TestGetWorkMonthHandler_PresenterIsGeneratedPerRequest は AC-12-15③の
// もう一方の対を検査する: 1回目は成功・2回目は「結果なし」となる invoker を
// 同一の handler へ2回投げ、2回目の応答が（1回目の結果を引きずらず）
// INTERNAL_ERROR になること（presenter がリクエストごとに生成されている
// こと＝AC-9-13-a）を検査する。
func TestGetWorkMonthHandler_PresenterIsGeneratedPerRequest(t *testing.T) {
	handler := lambda.NewGetWorkMonthHandler(buildSequencedGetWorkMonthInvoker(true, false))

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, newGetWorkMonthRequest())
	if rec1.Code != http.StatusOK {
		t.Fatalf("1回目: status = %d, want 200: body=%s", rec1.Code, rec1.Body.String())
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, newGetWorkMonthRequest())
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("2回目: status = %d, want 500（presenter をプロセス内で共有していると1回目の結果を引きずって200のままになる＝AC-9-13-a）: body=%s", rec2.Code, rec2.Body.String())
	}
}
