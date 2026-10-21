package lambda_test

// 検証対象: docs/specs/workmonth-implementation-design.md AC-10-22（E-8・E-9 の
// 結線）・AC-12-15③④・AC-12-17 ①・AC-12-19-g。契約 AC-12・AC-13（E-8・E-9）。
//
// 本ファイルは E-8・E-9 に固有の検査を置く。9本のルーティング（①②）は
// router_test.go、9本の応答の区別は full_assembly_test.go が持つ。
//
// 期待値は契約（AC-9・AC-10-4・AC-10-5・AC-12・AC-13）から導き、実装を走らせて得た
// 値を書き写していない。観測するのは http.Handler 越しの呼び出しと応答に限る
// （AC-13-19・AC-13-24）。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

func intP(v int) *int { return &v }

func strP(v string) *string { return &v }

// contractListRow は契約一覧（E-8）の行を Scan の並びどおりに組み立てる
// （実装設計 AC-9-20-c の11列。⑦〜⑩は *int、⑪は *string）。
func contractListRow(
	id, displayName string,
	lowerH, lowerM, upperH, upperM int,
	startYear, startMonth, endYear, endMonth *int,
	state *string,
) queryRow {
	return queryRow{id, displayName, lowerH, lowerM, upperH, upperM, startYear, startMonth, endYear, endMonth, state}
}

// trendWorkMonthRow は稼働の推移（E-9）の1回目の Query の行を Scan の並びで
// 組み立てる（AC-9-21-d。AC-9-15 の勤務月の行の12個と同じ並びと型）。
func trendWorkMonthRow(
	contractID string, year, month, lowerH, lowerM, upperH, upperM int, state string,
	excessH, excessM, shortfallH, shortfallM *int,
) queryRow {
	return queryRow{contractID, year, month, lowerH, lowerM, upperH, upperM, state, excessH, excessM, shortfallH, shortfallM}
}

// trendDailyRecordRow は稼働の推移（E-9）の2回目の Query の行を Scan の並びで
// 組み立てる（AC-9-21-e。①契約識別子に AC-9-15-i の11列が続く12列）。
func trendDailyRecordRow(contractID string, year, month, day int, start, end [2]int, brk *[4]int) queryRow {
	return append(queryRow{contractID}, dailyRecordRow(year, month, day, start, end, brk)...)
}

func engineerHeaders(id string) map[string]string {
	return map[string]string{"X-Actor-Id": id, "X-Actor-Role": "Engineer"}
}

func serve(handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// ---- 全体の組み立て（NewHandler）越しの認可・エラー・0件 --------------------

// TestFullAssembly_ContractsAndWorkTrends_Authorization は E-8・E-9 の認可の
// 両側を、組み立てた http.Handler 越しに検査する（契約 AC-12-12〜12-14・
// AC-13-8〜13-10・AC-12-19-e・AC-13-13。実装設計 AC-8-12・AC-12-19-g）。
// 弾かれた要求では SQL 実行 Fake が1回も呼ばれない（interactor が認可を先に
// 判定し、技術者の該当件数で結果を変えない）。本人は 200。
func TestFullAssembly_ContractsAndWorkTrends_Authorization(t *testing.T) {
	endpoints := []struct {
		name   string
		target string
		// seedOwner は本人の要求（200）のために積む Fake の応答。
		seedOwner func(db *fakeDB)
	}{
		{
			name:      "E-8",
			target:    "/contracts?engineerId=eng-auth&yearMonth=2026-10",
			seedOwner: func(db *fakeDB) { db.pushQuery(newFakeRows(), nil) },
		},
		{
			name:   "E-9",
			target: "/work-trends?engineerId=eng-auth",
			seedOwner: func(db *fakeDB) {
				db.pushQuery(newFakeRows(), nil)
				db.pushQuery(newFakeRows(), nil)
			},
		},
	}
	actors := []struct {
		name      string
		headers   map[string]string
		wantCode  string // 空なら本人（200）
		wantState int
	}{
		{name: "ゲスト（両ヘッダ不在）は 401 UNAUTHENTICATED", headers: nil, wantCode: "UNAUTHENTICATED", wantState: http.StatusUnauthorized},
		{name: "Approver は識別子が一致しても 403 FORBIDDEN_NOT_OWNER", headers: map[string]string{"X-Actor-Id": "eng-auth", "X-Actor-Role": "Approver"}, wantCode: "FORBIDDEN_NOT_OWNER", wantState: http.StatusForbidden},
		{name: "Engineer で識別子不一致は 403 FORBIDDEN_NOT_OWNER（該当0件でも 200 にしない）", headers: engineerHeaders("eng-other"), wantCode: "FORBIDDEN_NOT_OWNER", wantState: http.StatusForbidden},
		{name: "Engineer で識別子一致は 200（0件は空配列）", headers: engineerHeaders("eng-auth"), wantState: http.StatusOK},
	}

	for _, ep := range endpoints {
		for _, actor := range actors {
			t.Run(ep.name+"/"+actor.name, func(t *testing.T) {
				db := newFakeDB()
				ep.seedOwner(db)
				handler := lambda.NewHandler(db, &fakeClock{today: mustDate(t, 2026, 7, 20)})

				rec := serve(handler, http.MethodGet, ep.target, actor.headers)

				if actor.wantCode != "" {
					assertResponse(t, rec, wantResponse{status: actor.wantState, errorCode: actor.wantCode})
					if got := db.callCount(); got != 0 {
						t.Errorf("弾かれた要求で SQL 実行 Fake が %d 回呼ばれた（want 0。AC-7-19・AC-7-20）", got)
					}
					return
				}
				assertResponse(t, rec, wantResponse{status: http.StatusOK, bodyJSON: `{"items": []}`})
			})
		}
	}
}

// TestFullAssembly_ContractsAndWorkTrends_BrokenRowIsInternalError は永続化行の
// 破損が 500 INTERNAL_ERROR になり、破損した行を除いた 200 を返さないことを
// 検査する（契約 AC-12-18・AC-13-12。実装設計 AC-9-20-d・AC-9-21-i・AC-11-13）。
func TestFullAssembly_ContractsAndWorkTrends_BrokenRowIsInternalError(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		headers map[string]string
		seed    func(db *fakeDB)
	}{
		{
			name:    "E-8: 契約期間の開始 2026-07 が終了 2026-06 より後",
			target:  "/contracts?engineerId=eng-x&yearMonth=2026-10",
			headers: engineerHeaders("eng-x"),
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(
					contractListRow("ctr-0001", "正常", 140, 0, 180, 0, intP(2025), intP(4), nil, nil, nil),
					contractListRow("ctr-0002", "破損", 140, 0, 180, 0, intP(2026), intP(7), intP(2026), intP(6), nil),
				), nil)
			},
		},
		{
			name:    "E-8: 精算幅の下限 > 上限",
			target:  "/contracts?engineerId=eng-x&yearMonth=2026-10",
			headers: engineerHeaders("eng-x"),
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(
					contractListRow("ctr-0001", "破損", 180, 0, 140, 0, intP(2025), intP(4), nil, nil, nil),
				), nil)
			},
		},
		{
			name:    "E-9: Draft に超過が確定している（Reconstruct の失敗）",
			target:  "/work-trends?engineerId=eng-x",
			headers: engineerHeaders("eng-x"),
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(
					trendWorkMonthRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", intP(1), intP(0), intP(1), intP(0)),
				), nil)
				db.pushQuery(newFakeRows(), nil)
			},
		},
		{
			name:    "E-9: 一致する勤務月の無い稼働実績の行",
			target:  "/work-trends?engineerId=eng-x",
			headers: engineerHeaders("eng-x"),
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(
					trendWorkMonthRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", nil, nil, nil, nil),
				), nil)
				db.pushQuery(newFakeRows(
					trendDailyRecordRow("ctr-9999", 2026, 8, 1, [2]int{9, 0}, [2]int{18, 0}, nil),
				), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			tt.seed(db)
			handler := lambda.NewHandler(db, &fakeClock{today: mustDate(t, 2026, 7, 20)})

			rec := serve(handler, http.MethodGet, tt.target, tt.headers)

			assertResponse(t, rec, wantResponse{status: http.StatusInternalServerError, errorCode: "INTERNAL_ERROR"})
			if strings.Contains(rec.Body.String(), "ctr-") {
				t.Errorf("500 の応答に内部識別子が含まれる: %s（契約 AC-9-6）", rec.Body.String())
			}
		})
	}
}

// ---- リクエストごとの結線（AC-10-22 ④・AC-12-15③） --------------------------

// sequencedInvoker は Execute が呼ばれても、あらかじめ指定したとおりにしか出力を
// 呼ばない手書きの invoker（AC-12-2）。E-8・E-9 の両方で共有する。
type sequencedInvoker[Input any] struct {
	present func()
}

func (s sequencedInvoker[Input]) Execute(_ context.Context, _ Input) {
	if s.present != nil {
		s.present()
	}
}

func newContractsRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/contracts?engineerId=eng-0001&yearMonth=2026-10", nil)
	r.Header.Set("X-Actor-Id", "eng-0001")
	r.Header.Set("X-Actor-Role", "Engineer")
	return r
}

func newWorkTrendsRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/work-trends?engineerId=eng-0001", nil)
	r.Header.Set("X-Actor-Id", "eng-0001")
	r.Header.Set("X-Actor-Role", "Engineer")
	return r
}

// TestListContractsHandler_ResultlessInvoker_And_SucceedingInvoker は
// 「出力ポートを一度も呼ばない invoker → INTERNAL_ERROR」と「1回成功で呼ぶ
// invoker → 200」を対にして検査する（AC-12-15③・AC-9-13-c・AC-9-13-e）。
func TestListContractsHandler_ResultlessInvoker_And_SucceedingInvoker(t *testing.T) {
	tests := []struct {
		name       string
		presentOn  bool
		wantStatus int
		wantCode   string
	}{
		{name: "invoker が出力を一度も呼ばない → INTERNAL_ERROR", presentOn: false, wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
		{name: "invoker が出力を1回呼ぶ → 200", presentOn: true, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := lambda.NewListContractsHandler(func(output port.ListContractsOutputPort) lambda.ListContractsInvoker {
				return sequencedInvoker[port.ListContractsInput]{present: func() {
					if tt.presentOn {
						output.Present(port.ListContractsOutput{Items: []port.ContractOutputRow{}})
					}
				}}
			})

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, newContractsRequest())

			want := wantResponse{status: tt.wantStatus, errorCode: tt.wantCode}
			if tt.wantCode == "" {
				want.bodyJSON = `{"items": []}`
			}
			assertResponse(t, rec, want)
		})
	}
}

// TestListWorkTrendsHandler_ResultlessInvoker_And_SucceedingInvoker は E-9 の同じ対。
func TestListWorkTrendsHandler_ResultlessInvoker_And_SucceedingInvoker(t *testing.T) {
	tests := []struct {
		name       string
		presentOn  bool
		wantStatus int
		wantCode   string
	}{
		{name: "invoker が出力を一度も呼ばない → INTERNAL_ERROR", presentOn: false, wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
		{name: "invoker が出力を1回呼ぶ → 200", presentOn: true, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := lambda.NewListWorkTrendsHandler(func(output port.ListWorkTrendsOutputPort) lambda.ListWorkTrendsInvoker {
				return sequencedInvoker[port.ListWorkTrendsInput]{present: func() {
					if tt.presentOn {
						output.Present(port.ListWorkTrendsOutput{Items: []port.WorkTrendOutputRow{}})
					}
				}}
			})

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, newWorkTrendsRequest())

			want := wantResponse{status: tt.wantStatus, errorCode: tt.wantCode}
			if tt.wantCode == "" {
				want.bodyJSON = `{"items": []}`
			}
			assertResponse(t, rec, want)
		})
	}
}

// TestListContractsHandler_PresenterIsGeneratedPerRequest は、1回目は成功・2回目は
// 「結果なし」となる invoker を同一の handler へ2回投げ、2回目が INTERNAL_ERROR に
// なる（presenter をリクエストごとに生成している）ことを検査する
// （AC-9-13-a・AC-12-15③）。
func TestListContractsHandler_PresenterIsGeneratedPerRequest(t *testing.T) {
	call := 0
	handler := lambda.NewListContractsHandler(func(output port.ListContractsOutputPort) lambda.ListContractsInvoker {
		idx := call
		call++
		return sequencedInvoker[port.ListContractsInput]{present: func() {
			if idx == 0 {
				output.Present(port.ListContractsOutput{Items: []port.ContractOutputRow{}})
			}
		}}
	})

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, newContractsRequest())
	if rec1.Code != http.StatusOK {
		t.Fatalf("1回目: status = %d, want 200: body=%s", rec1.Code, rec1.Body.String())
	}
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, newContractsRequest())
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("2回目: status = %d, want 500（presenter を共有すると 1回目の結果を引きずる＝AC-9-13-a）: body=%s", rec2.Code, rec2.Body.String())
	}
}

// TestListWorkTrendsHandler_PresenterIsGeneratedPerRequest は E-9 の同じ検査。
func TestListWorkTrendsHandler_PresenterIsGeneratedPerRequest(t *testing.T) {
	call := 0
	handler := lambda.NewListWorkTrendsHandler(func(output port.ListWorkTrendsOutputPort) lambda.ListWorkTrendsInvoker {
		idx := call
		call++
		return sequencedInvoker[port.ListWorkTrendsInput]{present: func() {
			if idx == 0 {
				output.Present(port.ListWorkTrendsOutput{Items: []port.WorkTrendOutputRow{}})
			}
		}}
	})

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, newWorkTrendsRequest())
	if rec1.Code != http.StatusOK {
		t.Fatalf("1回目: status = %d, want 200: body=%s", rec1.Code, rec1.Body.String())
	}
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, newWorkTrendsRequest())
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("2回目: status = %d, want 500（AC-9-13-a）: body=%s", rec2.Code, rec2.Body.String())
	}
}

// recordingInvoker は Execute が受け取った入力 DTO を記録し、必ず出力を呼ぶ
// 手書きの invoker。controller を経由して入力 DTO が組み立てられることを見る。
type recordingContractsInvoker struct {
	output   port.ListContractsOutputPort
	recorded *[]port.ListContractsInput
}

func (s recordingContractsInvoker) Execute(_ context.Context, input port.ListContractsInput) {
	*s.recorded = append(*s.recorded, input)
	s.output.Present(port.ListContractsOutput{Items: []port.ContractOutputRow{}})
}

type recordingTrendsInvoker struct {
	output   port.ListWorkTrendsOutputPort
	recorded *[]port.ListWorkTrendsInput
}

func (s recordingTrendsInvoker) Execute(_ context.Context, input port.ListWorkTrendsInput) {
	*s.recorded = append(*s.recorded, input)
	s.output.Present(port.ListWorkTrendsOutput{Items: []port.WorkTrendOutputRow{}})
}

// TestListContractsHandler_CallsController は handler が controller.HandleListContracts を
// 呼び、クエリの値が入力 DTO に載ることを固定する（AC-10-22 ④・AC-9-5-h）。
func TestListContractsHandler_CallsController(t *testing.T) {
	var got []port.ListContractsInput
	handler := lambda.NewListContractsHandler(func(output port.ListContractsOutputPort) lambda.ListContractsInvoker {
		return recordingContractsInvoker{output: output, recorded: &got}
	})
	r := httptest.NewRequest(http.MethodGet, "/contracts?engineerId=eng-c1&yearMonth=2027-03", nil)
	r.Header.Set("X-Actor-Id", "eng-c1")
	r.Header.Set("X-Actor-Role", "Engineer")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	if len(got) != 1 {
		t.Fatalf("invoker.Execute の呼び出し回数 = %d, want 1（handler が controller を呼んでいない）: body=%s", len(got), rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}
	if got[0].EngineerID != "eng-c1" || got[0].YearMonth.Year() != 2027 || got[0].YearMonth.Month() != 3 {
		t.Errorf("入力 DTO の EngineerID/YearMonth = %q/%d-%d, want eng-c1/2027-3", got[0].EngineerID, got[0].YearMonth.Year(), got[0].YearMonth.Month())
	}
	wantActor := port.Actor{ID: "eng-c1", Role: port.RoleEngineer, Authenticated: true}
	if diff := cmp.Diff(wantActor, got[0].Actor); diff != "" {
		t.Errorf("入力 DTO の Actor が不一致 (-want +got):\n%s", diff)
	}
}

// TestListWorkTrendsHandler_CallsController は E-9 の同じ検査（AC-10-22 ④・AC-9-5-i）。
func TestListWorkTrendsHandler_CallsController(t *testing.T) {
	var got []port.ListWorkTrendsInput
	handler := lambda.NewListWorkTrendsHandler(func(output port.ListWorkTrendsOutputPort) lambda.ListWorkTrendsInvoker {
		return recordingTrendsInvoker{output: output, recorded: &got}
	})
	r := httptest.NewRequest(http.MethodGet, "/work-trends?engineerId=eng-t1", nil)
	r.Header.Set("X-Actor-Id", "eng-t1")
	r.Header.Set("X-Actor-Role", "Engineer")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	if len(got) != 1 {
		t.Fatalf("invoker.Execute の呼び出し回数 = %d, want 1（handler が controller を呼んでいない）: body=%s", len(got), rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}
	want := port.ListWorkTrendsInput{Actor: port.Actor{ID: "eng-t1", Role: port.RoleEngineer, Authenticated: true}, EngineerID: "eng-t1"}
	if diff := cmp.Diff(want, got[0]); diff != "" {
		t.Errorf("入力 DTO が不一致 (-want +got):\n%s", diff)
	}
}

// ---- 具体型の結線（AC-10-22 ③・AC-12-15④） ---------------------------------

// TestAssembly_ListContracts_WiresGatewayPresenterAndQueryParameters は、
// BuildListContractsInvoker で gateway・interactor・presenter が結線され、
// クエリの名前（engineerId・yearMonth）と controller の取り出しが一致する
// （区別できる値が SQL 実行 Fake の引数に現れる）ことを検査する
// （AC-10-22 ③・AC-12-15④ (i)(iii)(iv)）。Clock は渡さない（AC-10-22 ③）。
func TestAssembly_ListContracts_WiresGatewayPresenterAndQueryParameters(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		contractListRow("ctr-asm-8", "結線E8", 120, 0, 160, 0, intP(2024), intP(4), intP(2028), intP(3), strP("PendingApproval")),
	), nil)

	router := lambda.NewRouter(lambda.Endpoints{
		GetWorkMonth:      noopHandler,
		ListWorkMonths:    noopHandler,
		EnterDailyRecord:  noopHandler,
		DeleteDailyRecord: noopHandler,
		CloseWorkMonth:    noopHandler,
		ApproveWorkMonth:  noopHandler,
		RejectWorkMonth:   noopHandler,
		ListContracts:     lambda.NewListContractsHandler(lambda.BuildListContractsInvoker(db)),
		ListWorkTrends:    noopHandler,
	})

	rec := serve(router, http.MethodGet, "/contracts?engineerId=eng-asm-8&yearMonth=2027-03", engineerHeaders("eng-asm-8"))

	// (iii) presenter が結線されている（契約 AC-10-4 の形）。
	assertResponse(t, rec, wantResponse{
		status: http.StatusOK,
		bodyJSON: `{
			"items": [
				{
					"contractId": "ctr-asm-8",
					"contractDisplayName": "結線E8",
					"settlementRange": {
						"lowerBound": {"hours": 120, "minutes": 0},
						"upperBound": {"hours": 160, "minutes": 0}
					},
					"contractPeriod": {"start": "2024-04", "end": "2028-03"},
					"workMonth": {"yearMonth": "2027-03", "state": "PendingApproval", "generated": true}
				}
			]
		}`,
	})
	// (i) gateway が結線されている。
	if got := db.callCount(); got < 1 {
		t.Errorf("SQL 実行 Fake への記録 = %d, want 1 以上（gateway が結線されていない）", got)
	}
	// (iv) クエリの名前と controller の取り出しが一致している。
	for _, want := range []any{"eng-asm-8", 2027, 3} {
		if !db.hasArg(want) {
			t.Errorf("SQL 実行 Fake の引数に %v が現れない（クエリの名前と controller の取り出しが食い違っている）", want)
		}
	}
	// 応答の yearMonth は要求のそのまま（interactor が入力の年月を写す＝AC-7-19 ④）。
	if !strings.Contains(rec.Body.String(), `"yearMonth":"2027-03"`) {
		t.Errorf("応答に要求の yearMonth が現れない: %s", rec.Body.String())
	}
}

// TestAssembly_ListWorkTrends_WiresGatewayPresenterAndQueryParameters は E-9 の同じ検査
// （AC-10-22 ③・AC-12-15④ (i)(iii)(iv)）。
func TestAssembly_ListWorkTrends_WiresGatewayPresenterAndQueryParameters(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		trendWorkMonthRow("ctr-asm-9", 2026, 11, 120, 0, 160, 0, "Draft", nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(
		trendDailyRecordRow("ctr-asm-9", 2026, 11, 2, [2]int{9, 0}, [2]int{17, 0}, nil),
	), nil)

	router := lambda.NewRouter(lambda.Endpoints{
		GetWorkMonth:      noopHandler,
		ListWorkMonths:    noopHandler,
		EnterDailyRecord:  noopHandler,
		DeleteDailyRecord: noopHandler,
		CloseWorkMonth:    noopHandler,
		ApproveWorkMonth:  noopHandler,
		RejectWorkMonth:   noopHandler,
		ListContracts:     noopHandler,
		ListWorkTrends:    lambda.NewListWorkTrendsHandler(lambda.BuildListWorkTrendsInvoker(db)),
	})

	rec := serve(router, http.MethodGet, "/work-trends?engineerId=eng-asm-9", engineerHeaders("eng-asm-9"))

	assertResponse(t, rec, wantResponse{
		status: http.StatusOK,
		bodyJSON: `{
			"items": [
				{
					"contractId": "ctr-asm-9",
					"yearMonth": "2026-11",
					"state": "Draft",
					"settlementRange": {
						"lowerBound": {"hours": 120, "minutes": 0},
						"upperBound": {"hours": 160, "minutes": 0}
					},
					"totalHours": {"hours": 8, "minutes": 0},
					"excess": null,
					"shortfall": null
				}
			]
		}`,
	})
	if got := db.callCount(); got < 1 {
		t.Errorf("SQL 実行 Fake への記録 = %d, want 1 以上（gateway が結線されていない）", got)
	}
	if !db.hasArg("eng-asm-9") {
		t.Errorf("SQL 実行 Fake の引数に eng-asm-9 が現れない（クエリの名前と controller の取り出しが食い違っている）")
	}
}
