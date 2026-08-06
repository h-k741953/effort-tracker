package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-18・AC-12-13。
//
// 総件数の取得手段（別の集計クエリか窓関数か）は本仕様が固定しない
// （AC-13-20）。本ファイルは「行取得クエリ1回 → 件数取得クエリ1回」という
// 2クエリ構成を tester が選んだ一案として仮定する（doubles_test.go の
// 前書きと同じ位置づけ）。実装工程が異なる構成（例: 窓関数で1クエリに統合）
// を選ぶ場合、本ファイルの pushQuery の並びを調整する（本 AC が固定するのは
// AC-12-13 が列挙する項目までであり、SQL 文そのもの・クエリの回数は
// 検査しない＝AC-13-18）。

// TestWorkMonthQuery_List_MapsRowToReadModelAndReturnsTotal は、行が
// リードモデルへ変換されること（①）と総件数が返ること（②）を検証する
// （AC-9-18-a・AC-9-18-d・AC-9-18-e）。
func TestWorkMonthQuery_List_MapsRowToReadModelAndReturnsTotal(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthListRow("ctr-0001", "サンプル株式会社 / 基幹システム保守", 2026, 7, "PendingApproval"),
	), nil)
	db.pushQuery(newFakeRows([]any{1}), nil)

	q := gateway.NewWorkMonthQuery(db)
	got, total, err := q.List(context.Background(), port.WorkMonthQueryCondition{EngineerID: "eng-0001", Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	want := []port.WorkMonthQueryRow{
		{ContractID: "ctr-0001", ContractDisplayName: "サンプル株式会社 / 基幹システム保守", YearMonth: "2026-07", State: "PendingApproval"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("行 → リードモデルの変換が不一致 (-want +got):\n%s（AC-9-18-a）", diff)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1（AC-9-18-d・AC-9-18-e）", total)
	}
}

// TestWorkMonthQuery_List_EmptyResultIsNotWorkMonthNotFound は、該当0件が
// ErrWorkMonthNotFound へ変換されず、空スライス・総件数0として返ることを
// 検証する（AC-9-18-h。Find＝AC-9-15-e とは扱いが異なる）。
func TestWorkMonthQuery_List_EmptyResultIsNotWorkMonthNotFound(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(), nil)
	db.pushQuery(newFakeRows([]any{0}), nil)

	q := gateway.NewWorkMonthQuery(db)
	got, total, err := q.List(context.Background(), port.WorkMonthQueryCondition{EngineerID: "eng-nonexistent", Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if errors.Is(err, port.ErrWorkMonthNotFound) {
		t.Fatalf("0件が port.ErrWorkMonthNotFound へ変換されている（AC-9-18-h。Find＝AC-9-15-e とは扱いが異なる）")
	}
	if got == nil {
		t.Errorf("0件のとき nil が返っている（AC-9-18-h。空スライスであること）")
	}
	if len(got) != 0 {
		t.Errorf("行の件数 = %d, want 0", len(got))
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

// TestWorkMonthQuery_List_DriverErrorPassedThrough は、Query 自体が返した
// ドライバ由来のエラーがそのまま返ることを検証する（AC-9-19-a・AC-12-13④）。
func TestWorkMonthQuery_List_DriverErrorPassedThrough(t *testing.T) {
	driverErr := errors.New("connection reset by peer")
	db := newFakeDB()
	db.pushQuery(nil, driverErr)

	q := gateway.NewWorkMonthQuery(db)
	_, _, err := q.List(context.Background(), port.WorkMonthQueryCondition{Limit: 20, Offset: 0})
	if !errors.Is(err, driverErr) {
		t.Fatalf("List のエラー = %v, want errors.Is(err, driverErr)（AC-9-19-a）", err)
	}
}

// TestWorkMonthQuery_List_IterationErrorClosesRowsAndPropagates は、走査中の
// エラーを握り潰さずに伝播し、Rows を Close することを検証する（AC-12-13④）。
func TestWorkMonthQuery_List_IterationErrorClosesRowsAndPropagates(t *testing.T) {
	iterErr := errors.New("row scan failed mid-stream")
	rows := newFakeRows()
	rows.iterErr = iterErr
	db := newFakeDB()
	db.pushQuery(rows, nil)

	q := gateway.NewWorkMonthQuery(db)
	_, _, err := q.List(context.Background(), port.WorkMonthQueryCondition{Limit: 20, Offset: 0})
	if !errors.Is(err, iterErr) {
		t.Fatalf("List のエラー = %v, want errors.Is(err, iterErr)（走査中のエラーを握り潰さない。AC-12-13④）", err)
	}
	if !rows.closed {
		t.Errorf("走査中のエラー後に Rows が Close されていない（AC-12-13④）")
	}
}

// TestWorkMonthQuery_List_OmittedConditionsAreNotBoundAsFilters は、条件を
// 省略した（空文字列の）ときにその条件で絞り込まないことを、Fake が記録した
// 引数で観測する（AC-9-18-g・AC-12-13⑤）。
func TestWorkMonthQuery_List_OmittedConditionsAreNotBoundAsFilters(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		workMonthListRow("ctr-0001", "サンプル株式会社 / 基幹システム保守", 2026, 7, "Draft"),
	), nil)
	db.pushQuery(newFakeRows([]any{1}), nil)

	q := gateway.NewWorkMonthQuery(db)
	got, total, err := q.List(context.Background(), port.WorkMonthQueryCondition{EngineerID: "", State: "", Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(got) != 1 || total != 1 {
		t.Fatalf("Fake が設定した行・総件数が反映されていない（AC-9-18-g 検証の前提）: got=%v, total=%d", got, total)
	}

	for _, call := range db.queryLog {
		for _, arg := range call.args {
			if s, ok := arg.(string); ok && s == "" {
				t.Errorf("省略した条件（空文字列）が SQL の引数として渡っている（AC-9-18-g）: query=%q args=%v", call.query, call.args)
			}
		}
	}
}
