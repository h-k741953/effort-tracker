package lambda_test

// 検証対象: 契約 domain-api-http-contract.md の E-3 / E-1 の通し
//   - AC-4-5（時刻の値域外 → 400 TIME_OF_DAY_OUT_OF_RANGE）
//   - AC-4-5-a（時刻の組み合わせの不正 → 400 INVALID_TIME_COMBINATION）
//   - AC-4-5-b（妥当な時刻 → 200。送った時刻をそのまま返し、workingHours は行の稼働時間に一致）
//   - AC-4-14-a〜AC-4-14-e（保存した時刻の返却。E-3 の応答と、保存を経て読み直した E-1 の応答が同じ）
//   - 実装設計 AC-12-18（時刻の検証と保存のテスト）・AC-9-16-h・AC-9-15-i
//
// full_assembly_test.go と同じ形で、lambda.NewHandler に手書きの SQL 実行 Fake と固定の Clock を
// 渡して httptest で叩く（Lambda ランタイムも実 DB も起動しない）。入力と期待は
// daily-record-entry.md AC-2・AC-3 の表から読み、テストコードへ手で写さない
// （spec_table_test.go。実装設計 AC-12-18 ①と同じ表）。
//
// 担保しないもの: 実 DB を通した永続化（AC-13-4）。E-1 の読み直しは、E-3 が Tx 越しに書いた
// 稼働実績の挿入の引数（実装設計 AC-9-16-h ②の12個）から、読み出しの行（AC-9-15-i の11個）を
// 組み立てて Fake に返させる。書き込みと読み出しの並びが対応していることの検査であり、
// SQL 文の正しさ・永続化の原子性は観測しない（AC-13-18）。

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

const (
	timesTestContract = "ctr-t"
	timesTestEngineer = "eng-t"
	timesTestDate     = "2026-07-15"
)

// seedEnter は E-3 の前提を Fake に積む。契約 → 勤務月のヘッダ（Draft・生成済み）→ 稼働実績の行。
func seedEnter(db *fakeDB, existing ...queryRow) {
	db.pushQuery(newFakeRows(contractRow(timesTestContract, "契約T", timesTestEngineer, 140, 0, 180, 0)), nil)
	db.pushQuery(newFakeRows(workMonthHeaderRow(timesTestContract, 2026, 7, 140, 0, 180, 0, "Draft")), nil)
	db.pushQuery(newFakeRows(existing...), nil)
}

func timeJSON(t [2]int) string {
	return fmt.Sprintf(`{"hour":%d,"minute":%d}`, t[0], t[1])
}

// bodyOf は時刻の組から E-3 のリクエストボディ（契約 AC-4）を組み立てる。
func bodyOf(start, end [2]int, brk *[2][2]int) string {
	b := `null`
	if brk != nil {
		b = fmt.Sprintf(`{"start":%s,"end":%s}`, timeJSON(brk[0]), timeJSON(brk[1]))
	}
	return fmt.Sprintf(`{"startTime":%s,"endTime":%s,"break":%s}`, timeJSON(start), timeJSON(end), b)
}

// putDailyRecord は E-3 を叩く。clock の「当日」は 2026-07-20。
func putDailyRecord(t *testing.T, db *fakeDB, body string) *httptest.ResponseRecorder {
	t.Helper()
	handler := lambda.NewHandler(db, &fakeClock{today: mustDate(t, 2026, 7, 20)})
	req := httptest.NewRequest(http.MethodPut,
		"/work-months/"+timesTestContract+"/2026-07/daily-records/"+timesTestDate, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor-Id", timesTestEngineer)
	req.Header.Set("X-Actor-Role", "Engineer")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// getWorkMonth は E-1 を叩く（ゲスト＝ヘッダなしでよい。契約 AC-2-5）。
func getWorkMonth(t *testing.T, db *fakeDB) *httptest.ResponseRecorder {
	t.Helper()
	handler := lambda.NewHandler(db, &fakeClock{today: mustDate(t, 2026, 7, 20)})
	req := httptest.NewRequest(http.MethodGet, "/work-months/"+timesTestContract+"/2026-07", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("応答ボディの JSON デコードに失敗した: %v（body=%s）", err, rec.Body.String())
	}
	return got
}

func errorCodeOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body := decodeBody(t, rec)
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("エラー応答でない: status=%d body=%s", rec.Code, rec.Body.String())
	}
	code, _ := errObj["code"].(string)
	return code
}

// dailyRecordElement は応答の dailyRecords から1要素だけを取り出す。
func dailyRecordElement(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（body=%s）", rec.Code, rec.Body.String())
	}
	records, ok := decodeBody(t, rec)["dailyRecords"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("dailyRecords の件数が 1 でない: %s", rec.Body.String())
	}
	el, ok := records[0].(map[string]any)
	if !ok {
		t.Fatalf("dailyRecords[0] がオブジェクトでない: %#v", records[0])
	}
	return el
}

func todObj(t [2]int) map[string]any {
	return map[string]any{"hour": float64(t[0]), "minute": float64(t[1])}
}

// wantElement は行の時刻と稼働時間から、応答の dailyRecords の1要素の期待値を組み立てる
// （契約 AC-10-1。休憩なしは break が null でキーは残る）。
func wantElement(row specRow) map[string]any {
	brk := any(nil)
	if row.Break != nil {
		brk = map[string]any{"start": todObj(row.Break[0]), "end": todObj(row.Break[1])}
	}
	el := map[string]any{
		"date":      timesTestDate,
		"startTime": todObj(row.Start),
		"endTime":   todObj(row.End),
		"break":     brk,
	}
	if row.WantMinutes >= 0 {
		h := map[string]any{"hours": float64(row.WantMinutes / 60), "minutes": float64(row.WantMinutes % 60)}
		el["workingHours"] = h
		el["roundedWorkingHours"] = h
	}
	return el
}

// breakOf は specRow の休憩を bodyOf の引数の形にする。
func breakOf(row specRow) *[2][2]int { return row.Break }

// ---- 契約 AC-4-5-a・AC-4-5-b: 仕様書 AC-3 の表の全行を通す -------------------------------

// TestEnterDailyRecord_Assembly_SpecTableRows は、daily-record-entry.md AC-3 の 3-B・3-C の表の
// 時刻の組の行を E-3 へ送り、許可の行は 200（契約 AC-4-5-b）、弾く行は 400 INVALID_TIME_COMBINATION
// （契約 AC-4-5-a）になることを検証する。弾いた要求では保存（Tx.Exec・Commit）が行われない。
// 許可の行では、応答の当該日の要素が送った時刻をそのまま持ち、workingHours が行の稼働時間に一致する。
func TestEnterDailyRecord_Assembly_SpecTableRows(t *testing.T) {
	rows := loadDailyRecordSpecRows(t)
	requireRowIDs(t, rows,
		"3-5", "3-5-d", "3-6", "3-7", "3-7-a", "3-9-b", "3-11", "3-11-a", "3-11-b", "3-11-c", // 4-5-b
		"3-5-a", "3-5-b", "3-5-c", "3-5-e", "3-6-a", "3-6-b", "3-8", "3-8-a", "3-8-b", "3-9", "3-9-a", // 4-5-a
	)

	for _, row := range rows {
		t.Run(fmt.Sprintf("%s %v〜%v 休憩=%v", row.ID, row.Start, row.End, row.Break), func(t *testing.T) {
			db := newFakeDB()
			seedEnter(db)

			rec := putDailyRecord(t, db, bodyOf(row.Start, row.End, breakOf(row)))

			if row.Reject {
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400（body=%s）", rec.Code, rec.Body.String())
				}
				if code := errorCodeOf(t, rec); code != "INVALID_TIME_COMBINATION" {
					t.Errorf("error.code = %q, want INVALID_TIME_COMBINATION（契約 AC-4-5-a）", code)
				}
				if n := len(db.tx.execCalls); n != 0 || db.tx.commitCount != 0 {
					t.Errorf("弾いた要求で保存が行われた（Tx.Exec %d 回・Commit %d 回。契約 AC-9-4）", n, db.tx.commitCount)
				}
				return
			}

			if row.WantMinutes < 0 {
				t.Fatalf("許可の行の稼働時間を表から読めなかった")
			}
			got := dailyRecordElement(t, rec)
			if diff := cmp.Diff(wantElement(row), got); diff != "" {
				t.Errorf("応答の当該日の要素が不一致 (-want +got):\n%s（契約 AC-4-5-b）", diff)
			}
			if db.tx.commitCount != 1 {
				t.Errorf("Commit の呼び出し回数 = %d, want 1", db.tx.commitCount)
			}
		})
	}
}

// ---- 契約 AC-4-5: 時刻の値域外 --------------------------------------------------

// TestEnterDailyRecord_Assembly_TimeOfDayOutOfRange は、4つの時刻のどこに値域外の整数が現れても
// 400 TIME_OF_DAY_OUT_OF_RANGE になり（契約 AC-4-5）、INVALID_TIME_COMBINATION と取り違えない
// ことを検証する。値は daily-record-entry.md の 3-1-a（9:10・9:60）・3-2-a（時が負・25）・
// 3-2-b（24:00）の例。弾いた要求では保存が行われない。
func TestEnterDailyRecord_Assembly_TimeOfDayOutOfRange(t *testing.T) {
	positions := []struct {
		name  string
		build func(bad [2]int) string
	}{
		{"startTime", func(bad [2]int) string {
			return bodyOf(bad, [2]int{18, 0}, &[2][2]int{{12, 0}, {13, 0}})
		}},
		{"endTime", func(bad [2]int) string {
			return bodyOf([2]int{9, 0}, bad, &[2][2]int{{12, 0}, {13, 0}})
		}},
		{"break.start", func(bad [2]int) string {
			return bodyOf([2]int{9, 0}, [2]int{18, 0}, &[2][2]int{bad, {13, 0}})
		}},
		{"break.end", func(bad [2]int) string {
			return bodyOf([2]int{9, 0}, [2]int{18, 0}, &[2][2]int{{12, 0}, bad})
		}},
	}
	bads := [][2]int{{9, 10}, {9, 60}, {-1, 0}, {25, 0}, {24, 0}, {9, -15}}

	for _, pos := range positions {
		for _, bad := range bads {
			t.Run(fmt.Sprintf("%s に %v", pos.name, bad), func(t *testing.T) {
				db := newFakeDB()
				seedEnter(db)

				rec := putDailyRecord(t, db, pos.build(bad))

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400（body=%s）", rec.Code, rec.Body.String())
				}
				if code := errorCodeOf(t, rec); code != "TIME_OF_DAY_OUT_OF_RANGE" {
					t.Errorf("error.code = %q, want TIME_OF_DAY_OUT_OF_RANGE（契約 AC-4-5）", code)
				}
				if n := len(db.tx.execCalls); n != 0 || db.tx.commitCount != 0 {
					t.Errorf("弾いた要求で保存が行われた（Tx.Exec %d 回・Commit %d 回）", n, db.tx.commitCount)
				}
			})
		}
	}
}

// TestEnterDailyRecord_Assembly_RetiredWorkingHoursCodeIsNotEmitted は、廃止した
// WORKING_HOURS_OUT_OF_RANGE をどの入力でも出さないことを通しで確かめる（契約 D-16）。
// 旧形式のボディ（workingHours だけ）は項目の欠落＝400 INVALID_REQUEST（契約 AC-4-9-i）。
func TestEnterDailyRecord_Assembly_RetiredWorkingHoursCodeIsNotEmitted(t *testing.T) {
	bodies := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"旧形式のボディ", `{"workingHours":{"hours":8,"minutes":30}}`, "INVALID_REQUEST"},
		{"旧形式の値域外の稼働時間", `{"workingHours":{"hours":25,"minutes":0}}`, "INVALID_REQUEST"},
	}
	for _, tt := range bodies {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			seedEnter(db)

			rec := putDailyRecord(t, db, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400（body=%s）", rec.Code, rec.Body.String())
			}
			code := errorCodeOf(t, rec)
			if code == "WORKING_HOURS_OUT_OF_RANGE" {
				t.Errorf("廃止した code を返した（契約 D-16）")
			}
			if code != tt.wantCode {
				t.Errorf("error.code = %q, want %q（契約 AC-4-9-i）", code, tt.wantCode)
			}
		})
	}
}

// ---- 契約 AC-4-14-a〜e: 保存した時刻の返却と、保存を経た読み直し ------------------------

// TestEnterDailyRecord_Assembly_ReadBackOfSavedTimes は、daily-record-entry.md AC-2-6-a〜2-6-c の入力を
// E-3 で送り（4-14-a〜c）、その応答の当該日の要素と、保存を経て読み直した E-1 の応答の要素が
// 同じであること（4-14-e）を検証する。2-6-b（休憩なし）と 2-6-c（最小の時刻を持つ休憩）を対にする。
//
// E-1 の Fake は、E-3 が Tx 越しに書いた稼働実績の挿入の引数（実装設計 AC-9-16-h ② の12個）から
// 読み出しの行（AC-9-15-i の11個＝先頭の契約識別子を除いたもの）を組み立てて返す。
func TestEnterDailyRecord_Assembly_ReadBackOfSavedTimes(t *testing.T) {
	rows := loadReadBackSpecRows(t)
	requireRowIDs(t, rows, "2-6-a", "2-6-b", "2-6-c")

	for _, row := range rows {
		t.Run(row.ID, func(t *testing.T) {
			db := newFakeDB()
			seedEnter(db)

			put := putDailyRecord(t, db, bodyOf(row.Start, row.End, breakOf(row)))

			putEl := dailyRecordElement(t, put)
			want := wantElement(row)
			if row.WantMinutes < 0 {
				// 2-6-b は稼働時間を書かない。時刻の部分だけを期待値にし、稼働時間は 9時間0分
				// （9:00〜18:00／休憩なし＝daily-record-entry.md 3-11）を別の行から補う。
				want["workingHours"] = map[string]any{"hours": float64(9), "minutes": float64(0)}
				want["roundedWorkingHours"] = map[string]any{"hours": float64(9), "minutes": float64(0)}
			}
			if diff := cmp.Diff(want, putEl); diff != "" {
				t.Errorf("E-3 の応答の当該日の要素が不一致 (-want +got):\n%s（契約 AC-4-14-a〜c）", diff)
			}

			// 4-14-e: 保存した行を E-1 で読み直す
			if len(db.tx.execCalls) != 3 {
				t.Fatalf("Tx.Exec の呼び出し回数 = %d, want 3（勤務月・全削除・挿入1件。AC-9-16-h）", len(db.tx.execCalls))
			}
			insert := db.tx.execCalls[2].args
			if len(insert) != 12 {
				t.Fatalf("挿入の引数の個数 = %d, want 12（AC-9-16-h ②）", len(insert))
			}
			db2 := newFakeDB()
			db2.pushQuery(newFakeRows(contractRow(timesTestContract, "契約T", timesTestEngineer, 140, 0, 180, 0)), nil)
			db2.pushQuery(newFakeRows(workMonthHeaderRow(timesTestContract, 2026, 7, 140, 0, 180, 0, "Draft")), nil)
			db2.pushQuery(newFakeRows(queryRow(insert[1:])), nil)

			get := getWorkMonth(t, db2)

			if diff := cmp.Diff(putEl, dailyRecordElement(t, get)); diff != "" {
				t.Errorf("E-1 の応答の要素が直前の E-3 の応答と不一致 (-want +got):\n%s（契約 AC-4-14-e）", diff)
			}
		})
	}
}

// TestEnterDailyRecord_Assembly_EditToNoBreakLeavesNoOldBreak は、2-6-a の日に 2-6-d（9:00〜18:00／
// 休憩なし）を再入力（編集）したとき、E-3 の応答が 2-6-b と同じ要素になり（4-14-d）、編集前の
// 休憩の時刻を残さないこと（保存する休憩の4列が NULL）、読み直した E-1 も同じ要素であること
// （4-14-e）を検証する。
func TestEnterDailyRecord_Assembly_EditToNoBreakLeavesNoOldBreak(t *testing.T) {
	var a, b specRow
	for _, row := range loadReadBackSpecRows(t) {
		switch row.ID {
		case "2-6-a":
			a = row
		case "2-6-b":
			b = row
		}
	}
	if a.ID == "" || b.ID == "" {
		t.Fatalf("2-6-a / 2-6-b を表から読めなかった")
	}

	db := newFakeDB()
	// 編集前: 2-6-a の稼働実績が保存済み（読み出しの行は AC-9-15-i の11列）
	seedEnter(db, dailyRecordRow(2026, 7, 15, a.Start, a.End, &[4]int{a.Break[0][0], a.Break[0][1], a.Break[1][0], a.Break[1][1]}))

	put := putDailyRecord(t, db, bodyOf(b.Start, b.End, nil)) // 2-6-d

	putEl := dailyRecordElement(t, put)
	want := wantElement(b)
	want["workingHours"] = map[string]any{"hours": float64(9), "minutes": float64(0)}
	want["roundedWorkingHours"] = map[string]any{"hours": float64(9), "minutes": float64(0)}
	if diff := cmp.Diff(want, putEl); diff != "" {
		t.Errorf("編集後の E-3 の応答の要素が 2-6-b と不一致 (-want +got):\n%s（契約 AC-4-14-d）", diff)
	}
	if len(db.tx.execCalls) != 3 {
		t.Fatalf("Tx.Exec の呼び出し回数 = %d, want 3（AC-9-16-h）", len(db.tx.execCalls))
	}
	insert := db.tx.execCalls[2].args
	if len(insert) != 12 {
		t.Fatalf("挿入の引数の個数 = %d, want 12", len(insert))
	}
	for i := 8; i < 12; i++ {
		if p, ok := insert[i].(*int); !ok || p != nil {
			t.Errorf("挿入の引数 ⑨〜⑫ の %d 番目 = %#v, want (*int)(nil)（編集前の休憩の時刻を残さない。AC-9-16-g）", i+1, insert[i])
		}
	}

	db2 := newFakeDB()
	db2.pushQuery(newFakeRows(contractRow(timesTestContract, "契約T", timesTestEngineer, 140, 0, 180, 0)), nil)
	db2.pushQuery(newFakeRows(workMonthHeaderRow(timesTestContract, 2026, 7, 140, 0, 180, 0, "Draft")), nil)
	db2.pushQuery(newFakeRows(queryRow(insert[1:])), nil)
	if diff := cmp.Diff(putEl, dailyRecordElement(t, getWorkMonth(t, db2))); diff != "" {
		t.Errorf("E-1 の応答の要素が直前の E-3 の応答と不一致 (-want +got):\n%s（契約 AC-4-14-e）", diff)
	}
}
