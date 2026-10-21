package gateway_test

import (
	"context"
	"fmt"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
)

// テストダブルはすべて手書きのインメモリ Fake とする
// （ADR 0007・docs/specs/workmonth-implementation-design.md AC-12-2・AC-12-11）。
// モックライブラリは使わない。pgx も実 DB も使わない（AC-12-6）。
//
// 本ファイルが仮定する「行の形（Scan の引数の型と並び）」は、gateway が宣言する
// DB / gateway.Rows / gateway.Tx（AC-9-14-e・決定12）という Go レベルの境界を検査可能にするために
// tester が選んだ一案であり、SQL の列名・テーブル名を定めるものではない
// （AC-10-6 は未固定のまま）。実装工程がこの並びと異なる形を選ぶ場合、
// 本ファイルの並びに合わせて Scan を書くか、本ファイルを合わせて調整する
// （このテストが固定するのは AC-12-11 が列挙する4項目までであり、
// SQL 文そのものの正しさは検査しない＝AC-13-18）。

// ---- fakeRows --------------------------------------------------------------

// fakeRows は gateway.Rows（AC-9-14-e①）に対する手書きの Fake。行は [][]any として
// 事前に設定し、Scan は現在の行の値を dest（型は string / int / *int のいずれか）
// へ写す。*int の dest は NULL 許容の列（超過／不足。AC-9-15-c）を表し、
// 設定値が (*int)(nil) なら NULL、非 nil なら値ありとして写す。
type fakeRows struct {
	rows    [][]any
	idx     int
	scanErr error // Scan 自体が返すエラー（設定時のみ）
	iterErr error // 走査中のエラー（Err() が返す）
	closed  bool
}

func newFakeRows(rows ...[]any) *fakeRows {
	return &fakeRows{rows: rows}
}

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.idx == 0 || r.idx > len(r.rows) {
		return fmt.Errorf("fakeRows: Scan called without a current row")
	}
	row := r.rows[r.idx-1]
	if len(dest) != len(row) {
		return fmt.Errorf("fakeRows: scan destination count = %d, want %d", len(dest), len(row))
	}
	for i, d := range dest {
		if err := fakeScanAssign(d, row[i]); err != nil {
			return fmt.Errorf("fakeRows: column %d: %w", i, err)
		}
	}
	return nil
}

func (r *fakeRows) Close() error {
	r.closed = true
	return nil
}

func (r *fakeRows) Err() error { return r.iterErr }

// fakeScanAssign は Scan の1列分の代入を行う。サポートするのは
// *string / *int / **int（NULL 許容の int 列）のみ。
func fakeScanAssign(dest, value any) error {
	switch d := dest.(type) {
	case *string:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("value %#v is not a string", value)
		}
		*d = v
	case *int:
		v, ok := value.(int)
		if !ok {
			return fmt.Errorf("value %#v is not an int", value)
		}
		*d = v
	case **int:
		v, ok := value.(*int)
		if !ok {
			return fmt.Errorf("value %#v is not a *int", value)
		}
		*d = v
	case **string:
		// NULL 許容の文字列列（契約一覧の勤務月の状態。AC-9-20-c ⑪）。
		v, ok := value.(*string)
		if !ok {
			return fmt.Errorf("value %#v is not a *string", value)
		}
		*d = v
	default:
		return fmt.Errorf("unsupported scan destination type %T", dest)
	}
	return nil
}

// ---- fakeTx ------------------------------------------------------------

// execCall は gateway.Tx.Exec / DB.Exec への1回の呼び出しを記録する。
type execCall struct {
	query string
	args  []any
}

// fakeTx は gateway.Tx（AC-9-14-e③）に対する手書きの Fake。呼び出しの順序と回数を記録する
// （AC-12-11④。原子性そのもの・SQL 文の正しさは検査しない＝AC-13-18）。
type fakeTx struct {
	execCalls []execCall
	// execErrs はいずれか要素が非 nil なら「その番号の Exec 呼び出しがそのエラーで
	// 失敗する」ことを表す（0 始まり）。未設定の呼び出しは成功する。
	execErrs map[int]error

	commitCount   int
	rollbackCount int
	commitErr     error
}

func newFakeTx() *fakeTx { return &fakeTx{execErrs: map[int]error{}} }

func (tx *fakeTx) Query(_ context.Context, _ string, _ ...any) (gateway.Rows, error) {
	return nil, fmt.Errorf("fakeTx: Query is not configured for this test")
}

func (tx *fakeTx) Exec(_ context.Context, query string, args ...any) error {
	idx := len(tx.execCalls)
	tx.execCalls = append(tx.execCalls, execCall{query: query, args: args})
	if err, ok := tx.execErrs[idx]; ok {
		return err
	}
	return nil
}

func (tx *fakeTx) Commit(_ context.Context) error {
	tx.commitCount++
	return tx.commitErr
}

func (tx *fakeTx) Rollback(_ context.Context) error {
	tx.rollbackCount++
	return nil
}

// ---- fakeDB --------------------------------------------------------------

// queryResponse は DB.Query への1回分の応答（呼び出し順に消費する。
// SQL 文そのものの一致は検査しない＝AC-13-18）。
type queryResponse struct {
	rows gateway.Rows
	err  error
}

// fakeDB は DB（AC-9-14-a・決定12）に対する手書きの Fake。
type fakeDB struct {
	queryQueue []queryResponse
	queryCalls int
	// queryLog は Query への呼び出しを (query, args) で記録する
	// （省略した条件が引数として渡っていないことの検証用。AC-9-18-g・AC-12-13⑤）。
	queryLog []execCall
	// closedBeforeQuery[i] は i 回目（0 始まり）の Query を呼んだ時点で、直前の
	// Query が返した Rows が閉じられていたか（i=0 は常に true）。走査を閉じる前に
	// 次の Query を呼ばないこと（AC-9-14-f・AC-9-21-c）を観測する。直前の応答が
	// *fakeRows でない（nil 等）場合は true とする。
	closedBeforeQuery []bool

	execCalls []execCall

	beginQueue []beginResponse
	beginCalls int
}

type beginResponse struct {
	tx  gateway.Tx
	err error
}

func newFakeDB() *fakeDB { return &fakeDB{} }

// pushQuery は次の DB.Query 呼び出しへの応答を1つ積む（呼び出し順に消費）。
func (db *fakeDB) pushQuery(rows gateway.Rows, err error) {
	db.queryQueue = append(db.queryQueue, queryResponse{rows: rows, err: err})
}

// pushBegin は次の DB.Begin 呼び出しへの応答を1つ積む。
func (db *fakeDB) pushBegin(tx gateway.Tx, err error) {
	db.beginQueue = append(db.beginQueue, beginResponse{tx: tx, err: err})
}

func (db *fakeDB) Query(_ context.Context, query string, args ...any) (gateway.Rows, error) {
	db.queryLog = append(db.queryLog, execCall{query: query, args: args})
	previousClosed := true
	if db.queryCalls > 0 {
		if prev, ok := db.queryQueue[db.queryCalls-1].rows.(*fakeRows); ok && prev != nil {
			previousClosed = prev.closed
		}
	}
	db.closedBeforeQuery = append(db.closedBeforeQuery, previousClosed)
	if db.queryCalls >= len(db.queryQueue) {
		return nil, fmt.Errorf("fakeDB: Query called more times (%d) than configured (%d)", db.queryCalls+1, len(db.queryQueue))
	}
	resp := db.queryQueue[db.queryCalls]
	db.queryCalls++
	return resp.rows, resp.err
}

func (db *fakeDB) Exec(_ context.Context, query string, args ...any) error {
	db.execCalls = append(db.execCalls, execCall{query: query, args: args})
	return nil
}

func (db *fakeDB) Begin(_ context.Context) (gateway.Tx, error) {
	if db.beginCalls >= len(db.beginQueue) {
		return nil, fmt.Errorf("fakeDB: Begin called more times (%d) than configured (%d)", db.beginCalls+1, len(db.beginQueue))
	}
	resp := db.beginQueue[db.beginCalls]
	db.beginCalls++
	return resp.tx, resp.err
}

// ---- 行の組み立てヘルパ -----------------------------------------------------

// intPtr は int のポインタを返す（NULL 許容の列に値ありを表すため）。
func intPtr(v int) *int { return &v }

// workMonthHeaderRow は勤務月の行（ヘッダ）を Scan の並びどおりに組み立てる
// （契約識別子・年・月・精算幅下限（時・分）・精算幅上限（時・分）・状態・
// 超過（時・分。NULL 許容）・不足（時・分。NULL 許容）の12列）。
func workMonthHeaderRow(
	contractID string, year, month int,
	lowerH, lowerM, upperH, upperM int,
	state string,
	excessH, excessM, shortfallH, shortfallM *int,
) []any {
	return []any{contractID, year, month, lowerH, lowerM, upperH, upperM, state, excessH, excessM, shortfallH, shortfallM}
}

// dailyRecordRow は稼働実績の行を Scan の並びどおりに組み立てる
// （実装設計 AC-9-15-i の11列: ①年・②月・③日・④開始時刻の時・⑤分・⑥終了時刻の時・⑦分・
// ⑧休憩開始時刻の時・⑨分・⑩休憩終了時刻の時・⑪分。④〜⑪はいずれも *int＝NULL を nil で受ける）。
// 稼働時間の列は持たない（AC-9-15-b・AC-10-5）。
// start・end は {時, 分}。brk が nil なら休憩の4列がすべて NULL（休憩なし）。
func dailyRecordRow(year, month, day int, start, end [2]int, brk *[4]int) []any {
	row := []any{year, month, day, intPtr(start[0]), intPtr(start[1]), intPtr(end[0]), intPtr(end[1])}
	if brk == nil {
		return append(row, (*int)(nil), (*int)(nil), (*int)(nil), (*int)(nil))
	}
	return append(row, intPtr(brk[0]), intPtr(brk[1]), intPtr(brk[2]), intPtr(brk[3]))
}

// dailyRecordRowRaw は ④〜⑪ の8列を *int のまま直接指定して行を組み立てる。
// 破損した行（一部だけ NULL）を作るために使う（AC-9-15-i (i)(ii)）。
func dailyRecordRowRaw(year, month, day int, cols [8]*int) []any {
	row := []any{year, month, day}
	for _, c := range cols {
		row = append(row, c)
	}
	return row
}

// contractRow は契約の行を Scan の並びどおりに組み立てる（識別子・契約表示名・
// 技術者識別子・精算幅下限（時・分）・精算幅上限（時・分）の7列。AC-9-17-a）。
func contractRow(id, displayName, engineerID string, lowerH, lowerM, upperH, upperM int) []any {
	return []any{id, displayName, engineerID, lowerH, lowerM, upperH, upperM}
}

// workMonthListRow は一覧の行を Scan の並びどおりに組み立てる（契約識別子・
// 契約表示名・年・月・状態の5列。AC-9-18-a・AC-6-7-d）。年・月を分けるのは
// 既存の workMonthHeaderRow と揃えるため（tester の選択）。
func workMonthListRow(contractID, displayName string, year, month int, state string) []any {
	return []any{contractID, displayName, year, month, state}
}

// strPtr は string のポインタを返す（NULL 許容の文字列列に値ありを表すため）。
func strPtr(v string) *string { return &v }

// contractListRow は契約一覧の行を Scan の並びどおりに組み立てる
// （実装設計 AC-9-20-c の11列: ①契約識別子・②契約表示名・③〜⑥精算幅（下限の時・分、
// 上限の時・分）・⑦〜⑩契約期間（開始の年・月、終了の年・月。いずれも *int）・
// ⑪指定した年月の勤務月の状態（*string。未生成なら nil））。
func contractListRow(
	id, displayName string,
	lowerH, lowerM, upperH, upperM int,
	startYear, startMonth, endYear, endMonth *int,
	state *string,
) []any {
	return []any{id, displayName, lowerH, lowerM, upperH, upperM, startYear, startMonth, endYear, endMonth, state}
}

// trendDailyRecordRow は稼働の推移の2回目の Query の行を Scan の並びどおりに
// 組み立てる（実装設計 AC-9-21-e の12列: ①契約識別子 string に、AC-9-15-i の11列
// （dailyRecordRow）が続く）。
func trendDailyRecordRow(contractID string, year, month, day int, start, end [2]int, brk *[4]int) []any {
	return append([]any{contractID}, dailyRecordRow(year, month, day, start, end, brk)...)
}
