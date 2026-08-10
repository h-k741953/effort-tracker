package lambda_test

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-15④
// （AC-10-8③「具体型の結線」の境界）。SQL 実行インターフェース
// （gateway.DB の Fake）と port.Clock の Fake を注入して gateway・interactor を
// 組み立て、②が要求する形（buildInvoker 関数）にした handler を、①（router）
// 越しに実際に叩くことで、①〜③が実際に噛み合って動くことを確認する。
//
// (i)   gateway が結線されている: fakeDB への記録が1回以上ある。
// (ii)  Clock が結線されている: fakeClock.Today() が1回以上呼ばれる。
// (iii) presenter が結線されている: 応答が INTERNAL_ERROR にも「結果なし」
//       にもならない（AC-10-1 の成功応答の形で decode できる）。E-1 は
//       操作者ヘッダを付けないケース（ゲスト）を含める
//       （契約 AC-9-7-a②・AC-8-8。ゲストでも弾かれないことを固定する）。
// (iv)  ルーティングのパス変数の名前と controller の取り出しが一致している:
//       区別できる contractId を使い、fakeDB の記録にその値が現れる。
//
// SQL 文そのものの正しさ・列の意味・原子性・presenter の並行独立性は対象外
// （AC-13-19）。fakeDB には契約の行（contractRow）と一覧の行
// （workMonthListRow）を積むため Scan は実際に呼ばれる。ただし観測するのは
// 「記録が1回以上あること」と「引数にパス変数の値が現れること」だけであり、
// 列の意味そのものは検査しない（AC-13-18）。勤務月ヘッダは 0行（未生成）の
// 応答しか使わないため、その Scan だけは呼ばれない。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
)

// TestAssembly_GetWorkMonth_WiresGatewayPresenterAndPathVariables は
// AC-12-15④の(i)(iii)(iv)を、E-1（ゲスト・操作者ヘッダ不在）で検査する。
func TestAssembly_GetWorkMonth_WiresGatewayPresenterAndPathVariables(t *testing.T) {
	db := newFakeDB()
	// contractSelectQuery への応答（契約が見つかる）。
	db.pushQuery(newFakeRows(contractRow("ctr-assembly-1", "サンプル契約", "eng-assembly-1", 140, 0, 180, 0)), nil)
	// workMonthHeaderSelectQuery への応答（0行＝未生成）。
	db.pushQuery(newFakeRows(), nil)

	router := lambda.NewRouter(lambda.Endpoints{
		GetWorkMonth:      lambda.NewGetWorkMonthHandler(lambda.BuildGetWorkMonthInvoker(db)),
		ListWorkMonths:    noopHandler,
		EnterDailyRecord:  noopHandler,
		DeleteDailyRecord: noopHandler,
		CloseWorkMonth:    noopHandler,
		ApproveWorkMonth:  noopHandler,
		RejectWorkMonth:   noopHandler,
	})

	// 操作者ヘッダを付けない（ゲスト。契約 AC-9-7-a②・AC-8-8）。
	req := httptest.NewRequest(http.MethodGet, "/work-months/ctr-assembly-1/2027-03", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// (iii) presenter が結線されている。
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（ゲストでも弾かれない＝AC-9-7-a②・AC-8-8）: body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		ContractID string `json:"contractId"`
		Generated  bool   `json:"generated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("応答ボディの JSON デコードに失敗した（AC-10-1 の形になっていない）: %v（body=%s）", err, rec.Body.String())
	}
	if body.Generated {
		t.Errorf("generated = true, want false（0行＝未生成の応答のはず）")
	}
	// (iv) 応答側からもパス変数の取り違えが無いことを固定する（レビュー往復1
	// I-2。body.ContractID をデコードするだけで検査していなかった）。
	if body.ContractID != "ctr-assembly-1" {
		t.Errorf("応答の contractId = %q, want %q", body.ContractID, "ctr-assembly-1")
	}

	// (i) gateway が結線されている。
	if got := db.callCount(); got < 1 {
		t.Fatalf("SQL 実行 Fake への記録が無い（gateway が結線されていない）")
	}

	// (iv) ルーティングのパス変数の名前と controller の取り出しが一致している。
	if !db.hasArg("ctr-assembly-1") {
		t.Errorf("SQL 実行 Fake の記録に contractId %q が現れない（パス変数の取り違え）", "ctr-assembly-1")
	}
}

// TestAssembly_EnterDailyRecord_WiresGatewayAndClock は AC-12-15④の(i)(ii)を、
// E-3 で検査する。
func TestAssembly_EnterDailyRecord_WiresGatewayAndClock(t *testing.T) {
	db := newFakeDB()
	// contractSelectQuery への応答（契約が見つかる。操作者本人が技術者）。
	db.pushQuery(newFakeRows(contractRow("ctr-assembly-2", "サンプル契約2", "eng-assembly-2", 140, 0, 180, 0)), nil)
	// workMonthHeaderSelectQuery への応答（0行＝未生成。EnterDailyRecord は
	// この場合 workmonth.New で新規の下書きを組み立てる）。
	db.pushQuery(newFakeRows(), nil)

	clock := &fakeClock{today: mustDate(t, 2026, 7, 20)}

	router := lambda.NewRouter(lambda.Endpoints{
		GetWorkMonth:      noopHandler,
		ListWorkMonths:    noopHandler,
		EnterDailyRecord:  lambda.NewEnterDailyRecordHandler(lambda.BuildEnterDailyRecordInvoker(db, clock)),
		DeleteDailyRecord: noopHandler,
		CloseWorkMonth:    noopHandler,
		ApproveWorkMonth:  noopHandler,
		RejectWorkMonth:   noopHandler,
	})

	reqBody := `{"workingHours":{"hours":8,"minutes":0}}`
	req := httptest.NewRequest(http.MethodPut, "/work-months/ctr-assembly-2/2026-07/daily-records/2026-07-15", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor-Id", "eng-assembly-2")
	req.Header.Set("X-Actor-Role", "Engineer")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}

	// (i) gateway が結線されている。
	if got := db.callCount(); got < 1 {
		t.Fatalf("SQL 実行 Fake への記録が無い（gateway が結線されていない）")
	}

	// (ii) Clock が結線されている。
	if clock.calls < 1 {
		t.Fatalf("fakeClock.Today() が呼ばれていない（Clock が結線されていない）")
	}

	// (iv) 年月側のパス変数も取り違えていない（レビュー往復1 I-3。従来 hasArg
	// は string しか見ておらず、年月は int で渡るため直接観測できていなかった）。
	if !db.hasArg(2026) || !db.hasArg(7) {
		t.Errorf("SQL 実行 Fake の記録に年月 2026-07 が現れない（パス変数の取り違え）")
	}
}

// TestAssembly_ListWorkMonths_WiresQueryAndListPresenter は AC-12-15④の
// (i)(iii)(iv)を、E-2（ListWorkMonths）で検査する（レビュー往復1 C-1(a)）。
// AC-10-8②が名指しした唯一の分岐＝ListWorkMonthsPresenter を通る唯一の
// エンドポイントを固定する。
func TestAssembly_ListWorkMonths_WiresQueryAndListPresenter(t *testing.T) {
	db := newFakeDB()
	// 行取得クエリへの応答（1件）。
	db.pushQuery(newFakeRows(
		workMonthListRow("ctr-assembly-3", "サンプル契約3", 2026, 7, "PendingApproval"),
	), nil)
	// 件数取得クエリへの応答。
	db.pushQuery(newFakeRows(queryRow{1}), nil)

	router := lambda.NewRouter(lambda.Endpoints{
		GetWorkMonth:      noopHandler,
		ListWorkMonths:    lambda.NewListWorkMonthsHandler(lambda.BuildListWorkMonthsInvoker(db)),
		EnterDailyRecord:  noopHandler,
		DeleteDailyRecord: noopHandler,
		CloseWorkMonth:    noopHandler,
		ApproveWorkMonth:  noopHandler,
		RejectWorkMonth:   noopHandler,
	})

	req := httptest.NewRequest(http.MethodGet, "/work-months?engineerId=eng-assembly-3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// (iii) presenter（ListWorkMonthsPresenter）が結線されている。
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []struct {
			ContractID string `json:"contractId"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("応答ボディの JSON デコードに失敗した（AC-10-2 の一覧の形になっていない）: %v（body=%s）", err, rec.Body.String())
	}
	if len(body.Items) != 1 || body.Items[0].ContractID != "ctr-assembly-3" {
		t.Fatalf("items が期待と不一致: %+v", body)
	}
	if body.Total != 1 {
		t.Errorf("total = %d, want 1", body.Total)
	}

	// (i) gateway（WorkMonthQuery）が結線されている。
	if got := db.callCount(); got < 1 {
		t.Fatalf("SQL 実行 Fake への記録が無い（gateway が結線されていない）")
	}

	// (iv) ルーティングのパス変数の名前と controller の取り出しが一致している
	// （E-2 はクエリ文字列 engineerId だが、①〜③が噛み合っていることの検査は
	// 同じ意図）。
	if !db.hasArg("eng-assembly-3") {
		t.Errorf("SQL 実行 Fake の記録に engineerId %q が現れない", "eng-assembly-3")
	}
}
