package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-15・AC-9-16・AC-9-19-a・AC-12-11。
//
// db.Query は「勤務月のヘッダ行（0 or 1 行）」→「稼働実績の行（0..N 行）」の
// 順に呼ばれることを前提とする（本ファイルの前書き＝doubles_test.go を参照）。

func mustContractID(t *testing.T, value string) workmonth.ContractID {
	t.Helper()
	id, err := workmonth.NewContractID(value)
	if err != nil {
		t.Fatalf("NewContractID(%q) failed: %v", value, err)
	}
	return id
}

func mustYearMonth(t *testing.T, year, month int) workmonth.YearMonth {
	t.Helper()
	ym, err := workmonth.NewYearMonth(year, month)
	if err != nil {
		t.Fatalf("NewYearMonth(%d, %d) failed: %v", year, month, err)
	}
	return ym
}

func mustSettlementRange(t *testing.T, lowerH, lowerM, upperH, upperM int) workmonth.SettlementRange {
	t.Helper()
	lower, err := workmonth.NewWorkingHours(lowerH, lowerM)
	if err != nil {
		t.Fatalf("NewWorkingHours(%d, %d) failed: %v", lowerH, lowerM, err)
	}
	upper, err := workmonth.NewWorkingHours(upperH, upperM)
	if err != nil {
		t.Fatalf("NewWorkingHours(%d, %d) failed: %v", upperH, upperM, err)
	}
	r, err := workmonth.NewSettlementRange(lower, upper)
	if err != nil {
		t.Fatalf("NewSettlementRange failed: %v", err)
	}
	return r
}

func mustTimeOfDay(t *testing.T, hour, minute int) workmonth.TimeOfDay {
	t.Helper()
	tod, err := workmonth.NewTimeOfDay(hour, minute)
	if err != nil {
		t.Fatalf("NewTimeOfDay(%d, %d) failed: %v", hour, minute, err)
	}
	return tod
}

// mustDailyRecord は時刻から稼働実績を構築する。start・end は {時, 分}。
// brk が nil なら休憩なし、非 nil なら {休憩開始の時, 分, 休憩終了の時, 分}。
func mustDailyRecord(t *testing.T, year, month, day int, start, end [2]int, brk *[4]int) workmonth.DailyRecord {
	t.Helper()
	date, err := workmonth.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("NewDate failed: %v", err)
	}
	var b *workmonth.Break
	if brk != nil {
		built, err := workmonth.NewBreak(mustTimeOfDay(t, brk[0], brk[1]), mustTimeOfDay(t, brk[2], brk[3]))
		if err != nil {
			t.Fatalf("NewBreak failed: %v", err)
		}
		b = &built
	}
	record, err := workmonth.NewDailyRecord(date, mustTimeOfDay(t, start[0], start[1]), mustTimeOfDay(t, end[0], end[1]), b)
	if err != nil {
		t.Fatalf("NewDailyRecord failed: %v", err)
	}
	return record
}

// 仕様書 daily-record-entry.md AC-2-6 の入力（行の時刻のそのまま。表の値は仕様書が持つ）。
var (
	// 2-6-a: 9:15〜17:45／休憩 12:30〜13:15（稼働時間 7時間45分）
	startA, endA, breakA = [2]int{9, 15}, [2]int{17, 45}, [4]int{12, 30, 13, 15}
	// 2-6-b: 9:00〜18:00／休憩なし
	startB, endB = [2]int{9, 0}, [2]int{18, 0}
	// 2-6-c: 0:00〜23:45／休憩 0:00〜0:15（稼働時間 23時間30分）
	startC, endC, breakC = [2]int{0, 0}, [2]int{23, 45}, [4]int{0, 0, 0, 15}
)

// workMonthView は集約を go-cmp で比べるための射影。公開アクセサだけを読み、
// 実装の非公開フィールドの形に依存しない。
type workMonthView struct {
	ContractID string
	Year       int
	Month      int
	Lower      [2]int
	Upper      [2]int
	State      string
	Excess     *[2]int
	Shortfall  *[2]int
	Records    []recordView
}

type recordView struct {
	Date         [3]int
	Start        [2]int
	End          [2]int
	HasBreak     bool
	Break        [4]int
	WorkingHours [2]int
}

func viewOfWorkMonth(wm *workmonth.WorkMonth) workMonthView {
	v := workMonthView{
		ContractID: wm.ContractID().String(),
		Year:       wm.YearMonth().Year(),
		Month:      wm.YearMonth().Month(),
		Lower:      [2]int{wm.SettlementRange().LowerBound().Hours(), wm.SettlementRange().LowerBound().Minutes()},
		Upper:      [2]int{wm.SettlementRange().UpperBound().Hours(), wm.SettlementRange().UpperBound().Minutes()},
		State:      string(wm.State()),
	}
	if e, ok := wm.Excess(); ok {
		v.Excess = &[2]int{e.Hours(), e.Minutes()}
	}
	if sf, ok := wm.Shortfall(); ok {
		v.Shortfall = &[2]int{sf.Hours(), sf.Minutes()}
	}
	for _, r := range wm.DailyRecords() {
		rv := recordView{
			Date:         [3]int{r.Date().Year(), r.Date().Month(), r.Date().Day()},
			Start:        [2]int{r.StartTime().Hour(), r.StartTime().Minute()},
			End:          [2]int{r.EndTime().Hour(), r.EndTime().Minute()},
			WorkingHours: [2]int{r.WorkingHours().Hours(), r.WorkingHours().Minutes()},
		}
		if b, ok := r.Break(); ok {
			rv.HasBreak = true
			rv.Break = [4]int{b.Start().Hour(), b.Start().Minute(), b.End().Hour(), b.End().Minute()}
		}
		v.Records = append(v.Records, rv)
	}
	return v
}

// ---- ① 行 → 集約の変換（AC-9-15） -----------------------------------------

// TestWorkMonthRepository_Find_ReconstructsFromRows は、Fake が返す行から
// Reconstruct へ渡る引数が AC-9-15-b のとおりであることを検証する。
// 精算幅は「行の値」であり（契約の現在値は Find に渡らないため、これが唯一の
// 情報源になる＝AC-9-15-b）、稼働実績は「対象日と保存した時刻」を持ち、
// 稼働時間は行から読まず DailyRecord が時刻から算出する（AC-9-15-b・AC-3-13）。
// 行は AC-9-15-i の11列の並び。入力は daily-record-entry.md AC-2-6-a〜2-6-c。
func TestWorkMonthRepository_Find_ReconstructsFromRows(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("contract-1", 2026, 7, 8, 0, 20, 0, string(workmonth.StateDraft), nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(
		dailyRecordRow(2026, 7, 1, startA, endA, &breakA),
		dailyRecordRow(2026, 7, 2, startB, endB, nil),
		dailyRecordRow(2026, 7, 3, startC, endC, &breakC),
	), nil)

	repo := gateway.NewWorkMonthRepository(db)
	got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	want := workMonthView{
		ContractID: "contract-1", Year: 2026, Month: 7,
		Lower: [2]int{8, 0}, Upper: [2]int{20, 0},
		State: string(workmonth.StateDraft),
		Records: []recordView{
			// 2-6-a: 休憩あり。稼働時間 7時間45分は時刻からの算出値
			{Date: [3]int{2026, 7, 1}, Start: startA, End: endA, HasBreak: true, Break: breakA, WorkingHours: [2]int{7, 45}},
			// 2-6-b: 休憩なし。休憩の時刻を 0:00 等で埋めない
			{Date: [3]int{2026, 7, 2}, Start: startB, End: endB, HasBreak: false, Break: [4]int{}, WorkingHours: [2]int{9, 0}},
			// 2-6-c: 最小の時刻を持つ休憩は「休憩あり」（休憩なしと取り違えない）。稼働時間 23時間30分
			{Date: [3]int{2026, 7, 3}, Start: startC, End: endC, HasBreak: true, Break: breakC, WorkingHours: [2]int{23, 30}},
		},
	}
	if diff := cmp.Diff(want, viewOfWorkMonth(got)); diff != "" {
		t.Errorf("Find が返す集約が不一致 (-want +got):\n%s（AC-9-15-b・AC-9-15-i）", diff)
	}
}

// TestWorkMonthRepository_Find_DailyRecordQueryArguments は、稼働実績を引く Query の引数が
// 既存のまま（契約識別子 string・年 int・月 int の3つ）であることを検証する（AC-9-15-i）。
func TestWorkMonthRepository_Find_DailyRecordQueryArguments(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("contract-1", 2026, 7, 8, 0, 20, 0, string(workmonth.StateDraft), nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(), nil)

	repo := gateway.NewWorkMonthRepository(db)
	if _, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7)); err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	if len(db.queryLog) != 2 {
		t.Fatalf("Query の呼び出し回数 = %d, want 2（勤務月の行 → 稼働実績の行。AC-9-15-i）", len(db.queryLog))
	}
	want := []any{"contract-1", 2026, 7}
	if diff := cmp.Diff(want, db.queryLog[1].args); diff != "" {
		t.Errorf("稼働実績を引く Query の引数が不一致 (-want +got):\n%s（AC-9-15-i）", diff)
	}
}

// TestWorkMonthRepository_Find_CorruptedDailyRecordRows は、破損した稼働実績の行を
// ErrInvalidValue で返し、利用者向けの時刻の番兵（AC-3-14）にも他の番兵にも
// 当たらないことを検証する（AC-9-15-h・AC-9-15-i・AC-12-18 ② (iv)）。
// 永続化行の破損を 400 として外へ出さないため（AC-11-5・AC-11-13）。
//
// 正常な行（2-6-a）と対にして置く（対にしないと「常に ErrInvalidValue を返す」実装が
// Green になる）。時刻の値は daily-record-entry.md AC-3 の「弾く」行の例を使う。
func TestWorkMonthRepository_Find_CorruptedDailyRecordRows(t *testing.T) {
	p := intPtr
	n := (*int)(nil)
	tests := []struct {
		name    string
		row     []any
		wantErr bool
	}{
		{name: "正常: 2-6-a の行", row: dailyRecordRow(2026, 7, 1, startA, endA, &breakA), wantErr: false},
		{name: "正常: 2-6-b の行（休憩の4値がすべて nil）", row: dailyRecordRow(2026, 7, 1, startB, endB, nil), wantErr: false},
		{name: "3-2-b: 終了時刻の時が 24", row: dailyRecordRow(2026, 7, 1, [2]int{9, 0}, [2]int{24, 0}, nil), wantErr: true},
		{name: "3-1-a: 開始時刻の分が 10", row: dailyRecordRow(2026, 7, 1, [2]int{9, 10}, [2]int{18, 0}, nil), wantErr: true},
		{name: "3-5-a: 終了＝開始", row: dailyRecordRow(2026, 7, 1, [2]int{9, 0}, [2]int{9, 0}, nil), wantErr: true},
		{name: "3-5-b: 終了が開始より前", row: dailyRecordRow(2026, 7, 1, [2]int{18, 0}, [2]int{9, 0}, nil), wantErr: true},
		{name: "3-6-a: 休憩終了＝休憩開始", row: dailyRecordRow(2026, 7, 1, [2]int{9, 0}, [2]int{18, 0}, &[4]int{12, 0, 12, 0}), wantErr: true},
		{name: "3-8: 休憩開始が開始より前", row: dailyRecordRow(2026, 7, 1, [2]int{9, 0}, [2]int{18, 0}, &[4]int{8, 45, 10, 0}), wantErr: true},
		{name: "3-9: 稼働 0 になる休憩", row: dailyRecordRow(2026, 7, 1, [2]int{9, 0}, [2]int{18, 0}, &[4]int{9, 0, 18, 0}), wantErr: true},
		{name: "休憩の4値の一部だけ nil: ⑧休憩開始の時のみ nil", row: dailyRecordRowRaw(2026, 7, 1, [8]*int{p(9), p(0), p(18), p(0), n, p(0), p(13), p(0)}), wantErr: true},
		{name: "休憩の4値の一部だけ nil: ⑪休憩終了の分のみ nil", row: dailyRecordRowRaw(2026, 7, 1, [8]*int{p(9), p(0), p(18), p(0), p(12), p(0), p(13), n}), wantErr: true},
		{name: "休憩の4値の一部だけ nil: 休憩開始の2列は値あり・休憩終了の2列が nil", row: dailyRecordRowRaw(2026, 7, 1, [8]*int{p(9), p(0), p(18), p(0), p(12), p(0), n, n}), wantErr: true},
		{name: "開始時刻の時 ④ が nil", row: dailyRecordRowRaw(2026, 7, 1, [8]*int{n, p(0), p(18), p(0), n, n, n, n}), wantErr: true},
		{name: "開始時刻の分 ⑤ が nil", row: dailyRecordRowRaw(2026, 7, 1, [8]*int{p(9), n, p(18), p(0), n, n, n, n}), wantErr: true},
		{name: "終了時刻の時 ⑥ が nil", row: dailyRecordRowRaw(2026, 7, 1, [8]*int{p(9), p(0), n, p(0), n, n, n, n}), wantErr: true},
		{name: "終了時刻の分 ⑦ が nil", row: dailyRecordRowRaw(2026, 7, 1, [8]*int{p(9), p(0), p(18), n, n, n, n, n}), wantErr: true},
	}

	userFacing := []struct {
		name string
		err  error
	}{
		{"ErrTimeOfDayOutOfRange", workmonth.ErrTimeOfDayOutOfRange},
		{"ErrInvalidTimeCombination", workmonth.ErrInvalidTimeCombination},
		{"ErrNotEditable", workmonth.ErrNotEditable},
		{"ErrDateOutOfMonth", workmonth.ErrDateOutOfMonth},
		{"ErrFutureDate", workmonth.ErrFutureDate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			db.pushQuery(newFakeRows(
				workMonthHeaderRow("contract-1", 2026, 7, 8, 0, 20, 0, string(workmonth.StateDraft), nil, nil, nil, nil),
			), nil)
			db.pushQuery(newFakeRows(tt.row), nil)

			repo := gateway.NewWorkMonthRepository(db)
			got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("正常な行で Find が失敗した: %v", err)
				}
				if got == nil {
					t.Fatalf("正常な行で Find の戻り値 = nil")
				}
				return
			}
			if !errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("Find のエラー = %v, want errors.Is(err, ErrInvalidValue)（AC-9-15-h）", err)
			}
			for _, u := range userFacing {
				if errors.Is(err, u.err) {
					t.Errorf("Find のエラー %v が利用者向けの番兵 %s に当たる（永続化行の破損を 400 として外へ出さない。AC-9-15-h）", err, u.name)
				}
			}
			if got != nil {
				t.Errorf("Find の戻り値 = %+v, want nil（破損した行から集約を返さない）", got)
			}
		})
	}
}

// TestWorkMonthRepository_Find_ExcessShortfallNullMapsToUndetermined は、
// 超過／不足の列が NULL のとき「未確定」として写ることを検証する（AC-9-15-c）。
// gateway 側で再計算・補完しない（アクセサの第2戻り値が false のまま）。
func TestWorkMonthRepository_Find_ExcessShortfallNullMapsToUndetermined(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("contract-1", 2026, 7, 8, 0, 20, 0, string(workmonth.StateDraft), nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(), nil)

	repo := gateway.NewWorkMonthRepository(db)
	got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if got == nil {
		t.Fatalf("Find の戻り値 = nil, want 非 nil の集約")
	}

	if _, ok := got.Excess(); ok {
		t.Errorf("Excess の第2戻り値 = true, want false（NULL は未確定。AC-9-15-c）")
	}
	if _, ok := got.Shortfall(); ok {
		t.Errorf("Shortfall の第2戻り値 = true, want false（NULL は未確定。AC-9-15-c）")
	}
}

// TestWorkMonthRepository_Find_ExcessShortfallValueMapsToDetermined は、
// 超過／不足の列に値があるとき「確定済み」としてそのまま写ることを検証する
// （AC-9-15-c。gateway 側で再計算しない）。
func TestWorkMonthRepository_Find_ExcessShortfallValueMapsToDetermined(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow(
			"contract-1", 2026, 7, 8, 0, 20, 0, string(workmonth.StatePendingApproval),
			intPtr(0), intPtr(0), intPtr(1), intPtr(30),
		),
	), nil)
	db.pushQuery(newFakeRows(), nil)

	repo := gateway.NewWorkMonthRepository(db)
	got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if got == nil {
		t.Fatalf("Find の戻り値 = nil, want 非 nil の集約")
	}

	excess, ok := got.Excess()
	if !ok {
		t.Fatalf("Excess の第2戻り値 = false, want true（値ありは確定済み。AC-9-15-c）")
	}
	if excess.Hours() != 0 || excess.Minutes() != 0 {
		t.Errorf("Excess = %d時間%d分, want 0時間0分", excess.Hours(), excess.Minutes())
	}
	shortfall, ok := got.Shortfall()
	if !ok {
		t.Fatalf("Shortfall の第2戻り値 = false, want true（値ありは確定済み。AC-9-15-c）")
	}
	if shortfall.Hours() != 1 || shortfall.Minutes() != 30 {
		t.Errorf("Shortfall = %d時間%d分, want 1時間30分", shortfall.Hours(), shortfall.Minutes())
	}
}

// TestWorkMonthRepository_Find_PropagatesReconstructError は、Reconstruct の
// 失敗（集約の不変条件違反）を握り潰さずそのまま返すことを検証する（AC-9-15-d）。
// 同一対象日の重複行は「対象日で一意」の不変条件（AC-2-6）に違反する。
func TestWorkMonthRepository_Find_PropagatesReconstructError(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("contract-1", 2026, 7, 8, 0, 20, 0, string(workmonth.StateDraft), nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(
		dailyRecordRow(2026, 7, 1, startB, endB, nil),
		dailyRecordRow(2026, 7, 1, startA, endA, &breakA),
	), nil)

	repo := gateway.NewWorkMonthRepository(db)
	got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))
	if !errors.Is(err, workmonth.ErrInvalidValue) {
		t.Fatalf("Find のエラー = %v, want errors.Is(err, workmonth.ErrInvalidValue)（AC-9-15-d）", err)
	}
	if got != nil {
		t.Errorf("Find の戻り値 = %+v, want nil（Reconstruct が失敗したら集約を返さない）", got)
	}
}

// TestWorkMonthRepository_Find_NotFoundReturnsSentinel は、行が無いとき
// port.ErrWorkMonthNotFound を返し、空の集約を作らないことを検証する（AC-9-15-e）。
func TestWorkMonthRepository_Find_NotFoundReturnsSentinel(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(), nil)

	repo := gateway.NewWorkMonthRepository(db)
	got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))
	if !errors.Is(err, port.ErrWorkMonthNotFound) {
		t.Fatalf("Find のエラー = %v, want errors.Is(err, port.ErrWorkMonthNotFound)（AC-9-15-e）", err)
	}
	if got != nil {
		t.Errorf("Find の戻り値 = %+v, want nil（未生成と生成済みを区別するため空の集約を作らない。AC-9-15-e）", got)
	}
}

// ---- ③ エラーの変換（AC-9-19） ---------------------------------------------

// TestWorkMonthRepository_Find_OtherDriverErrorPassedThrough は、「行が無い」
// 以外のドライバ由来のエラーを port の番兵へ変換せずそのまま返すことを検証する
// （AC-9-19-a）。
func TestWorkMonthRepository_Find_OtherDriverErrorPassedThrough(t *testing.T) {
	driverErr := errors.New("connection reset by peer")
	db := newFakeDB()
	db.pushQuery(nil, driverErr)

	repo := gateway.NewWorkMonthRepository(db)
	_, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))
	if !errors.Is(err, driverErr) {
		t.Fatalf("Find のエラー = %v, want errors.Is(err, driverErr)（AC-9-19-a: それ以外はそのまま返す）", err)
	}
	if errors.Is(err, port.ErrWorkMonthNotFound) {
		t.Errorf("Find のエラーが port.ErrWorkMonthNotFound に化けている（AC-9-19-a 違反）: %v", err)
	}
}

// ---- ②集約 → 行の書き込み（AC-9-16）／④トランザクションの使い方（AC-9-16-a） ----

// closedWorkMonth は締め済（PendingApproval・超過／不足確定済み）の勤務月を組み立てる。
func closedWorkMonth(t *testing.T) *workmonth.WorkMonth {
	t.Helper()
	wm, err := workmonth.New(mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7), mustSettlementRange(t, 8, 0, 20, 0))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	today := mustDateOnly(t, 2026, 7, 31)
	if err := wm.EnterDailyRecord(mustDailyRecord(t, 2026, 7, 1, startA, endA, &breakA), today); err != nil {
		t.Fatalf("EnterDailyRecord failed: %v", err)
	}
	if err := wm.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	return wm
}

func mustDateOnly(t *testing.T, year, month, day int) workmonth.Date {
	t.Helper()
	d, err := workmonth.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("NewDate failed: %v", err)
	}
	return d
}

// Save が Tx 越しに行う Exec の呼び出しは位置で見分ける（AC-9-16-h ①・AC-12-18 ②）。
// 勤務月の行の書き込みと稼働実績の挿入は、いずれも12引数になり、引数の個数では
// 見分けられない。SQL 文そのものは検査しない（AC-13-18）。
//
//	1回目: 勤務月の行の書き込み（12引数）
//	2回目: 当該勤務月の稼働実績の行の全削除（3引数: 契約識別子・年・月）
//	3回目以降: 稼働実績1件ごとの挿入（12引数）。DailyRecords() の並び（対象日の昇順）で1件1回

// nilInt は NULL を表す (*int)(nil)。
var nilInt = (*int)(nil)

// insertArgs は稼働実績1件の挿入の引数（AC-9-16-h ②の12個の並び）を組み立てる。
func insertArgs(contractID string, year, month, day int, start, end [2]int, brk *[4]int) []any {
	args := []any{contractID, year, month, day, start[0], start[1], end[0], end[1]}
	if brk == nil {
		return append(args, nilInt, nilInt, nilInt, nilInt)
	}
	return append(args, intPtr(brk[0]), intPtr(brk[1]), intPtr(brk[2]), intPtr(brk[3]))
}

func argsOf(calls []execCall) [][]any {
	out := make([][]any, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.args)
	}
	return out
}

// TestWorkMonthRepository_Save_CommitsAfterWritingBothTables は、Save が
// Begin で得た Tx 越しに勤務月の行と稼働実績の行の両方を書き、成功時に確定を
// 1回呼ぶことを検証する（AC-9-16-a・AC-10-7・AC-12-11④）。
func TestWorkMonthRepository_Save_CommitsAfterWritingBothTables(t *testing.T) {
	db := newFakeDB()
	tx := newFakeTx()
	db.pushBegin(tx, nil)

	repo := gateway.NewWorkMonthRepository(db)
	target := closedWorkMonth(t)
	if err := repo.Save(context.Background(), target); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if db.beginCalls != 1 {
		t.Errorf("Begin の呼び出し回数 = %d, want 1（AC-9-16-a）", db.beginCalls)
	}
	if tx.commitCount != 1 {
		t.Errorf("Commit の呼び出し回数 = %d, want 1", tx.commitCount)
	}
	if tx.rollbackCount != 0 {
		t.Errorf("Rollback の呼び出し回数 = %d, want 0（成功時は取消しを呼ばない）", tx.rollbackCount)
	}
	if len(tx.execCalls) == 0 {
		t.Fatalf("Tx.Exec が1度も呼ばれていない（勤務月の行と稼働実績の行の両方を書く。AC-9-16-a）")
	}
}

// TestWorkMonthRepository_Save_ExecOrderAndArguments は、Tx 越しの Exec の呼び出し順と
// 稼働実績の挿入の引数（AC-9-16-h）を検証する。
//  1. 勤務月の行（12引数）→ 2. 稼働実績の全削除（3引数）→ 3. 以降は稼働実績1件ごとの挿入（12引数）
//  2. 挿入の引数は時刻のみ（稼働時間・丸め値を含まない。AC-9-16-c）。休憩なしは ⑨〜⑫ が
//     4つとも (*int)(nil)、休憩ありは4つとも非 nil（AC-9-16-g）。
//
// 保存後の稼働実績の行集合が集約の稼働実績と一致すること（削除された対象日の行を残さない。
// AC-9-16-b）もここで固定する。入力は daily-record-entry.md AC-2-6-a〜2-6-c。
func TestWorkMonthRepository_Save_ExecOrderAndArguments(t *testing.T) {
	db := newFakeDB()
	tx := newFakeTx()
	db.pushBegin(tx, nil)

	repo := gateway.NewWorkMonthRepository(db)
	wm, err := workmonth.New(mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7), mustSettlementRange(t, 8, 0, 20, 0))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	today := mustDateOnly(t, 2026, 7, 31)
	// 入力順は昇順でない（挿入が DailyRecords() の昇順に従うことを示す）
	for _, r := range []workmonth.DailyRecord{
		mustDailyRecord(t, 2026, 7, 3, startC, endC, &breakC),
		mustDailyRecord(t, 2026, 7, 1, startA, endA, &breakA),
		mustDailyRecord(t, 2026, 7, 2, startB, endB, nil),
		mustDailyRecord(t, 2026, 7, 4, startB, endB, nil),
	} {
		if err := wm.EnterDailyRecord(r, today); err != nil {
			t.Fatalf("EnterDailyRecord failed: %v", err)
		}
	}
	// 7/4 を削除する。保存される稼働実績の行は 7/1〜7/3 のみになるはず（AC-9-16-b）。
	if err := wm.DeleteDailyRecord(mustDateOnly(t, 2026, 7, 4)); err != nil {
		t.Fatalf("DeleteDailyRecord failed: %v", err)
	}

	if err := repo.Save(context.Background(), wm); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if len(tx.execCalls) != 5 {
		t.Fatalf("Tx.Exec の呼び出し回数 = %d, want 5（勤務月 1 + 全削除 1 + 稼働実績 3。AC-9-16-h ①）", len(tx.execCalls))
	}
	if n := len(tx.execCalls[0].args); n != 12 {
		t.Errorf("1回目（勤務月の行）の引数の個数 = %d, want 12", n)
	}
	wantDelete := []any{"contract-1", 2026, 7}
	if diff := cmp.Diff(wantDelete, tx.execCalls[1].args); diff != "" {
		t.Errorf("2回目（稼働実績の全削除）の引数が不一致 (-want +got):\n%s（AC-9-16-h ①・AC-9-16-b）", diff)
	}
	wantInserts := [][]any{
		insertArgs("contract-1", 2026, 7, 1, startA, endA, &breakA),
		insertArgs("contract-1", 2026, 7, 2, startB, endB, nil),
		insertArgs("contract-1", 2026, 7, 3, startC, endC, &breakC),
	}
	if diff := cmp.Diff(wantInserts, argsOf(tx.execCalls[2:])); diff != "" {
		t.Errorf("3回目以降（稼働実績の挿入）の引数が不一致 (-want +got):\n%s（AC-9-16-h ②・AC-9-16-c・AC-9-16-g）", diff)
	}
}

// TestWorkMonthRepository_Save_BreakColumnsAreNullAfterEditToNoBreak は、休憩ありの日を
// 休憩なしへ編集（再入力）した後の Save で、休憩の4列が NULL に戻ることを検証する
// （AC-9-16-g・daily-record-entry.md AC-2-6-d）。編集前の休憩の時刻を残さない。
// 最小の時刻を持つ休憩（2-6-c）は NULL にならないことと対にする（AC-2-6-c）。
func TestWorkMonthRepository_Save_BreakColumnsAreNullAfterEditToNoBreak(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, wm *workmonth.WorkMonth, today workmonth.Date)
		want  []any
	}{
		{
			name: "2-6-a の日を 2-6-d（9:00〜18:00／休憩なし）で再入力すると休憩の4列が NULL",
			build: func(t *testing.T, wm *workmonth.WorkMonth, today workmonth.Date) {
				for _, r := range []workmonth.DailyRecord{
					mustDailyRecord(t, 2026, 7, 1, startA, endA, &breakA),
					mustDailyRecord(t, 2026, 7, 1, startB, endB, nil),
				} {
					if err := wm.EnterDailyRecord(r, today); err != nil {
						t.Fatalf("EnterDailyRecord failed: %v", err)
					}
				}
			},
			want: insertArgs("contract-1", 2026, 7, 1, startB, endB, nil),
		},
		{
			name: "2-6-c: 0:00〜0:15 の休憩は NULL にしない（値 0 を持つ非 nil）",
			build: func(t *testing.T, wm *workmonth.WorkMonth, today workmonth.Date) {
				if err := wm.EnterDailyRecord(mustDailyRecord(t, 2026, 7, 1, startC, endC, &breakC), today); err != nil {
					t.Fatalf("EnterDailyRecord failed: %v", err)
				}
			},
			want: insertArgs("contract-1", 2026, 7, 1, startC, endC, &breakC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			tx := newFakeTx()
			db.pushBegin(tx, nil)
			wm, err := workmonth.New(mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7), mustSettlementRange(t, 8, 0, 20, 0))
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}
			tt.build(t, wm, mustDateOnly(t, 2026, 7, 31))

			if err := gateway.NewWorkMonthRepository(db).Save(context.Background(), wm); err != nil {
				t.Fatalf("Save failed: %v", err)
			}

			if len(tx.execCalls) != 3 {
				t.Fatalf("Tx.Exec の呼び出し回数 = %d, want 3", len(tx.execCalls))
			}
			if diff := cmp.Diff(tt.want, tx.execCalls[2].args); diff != "" {
				t.Errorf("稼働実績の挿入の引数が不一致 (-want +got):\n%s（AC-9-16-g）", diff)
			}
		})
	}
}

// TestWorkMonthRepository_Save_ExcessShortfallNullWhenUndetermined は、
// 超過／不足が未確定（Draft）のとき NULL として書くことを検証する（AC-9-16-d）。
func TestWorkMonthRepository_Save_ExcessShortfallNullWhenUndetermined(t *testing.T) {
	db := newFakeDB()
	tx := newFakeTx()
	db.pushBegin(tx, nil)

	repo := gateway.NewWorkMonthRepository(db)
	wm, err := workmonth.New(mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7), mustSettlementRange(t, 8, 0, 20, 0))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := repo.Save(context.Background(), wm); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 勤務月の行の書き込みは1回目の Exec で、引数は12個（AC-9-16-h ①）。
	if len(tx.execCalls) == 0 || len(tx.execCalls[0].args) != 12 {
		t.Fatalf("1回目の Exec（勤務月の行）の引数が12個でない: %+v", tx.execCalls)
	}
	header := tx.execCalls[0]
	excessH, excessM := header.args[8], header.args[9]
	shortfallH, shortfallM := header.args[10], header.args[11]
	for name, v := range map[string]any{
		"excessH": excessH, "excessM": excessM, "shortfallH": shortfallH, "shortfallM": shortfallM,
	} {
		if p, ok := v.(*int); !ok || p != nil {
			t.Errorf("%s = %#v, want (*int)(nil)（未確定は NULL。AC-9-16-d）", name, v)
		}
	}
}

// TestWorkMonthRepository_Save_RollsBackAndDoesNotCommitOnFailure は、途中の
// 失敗で取消し（Rollback）を呼び確定（Commit）を呼ばないことを検証する
// （AC-9-16-a・AC-12-11④）。エラーは握り潰さずそのまま返す。
func TestWorkMonthRepository_Save_RollsBackAndDoesNotCommitOnFailure(t *testing.T) {
	db := newFakeDB()
	tx := newFakeTx()
	execFailure := fmt.Errorf("write failed")
	tx.execErrs[0] = execFailure
	db.pushBegin(tx, nil)

	repo := gateway.NewWorkMonthRepository(db)
	target := closedWorkMonth(t)
	err := repo.Save(context.Background(), target)

	if !errors.Is(err, execFailure) {
		t.Fatalf("Save のエラー = %v, want errors.Is(err, execFailure)（途中の失敗を握り潰さない）", err)
	}
	if tx.commitCount != 0 {
		t.Errorf("Commit の呼び出し回数 = %d, want 0（途中で失敗したら確定を呼ばない。AC-9-16-a）", tx.commitCount)
	}
	if tx.rollbackCount != 1 {
		t.Errorf("Rollback の呼び出し回数 = %d, want 1（途中の失敗では取消しを呼ぶ。AC-9-16-a）", tx.rollbackCount)
	}
}

// TestWorkMonthRepository_Save_ReturnsBeginErrorAsIs は、Begin 自体の失敗を
// 握り潰さずそのまま返すことを検証する。
func TestWorkMonthRepository_Save_ReturnsBeginErrorAsIs(t *testing.T) {
	beginErr := errors.New("could not begin transaction")
	db := newFakeDB()
	db.pushBegin(nil, beginErr)

	repo := gateway.NewWorkMonthRepository(db)
	target := closedWorkMonth(t)
	err := repo.Save(context.Background(), target)

	if !errors.Is(err, beginErr) {
		t.Fatalf("Save のエラー = %v, want errors.Is(err, beginErr)", err)
	}
}
