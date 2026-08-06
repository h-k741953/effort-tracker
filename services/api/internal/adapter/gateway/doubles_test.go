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

// dailyRecordRow は稼働実績の行を Scan の並びどおりに組み立てる（年・月・日・
// 稼働時間（時・分）の5列）。
func dailyRecordRow(year, month, day, hours, minutes int) []any {
	return []any{year, month, day, hours, minutes}
}

// contractRow は契約の行を Scan の並びどおりに組み立てる（識別子・契約表示名・
// 技術者識別子・精算幅下限（時・分）・精算幅上限（時・分）の7列。AC-9-17-a）。
func contractRow(id, displayName, engineerID string, lowerH, lowerM, upperH, upperM int) []any {
	return []any{id, displayName, engineerID, lowerH, lowerM, upperH, upperM}
}

// workMonthListRow は一覧の行を Scan の並びどおりに組み立てる（契約識別子・
// 契約表示名・年・月・状態の5列。AC-9-18-a・AC-6-7-d）。年・月を分けるのは
// 既存の workMonthHeaderRow・dailyRecordRow と揃えるため（tester の選択）。
func workMonthListRow(contractID, displayName string, year, month int, state string) []any {
	return []any{contractID, displayName, year, month, state}
}
