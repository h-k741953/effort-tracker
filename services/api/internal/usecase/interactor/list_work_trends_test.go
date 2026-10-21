package interactor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/interactor"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-7-20（ListWorkTrends の
//     入力・依存・責務順序）・AC-8-12（認可）・AC-7-21（入出力 DTO）・
//     AC-12-19-b（①〜⑨。12-19-a の①〜④・⑦〜⑨と同じ組）
//   - docs/specs/domain-api-http-contract.md AC-9（判定順序。順1 → 順4）・AC-13

// reconstructTrendMonth は推移の前提となる集約を Reconstruct で組み立てる。
// excess / shortfall は状態との整合（AC-5-9）を満たす値を呼び出し側が渡す。
func reconstructTrendMonth(
	t *testing.T,
	contractID string,
	year, month int,
	settlement workmonth.SettlementRange,
	state workmonth.State,
	records []workmonth.DailyRecord,
	excess, shortfall *workmonth.WorkingHours,
) *workmonth.WorkMonth {
	t.Helper()
	target, err := workmonth.Reconstruct(mustContractID(t, contractID), mustYearMonth(t, year, month), settlement, state, records, excess, shortfall)
	if err != nil {
		t.Fatalf("前提の構築に失敗: Reconstruct(%s %d-%02d %s): %v", contractID, year, month, state, err)
	}
	return target
}

func whPtr(t *testing.T, hours, minutes int) *workmonth.WorkingHours {
	t.Helper()
	w := mustWorkingHours(t, hours, minutes)
	return &w
}

// TestListWorkTrends_Authorization は AC-12-19-b（12-19-a の①〜④と同じ組）を検証する。
func TestListWorkTrends_Authorization(t *testing.T) {
	tests := []struct {
		name    string
		actor   port.Actor
		wantErr error
	}{
		{name: "① 未認証は ErrUnauthenticated", actor: guestActor(), wantErr: port.ErrUnauthenticated},
		{name: "② Approver で識別子一致でも ErrNotOwner", actor: ownerActor(port.RoleApprover), wantErr: port.ErrNotOwner},
		{name: "③ Engineer で識別子不一致は ErrNotOwner", actor: foreignActor(port.RoleEngineer), wantErr: port.ErrNotOwner},
		{name: "③' Approver で識別子不一致も ErrNotOwner", actor: foreignActor(port.RoleApprover), wantErr: port.ErrNotOwner},
		{name: "④ Engineer で識別子一致は成功", actor: ownerActor(port.RoleEngineer)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &fakeWorkTrendQuery{months: []*workmonth.WorkMonth{}}
			output := &spyListWorkTrendsOutputPort{}

			interactor.NewListWorkTrends(query, output).Execute(context.Background(), port.ListWorkTrendsInput{
				Actor:      tt.actor,
				EngineerID: testEngineerID,
			})

			if tt.wantErr != nil {
				err := output.onlyPresentedError(t)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if len(query.calls) != 0 {
					t.Errorf("弾かれたのに WorkTrendQuery が呼ばれた（回数 = %d, want 0。AC-7-20）", len(query.calls))
				}
				return
			}

			output.onlyPresented(t)
			if diff := cmp.Diff([]string{testEngineerID}, query.calls); diff != "" {
				t.Errorf("WorkTrendQuery への技術者識別子が入力のままでない (-want +got):\n%s（AC-12-19-b ④）", diff)
			}
		})
	}
}

// TestListWorkTrends_MapsAggregates は AC-12-19-b ⑤⑥⑦を検証する。
//   - ⑤ Draft の集約 → Excess・Shortfall が nil、PendingApproval / Approved → 保持した値
//   - ⑥ SettlementRange は集約のスナップショット、TotalHours は TotalHours() と一致
//   - ⑦ 行の順が Fake の返した順のまま
func TestListWorkTrends_MapsAggregates(t *testing.T) {
	months := []*workmonth.WorkMonth{
		reconstructTrendMonth(t, "ctr-0001", 2026, 8, mustSettlementRange(t, 140, 180), workmonth.StateApproved,
			[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 8, 1, 8, 0), mustDailyRecord(t, 2026, 8, 2, 7, 30)},
			whPtr(t, 0, 15), whPtr(t, 0, 0)),
		reconstructTrendMonth(t, "ctr-0001", 2026, 9, mustSettlementRange(t, 100, 160), workmonth.StateDraft,
			[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 9, 1, 6, 45)},
			nil, nil),
		reconstructTrendMonth(t, "ctr-0002", 2026, 7, mustSettlementRange(t, 150, 190), workmonth.StatePendingApproval,
			[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			whPtr(t, 0, 0), whPtr(t, 3, 30)),
	}
	query := &fakeWorkTrendQuery{months: months}
	output := &spyListWorkTrendsOutputPort{}

	interactor.NewListWorkTrends(query, output).Execute(context.Background(), port.ListWorkTrendsInput{
		Actor:      ownerActor(port.RoleEngineer),
		EngineerID: testEngineerID,
	})

	want := port.ListWorkTrendsOutput{Items: []port.WorkTrendOutputRow{
		{
			ContractID: "ctr-0001",
			YearMonth:  "2026-08",
			State:      "Approved",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 140},
				UpperBound: port.Hours{Hours: 180},
			},
			TotalHours: port.Hours{Hours: 15, Minutes: 30},
			Excess:     &port.Hours{Hours: 0, Minutes: 15},
			Shortfall:  &port.Hours{Hours: 0, Minutes: 0},
		},
		{
			ContractID: "ctr-0001",
			YearMonth:  "2026-09",
			State:      "Draft",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 100},
				UpperBound: port.Hours{Hours: 160},
			},
			TotalHours: port.Hours{Hours: 6, Minutes: 45},
			Excess:     nil,
			Shortfall:  nil,
		},
		{
			ContractID: "ctr-0002",
			YearMonth:  "2026-07",
			State:      "PendingApproval",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 150},
				UpperBound: port.Hours{Hours: 190},
			},
			TotalHours: port.Hours{Hours: 8},
			Excess:     &port.Hours{Hours: 0, Minutes: 0},
			Shortfall:  &port.Hours{Hours: 3, Minutes: 30},
		},
	}}
	if diff := cmp.Diff(want, output.onlyPresented(t)); diff != "" {
		t.Errorf("出力 DTO が不一致 (-want +got):\n%s（AC-7-20④・AC-12-19-b ⑤〜⑦）", diff)
	}
}

// TestListWorkTrends_EmptyResultIsEmptySliceNotNil は AC-12-19-b ⑧を検証する。
func TestListWorkTrends_EmptyResultIsEmptySliceNotNil(t *testing.T) {
	tests := []struct {
		name   string
		months []*workmonth.WorkMonth
	}{
		{name: "Fake が空のスライスを返す", months: []*workmonth.WorkMonth{}},
		{name: "Fake が nil を返す", months: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &fakeWorkTrendQuery{months: tt.months}
			output := &spyListWorkTrendsOutputPort{}

			interactor.NewListWorkTrends(query, output).Execute(context.Background(), port.ListWorkTrendsInput{
				Actor:      ownerActor(port.RoleEngineer),
				EngineerID: testEngineerID,
			})

			got := output.onlyPresented(t)
			if got.Items == nil {
				t.Errorf("Items が nil（0件でも空スライスであること。AC-7-21・AC-12-19-b ⑧）")
			}
			if len(got.Items) != 0 {
				t.Errorf("Items の件数 = %d, want 0", len(got.Items))
			}
		})
	}
}

// TestListWorkTrends_QueryErrorPassesThrough は AC-12-19-b ⑨を検証する。
func TestListWorkTrends_QueryErrorPassesThrough(t *testing.T) {
	driverErr := errors.New("fake: query failed unexpectedly")
	tests := []struct {
		name string
		err  error
	}{
		{name: "ErrInvalidValue", err: workmonth.ErrInvalidValue},
		{name: "ドライバ由来のエラー", err: driverErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &fakeWorkTrendQuery{err: tt.err}
			output := &spyListWorkTrendsOutputPort{}

			interactor.NewListWorkTrends(query, output).Execute(context.Background(), port.ListWorkTrendsInput{
				Actor:      ownerActor(port.RoleEngineer),
				EngineerID: testEngineerID,
			})

			if err := output.onlyPresentedError(t); !errors.Is(err, tt.err) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)（AC-7-20③）", err, tt.err)
			}
		})
	}
}
