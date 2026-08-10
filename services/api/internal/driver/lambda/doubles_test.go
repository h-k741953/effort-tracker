package lambda_test

// 本ファイルはテストダブルをすべて手書きのインメモリ Fake として用意する
// （ADR 0007・docs/rules/development-process.md「TDD」）。モックライブラリは
// 使わない。aws-lambda-go も pgx も実 DB も import しない
// （docs/specs/workmonth-implementation-design.md AC-12-6・AC-12-15・D-11）。
//
// gateway.DB / gateway.Rows / gateway.Tx に対する Fake の Scan 引数の並びは
// services/api/internal/adapter/gateway/doubles_test.go が既に固定した並び
// （contractSelectQuery の7列・workMonthHeaderSelectQuery の12列）にそのまま
// 合わせる。SQL 文そのものの正しさは検査しない（AC-13-18）。本ファイルの
// テストは「0行（未生成）」の応答しか使わないため、Scan は事実上呼ばれない
// （fakeRows.Next() が最初から false を返す）。

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// ---- ①のための http.Handler spy ---------------------------------------

// handlerSpy は http.Handler を満たす手書きの spy（AC-12-15①「経路の対応」の
// 検査に使う）。呼ばれた回数を記録し、固定の200を返す。
type handlerSpy struct {
	calls int
}

func (s *handlerSpy) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s.calls++
	w.WriteHeader(http.StatusOK)
}

// noopHandler は assembly_test.go で「今回の検査対象ではないエンドポイント」
// を埋めるためだけの http.Handler（呼ばれないことを期待する）。
var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// ---- ④のための SQL 実行インターフェースの Fake（gateway.DB / Rows / Tx） -----

// callRecord は Query・Exec への1回の呼び出しを記録する。
type callRecord struct {
	query string
	args  []any
}

// fakeRows は gateway.Rows に対する手書きの Fake。行は queryRow の列として
// 事前に設定する。
type queryRow []any

type fakeRows struct {
	rows []queryRow
	idx  int
}

func newFakeRows(rows ...queryRow) *fakeRows {
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

func (r *fakeRows) Close() error { return nil }
func (r *fakeRows) Err() error   { return nil }

// fakeScanAssign は Scan の1列分の代入を行う。サポートするのは
// *string / *int / **int（NULL 許容の int 列）のみ
// （adapter/gateway/doubles_test.go と同じ並び）。
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

// contractRow は契約の行を Scan の並びどおりに組み立てる（識別子・契約表示名・
// 技術者識別子・精算幅下限（時・分）・精算幅上限（時・分）の7列。
// adapter/gateway/contract_repository.go の Scan 順に合わせる）。
func contractRow(id, displayName, engineerID string, lowerH, lowerM, upperH, upperM int) queryRow {
	return queryRow{id, displayName, engineerID, lowerH, lowerM, upperH, upperM}
}

// workMonthListRow は一覧（E-2）の行を Scan の並びどおりに組み立てる
// （契約識別子・契約表示名・年・月・状態の5列。
// adapter/gateway/work_month_query.go の List の Scan 順に合わせる）。
func workMonthListRow(contractID, displayName string, year, month int, state string) queryRow {
	return queryRow{contractID, displayName, year, month, state}
}

// fakeTx は gateway.Tx に対する手書きの Fake。Exec の呼び出しを記録するだけで、
// 常に成功する（原子性そのもの・SQL 文の正しさは検査しない＝AC-13-18）。
type fakeTx struct {
	execCalls     []callRecord
	commitCount   int
	rollbackCount int
}

func (tx *fakeTx) Query(_ context.Context, _ string, _ ...any) (gateway.Rows, error) {
	return nil, fmt.Errorf("fakeTx: Query is not configured for this test")
}

func (tx *fakeTx) Exec(_ context.Context, query string, args ...any) error {
	tx.execCalls = append(tx.execCalls, callRecord{query: query, args: args})
	return nil
}

func (tx *fakeTx) Commit(_ context.Context) error {
	tx.commitCount++
	return nil
}

func (tx *fakeTx) Rollback(_ context.Context) error {
	tx.rollbackCount++
	return nil
}

// queryResponse は DB.Query への1回分の応答（呼び出し順に消費する）。
type queryResponse struct {
	rows gateway.Rows
	err  error
}

// fakeDB は gateway.DB に対する手書きの Fake（AC-12-15④(i) の検査に使う）。
type fakeDB struct {
	queryQueue []queryResponse
	queryLog   []callRecord
	execLog    []callRecord
	tx         *fakeTx
}

func newFakeDB() *fakeDB { return &fakeDB{tx: &fakeTx{}} }

// pushQuery は次の DB.Query 呼び出しへの応答を1つ積む（呼び出し順に消費）。
func (db *fakeDB) pushQuery(rows gateway.Rows, err error) {
	db.queryQueue = append(db.queryQueue, queryResponse{rows: rows, err: err})
}

func (db *fakeDB) Query(_ context.Context, query string, args ...any) (gateway.Rows, error) {
	db.queryLog = append(db.queryLog, callRecord{query: query, args: args})
	if len(db.queryQueue) == 0 {
		return nil, fmt.Errorf("fakeDB: Query called but no response was configured")
	}
	resp := db.queryQueue[0]
	db.queryQueue = db.queryQueue[1:]
	return resp.rows, resp.err
}

func (db *fakeDB) Exec(_ context.Context, query string, args ...any) error {
	db.execLog = append(db.execLog, callRecord{query: query, args: args})
	return nil
}

func (db *fakeDB) Begin(_ context.Context) (gateway.Tx, error) {
	return db.tx, nil
}

// callCount は Query・Exec・Tx.Exec を合わせた呼び出し総数
// （AC-12-15④(i)「SQL 実行 Fake への記録が1回以上あること」を検査するための
// 最小限の観測。どの SQL かは観測しない＝AC-13-18）。
func (db *fakeDB) callCount() int {
	return len(db.queryLog) + len(db.execLog) + len(db.tx.execCalls)
}

// hasArg は Query・Exec・Tx.Exec のいずれかの記録に、引数として want が
// 現れているかを返す（AC-12-15④(iv) の検査に使う）。文字列（contractId）だけ
// でなく int（年・月。レビュー往復1 I-3）も比較できるよう any で受け取る。
func (db *fakeDB) hasArg(want any) bool {
	all := append(append(append([]callRecord{}, db.queryLog...), db.execLog...), db.tx.execCalls...)
	for _, call := range all {
		for _, arg := range call.args {
			if arg == want {
				return true
			}
		}
	}
	return false
}

// ---- ④のための port.Clock の Fake -----------------------------------------

// fakeClock は port.Clock に対する手書きの Fake（AC-12-15④(ii) の検査に使う）。
type fakeClock struct {
	today workmonth.Date
	calls int
}

func (c *fakeClock) Today() workmonth.Date {
	c.calls++
	return c.today
}

var _ port.Clock = (*fakeClock)(nil)

// ---- 共通ヘルパ -------------------------------------------------------------

// mustDate は workmonth.Date を組み立てる。テストの固定値はすべて仕様が定める
// 書式（YYYY-MM-DD）に沿った妥当な暦日であり、業務ルールの推測を含まない。
func mustDate(t *testing.T, year, month, day int) workmonth.Date {
	t.Helper()
	d, err := workmonth.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("workmonth.NewDate(%d, %d, %d) が失敗した: %v", year, month, day, err)
	}
	return d
}
