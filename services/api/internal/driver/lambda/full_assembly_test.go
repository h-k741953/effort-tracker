package lambda_test

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-17 ①
// （AC-10-16「④ 全体の組み立て」）。
//
// SQL 実行インターフェースの手書き Fake（doubles_test.go の fakeDB）と、固定の
// 「当日」を返す手書きの Clock（同 fakeClock）を渡して組み立てた1つの
// http.Handler を、net/http/httptest で直接叩く（AC-12-15 と同じ形。Lambda
// ランタイムも実 DB も起動しない）。
//
// 期待値は契約（domain-api-http-contract.md の E-1〜E-7・AC-9-1・AC-10-1・
// AC-10-2）と実装設計 AC-11-12・AC-11-13 から導いており、実装を走らせて得た値を
// 書き写していない（AC-13-19 ④）。どの状態の勤務月を Fake に返させるかは
// 本テストが選び、状態ごとの承認・差戻しの可否の正解は approval.md（契約
// AC-7-2・AC-8-2 が写した表）に依る（AC-13-1。業務ルールを本テストで作らない）。
//
// 本 AC が要求するのは「7つの期待値が互いに区別できること」である。
// TestFullAssembly_ExpectationsAreMutuallyDistinguishable がそれを表明する。
// 区別できない組が残る場合、その組の取り違えは Red にならない（AC-13-24 ②）。
// **本テストは「エンドポイントの位置への代入の取り違えがすべて防げること」を
// 主張しない。** 主張するのは、下記7組の期待値が互いに異なること、および実際の
// 応答がそれぞれの期待値と一致することだけである。
//
// 対にして置いた組（AC-12-17 ① が最低限として挙げるもの）:
//   (i)   E-6 と E-7  — 同じ Fake（同じ契約・同じ状態 Draft・同じ操作者）の下で
//                       INVALID_STATE_FOR_APPROVE / INVALID_STATE_FOR_REJECT
//                       に分かれる（AC-11-12。契約 AC-7-2・AC-8-2）。
//   (ii)  E-5 と E-6・E-7 — 同じパスの末尾だけが異なる3つ。
//   (iii) E-3 と E-4  — 同じパスでメソッドが異なり、要求本体を読むか否かも
//                       異なる。操作者ヘッダを与える（与えないと双方
//                       UNAUTHENTICATED で区別できない＝AC-9-7-a）。
//   (iv)  E-1 と E-2  — 成功時の本体が勤務月1件と一覧で異なる（AC-9-13-d・
//                       契約 AC-10-1 / AC-10-2）。
//
// 担保しないもの: Lambda ランタイムへの登録・実行、実接続、本番で実際に
// 注入される実装、main() に残った部分（AC-13-24）。SQL 文そのものの正しさ・
// 原子性・並び順も観測しない（AC-13-18・AC-13-24 ⑤）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
)

// wantResponse は1エンドポイント分の期待応答。成功応答は本体そのもの
// （契約 AC-10-1 / AC-10-2）で、エラー応答は `code`（契約 AC-9-1。`message` は
// 診断用で契約が値を定めないため期待値にしない）で表す。
type wantResponse struct {
	status    int
	bodyJSON  string // 成功応答の期待本体。エラー応答では空。
	errorCode string // エラー応答の期待 code。成功応答では空。
}

// fullAssemblyCase は契約 E-1〜E-7 の1本分。
type fullAssemblyCase struct {
	name    string
	seed    func(db *fakeDB)
	method  string
	target  string
	headers map[string]string
	body    string
	want    wantResponse
}

// fixedToday は手書きの Clock が返す固定の「当日」。基準タイムゾーンの値は
// 書かない（AC-13-1。タイムゾーンの解決は driver/lambda の SystemClock の
// 責務であり、本テストは Clock を差し替える）。
const (
	fixedTodayYear  = 2026
	fixedTodayMonth = 7
	fixedTodayDay   = 20
)

// fullAssemblyCases は7エンドポイント分の要求と期待応答。
//
// E-5・E-6・E-7 は同じ契約・同じ状態（Draft）・同じ操作者（承認者ロールで
// 本人ではない）を与え、パスの末尾だけを変える。判定順序（契約 AC-9）に従い
// 締めは順4（認可）で FORBIDDEN_NOT_OWNER、承認・差戻しは順4を通過して順5
// （状態）で INVALID_STATE_FOR_APPROVE / INVALID_STATE_FOR_REJECT になる。
func fullAssemblyCases(t *testing.T) []fullAssemblyCase {
	t.Helper()

	return []fullAssemblyCase{
		{
			// E-1: GET /work-months/{contractId}/{yearMonth}（契約 AC-2-1・
			// AC-2-5。操作者ヘッダ不在＝ゲストでも 200）。
			name: "E-1 勤務月1件の取得",
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(contractRow("ctr-e1", "契約E1", "eng-e1", 140, 0, 180, 0)), nil)
				db.pushQuery(newFakeRows(workMonthHeaderRow("ctr-e1", 2026, 7, 140, 0, 180, 0, "Draft")), nil)
				db.pushQuery(newFakeRows(dailyRecordRow(2026, 7, 5, 7, 30)), nil)
			},
			method: http.MethodGet,
			target: "/work-months/ctr-e1/2026-07",
			want: wantResponse{
				status: http.StatusOK,
				bodyJSON: `{
					"contractId": "ctr-e1",
					"contractDisplayName": "契約E1",
					"yearMonth": "2026-07",
					"state": "Draft",
					"generated": true,
					"settlementRange": {
						"lowerBound": {"hours": 140, "minutes": 0},
						"upperBound": {"hours": 180, "minutes": 0}
					},
					"totalHours": {"hours": 7, "minutes": 30},
					"excess": null,
					"shortfall": null,
					"dailyRecords": [
						{
							"date": "2026-07-05",
							"workingHours": {"hours": 7, "minutes": 30},
							"roundedWorkingHours": {"hours": 7, "minutes": 30}
						}
					]
				}`,
			},
		},
		{
			// E-2: GET /work-months（契約 AC-3-1・AC-10-2）。limit / offset は
			// 既定値を契約が固定しないため（AC-3-5）明示して与え、「実際に適用
			// された値」が返ることを期待値にする。
			name: "E-2 勤務月の一覧",
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(workMonthListRow("ctr-e2", "契約E2", 2026, 7, "PendingApproval")), nil)
				db.pushQuery(newFakeRows(queryRow{1}), nil)
			},
			method: http.MethodGet,
			target: "/work-months?engineerId=eng-e2&limit=20&offset=0",
			want: wantResponse{
				status: http.StatusOK,
				bodyJSON: `{
					"items": [
						{
							"contractId": "ctr-e2",
							"contractDisplayName": "契約E2",
							"yearMonth": "2026-07",
							"state": "PendingApproval"
						}
					],
					"total": 1,
					"limit": 20,
					"offset": 0
				}`,
			},
		},
		{
			// E-3: PUT .../daily-records/{date}（契約 AC-4-1）。E-4 と同じパス・
			// 同じ Fake の内容で、メソッドと要求本体の有無だけが異なる。
			// 既にレコードのある日への送信は編集として扱う（契約 AC-4-1）。
			name: "E-3 稼働実績の入力・編集",
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(contractRow("ctr-e34", "契約E34", "eng-e34", 140, 0, 180, 0)), nil)
				db.pushQuery(newFakeRows(workMonthHeaderRow("ctr-e34", 2026, 7, 140, 0, 180, 0, "Draft")), nil)
				db.pushQuery(newFakeRows(dailyRecordRow(2026, 7, 15, 6, 0)), nil)
			},
			method: http.MethodPut,
			target: "/work-months/ctr-e34/2026-07/daily-records/2026-07-15",
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-Actor-Id":   "eng-e34",
				"X-Actor-Role": "Engineer",
			},
			body: `{"workingHours":{"hours":8,"minutes":0}}`,
			want: wantResponse{
				status: http.StatusOK,
				bodyJSON: `{
					"contractId": "ctr-e34",
					"contractDisplayName": "契約E34",
					"yearMonth": "2026-07",
					"state": "Draft",
					"generated": true,
					"settlementRange": {
						"lowerBound": {"hours": 140, "minutes": 0},
						"upperBound": {"hours": 180, "minutes": 0}
					},
					"totalHours": {"hours": 8, "minutes": 0},
					"excess": null,
					"shortfall": null,
					"dailyRecords": [
						{
							"date": "2026-07-15",
							"workingHours": {"hours": 8, "minutes": 0},
							"roundedWorkingHours": {"hours": 8, "minutes": 0}
						}
					]
				}`,
			},
		},
		{
			// E-4: DELETE .../daily-records/{date}（契約 AC-5-1）。E-3 と同じ
			// パス・同じ Fake の内容で、削除後は当該日のレコードが消える。
			name: "E-4 稼働実績の削除",
			seed: func(db *fakeDB) {
				db.pushQuery(newFakeRows(contractRow("ctr-e34", "契約E34", "eng-e34", 140, 0, 180, 0)), nil)
				db.pushQuery(newFakeRows(workMonthHeaderRow("ctr-e34", 2026, 7, 140, 0, 180, 0, "Draft")), nil)
				db.pushQuery(newFakeRows(dailyRecordRow(2026, 7, 15, 6, 0)), nil)
			},
			method: http.MethodDelete,
			target: "/work-months/ctr-e34/2026-07/daily-records/2026-07-15",
			headers: map[string]string{
				"X-Actor-Id":   "eng-e34",
				"X-Actor-Role": "Engineer",
			},
			want: wantResponse{
				status: http.StatusOK,
				bodyJSON: `{
					"contractId": "ctr-e34",
					"contractDisplayName": "契約E34",
					"yearMonth": "2026-07",
					"state": "Draft",
					"generated": true,
					"settlementRange": {
						"lowerBound": {"hours": 140, "minutes": 0},
						"upperBound": {"hours": 180, "minutes": 0}
					},
					"totalHours": {"hours": 0, "minutes": 0},
					"excess": null,
					"shortfall": null,
					"dailyRecords": []
				}`,
			},
		},
		{
			// E-5: POST .../close。操作者は承認者ロールで本人ではないため、
			// 判定順序（契約 AC-9）の順4（認可）で FORBIDDEN_NOT_OWNER
			// （契約 AC-6-3・AC-6-9。承認者の代行締めは無い）。
			name:   "E-5 締め",
			seed:   seedE567,
			method: http.MethodPost,
			target: "/work-months/ctr-e567/2026-07/close",
			headers: map[string]string{
				"X-Actor-Id":   "apr-e567",
				"X-Actor-Role": "Approver",
			},
			want: wantResponse{
				status:    http.StatusForbidden,
				errorCode: "FORBIDDEN_NOT_OWNER",
			},
		},
		{
			// E-6: POST .../approve。順4（認可）は通過し（承認者ロール・本人
			// でない）、順5（状態）で Draft のため 409
			// INVALID_STATE_FOR_APPROVE（契約 AC-7-2・AC-9、AC-11-12）。
			name:   "E-6 承認",
			seed:   seedE567,
			method: http.MethodPost,
			target: "/work-months/ctr-e567/2026-07/approve",
			headers: map[string]string{
				"X-Actor-Id":   "apr-e567",
				"X-Actor-Role": "Approver",
			},
			want: wantResponse{
				status:    http.StatusConflict,
				errorCode: "INVALID_STATE_FOR_APPROVE",
			},
		},
		{
			// E-7: POST .../reject。E-6 と同じ Fake・同じ操作者で、`code` だけが
			// 分かれる（契約 AC-8-2・AC-11-12）。
			name:   "E-7 差戻し",
			seed:   seedE567,
			method: http.MethodPost,
			target: "/work-months/ctr-e567/2026-07/reject",
			headers: map[string]string{
				"X-Actor-Id":   "apr-e567",
				"X-Actor-Role": "Approver",
			},
			want: wantResponse{
				status:    http.StatusConflict,
				errorCode: "INVALID_STATE_FOR_REJECT",
			},
		},
	}
}

// seedE567 は E-5・E-6・E-7 に共通の Fake の内容（同じ契約・状態 Draft・
// 稼働実績なし）。3本は末尾のパスだけが異なる（AC-12-17 ① (ii)）。
func seedE567(db *fakeDB) {
	db.pushQuery(newFakeRows(contractRow("ctr-e567", "契約E567", "eng-e567", 140, 0, 180, 0)), nil)
	db.pushQuery(newFakeRows(workMonthHeaderRow("ctr-e567", 2026, 7, 140, 0, 180, 0, "Draft")), nil)
	db.pushQuery(newFakeRows(), nil)
}

// TestFullAssembly_EachEndpointRespondsDistinctly は、AC-10-16 が組み立てた
// 1つの http.Handler に契約 E-1〜E-7 の7つの要求を与え、そのエンドポイントに
// 固有の応答が返ることを確認する（AC-12-17 ①）。
func TestFullAssembly_EachEndpointRespondsDistinctly(t *testing.T) {
	for _, tt := range fullAssemblyCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			tt.seed(db)
			clock := &fakeClock{today: mustDate(t, fixedTodayYear, fixedTodayMonth, fixedTodayDay)}

			handler := lambda.NewHandler(db, clock)

			var reqBody *strings.Reader
			if tt.body != "" {
				reqBody = strings.NewReader(tt.body)
			}
			var req *http.Request
			if reqBody == nil {
				req = httptest.NewRequest(tt.method, tt.target, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.target, reqBody)
			}
			for name, value := range tt.headers {
				req.Header.Set(name, value)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assertResponse(t, rec, tt.want)
		})
	}
}

// TestFullAssembly_ExpectationsAreMutuallyDistinguishable は、7つの期待値が
// 互いに区別できることを表明する（AC-12-17 ①「本 AC が要求するのは、7つの
// 期待値が互いに区別できること」）。区別できない組があれば、その組の
// エンドポイントの取り違えは上のテストでも Red にならない（AC-13-24 ②）ため、
// 期待値そのものを検査対象にする。
func TestFullAssembly_ExpectationsAreMutuallyDistinguishable(t *testing.T) {
	cases := fullAssemblyCases(t)

	seen := make(map[string]string, len(cases))
	for _, tt := range cases {
		key := fmt.Sprintf("%d|%s|%s", tt.want.status, tt.want.errorCode, canonicalJSON(t, tt.want.bodyJSON))
		if other, dup := seen[key]; dup {
			t.Errorf("期待応答が %q と %q で同一である（この組の取り違えは Red にならない＝AC-13-24 ②）: %s", other, tt.name, key)
			continue
		}
		seen[key] = tt.name
	}

	if len(cases) != 7 {
		t.Fatalf("エンドポイントの本数 = %d, want 7（契約 E-1〜E-7）", len(cases))
	}
}

// assertResponse は応答を期待値と突き合わせる。成功応答は本体そのものを、
// エラー応答は契約 AC-9-1 の形と `code` を検査する（`message` は診断用で
// 契約が値を定めないため期待値にしない）。
func assertResponse(t *testing.T, rec *httptest.ResponseRecorder, want wantResponse) {
	t.Helper()

	if rec.Code != want.status {
		t.Errorf("status = %d, want %d（body=%s）", rec.Code, want.status, rec.Body.String())
	}

	var got any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("応答ボディの JSON デコードに失敗した: %v（body=%s）", err, rec.Body.String())
	}

	if want.errorCode != "" {
		assertErrorBody(t, got, want.errorCode)
		return
	}

	var expected any
	if err := json.Unmarshal([]byte(want.bodyJSON), &expected); err != nil {
		t.Fatalf("期待ボディの JSON デコードに失敗した（テスト側の誤り）: %v", err)
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("応答ボディが契約の期待と不一致 (-want +got):\n%s", diff)
	}
}

// assertErrorBody は契約 AC-9-1 のエラーボディの形（`error.code` と
// `error.message` だけを持ち、フィールドを増やさない）と `code` を検査する。
func assertErrorBody(t *testing.T, got any, wantCode string) {
	t.Helper()

	body, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("エラー応答がオブジェクトでない: %#v", got)
	}
	if diff := cmp.Diff([]string{"error"}, sortedKeys(body)); diff != "" {
		t.Errorf("エラー応答のトップレベルのキーが契約 AC-9-1 と不一致 (-want +got):\n%s", diff)
	}

	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("error がオブジェクトでない: %#v", body["error"])
	}
	if diff := cmp.Diff([]string{"code", "message"}, sortedKeys(errObj)); diff != "" {
		t.Errorf("error のキーが契約 AC-9-1 と不一致 (-want +got):\n%s", diff)
	}

	if code, _ := errObj["code"].(string); code != wantCode {
		t.Errorf("error.code = %v, want %q", errObj["code"], wantCode)
	}
	if message, _ := errObj["message"].(string); message == "" {
		t.Errorf("error.message が空である（契約 AC-9-1 は診断用の短い説明を要求する）")
	}
}

// sortedKeys は JSON オブジェクトのキーを昇順で返す（キー集合の比較用）。
func sortedKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// canonicalJSON は JSON を正規化した文字列にする（期待値どうしの区別に使う）。
func canonicalJSON(t *testing.T, s string) string {
	t.Helper()

	if s == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("期待ボディの JSON デコードに失敗した（テスト側の誤り）: %v", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("期待ボディの JSON 再直列化に失敗した（テスト側の誤り）: %v", err)
	}
	return string(b)
}

// workMonthHeaderRow は勤務月ヘッダ行を Scan の並びで組み立てる（契約識別子・
// 年・月・精算幅下限（時・分）・精算幅上限（時・分）・状態・超過（時・分）・
// 不足（時・分）の12列。adapter/gateway/work_month_repository.go の Scan 順）。
// 超過／不足はいずれも NULL（未確定）とする（AC-5-2）。
func workMonthHeaderRow(contractID string, year, month, lowerH, lowerM, upperH, upperM int, state string) queryRow {
	var null *int
	return queryRow{contractID, year, month, lowerH, lowerM, upperH, upperM, state, null, null, null, null}
}

// dailyRecordRow は稼働実績の行を Scan の並びで組み立てる（年・月・日・
// 稼働時間（時・分）の5列）。
func dailyRecordRow(year, month, day, hours, minutes int) queryRow {
	return queryRow{year, month, day, hours, minutes}
}
