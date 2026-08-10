package lambda_test

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-15①②
// （AC-10-8①「ルーティング」の境界。Lambda ランタイムも実DBも起動せず、
// lambda.NewRouter が返す http.Handler を httptest 越しに直接叩く）。
//
// パス・メソッドの文字列はすべて docs/specs/domain-api-http-contract.md
// の E-1〜E-7（79〜87行）をそのまま用いる。実装コードからの逆算はしていない。
//
// ①は E-3/E-4（同一パス・メソッド違い）と E-5/E-6/E-7（同一メソッド・末尾
// 違い）をそれぞれ対にして endpointTable に含め、各ケースで「対象の
// handlerSpy だけが1回呼ばれ、他は一度も呼ばれない」ことを検査することで
// 経路の取り違えを検出する。
//
// ②は①と同じ7エンドポイントの組について、定義されたパスに定義されていない
// メソッドを当てたケースと、定義されていないパスのケースを1件加え、
// 「どの handlerSpy も呼ばれず、応答が 404 / NOT_FOUND であること」
// （契約 AC-1-11・AC-9・AC-11-13）を検査する。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
)

// routerSpies は7エンドポイント分の handlerSpy をまとめて持つ。
type routerSpies struct {
	getWorkMonth      handlerSpy
	listWorkMonths    handlerSpy
	enterDailyRecord  handlerSpy
	deleteDailyRecord handlerSpy
	closeWorkMonth    handlerSpy
	approveWorkMonth  handlerSpy
	rejectWorkMonth   handlerSpy
}

func (s *routerSpies) endpoints() lambda.Endpoints {
	return lambda.Endpoints{
		GetWorkMonth:      &s.getWorkMonth,
		ListWorkMonths:    &s.listWorkMonths,
		EnterDailyRecord:  &s.enterDailyRecord,
		DeleteDailyRecord: &s.deleteDailyRecord,
		CloseWorkMonth:    &s.closeWorkMonth,
		ApproveWorkMonth:  &s.approveWorkMonth,
		RejectWorkMonth:   &s.rejectWorkMonth,
	}
}

// named は「名前 → その handlerSpy」のマップを返す（endpointTable.which と
// 突き合わせるため）。
func (s *routerSpies) named() map[string]*handlerSpy {
	return map[string]*handlerSpy{
		"GetWorkMonth":      &s.getWorkMonth,
		"ListWorkMonths":    &s.listWorkMonths,
		"EnterDailyRecord":  &s.enterDailyRecord,
		"DeleteDailyRecord": &s.deleteDailyRecord,
		"CloseWorkMonth":    &s.closeWorkMonth,
		"ApproveWorkMonth":  &s.approveWorkMonth,
		"RejectWorkMonth":   &s.rejectWorkMonth,
	}
}

// endpointTable は契約 E-1〜E-7（domain-api-http-contract.md 79〜87行）の
// メソッド・パスをそのまま用いる。パス変数は具体値で埋める。
var endpointTable = []struct {
	name   string
	method string
	path   string
	which  string
}{
	{"E-1 GetWorkMonth", http.MethodGet, "/work-months/ctr-0001/2026-07", "GetWorkMonth"},
	{"E-2 ListWorkMonths", http.MethodGet, "/work-months", "ListWorkMonths"},
	{"E-3 EnterDailyRecord", http.MethodPut, "/work-months/ctr-0001/2026-07/daily-records/2026-07-01", "EnterDailyRecord"},
	{"E-4 DeleteDailyRecord", http.MethodDelete, "/work-months/ctr-0001/2026-07/daily-records/2026-07-01", "DeleteDailyRecord"},
	{"E-5 CloseWorkMonth", http.MethodPost, "/work-months/ctr-0001/2026-07/close", "CloseWorkMonth"},
	{"E-6 ApproveWorkMonth", http.MethodPost, "/work-months/ctr-0001/2026-07/approve", "ApproveWorkMonth"},
	{"E-7 RejectWorkMonth", http.MethodPost, "/work-months/ctr-0001/2026-07/reject", "RejectWorkMonth"},
}

// TestRouter_DispatchesExactlyOneEndpoint は AC-12-15①を検査する。
func TestRouter_DispatchesExactlyOneEndpoint(t *testing.T) {
	for _, tt := range endpointTable {
		t.Run(tt.name, func(t *testing.T) {
			spies := &routerSpies{}
			router := lambda.NewRouter(spies.endpoints())

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			for name, spy := range spies.named() {
				want := 0
				if name == tt.which {
					want = 1
				}
				if spy.calls != want {
					t.Errorf("%s の呼び出し回数 = %d, want %d（%s %s。AC-12-15①）", name, spy.calls, want, tt.method, tt.path)
				}
			}
		})
	}
}

// TestRouter_UndefinedPathOrMethod_ReturnsNotFound は AC-12-15②を検査する
// （契約 AC-1-11・AC-9 の error-code 表の NOT_FOUND 行・AC-11-13）。
func TestRouter_UndefinedPathOrMethod_ReturnsNotFound(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"E-1のパスに定義のないメソッド(POST)", http.MethodPost, "/work-months/ctr-0001/2026-07"},
		{"E-2のパスに定義のないメソッド(POST)", http.MethodPost, "/work-months"},
		{"E-3/E-4のパスに定義のないメソッド(GET)", http.MethodGet, "/work-months/ctr-0001/2026-07/daily-records/2026-07-01"},
		{"E-5のパスに定義のないメソッド(GET)", http.MethodGet, "/work-months/ctr-0001/2026-07/close"},
		{"E-6のパスに定義のないメソッド(GET)", http.MethodGet, "/work-months/ctr-0001/2026-07/approve"},
		{"E-7のパスに定義のないメソッド(GET)", http.MethodGet, "/work-months/ctr-0001/2026-07/reject"},
		{"定義のないパス", http.MethodGet, "/unknown-resource"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spies := &routerSpies{}
			router := lambda.NewRouter(spies.endpoints())

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			for name, spy := range spies.named() {
				if spy.calls != 0 {
					t.Errorf("%s が呼ばれてはいけないのに呼ばれた（%s %s。AC-12-15②）", name, tt.method, tt.path)
				}
			}

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404（契約 AC-1-11）: body=%s", rec.Code, rec.Body.String())
			}

			var body presenter.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("応答ボディの JSON デコードに失敗した: %v（body=%s）", err, rec.Body.String())
			}
			if body.Error.Code != "NOT_FOUND" {
				t.Errorf("error.code = %q, want %q（契約 AC-9・AC-11-13）", body.Error.Code, "NOT_FOUND")
			}
		})
	}
}
