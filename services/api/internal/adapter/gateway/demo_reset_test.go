package gateway_test

// 検証対象: docs/specs/demo-data-reset.md AC-15（15-1〜15-9・15-11）と、それが
// 担う AC-1（破棄の範囲）・AC-6-2〜6-3・AC-6-5・AC-6-7〜6-9・AC-6-11 の gateway 側。
// 検査の単位は AC-14-2（SQL 実行の受け口を手書きのインメモリ Fake へ差し替える。
// 実 DB を呼ばない。実 DB での原子性は担保しない＝AC-15-12・AC-13-4）。
//
// 本ファイルは AC-15-2 が tester 工程に委ねた「手段の名前と引数の型」を次のとおり定める
// （最小の署名。使う引数だけを持つ）。
//
//	type DemoContract struct {
//	    ID              workmonth.ContractID
//	    DisplayName     string
//	    EngineerID      string
//	    SettlementRange workmonth.SettlementRange
//	    PeriodStart     workmonth.YearMonth  // 契約期間の開始の年月（NULL を取らない＝AC-3-1）
//	    PeriodEnd       *workmonth.YearMonth // nil は終了未定（終了の年・月がともに NULL＝AC-3-2）
//	}
//	func NewDemoResetter(db DB) *DemoResetter
//	func (r *DemoResetter) Reset(ctx context.Context, contracts []DemoContract, workMonths []*workmonth.WorkMonth) error
//
// 契約期間の型に usecase/port の ContractPeriod を使わないのは、リセット用 cmd が
// usecase を import できないため（AC-12-11 ④）。
//
// Fake に対して固定する文の並び（Tx 越し。SQL 文そのものは検査しない＝docs/specs/workmonth-implementation-design.md の AC-13-18）:
//
//	① 契約ごとに、この順で:
//	   Query（引数は契約識別子の1つ。応答の行は「技術者識別子」の1列＝string。行が無ければ契約が無い）
//	   → 行が無ければ Exec で挿入（11引数: 識別子・契約表示名・技術者識別子・精算幅下限の時・分・
//	     上限の時・分・開始の年・開始の月・終了の年 *int・終了の月 *int）
//	   → 行があれば技術者識別子を照合し、Exec で契約期間の4列だけを書き換える
//	     （5引数: 識別子・開始の年・開始の月・終了の年 *int・終了の月 *int）
//	② 勤務月と稼働実績の全行の破棄: 引数を持たない Exec（1回以上。AC-1-1・AC-1-2 の
//	   「選り分けない」は、破棄の文が引数を取らないことで観測する）
//	③ 各集約について、Save（AC-9-16-h）が行う Exec と同じ列・同じ値・同じ順
//	   （AC-15-5。文の本文も Save と同一）
//	最後に Commit をちょうど1回。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// ---- 手書きの Fake（モックライブラリを入れない＝ADR 0007） ----------------

// resetStmt は Tx 越しの1つの文（Query または Exec）の記録。
type resetStmt struct {
	kind  string // "query" または "exec"
	query string
	args  []any
}

// resetTx は gateway.Tx の手書きの Fake。Query と Exec を1本の並びで記録し、
// 失敗は「その並びの何番目の文か」で注入する（failAt。-1 は注入しない）。
type resetTx struct {
	stmts []resetStmt
	// lookup は契約識別子から、DB にある契約の技術者識別子と、その契約があるかを返す。
	lookup func(contractID string) (engineerID string, exists bool)

	failAt  int
	failErr error

	commitCount   int
	rollbackCount int
	commitErr     error
}

func newResetTx(lookup func(string) (string, bool)) *resetTx {
	return &resetTx{lookup: lookup, failAt: -1}
}

func (tx *resetTx) Query(_ context.Context, query string, args ...any) (gateway.Rows, error) {
	idx := len(tx.stmts)
	tx.stmts = append(tx.stmts, resetStmt{kind: "query", query: query, args: args})
	if idx == tx.failAt {
		return nil, tx.failErr
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("resetTx: 契約の照会の引数は契約識別子の1つのはず（got %d）", len(args))
	}
	id, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("resetTx: 契約の照会の引数が string でない: %#v", args[0])
	}
	engineer, exists := tx.lookup(id)
	if !exists {
		return newFakeRows(), nil
	}
	return newFakeRows([]any{engineer}), nil
}

func (tx *resetTx) Exec(_ context.Context, query string, args ...any) error {
	idx := len(tx.stmts)
	tx.stmts = append(tx.stmts, resetStmt{kind: "exec", query: query, args: args})
	if idx == tx.failAt {
		return tx.failErr
	}
	return nil
}

func (tx *resetTx) Commit(_ context.Context) error {
	tx.commitCount++
	return tx.commitErr
}

func (tx *resetTx) Rollback(_ context.Context) error {
	tx.rollbackCount++
	return nil
}

// ---- 入力の組み立て --------------------------------------------------------

const (
	demoEngineerID  = "demo-engineer-not-a-uuid"
	otherEngineerID = "other-engineer-must-not-leak"
)

func yearMonthPtr(t *testing.T, year, month int) *workmonth.YearMonth {
	t.Helper()
	ym := mustYearMonth(t, year, month)
	return &ym
}

// demoWorkMonth は集約の操作で勤務月を組み立てる（Reconstruct を使わない）。
// finalize は "draft"（そのまま）・"close"（締める）・"approve"（締めて承認する）。
func demoWorkMonth(
	t *testing.T, contractID string, year, month int, rng workmonth.SettlementRange,
	records []workmonth.DailyRecord, finalize string,
) *workmonth.WorkMonth {
	t.Helper()
	wm, err := workmonth.New(mustContractID(t, contractID), mustYearMonth(t, year, month), rng)
	if err != nil {
		t.Fatalf("workmonth.New failed: %v", err)
	}
	today := mustDateOnly(t, year, month, 28)
	for _, r := range records {
		if err := wm.EnterDailyRecord(r, today); err != nil {
			t.Fatalf("EnterDailyRecord failed: %v", err)
		}
	}
	if finalize == "close" || finalize == "approve" {
		if err := wm.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}
	if finalize == "approve" {
		if err := wm.Approve(); err != nil {
			t.Fatalf("Approve failed: %v", err)
		}
	}
	return wm
}

// demoResetFixtures は2契約と、4つの集約（A の2026-08 Approved・A の2026-09 Draft（稼働実績0件）・
// B の2026-09 Draft・A の2026-10 Draft）を返す。同じ年月（2026-09）を別の契約が持つ
// （勤務月は契約 × 年月で一意＝AC-15-8 の陽性対照）。
func demoResetFixtures(t *testing.T) ([]gateway.DemoContract, []*workmonth.WorkMonth) {
	t.Helper()
	rangeA := mustSettlementRange(t, 140, 0, 180, 0)
	rangeB := mustSettlementRange(t, 20, 0, 40, 0)
	contracts := []gateway.DemoContract{
		{
			ID: mustContractID(t, "demo-contract-a"), DisplayName: "契約A", EngineerID: demoEngineerID,
			SettlementRange: rangeA, PeriodStart: mustYearMonth(t, 2026, 4), PeriodEnd: nil,
		},
		{
			ID: mustContractID(t, "demo-contract-b"), DisplayName: "契約B", EngineerID: demoEngineerID,
			SettlementRange: rangeB, PeriodStart: mustYearMonth(t, 2026, 7), PeriodEnd: yearMonthPtr(t, 2026, 9),
		},
	}
	aggregates := []*workmonth.WorkMonth{
		demoWorkMonth(t, "demo-contract-a", 2026, 8, rangeA, []workmonth.DailyRecord{
			mustDailyRecord(t, 2026, 8, 3, startA, endA, &breakA),
			mustDailyRecord(t, 2026, 8, 4, startB, endB, nil),
		}, "approve"),
		demoWorkMonth(t, "demo-contract-a", 2026, 9, rangeA, nil, "draft"),
		demoWorkMonth(t, "demo-contract-b", 2026, 9, rangeB, []workmonth.DailyRecord{
			mustDailyRecord(t, 2026, 9, 5, startA, endA, &breakA),
		}, "draft"),
		demoWorkMonth(t, "demo-contract-a", 2026, 10, rangeA, []workmonth.DailyRecord{
			mustDailyRecord(t, 2026, 10, 1, startB, endB, nil),
		}, "draft"),
	}
	return contracts, aggregates
}

// saveSequences は、各集約を既存の Save（AC-9-16）で保存したときに Fake へ記録される
// Exec の並びを集約ごとに返す（AC-15-5 の突き合わせ先）。
func saveSequences(t *testing.T, aggregates []*workmonth.WorkMonth) [][]execCall {
	t.Helper()
	out := make([][]execCall, 0, len(aggregates))
	for _, wm := range aggregates {
		db := newFakeDB()
		tx := newFakeTx()
		db.pushBegin(tx, nil)
		if err := gateway.NewWorkMonthRepository(db).Save(context.Background(), wm); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		out = append(out, tx.execCalls)
	}
	return out
}

func contractIDSet(contracts []gateway.DemoContract) map[string]bool {
	set := map[string]bool{}
	for _, c := range contracts {
		set[c.ID.String()] = true
	}
	return set
}

// periodArgs は契約期間の4列の引数（開始の年・月、終了の年・月。終了未定なら (*int)(nil) 2つ）。
func periodArgs(c gateway.DemoContract) []any {
	args := []any{c.PeriodStart.Year(), c.PeriodStart.Month()}
	if c.PeriodEnd == nil {
		return append(args, nilInt, nilInt)
	}
	return append(args, intPtr(c.PeriodEnd.Year()), intPtr(c.PeriodEnd.Month()))
}

func insertContractArgs(c gateway.DemoContract) []any {
	lower, upper := c.SettlementRange.LowerBound(), c.SettlementRange.UpperBound()
	args := []any{
		c.ID.String(), c.DisplayName, c.EngineerID,
		lower.Hours(), lower.Minutes(), upper.Hours(), upper.Minutes(),
	}
	return append(args, periodArgs(c)...)
}

func updateContractArgs(c gateway.DemoContract) []any {
	return append([]any{c.ID.String()}, periodArgs(c)...)
}

// runReset は新しい Fake に対して Reset を1回呼ぶ。
func runReset(
	t *testing.T, tx *resetTx, contracts []gateway.DemoContract, aggregates []*workmonth.WorkMonth,
) (*fakeDB, error) {
	t.Helper()
	db := newFakeDB()
	db.pushBegin(tx, nil)
	err := gateway.NewDemoResetter(db).Reset(context.Background(), contracts, aggregates)
	return db, err
}

// 契約が全部ある／全部無い／A だけある、の照会の応答。
func allExist(contracts []gateway.DemoContract) func(string) (string, bool) {
	return func(id string) (string, bool) {
		for _, c := range contracts {
			if c.ID.String() == id {
				return c.EngineerID, true
			}
		}
		return "", false
	}
}

func noneExist(string) (string, bool) { return "", false }

// ---- AC-15-3・AC-6-2・AC-6-3・AC-6-7・AC-6-8・AC-1・AC-15-5・AC-15-9 -----------

// TestDemoResetter_Reset_StatementsAndCommit は、正常系で Tx 越しに行う文の並びを固定する。
// ①契約ごとの照会 → 挿入または期間の書き換え、②引数を持たない破棄の Exec（1回以上）、
// ③各集約の Save と同じ Exec、の順で、Begin・Commit はそれぞれちょうど1回、Rollback は0回。
func TestDemoResetter_Reset_StatementsAndCommit(t *testing.T) {
	contracts, aggregates := demoResetFixtures(t)
	saves := saveSequences(t, aggregates)

	tests := []struct {
		name   string
		lookup func(string) (string, bool)
		// exists は契約識別子ごとの「DB に既にあるか」（期待する挿入／書き換えの別）。
		exists map[string]bool
	}{
		{"デモの契約が無い（AC-6-2: 挿入）", noneExist, map[string]bool{}},
		{"デモの契約が既にある（AC-6-3・AC-6-7: 期間の4列だけ書き換え）", allExist(contracts),
			map[string]bool{"demo-contract-a": true, "demo-contract-b": true}},
		{"A だけある（契約ごとに挿入と書き換えを分ける）",
			func(id string) (string, bool) {
				if id == "demo-contract-a" {
					return demoEngineerID, true
				}
				return "", false
			},
			map[string]bool{"demo-contract-a": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := newResetTx(tt.lookup)
			db, err := runReset(t, tx, contracts, aggregates)
			if err != nil {
				t.Fatalf("Reset failed: %v", err)
			}
			if db.beginCalls != 1 {
				t.Errorf("Begin の呼び出し回数 = %d, want 1（トランザクションはちょうど1つ＝AC-15-3）", db.beginCalls)
			}
			if tx.commitCount != 1 {
				t.Errorf("Commit の呼び出し回数 = %d, want 1", tx.commitCount)
			}
			if tx.rollbackCount != 0 {
				t.Errorf("Rollback の呼び出し回数 = %d, want 0（成功時は取消しを呼ばない）", tx.rollbackCount)
			}
			if len(db.queryLog) != 0 || len(db.execCalls) != 0 {
				t.Errorf("Tx の外で文が実行された（DB.Query=%d, DB.Exec=%d）。すべて1つの Tx の中で行う（AC-15-3）",
					len(db.queryLog), len(db.execCalls))
			}

			// ① 契約ごとの [照会, 挿入 または 書き換え]。
			headLen := 2 * len(contracts)
			if len(tx.stmts) < headLen {
				t.Fatalf("文が %d 件しかない。① だけで %d 件要る", len(tx.stmts), headLen)
			}
			type shape struct {
				Kind string
				Args []any
			}
			var wantHead []shape
			for _, c := range contracts {
				wantHead = append(wantHead, shape{"query", []any{c.ID.String()}})
				if tt.exists[c.ID.String()] {
					wantHead = append(wantHead, shape{"exec", updateContractArgs(c)})
				} else {
					wantHead = append(wantHead, shape{"exec", insertContractArgs(c)})
				}
			}
			var gotHead []shape
			for _, s := range tx.stmts[:headLen] {
				gotHead = append(gotHead, shape{s.kind, s.args})
			}
			if diff := cmp.Diff(wantHead, gotHead); diff != "" {
				t.Errorf("① 契約の照会と挿入／書き換えの並びと引数 (-want +got):\n%s", diff)
			}

			// ② 引数を持たない破棄の Exec が1回以上。
			discardEnd := headLen
			for discardEnd < len(tx.stmts) && tx.stmts[discardEnd].kind == "exec" && len(tx.stmts[discardEnd].args) == 0 {
				discardEnd++
			}
			if discardEnd == headLen {
				t.Fatalf("② 引数を持たない破棄の Exec が無い（全行を破棄する＝AC-1-1・AC-1-2・AC-15-3）")
			}

			// ③ 各集約の Save と同じ Exec が、渡した順に続き、それで全部である（AC-15-5）。
			var wantTail []execCall
			for _, seq := range saves {
				wantTail = append(wantTail, seq...)
			}
			var gotTail []execCall
			for _, s := range tx.stmts[discardEnd:] {
				if s.kind != "exec" {
					t.Errorf("③ に Exec 以外の文がある: %#v", s)
					continue
				}
				gotTail = append(gotTail, execCall{query: s.query, args: s.args})
			}
			if diff := cmp.Diff(wantTail, gotTail, cmp.AllowUnexported(execCall{})); diff != "" {
				t.Errorf("③ 集約の保存の Exec が Save と一致しない (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDemoResetter_Reset_ContractWritesTargetOnlyPassedIdentifiers は AC-15-9・AC-6-9 を固定する。
// 契約の行に当てる書き換え（5引数）・挿入（11引数）の対象が、渡した識別子だけであり、
// 契約ごとにちょうど1回であること。
func TestDemoResetter_Reset_ContractWritesTargetOnlyPassedIdentifiers(t *testing.T) {
	contracts, aggregates := demoResetFixtures(t)
	tx := newResetTx(allExist(contracts))
	if _, err := runReset(t, tx, contracts, aggregates); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	passed := contractIDSet(contracts)
	var targets []string
	for _, s := range tx.stmts {
		if s.kind != "exec" || (len(s.args) != 5 && len(s.args) != 11) {
			continue
		}
		id, ok := s.args[0].(string)
		if !ok {
			t.Fatalf("契約への文の先頭の引数が識別子（string）でない: %#v", s.args[0])
		}
		if !passed[id] {
			t.Errorf("契約への文の対象 %q は渡した識別子でない（AC-15-9・AC-6-9）", id)
		}
		targets = append(targets, id)
	}
	want := []string{"demo-contract-a", "demo-contract-b"}
	if diff := cmp.Diff(want, targets); diff != "" {
		t.Errorf("契約への文の対象（契約ごとにちょうど1回、渡した順）(-want +got):\n%s", diff)
	}
}

// ---- AC-15-4・AC-15-4-a・AC-6-5・AC-6-11・AC-15-11 -----------------------------

// TestDemoResetter_Reset_FailureRollsBackAndStops は、失敗を各位置へ注入し、
// Begin 1回・Commit 0回・Rollback 1回で、失敗の位置より後の文が記録されず、
// エラーを返すことを固定する（AC-15-4-a）。位置は正常系の並びから求める。
func TestDemoResetter_Reset_FailureRollsBackAndStops(t *testing.T) {
	contracts, aggregates := demoResetFixtures(t)
	saves := saveSequences(t, aggregates)

	baseline := newResetTx(allExist(contracts))
	if _, err := runReset(t, baseline, contracts, aggregates); err != nil {
		t.Fatalf("baseline Reset failed: %v", err)
	}
	headLen := 2 * len(contracts)
	discardEnd := headLen
	for discardEnd < len(baseline.stmts) && len(baseline.stmts[discardEnd].args) == 0 {
		discardEnd++
	}
	if discardEnd == headLen {
		t.Fatalf("baseline に破棄の Exec が無い。位置を求められない")
	}
	start3 := discardEnd
	last := len(baseline.stmts) - 1

	positions := []struct {
		name string
		at   int
	}{
		{"①の1件目の契約（照会）", 0},
		{"①の1件目の契約（書き換え）", 1},
		{"②（最初の破棄）", headLen},
		{"②（最後の破棄）", discardEnd - 1},
		{"③の1件目の集約", start3},
		{"③の途中の集約（2つ目の集約の最初の文）", start3 + len(saves[0])},
		{"③の途中の集約（3つ目の集約の稼働実績の挿入）", start3 + len(saves[0]) + len(saves[1]) + 2},
		{"③の最後の集約（最後の文）", last},
	}

	for _, p := range positions {
		t.Run(p.name, func(t *testing.T) {
			injected := errors.New("injected failure")
			tx := newResetTx(allExist(contracts))
			tx.failAt, tx.failErr = p.at, injected
			db, err := runReset(t, tx, contracts, aggregates)

			if err == nil {
				t.Fatalf("Reset は失敗を注入した位置 %d でエラーを返さなければならない", p.at)
			}
			if !errors.Is(err, injected) {
				t.Errorf("返ったエラーが注入した原因を %%w で包んでいない: %v", err)
			}
			if db.beginCalls != 1 {
				t.Errorf("Begin の呼び出し回数 = %d, want 1", db.beginCalls)
			}
			if tx.commitCount != 0 {
				t.Errorf("Commit の呼び出し回数 = %d, want 0（失敗したらコミットしない）", tx.commitCount)
			}
			if tx.rollbackCount != 1 {
				t.Errorf("Rollback の呼び出し回数 = %d, want 1", tx.rollbackCount)
			}
			if got, want := len(tx.stmts), p.at+1; got != want {
				t.Errorf("記録された文の数 = %d, want %d（失敗の位置より後の文を実行しない）", got, want)
			}
			assertNoStatementTextIn(t, err, tx.stmts)
		})
	}

	// 契約が無いときの挿入（AC-6-2）の失敗も、①の失敗として同じ扱いになる。
	t.Run("①の1件目の契約（挿入）", func(t *testing.T) {
		injected := errors.New("injected insert failure")
		tx := newResetTx(noneExist)
		tx.failAt, tx.failErr = 1, injected
		db, err := runReset(t, tx, contracts, aggregates)

		if err == nil || !errors.Is(err, injected) {
			t.Fatalf("Reset は挿入の失敗をエラーとして返さなければならない: %v", err)
		}
		if db.beginCalls != 1 || tx.commitCount != 0 || tx.rollbackCount != 1 {
			t.Errorf("Begin=%d Commit=%d Rollback=%d, want 1/0/1", db.beginCalls, tx.commitCount, tx.rollbackCount)
		}
		if got, want := len(tx.stmts), 2; got != want {
			t.Errorf("記録された文の数 = %d, want %d（失敗の位置より後の文を実行しない）", got, want)
		}
		assertNoStatementTextIn(t, err, tx.stmts)
	})

	t.Run("コミットの失敗", func(t *testing.T) {
		injected := errors.New("injected commit failure")
		tx := newResetTx(allExist(contracts))
		tx.commitErr = injected
		db, err := runReset(t, tx, contracts, aggregates)

		if err == nil || !errors.Is(err, injected) {
			t.Fatalf("Reset はコミットの失敗をエラーとして返さなければならない: %v", err)
		}
		if db.beginCalls != 1 {
			t.Errorf("Begin の呼び出し回数 = %d, want 1", db.beginCalls)
		}
		if tx.commitCount != 1 {
			t.Errorf("Commit の試み = %d, want 1（ロールバックの呼び出しの有無は問わない）", tx.commitCount)
		}
		if got, want := len(tx.stmts), len(baseline.stmts); got != want {
			t.Errorf("記録された文の数 = %d, want %d（コミットの前に全文を実行する）", got, want)
		}
		assertNoStatementTextIn(t, err, tx.stmts)
	})
}

// assertNoStatementTextIn は、返ったエラーの文言に SQL 文を含めないこと（AC-15-11）を確かめる。
func assertNoStatementTextIn(t *testing.T, err error, stmts []resetStmt) {
	t.Helper()
	for _, s := range stmts {
		if strings.TrimSpace(s.query) == "" {
			continue
		}
		if strings.Contains(err.Error(), strings.TrimSpace(s.query)) {
			t.Errorf("エラーの文言に SQL 文が含まれている（AC-15-11）: %q", err.Error())
		}
	}
}

// TestDemoResetter_Reset_EngineerMismatchFails は AC-6-5・AC-15-4（①の照合の不一致）を固定する。
// DB にある契約の技術者識別子が、渡した技術者識別子と一致しないとき、失敗してロールバックする。
// 契約の行を書き換えて合わせない（その契約への Exec が無い）。取得した値を文言に含めない（AC-15-11）。
func TestDemoResetter_Reset_EngineerMismatchFails(t *testing.T) {
	contracts, aggregates := demoResetFixtures(t)

	for _, mismatched := range []int{0, 1} {
		id := contracts[mismatched].ID.String()
		t.Run(fmt.Sprintf("%d 番目の契約が不一致 (%s)", mismatched, id), func(t *testing.T) {
			tx := newResetTx(func(contractID string) (string, bool) {
				if contractID == id {
					return otherEngineerID, true
				}
				return demoEngineerID, true
			})
			db, err := runReset(t, tx, contracts, aggregates)

			if err == nil {
				t.Fatalf("技術者識別子の不一致でエラーを返さなければならない（AC-6-5）")
			}
			if db.beginCalls != 1 {
				t.Errorf("Begin の呼び出し回数 = %d, want 1", db.beginCalls)
			}
			if tx.commitCount != 0 {
				t.Errorf("Commit の呼び出し回数 = %d, want 0", tx.commitCount)
			}
			if tx.rollbackCount != 1 {
				t.Errorf("Rollback の呼び出し回数 = %d, want 1", tx.rollbackCount)
			}
			if len(tx.stmts) == 0 {
				t.Fatalf("文が1つも実行されていない")
			}
			lastStmt := tx.stmts[len(tx.stmts)-1]
			if lastStmt.kind != "query" || len(lastStmt.args) != 1 || lastStmt.args[0] != id {
				t.Errorf("最後の文は不一致の契約 %q の照会のはず（以降の文を実行しない）: %#v", id, lastStmt)
			}
			for _, s := range tx.stmts {
				if s.kind == "exec" && len(s.args) > 0 && s.args[0] == id {
					t.Errorf("不一致の契約に Exec を当てた（契約の行を書き換えて合わせない＝AC-6-5）: %#v", s)
				}
			}
			if strings.Contains(err.Error(), otherEngineerID) {
				t.Errorf("エラーの文言に DB から取得した値が含まれている（AC-15-11）: %q", err.Error())
			}
			assertNoStatementTextIn(t, err, tx.stmts)
		})
	}
}

// ---- AC-15-6・AC-15-7・AC-15-8 ------------------------------------------------

// TestDemoResetter_Reset_RejectsBadInputWithoutAnyStatement は、入力が不正なとき
// トランザクションを開かず何の文も実行せずにエラーを返すことを固定する。
func TestDemoResetter_Reset_RejectsBadInputWithoutAnyStatement(t *testing.T) {
	contracts, aggregates := demoResetFixtures(t)
	rangeA := mustSettlementRange(t, 140, 0, 180, 0)

	tests := []struct {
		name       string
		aggregates []*workmonth.WorkMonth
	}{
		{"②の列が空（全行を破棄して空のデモを残さない＝AC-15-6）", nil},
		{"②の列が空のスライス（AC-15-6）", []*workmonth.WorkMonth{}},
		{"①の契約のいずれでもない契約の集約がある（AC-15-7）",
			append(append([]*workmonth.WorkMonth{}, aggregates...),
				demoWorkMonth(t, "not-a-demo-contract", 2026, 10, rangeA, nil, "draft"))},
		{"同じ契約・同じ年月の集約が2つある（AC-15-8）",
			append(append([]*workmonth.WorkMonth{}, aggregates...),
				demoWorkMonth(t, "demo-contract-a", 2026, 10, rangeA, nil, "draft"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := newResetTx(allExist(contracts))
			db, err := runReset(t, tx, contracts, tt.aggregates)

			if err == nil {
				t.Fatalf("エラーを返さなければならない")
			}
			if db.beginCalls != 0 {
				t.Errorf("Begin の呼び出し回数 = %d, want 0（トランザクションを開かない）", db.beginCalls)
			}
			if len(tx.stmts) != 0 || len(db.queryLog) != 0 || len(db.execCalls) != 0 {
				t.Errorf("何の文も実行してはならない（Tx=%d, DB.Query=%d, DB.Exec=%d）",
					len(tx.stmts), len(db.queryLog), len(db.execCalls))
			}
		})
	}
}
