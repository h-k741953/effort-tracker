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

func mustDailyRecord(t *testing.T, year, month, day, hours, minutes int) workmonth.DailyRecord {
	t.Helper()
	date, err := workmonth.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("NewDate failed: %v", err)
	}
	wh, err := workmonth.NewWorkingHours(hours, minutes)
	if err != nil {
		t.Fatalf("NewWorkingHours failed: %v", err)
	}
	record, err := workmonth.NewDailyRecord(date, wh)
	if err != nil {
		t.Fatalf("NewDailyRecord failed: %v", err)
	}
	return record
}

// workMonthCmpOpts は WorkMonth の比較用に、非公開フィールドを持つ値オブジェクトの
// 比較を許可する（AC-12-4）。
var workMonthCmpOpts = cmp.AllowUnexported(
	workmonth.WorkMonth{},
	workmonth.ContractID{}, workmonth.YearMonth{}, workmonth.Date{},
	workmonth.WorkingHours{}, workmonth.DailyRecord{}, workmonth.SettlementRange{},
)

// ---- ① 行 → 集約の変換（AC-9-15） -----------------------------------------

// TestWorkMonthRepository_Find_ReconstructsFromRows は、Fake が返す行から
// Reconstruct へ渡る引数が AC-9-15-b のとおりであることを検証する。
// 精算幅は「行の値」であり（契約の現在値は Find に渡らないため、これが唯一の
// 情報源になる＝AC-9-15-b）、稼働実績は「対象日と入力された稼働時間」のみを持つ。
func TestWorkMonthRepository_Find_ReconstructsFromRows(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthHeaderRow("contract-1", 2026, 7, 8, 0, 20, 0, string(workmonth.StateDraft), nil, nil, nil, nil),
	), nil)
	db.pushQuery(newFakeRows(
		dailyRecordRow(2026, 7, 1, 7, 30),
		dailyRecordRow(2026, 7, 2, 8, 0),
	), nil)

	repo := gateway.NewWorkMonthRepository(db)
	got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	want, err := workmonth.Reconstruct(
		mustContractID(t, "contract-1"),
		mustYearMonth(t, 2026, 7),
		mustSettlementRange(t, 8, 0, 20, 0),
		workmonth.StateDraft,
		[]workmonth.DailyRecord{
			mustDailyRecord(t, 2026, 7, 1, 7, 30),
			mustDailyRecord(t, 2026, 7, 2, 8, 0),
		},
		nil, nil,
	)
	if err != nil {
		t.Fatalf("Reconstruct(want) failed: %v", err)
	}

	if diff := cmp.Diff(want, got, workMonthCmpOpts); diff != "" {
		t.Errorf("Find が返す集約が不一致 (-want +got):\n%s（AC-9-15-b）", diff)
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
		dailyRecordRow(2026, 7, 1, 7, 30),
		dailyRecordRow(2026, 7, 1, 8, 0),
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
	if err := wm.EnterDailyRecord(mustDailyRecord(t, 2026, 7, 1, 7, 30), today); err != nil {
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

// findExecCall は execCalls から、要素数が want と一致する最初の呼び出しを返す
// （ヘッダ upsert とレコード削除／挿入で引数の個数が異なることを利用して区別する。
// SQL 文そのものは検査しない＝AC-13-18）。
func execCallsWithArgCount(calls []execCall, n int) []execCall {
	var matched []execCall
	for _, c := range calls {
		if len(c.args) == n {
			matched = append(matched, c)
		}
	}
	return matched
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

// TestWorkMonthRepository_Save_WritesExactDailyRecordSet は、保存後の稼働実績の
// 行集合が集約の稼働実績と一致すること（削除された対象日の行を残さない。
// AC-9-16-b）と、書き込む値が入力された稼働時間のみであること（丸め値・総稼働
// 時間の列を持たない。AC-9-16-c）を検証する。
func TestWorkMonthRepository_Save_WritesExactDailyRecordSet(t *testing.T) {
	db := newFakeDB()
	tx := newFakeTx()
	db.pushBegin(tx, nil)

	repo := gateway.NewWorkMonthRepository(db)
	wm, err := workmonth.New(mustContractID(t, "contract-1"), mustYearMonth(t, 2026, 7), mustSettlementRange(t, 8, 0, 20, 0))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	today := mustDateOnly(t, 2026, 7, 31)
	if err := wm.EnterDailyRecord(mustDailyRecord(t, 2026, 7, 1, 7, 30), today); err != nil {
		t.Fatalf("EnterDailyRecord failed: %v", err)
	}
	if err := wm.EnterDailyRecord(mustDailyRecord(t, 2026, 7, 2, 8, 0), today); err != nil {
		t.Fatalf("EnterDailyRecord failed: %v", err)
	}
	// 7/2 を削除する。保存される稼働実績の行は 7/1 のみになるはず（AC-9-16-b）。
	if err := wm.DeleteDailyRecord(mustDateOnly(t, 2026, 7, 2)); err != nil {
		t.Fatalf("DeleteDailyRecord failed: %v", err)
	}

	if err := repo.Save(context.Background(), wm); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 稼働実績1件分の Exec 呼び出しは6引数（契約識別子・年・月・日・時・分。
	// AC-9-16-c）とする tester の取り決め（doubles_test.go の前書き。契約識別子を
	// 書かないと、稼働実績の主キーである「契約 × 年月 × 対象日」が保存されず、
	// 保存した行を二度と読み出せない＝AC-10-5・AC-9-16-b 違反）。丸め値・
	// 総稼働時間の列は引数に含まれない。
	recordCalls := execCallsWithArgCount(tx.execCalls, 6)
	want := []execCall{{args: []any{"contract-1", 2026, 7, 1, 7, 30}}}
	if diff := cmp.Diff(want, recordCalls, cmp.Comparer(func(a, b execCall) bool {
		return cmp.Equal(a.args, b.args)
	})); diff != "" {
		t.Errorf("稼働実績の書き込みが不一致 (-want +got):\n%s（AC-9-16-b・AC-9-16-c）", diff)
	}
}

// TestWorkMonthRepository_Save_DeletesDailyRecordsBeforeInserting は、Save が
// 稼働実績の全削除（DELETE）を発行することを検証する（AC-9-16-b・AC-12-11②）。
// 削除の手順ごと欠落しても、単に保存後の行集合を突き合わせるだけの検証では
// 検出できない（挿入前に対象の勤務月分の行が0件だから、削除してもしなくても
// 結果の行集合は変わらない）。そのため DELETE の発行そのものを、他の Exec
// 呼び出し（12引数のヘッダ upsert・6引数の稼働実績 insert）と引数の個数で
// 判別してアサートする（DELETE は契約識別子・年・月の3引数。doubles_test.go の
// 前書きに定める tester の取り決め）。
func TestWorkMonthRepository_Save_DeletesDailyRecordsBeforeInserting(t *testing.T) {
	db := newFakeDB()
	tx := newFakeTx()
	db.pushBegin(tx, nil)

	repo := gateway.NewWorkMonthRepository(db)
	target := closedWorkMonth(t)
	if err := repo.Save(context.Background(), target); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	deleteCalls := execCallsWithArgCount(tx.execCalls, 3)
	want := []execCall{{args: []any{"contract-1", 2026, 7}}}
	if diff := cmp.Diff(want, deleteCalls, cmp.Comparer(func(a, b execCall) bool {
		return cmp.Equal(a.args, b.args)
	})); diff != "" {
		t.Errorf("稼働実績の全削除 (DELETE) の呼び出しが不一致 (-want +got):\n%s（AC-9-16-b・AC-12-11②）", diff)
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

	// 勤務月のヘッダ行の Exec 呼び出しは12引数（doubles_test.go の workMonthHeaderRow
	// と対称の並び）とする tester の取り決め。
	headerCalls := execCallsWithArgCount(tx.execCalls, 12)
	if len(headerCalls) != 1 {
		t.Fatalf("ヘッダ行の Exec 呼び出し回数 = %d, want 1", len(headerCalls))
	}
	excessH, excessM := headerCalls[0].args[8], headerCalls[0].args[9]
	shortfallH, shortfallM := headerCalls[0].args[10], headerCalls[0].args[11]
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
