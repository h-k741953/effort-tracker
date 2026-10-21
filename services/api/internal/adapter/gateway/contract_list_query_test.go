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

// 検証対象の受け入れ条件: 実装設計 AC-9-20（a〜h）・AC-6-10・AC-9-14-f・AC-9-19-a・
// AC-12-19-e。契約 AC-12-18（永続化行の破損）。
//
// Fake の行は doubles_test.go の contractListRow で AC-9-20-c の11個の並びと型
// （⑦〜⑩は *int、⑪は *string）に組み立てる。fakeRows.Scan は個数と型が合わない
// 行を Scan の失敗にするため、並びと型の食い違いは Red になる。

// contractListView は port.ContractListRow を go-cmp で比べるための射影
// （値オブジェクトの非公開フィールドに依存しない）。
type contractListView struct {
	ContractID  string
	DisplayName string
	Lower       [2]int
	Upper       [2]int
	StartYear   int
	StartMonth  int
	End         *[2]int
	State       *string
}

func viewOfContractListRow(row port.ContractListRow) contractListView {
	v := contractListView{
		ContractID:  row.ContractID.String(),
		DisplayName: row.DisplayName,
		Lower:       [2]int{row.SettlementRange.LowerBound().Hours(), row.SettlementRange.LowerBound().Minutes()},
		Upper:       [2]int{row.SettlementRange.UpperBound().Hours(), row.SettlementRange.UpperBound().Minutes()},
		StartYear:   row.Period.Start.Year(),
		StartMonth:  row.Period.Start.Month(),
		State:       row.WorkMonthState,
	}
	if row.Period.End != nil {
		v.End = &[2]int{row.Period.End.Year(), row.Period.End.Month()}
	}
	return v
}

func queryContractList(db *fakeDB) ([]port.ContractListRow, error) {
	ym, err := workmonth.NewYearMonth(2026, 10)
	if err != nil {
		panic(err)
	}
	return gateway.NewContractListQuery(db).ListByEngineer(context.Background(), "eng-0001", ym)
}

// TestContractListQuery_ListByEngineer_QueryArguments は Query が1回だけ・
// 引数が技術者識別子 string・年 int・月 int の3つの並びであることを検証する
// （AC-9-20-a・AC-12-19-e）。
func TestContractListQuery_ListByEngineer_QueryArguments(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(), nil)

	if _, err := queryContractList(db); err != nil {
		t.Fatalf("ListByEngineer failed: %v", err)
	}

	if db.queryCalls != 1 {
		t.Fatalf("Query の呼び出し回数 = %d, want 1（契約ごとに勤務月を引かない。AC-9-20-a）", db.queryCalls)
	}
	if diff := cmp.Diff([]any{"eng-0001", 2026, 10}, db.queryLog[0].args); diff != "" {
		t.Errorf("Query の引数が不一致 (-want +got):\n%s（AC-9-20-a: 技術者識別子 string・年 int・月 int）", diff)
	}
}

// TestContractListQuery_ListByEngineer_MapsRows は11個の Scan から
// port.ContractListRow へ写り、行の順が Fake の返した順のままであることを検証する
// （AC-9-20-c・AC-9-20-d (ii)(iii)(v)(vi)・AC-6-10-d・AC-6-10-e・AC-12-19-e）。
// 終了あり・終了未定・開始＝終了・勤務月の状態 nil / 非 nil を対にして置く。
func TestContractListQuery_ListByEngineer_MapsRows(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		contractListRow("ctr-0001", "サンプル株式会社 / 基幹システム保守", 140, 0, 180, 0, intPtr(2025), intPtr(4), intPtr(2027), intPtr(3), strPtr("Approved")),
		contractListRow("ctr-0002", "サンプル商事 / 受発注刷新", 150, 30, 190, 15, intPtr(2025), intPtr(4), nil, nil, nil),
		contractListRow("ctr-0003", "開始＝終了の契約", 100, 0, 160, 0, intPtr(2026), intPtr(6), intPtr(2026), intPtr(6), strPtr("Draft")),
		// ⑪ は値を検査せず素通しする（AC-9-20-d (vi)）。
		contractListRow("ctr-0004", "未定義の状態文字列", 0, 0, 0, 0, intPtr(2026), intPtr(1), nil, nil, strPtr("Bogus")),
	), nil)

	got, err := queryContractList(db)
	if err != nil {
		t.Fatalf("ListByEngineer failed: %v", err)
	}

	var views []contractListView
	for _, row := range got {
		views = append(views, viewOfContractListRow(row))
	}
	want := []contractListView{
		{ContractID: "ctr-0001", DisplayName: "サンプル株式会社 / 基幹システム保守", Lower: [2]int{140, 0}, Upper: [2]int{180, 0}, StartYear: 2025, StartMonth: 4, End: &[2]int{2027, 3}, State: strPtr("Approved")},
		{ContractID: "ctr-0002", DisplayName: "サンプル商事 / 受発注刷新", Lower: [2]int{150, 30}, Upper: [2]int{190, 15}, StartYear: 2025, StartMonth: 4, End: nil, State: nil},
		{ContractID: "ctr-0003", DisplayName: "開始＝終了の契約", Lower: [2]int{100, 0}, Upper: [2]int{160, 0}, StartYear: 2026, StartMonth: 6, End: &[2]int{2026, 6}, State: strPtr("Draft")},
		{ContractID: "ctr-0004", DisplayName: "未定義の状態文字列", Lower: [2]int{0, 0}, Upper: [2]int{0, 0}, StartYear: 2026, StartMonth: 1, End: nil, State: strPtr("Bogus")},
	}
	if diff := cmp.Diff(want, views); diff != "" {
		t.Errorf("行 → リードモデルの変換が不一致 (-want +got):\n%s（AC-9-20-c・AC-9-20-d）", diff)
	}
}

// TestContractListQuery_ListByEngineer_BrokenRowIsInvalidValue は AC-9-20-d の
// (i)〜(v) の各破損が ErrInvalidValue になることを検証する（契約 AC-12-18）。
// 破損の行は最初に当たったエラーで全体を打ち切る（残りの行を写さない・
// 一部の行だけを返さない）ため、正常な行を前後に置いて両方を確認する。
func TestContractListQuery_ListByEngineer_BrokenRowIsInvalidValue(t *testing.T) {
	good := contractListRow("ctr-0001", "正常", 140, 0, 180, 0, intPtr(2025), intPtr(4), nil, nil, nil)
	tests := []struct {
		name   string
		broken []any
	}{
		{name: "(i) 開始の年が nil", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, nil, intPtr(4), nil, nil, nil)},
		{name: "(i) 開始の月が nil", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2025), nil, nil, nil, nil)},
		{name: "(i) 開始の年月がともに nil", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, nil, nil, nil, nil, nil)},
		{name: "(ii) 終了の年だけ nil", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2025), intPtr(4), nil, intPtr(3), nil)},
		{name: "(ii) 終了の月だけ nil", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2025), intPtr(4), intPtr(2027), nil, nil)},
		{name: "(iii) 開始の月が 13", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2025), intPtr(13), nil, nil, nil)},
		{name: "(iii) 開始の月が 0", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2025), intPtr(0), nil, nil, nil)},
		{name: "(iii) 終了の月が 13", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2025), intPtr(4), intPtr(2027), intPtr(13), nil)},
		{name: "(iv) 開始 2026-07 > 終了 2026-06（年が等しく月が大きい）", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2026), intPtr(7), intPtr(2026), intPtr(6), nil)},
		{name: "(iv) 開始 2027-01 > 終了 2026-12（年が大きい）", broken: contractListRow("ctr-0009", "x", 140, 0, 180, 0, intPtr(2027), intPtr(1), intPtr(2026), intPtr(12), nil)},
		{name: "(v) 契約識別子が空", broken: contractListRow("", "x", 140, 0, 180, 0, intPtr(2025), intPtr(4), nil, nil, nil)},
		{name: "(v) 精算幅の分が 60", broken: contractListRow("ctr-0009", "x", 140, 60, 180, 0, intPtr(2025), intPtr(4), nil, nil, nil)},
		{name: "(v) 精算幅の時が負", broken: contractListRow("ctr-0009", "x", -1, 0, 180, 0, intPtr(2025), intPtr(4), nil, nil, nil)},
		{name: "(v) 精算幅の下限 > 上限", broken: contractListRow("ctr-0009", "x", 180, 0, 140, 0, intPtr(2025), intPtr(4), nil, nil, nil)},
	}

	for _, position := range []string{"先頭", "2行目"} {
		for _, tt := range tests {
			t.Run(position+"/"+tt.name, func(t *testing.T) {
				var rowList [][]any
				if position == "先頭" {
					rowList = [][]any{tt.broken, good}
				} else {
					rowList = [][]any{good, tt.broken}
				}
				rows := newFakeRows(rowList...)
				db := newFakeDB()
				db.pushQuery(rows, nil)

				got, err := queryContractList(db)
				if !errors.Is(err, workmonth.ErrInvalidValue) {
					t.Fatalf("ListByEngineer のエラー = %v, want errors.Is(err, workmonth.ErrInvalidValue)（AC-9-20-d）", err)
				}
				if errors.Is(err, port.ErrContractNotFound) {
					t.Errorf("破損が ErrContractNotFound に変換されている: %v", err)
				}
				if len(got) != 0 {
					t.Errorf("エラーなのに行を返している: %+v（一部の行を返さない。AC-9-20-d）", got)
				}
				if !rows.closed {
					t.Errorf("エラーで抜けた経路で Rows が Close されていない（AC-9-20-g）")
				}
			})
		}
	}
}

// TestContractListQuery_ListByEngineer_BoundaryStartEqualsEndPasses は開始＝終了の
// 行がエラーにならないことを単独で検証する（AC-9-20-d (iv)・契約 AC-12-10）。
// 「<」を「<=」に取り違える実装を Red にするため、月違い・年違いの隣接ケースを対にする。
func TestContractListQuery_ListByEngineer_BoundaryStartEqualsEndPasses(t *testing.T) {
	tests := []struct {
		name                         string
		startYear, startMon, endYear int
		endMon                       int
	}{
		{name: "開始＝終了（2026-06〜2026-06）", startYear: 2026, startMon: 6, endYear: 2026, endMon: 6},
		{name: "終了が開始の翌月（2026-06〜2026-07）", startYear: 2026, startMon: 6, endYear: 2026, endMon: 7},
		{name: "年をまたぐ（2026-12〜2027-01）", startYear: 2026, startMon: 12, endYear: 2027, endMon: 1},
		{name: "開始の月が終了の月より大きくても終了の年が後なら通る（2026-11〜2027-02）", startYear: 2026, startMon: 11, endYear: 2027, endMon: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newFakeDB()
			db.pushQuery(newFakeRows(
				contractListRow("ctr-0001", "x", 140, 0, 180, 0, intPtr(tt.startYear), intPtr(tt.startMon), intPtr(tt.endYear), intPtr(tt.endMon), nil),
			), nil)

			got, err := queryContractList(db)
			if err != nil {
				t.Fatalf("ListByEngineer failed: %v（開始が終了より後でないので通る。AC-9-20-d (iv)）", err)
			}
			if len(got) != 1 {
				t.Fatalf("行数 = %d, want 1", len(got))
			}
		})
	}
}

// TestContractListQuery_ListByEngineer_EmptyIsEmptySliceNotError は0行で空の
// スライス（非 nil）・エラーなしを返し、ErrContractNotFound へ変換しないことを
// 検証する（AC-9-20-f・AC-6-10-f・AC-12-19-e）。
func TestContractListQuery_ListByEngineer_EmptyIsEmptySliceNotError(t *testing.T) {
	db := newFakeDB()
	rows := newFakeRows()
	db.pushQuery(rows, nil)

	got, err := queryContractList(db)
	if errors.Is(err, port.ErrContractNotFound) {
		t.Fatalf("0件が port.ErrContractNotFound へ変換されている（AC-9-20-f）")
	}
	if err != nil {
		t.Fatalf("ListByEngineer failed: %v", err)
	}
	if got == nil {
		t.Errorf("0件のとき nil が返っている（空のスライスであること。AC-6-10-f）")
	}
	if len(got) != 0 {
		t.Errorf("行数 = %d, want 0", len(got))
	}
	if !rows.closed {
		t.Errorf("0行でも Rows が Close されていない（AC-9-20-g）")
	}
}

// TestContractListQuery_ListByEngineer_ClosesRowsAndPassesDriverErrors は走査を
// 全経路で閉じ、ドライバ由来のエラーをそのまま返すことを検証する
// （AC-9-20-g・AC-9-19-a・AC-12-19-e）。
func TestContractListQuery_ListByEngineer_ClosesRowsAndPassesDriverErrors(t *testing.T) {
	scanErr := errors.New("fake: scan failed")
	iterErr := errors.New("fake: iteration failed mid-stream")
	queryErr := errors.New("fake: connection reset by peer")

	t.Run("成功した経路で閉じる", func(t *testing.T) {
		rows := newFakeRows(contractListRow("ctr-0001", "x", 140, 0, 180, 0, intPtr(2025), intPtr(4), nil, nil, nil))
		db := newFakeDB()
		db.pushQuery(rows, nil)
		if _, err := queryContractList(db); err != nil {
			t.Fatalf("ListByEngineer failed: %v", err)
		}
		if !rows.closed {
			t.Errorf("成功した経路で Rows が Close されていない（AC-9-20-g）")
		}
	})

	t.Run("Scan の失敗はそのまま返し、閉じる", func(t *testing.T) {
		rows := newFakeRows(contractListRow("ctr-0001", "x", 140, 0, 180, 0, intPtr(2025), intPtr(4), nil, nil, nil))
		rows.scanErr = scanErr
		db := newFakeDB()
		db.pushQuery(rows, nil)
		_, err := queryContractList(db)
		if !errors.Is(err, scanErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, scanErr)（AC-9-19-a）", err)
		}
		if !rows.closed {
			t.Errorf("Scan の失敗後に Rows が Close されていない（AC-9-20-g）")
		}
	})

	t.Run("走査中のエラーはそのまま返し、閉じる", func(t *testing.T) {
		rows := newFakeRows()
		rows.iterErr = iterErr
		db := newFakeDB()
		db.pushQuery(rows, nil)
		_, err := queryContractList(db)
		if !errors.Is(err, iterErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, iterErr)（AC-9-20-g）", err)
		}
		if !rows.closed {
			t.Errorf("走査中のエラー後に Rows が Close されていない（AC-9-20-g）")
		}
	})

	t.Run("Query 自体の失敗はそのまま返す", func(t *testing.T) {
		db := newFakeDB()
		db.pushQuery(nil, queryErr)
		_, err := queryContractList(db)
		if !errors.Is(err, queryErr) {
			t.Fatalf("エラー = %v, want errors.Is(err, queryErr)（AC-9-19-a）", err)
		}
	})
}
