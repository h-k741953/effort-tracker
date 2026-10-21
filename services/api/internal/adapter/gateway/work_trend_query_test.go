package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: 実装設計 AC-9-21（a〜k）・AC-6-11・AC-9-14-f・
// AC-9-15（a〜d・h・i）・AC-9-19-a・AC-12-19-f。契約 AC-13-12。
//
// 1回目の Query が勤務月の行（AC-9-21-d。12個）、2回目が稼働実績の行
// （AC-9-21-e。12個）。workMonthHeaderRow / trendDailyRecordRow（doubles_test.go）で
// 並びと型に組み立てる。Fake の Scan は個数と型が合わない行を失敗にする。

func queryWorkTrends(db *fakeDB) ([]*workmonth.WorkMonth, error) {
	return gateway.NewWorkTrendQuery(db).ListByEngineer(context.Background(), "eng-0001")
}

func viewsOfWorkMonths(months []*workmonth.WorkMonth) []workMonthView {
	var views []workMonthView
	for _, m := range months {
		views = append(views, viewOfWorkMonth(m))
	}
	return views
}

// TestWorkTrendQuery_ListByEngineer_QueryCallsAndOrder は Query が2回で、いずれも
// 引数が技術者識別子 string の1つだけであること、1回目の走査を閉じてから2回目を
// 呼ぶことを検証する（AC-9-21-a・AC-9-21-c・AC-9-14-f・AC-12-19-f）。
func TestWorkTrendQuery_ListByEngineer_QueryCallsAndOrder(t *testing.T) {
	headerRows := newFakeRows(
		workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", nil, nil, nil, nil),
	)
	recordRows := newFakeRows(
		trendDailyRecordRow("ctr-0001", 2026, 8, 1, startB, endB, nil),
	)
	db := newFakeDB()
	db.pushQuery(headerRows, nil)
	db.pushQuery(recordRows, nil)

	if _, err := queryWorkTrends(db); err != nil {
		t.Fatalf("ListByEngineer failed: %v", err)
	}

	if db.queryCalls != 2 {
		t.Fatalf("Query の呼び出し回数 = %d, want 2（勤務月ごとに稼働実績を引かない。AC-9-21-a）", db.queryCalls)
	}
	for i, call := range db.queryLog {
		if diff := cmp.Diff([]any{"eng-0001"}, call.args); diff != "" {
			t.Errorf("%d 回目の Query の引数が不一致 (-want +got):\n%s（AC-9-21-a: 技術者識別子 string の1つだけ）", i+1, diff)
		}
	}
	if len(db.closedBeforeQuery) == 2 && !db.closedBeforeQuery[1] {
		t.Errorf("1回目の走査を閉じる前に2回目の Query を呼んでいる（AC-9-21-c・AC-9-14-f）")
	}
	if !headerRows.closed || !recordRows.closed {
		t.Errorf("走査が閉じられていない: header=%t records=%t（AC-9-21-c）", headerRows.closed, recordRows.closed)
	}
}

// TestWorkTrendQuery_ListByEngineer_AssignsRecordsByContractAndMonth は、2契約 × 2月の
// 勤務月へ稼働実績の行が「契約識別子 × 年 × 月」で割り当たり、戻り値が契約識別子の
// 昇順・同じ契約の中は年月の昇順であること、精算幅・状態・超過／不足が勤務月の行の
// 値（スナップショット）であることを検証する
// （AC-9-21-b・AC-9-21-d・AC-9-21-e・AC-9-21-g・AC-9-21-h・AC-12-19-f）。
// 稼働実績の行は 2回目の並びが固定されないこと（AC-9-21-b）に合わせ、勤務月の順とは
// 無関係な順に置く。
func TestWorkTrendQuery_ListByEngineer_AssignsRecordsByContractAndMonth(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Approved", intPtr(0), intPtr(15), intPtr(0), intPtr(0)),
		workMonthHeaderRow("ctr-0001", 2026, 9, 100, 0, 160, 30, "Draft", nil, nil, nil, nil),
		workMonthHeaderRow("ctr-0002", 2026, 8, 150, 0, 190, 0, "PendingApproval", intPtr(0), intPtr(0), intPtr(3), intPtr(30)),
		workMonthHeaderRow("ctr-0002", 2026, 9, 120, 0, 170, 0, "Draft", nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(
		trendDailyRecordRow("ctr-0002", 2026, 9, 3, startB, endB, nil),
		trendDailyRecordRow("ctr-0001", 2026, 9, 1, startA, endA, &breakA),
		trendDailyRecordRow("ctr-0002", 2026, 8, 2, startB, endB, nil),
		trendDailyRecordRow("ctr-0001", 2026, 8, 5, startB, endB, nil),
		trendDailyRecordRow("ctr-0001", 2026, 8, 2, startB, endB, nil),
	), nil)

	got, err := queryWorkTrends(db)
	if err != nil {
		t.Fatalf("ListByEngineer failed: %v", err)
	}

	want := []workMonthView{
		{
			ContractID: "ctr-0001", Year: 2026, Month: 8, Lower: [2]int{140, 0}, Upper: [2]int{180, 0}, State: "Approved",
			Excess: &[2]int{0, 15}, Shortfall: &[2]int{0, 0},
			Records: []recordView{
				{Date: [3]int{2026, 8, 2}, Start: startB, End: endB, WorkingHours: [2]int{9, 0}},
				{Date: [3]int{2026, 8, 5}, Start: startB, End: endB, WorkingHours: [2]int{9, 0}},
			},
		},
		{
			ContractID: "ctr-0001", Year: 2026, Month: 9, Lower: [2]int{100, 0}, Upper: [2]int{160, 30}, State: "Draft",
			Records: []recordView{
				{Date: [3]int{2026, 9, 1}, Start: startA, End: endA, HasBreak: true, Break: breakA, WorkingHours: [2]int{7, 45}},
			},
		},
		{
			ContractID: "ctr-0002", Year: 2026, Month: 8, Lower: [2]int{150, 0}, Upper: [2]int{190, 0}, State: "PendingApproval",
			Excess: &[2]int{0, 0}, Shortfall: &[2]int{3, 30},
			Records: []recordView{
				{Date: [3]int{2026, 8, 2}, Start: startB, End: endB, WorkingHours: [2]int{9, 0}},
			},
		},
		{
			ContractID: "ctr-0002", Year: 2026, Month: 9, Lower: [2]int{120, 0}, Upper: [2]int{170, 0}, State: "Draft",
			Records: []recordView{
				{Date: [3]int{2026, 9, 3}, Start: startB, End: endB, WorkingHours: [2]int{9, 0}},
			},
		},
	}
	if diff := cmp.Diff(want, viewsOfWorkMonths(got)); diff != "" {
		t.Errorf("復元した集約が不一致 (-want +got):\n%s（AC-9-21-b・g・h）", diff)
	}
}

// TestWorkTrendQuery_ListByEngineer_AssignsRecordsByYear は、月が同じで年だけが異なる
// 2つの勤務月（同じ契約）へ、稼働実績の行が年まで含めて割り当たることを検証する
// （AC-9-21-g「契約識別子 × 年 × 月」。年の比較を落とした実装を Red にする）。
func TestWorkTrendQuery_ListByEngineer_AssignsRecordsByYear(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("ctr-0001", 2025, 8, 140, 0, 180, 0, "Draft", nil, nil, nil, nil),
		workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(
		trendDailyRecordRow("ctr-0001", 2026, 8, 3, startB, endB, nil),
		trendDailyRecordRow("ctr-0001", 2025, 8, 4, startB, endB, nil),
	), nil)

	got, err := queryWorkTrends(db)
	if err != nil {
		t.Fatalf("ListByEngineer failed（年違いの稼働実績を別の年の勤務月へ割り当てると Reconstruct が弾く）: %v", err)
	}

	want := []workMonthView{
		{
			ContractID: "ctr-0001", Year: 2025, Month: 8, Lower: [2]int{140, 0}, Upper: [2]int{180, 0}, State: "Draft",
			Records: []recordView{{Date: [3]int{2025, 8, 4}, Start: startB, End: endB, WorkingHours: [2]int{9, 0}}},
		},
		{
			ContractID: "ctr-0001", Year: 2026, Month: 8, Lower: [2]int{140, 0}, Upper: [2]int{180, 0}, State: "Draft",
			Records: []recordView{{Date: [3]int{2026, 8, 3}, Start: startB, End: endB, WorkingHours: [2]int{9, 0}}},
		},
	}
	if diff := cmp.Diff(want, viewsOfWorkMonths(got)); diff != "" {
		t.Errorf("復元した集約が不一致 (-want +got):\n%s（AC-9-21-g）", diff)
	}
}

// TestWorkTrendQuery_ListByEngineer_MonthWithoutRecords は稼働実績の行が1件も無い
// 勤務月（稼働実績0件の生成済みの勤務月）が、空の稼働実績を持つ集約として返ることを
// 検証する（AC-9-21-h。Draft 以外でも成立する）。
func TestWorkTrendQuery_ListByEngineer_MonthWithoutRecords(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(), nil)

	got, err := queryWorkTrends(db)
	if err != nil {
		t.Fatalf("ListByEngineer failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("集約の数 = %d, want 1", len(got))
	}
	if records := got[0].DailyRecords(); len(records) != 0 {
		t.Errorf("稼働実績の数 = %d, want 0", len(records))
	}
}

// TestWorkTrendQuery_ListByEngineer_EmptyIsEmptySliceNotError は0行で空のスライス
// （非 nil）・エラーなしを返し、ErrWorkMonthNotFound へ変換しないことを検証する
// （AC-9-21-k・AC-6-11-f・AC-12-19-f）。
func TestWorkTrendQuery_ListByEngineer_EmptyIsEmptySliceNotError(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(), nil)
	db.pushQuery(newFakeRows(), nil)

	got, err := queryWorkTrends(db)
	if errors.Is(err, port.ErrWorkMonthNotFound) {
		t.Fatalf("0件が port.ErrWorkMonthNotFound へ変換されている（AC-9-21-k）")
	}
	if err != nil {
		t.Fatalf("ListByEngineer failed: %v", err)
	}
	if got == nil {
		t.Errorf("0件のとき nil が返っている（空のスライスであること。AC-6-11-f）")
	}
	if len(got) != 0 {
		t.Errorf("集約の数 = %d, want 0", len(got))
	}
}

// TestWorkTrendQuery_ListByEngineer_BrokenRowsReturnWholeError は、1行でも構築・
// 復元に失敗したら全体をそのエラーで返し、一部の集約を返さないことを検証する
// （AC-9-21-f・g・h・i・契約 AC-13-12・AC-12-19-f）。
func TestWorkTrendQuery_ListByEngineer_BrokenRowsReturnWholeError(t *testing.T) {
	goodHeader := workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", nil, nil, nil, nil)
	tests := []struct {
		name    string
		headers [][]any
		records [][]any
	}{
		{
			name:    "一致する勤務月の無い稼働実績の行（契約識別子違い）は ErrInvalidValue（AC-9-21-g）",
			headers: [][]any{goodHeader},
			records: [][]any{trendDailyRecordRow("ctr-0002", 2026, 8, 1, startB, endB, nil)},
		},
		{
			name:    "一致する勤務月の無い稼働実績の行（月違い）は ErrInvalidValue（AC-9-21-g）",
			headers: [][]any{goodHeader},
			records: [][]any{trendDailyRecordRow("ctr-0001", 2026, 9, 1, startB, endB, nil)},
		},
		{
			name:    "Reconstruct の失敗: Draft に超過が確定している（AC-9-21-h・AC-9-15-d）",
			headers: [][]any{workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", intPtr(0), intPtr(15), intPtr(0), intPtr(0))},
			records: nil,
		},
		{
			name:    "Reconstruct の失敗: 未定義の状態（AC-9-15-f）",
			headers: [][]any{workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Bogus", nil, nil, nil, nil)},
			records: nil,
		},
		{
			name:    "Reconstruct の失敗: 同じ対象日の稼働実績が重複（AC-9-21-h・AC-9-15-d）",
			headers: [][]any{goodHeader},
			records: [][]any{trendDailyRecordRow("ctr-0001", 2026, 8, 1, startB, endB, nil), trendDailyRecordRow("ctr-0001", 2026, 8, 1, startB, endB, nil)},
		},
		{
			name:    "稼働実績の行: 開始時刻が NULL（AC-9-21-f・AC-9-15-i (i)）",
			headers: [][]any{goodHeader},
			records: [][]any{append([]any{"ctr-0001"}, dailyRecordRowRaw(2026, 8, 1, [8]*int{nil, intPtr(0), intPtr(18), intPtr(0), nil, nil, nil, nil})...)},
		},
		{
			name:    "稼働実績の行: 休憩の一部だけ NULL（AC-9-21-f・AC-9-15-i (ii)）",
			headers: [][]any{goodHeader},
			records: [][]any{append([]any{"ctr-0001"}, dailyRecordRowRaw(2026, 8, 1, [8]*int{intPtr(9), intPtr(0), intPtr(18), intPtr(0), intPtr(12), nil, intPtr(13), intPtr(0)})...)},
		},
		{
			name:    "稼働実績の行: 時刻が値域外（AC-9-21-f・AC-9-15-i (iii)）",
			headers: [][]any{goodHeader},
			records: [][]any{append([]any{"ctr-0001"}, dailyRecordRowRaw(2026, 8, 1, [8]*int{intPtr(25), intPtr(0), intPtr(18), intPtr(0), nil, nil, nil, nil})...)},
		},
		{
			name:    "勤務月の行: 精算幅の下限 > 上限（AC-9-21-h）",
			headers: [][]any{workMonthHeaderRow("ctr-0001", 2026, 8, 180, 0, 140, 0, "Draft", nil, nil, nil, nil)},
			records: nil,
		},
		{
			name:    "2行目の勤務月が破損していても全体を失敗にする（AC-9-21-i）",
			headers: [][]any{goodHeader, workMonthHeaderRow("ctr-0001", 2026, 9, 140, 0, 180, 0, "Draft", intPtr(1), intPtr(0), intPtr(1), intPtr(0))},
			records: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			db.pushQuery(newFakeRows(tt.headers...), nil)
			recordRows := newFakeRows(tt.records...)
			db.pushQuery(recordRows, nil)

			got, err := queryWorkTrends(db)
			if !errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("ListByEngineer のエラー = %v, want errors.Is(err, workmonth.ErrInvalidValue)", err)
			}
			if len(got) != 0 {
				t.Errorf("エラーなのに集約を返している: %d 件（一部の集約を返さない。AC-9-21-i）", len(got))
			}
		})
	}
}

// TestWorkTrendQuery_ListByEngineer_ClosesRowsAndPassesDriverErrors は走査を全経路で
// 閉じ、ドライバ由来のエラーをそのまま返すことを検証する（AC-9-21-c・AC-9-19-a）。
func TestWorkTrendQuery_ListByEngineer_ClosesRowsAndPassesDriverErrors(t *testing.T) {
	scanErr := errors.New("fake: scan failed")
	iterErr := errors.New("fake: iteration failed mid-stream")
	queryErr := errors.New("fake: connection reset by peer")
	goodHeader := workMonthHeaderRow("ctr-0001", 2026, 8, 140, 0, 180, 0, "Draft", nil, nil, nil, nil)

	t.Run("1回目の Query 自体の失敗はそのまま返し、2回目を呼ばない", func(t *testing.T) {
		db := newFakeDB()
		db.pushQuery(nil, queryErr)
		_, err := queryWorkTrends(db)
		if !errors.Is(err, queryErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, queryErr)", err)
		}
		if db.queryCalls != 1 {
			t.Errorf("Query の呼び出し回数 = %d, want 1", db.queryCalls)
		}
	})

	t.Run("1回目の Scan の失敗はそのまま返し、閉じ、2回目を呼ばない", func(t *testing.T) {
		rows := newFakeRows(goodHeader)
		rows.scanErr = scanErr
		db := newFakeDB()
		db.pushQuery(rows, nil)
		_, err := queryWorkTrends(db)
		if !errors.Is(err, scanErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, scanErr)", err)
		}
		if !rows.closed {
			t.Errorf("Scan の失敗後に Rows が Close されていない（AC-9-21-c）")
		}
		if db.queryCalls != 1 {
			t.Errorf("Query の呼び出し回数 = %d, want 1（失敗後に2回目を呼ばない）", db.queryCalls)
		}
	})

	t.Run("1回目の走査中のエラーはそのまま返し、閉じる", func(t *testing.T) {
		rows := newFakeRows()
		rows.iterErr = iterErr
		db := newFakeDB()
		db.pushQuery(rows, nil)
		_, err := queryWorkTrends(db)
		if !errors.Is(err, iterErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, iterErr)", err)
		}
		if !rows.closed {
			t.Errorf("走査中のエラー後に Rows が Close されていない（AC-9-21-c）")
		}
	})

	t.Run("2回目の Query 自体の失敗はそのまま返し、1回目の走査は閉じている", func(t *testing.T) {
		headerRows := newFakeRows(goodHeader)
		db := newFakeDB()
		db.pushQuery(headerRows, nil)
		db.pushQuery(nil, queryErr)
		_, err := queryWorkTrends(db)
		if !errors.Is(err, queryErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, queryErr)", err)
		}
		if !headerRows.closed {
			t.Errorf("1回目の走査が閉じられていない（AC-9-21-c）")
		}
	})

	t.Run("2回目の Scan の失敗はそのまま返し、閉じる", func(t *testing.T) {
		recordRows := newFakeRows(trendDailyRecordRow("ctr-0001", 2026, 8, 1, startB, endB, nil))
		recordRows.scanErr = scanErr
		db := newFakeDB()
		db.pushQuery(newFakeRows(goodHeader), nil)
		db.pushQuery(recordRows, nil)
		_, err := queryWorkTrends(db)
		if !errors.Is(err, scanErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, scanErr)", err)
		}
		if !recordRows.closed {
			t.Errorf("2回目の Scan の失敗後に Rows が Close されていない（AC-9-21-c）")
		}
	})

	t.Run("2回目の走査中のエラーはそのまま返し、閉じる", func(t *testing.T) {
		recordRows := newFakeRows()
		recordRows.iterErr = iterErr
		db := newFakeDB()
		db.pushQuery(newFakeRows(goodHeader), nil)
		db.pushQuery(recordRows, nil)
		_, err := queryWorkTrends(db)
		if !errors.Is(err, iterErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, iterErr)", err)
		}
		if !recordRows.closed {
			t.Errorf("2回目の走査中のエラー後に Rows が Close されていない（AC-9-21-c）")
		}
	})
}
