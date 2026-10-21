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
//   - docs/specs/workmonth-implementation-design.md AC-7-19（ListContracts の
//     入力・依存・責務順序）・AC-8-12（認可）・AC-7-21（入出力 DTO）・
//     AC-12-19-a（①〜⑨）
//   - docs/specs/domain-api-http-contract.md AC-9（判定順序。順1 → 順4）・AC-12-6・12-7

func strPtr(s string) *string { return &s }

func ymPtr(ym workmonth.YearMonth) *workmonth.YearMonth { return &ym }

// TestListContracts_Authorization は AC-12-19-a ①〜④（認可の両側）を検証する。
// ①〜③では ContractListQuery が0回、④では1回で、受け取った技術者識別子と年月が
// 入力のままであること。
func TestListContracts_Authorization(t *testing.T) {
	tests := []struct {
		name    string
		actor   port.Actor
		wantErr error // nil なら成功
	}{
		{name: "① 未認証は ErrUnauthenticated", actor: guestActor(), wantErr: port.ErrUnauthenticated},
		{name: "② Approver で識別子一致でも ErrNotOwner", actor: ownerActor(port.RoleApprover), wantErr: port.ErrNotOwner},
		{name: "③ Engineer で識別子不一致は ErrNotOwner", actor: foreignActor(port.RoleEngineer), wantErr: port.ErrNotOwner},
		{name: "③' Approver で識別子不一致も ErrNotOwner", actor: foreignActor(port.RoleApprover), wantErr: port.ErrNotOwner},
		{name: "④ Engineer で識別子一致は成功", actor: ownerActor(port.RoleEngineer)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &fakeContractListQuery{rows: []port.ContractListRow{}}
			output := &spyListContractsOutputPort{}
			ym := mustYearMonth(t, 2026, 10)

			interactor.NewListContracts(query, output).Execute(context.Background(), port.ListContractsInput{
				Actor:      tt.actor,
				EngineerID: testEngineerID,
				YearMonth:  ym,
			})

			if tt.wantErr != nil {
				err := output.onlyPresentedError(t)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if len(query.calls) != 0 {
					t.Errorf("弾かれたのに ContractListQuery が呼ばれた（回数 = %d, want 0。AC-7-19）", len(query.calls))
				}
				return
			}

			output.onlyPresented(t)
			want := []contractListCall{{engineerID: testEngineerID, yearMonth: ym}}
			if diff := cmp.Diff(want, query.calls, cmp.AllowUnexported(contractListCall{}, workmonth.YearMonth{})); diff != "" {
				t.Errorf("ContractListQuery への引数が入力のままでない (-want +got):\n%s（AC-12-19-a ④）", diff)
			}
		})
	}
}

// TestListContracts_MapsRows は AC-12-19-a ⑤〜⑦を検証する。
//   - ⑤ WorkMonthState が nil → State "Draft"・Generated 偽、非 nil → その値・真
//   - ⑥ End が nil → nil、非 nil → "YYYY-MM"
//   - ⑦ 行の順が Fake の返した順のまま
func TestListContracts_MapsRows(t *testing.T) {
	ym := mustYearMonth(t, 2026, 10)
	rows := []port.ContractListRow{
		{
			ContractID:      mustContractID(t, "ctr-0001"),
			DisplayName:     "サンプル株式会社 / 基幹システム保守",
			SettlementRange: mustSettlementRange(t, 140, 180),
			Period:          port.ContractPeriod{Start: mustYearMonth(t, 2025, 4), End: ymPtr(mustYearMonth(t, 2027, 3))},
			WorkMonthState:  strPtr("Approved"),
		},
		{
			ContractID:      mustContractID(t, "ctr-0002"),
			DisplayName:     "サンプル商事 / 受発注刷新",
			SettlementRange: mustSettlementRange(t, 150, 190),
			Period:          port.ContractPeriod{Start: mustYearMonth(t, 2025, 4), End: nil},
			WorkMonthState:  nil,
		},
		{
			// 生成済みの Draft は未生成（nil）と区別され、Generated は真になる（AC-6-10-e）。
			ContractID:      mustContractID(t, "ctr-0003"),
			DisplayName:     "開始＝終了の契約",
			SettlementRange: mustSettlementRange(t, 100, 160),
			Period:          port.ContractPeriod{Start: mustYearMonth(t, 2026, 6), End: ymPtr(mustYearMonth(t, 2026, 6))},
			WorkMonthState:  strPtr("Draft"),
		},
	}
	query := &fakeContractListQuery{rows: rows}
	output := &spyListContractsOutputPort{}

	interactor.NewListContracts(query, output).Execute(context.Background(), port.ListContractsInput{
		Actor:      ownerActor(port.RoleEngineer),
		EngineerID: testEngineerID,
		YearMonth:  ym,
	})

	want := port.ListContractsOutput{Items: []port.ContractOutputRow{
		{
			ContractID:          "ctr-0001",
			ContractDisplayName: "サンプル株式会社 / 基幹システム保守",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 140},
				UpperBound: port.Hours{Hours: 180},
			},
			ContractPeriod: port.ContractPeriodOutput{Start: "2025-04", End: strPtr("2027-03")},
			WorkMonth:      port.ContractWorkMonthOutput{YearMonth: "2026-10", State: "Approved", Generated: true},
		},
		{
			ContractID:          "ctr-0002",
			ContractDisplayName: "サンプル商事 / 受発注刷新",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 150},
				UpperBound: port.Hours{Hours: 190},
			},
			ContractPeriod: port.ContractPeriodOutput{Start: "2025-04", End: nil},
			WorkMonth:      port.ContractWorkMonthOutput{YearMonth: "2026-10", State: "Draft", Generated: false},
		},
		{
			ContractID:          "ctr-0003",
			ContractDisplayName: "開始＝終了の契約",
			SettlementRange: port.SettlementRangeOutput{
				LowerBound: port.Hours{Hours: 100},
				UpperBound: port.Hours{Hours: 160},
			},
			ContractPeriod: port.ContractPeriodOutput{Start: "2026-06", End: strPtr("2026-06")},
			WorkMonth:      port.ContractWorkMonthOutput{YearMonth: "2026-10", State: "Draft", Generated: true},
		},
	}}
	if diff := cmp.Diff(want, output.onlyPresented(t)); diff != "" {
		t.Errorf("出力 DTO が不一致 (-want +got):\n%s（AC-7-19④・AC-12-19-a ⑤〜⑦）", diff)
	}
}

// TestListContracts_EmptyResultIsEmptySliceNotNil は AC-12-19-a ⑧を検証する。
// Fake が nil スライスを返しても、出力の Items は空で非 nil。
func TestListContracts_EmptyResultIsEmptySliceNotNil(t *testing.T) {
	tests := []struct {
		name string
		rows []port.ContractListRow
	}{
		{name: "Fake が空のスライスを返す", rows: []port.ContractListRow{}},
		{name: "Fake が nil を返す", rows: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &fakeContractListQuery{rows: tt.rows}
			output := &spyListContractsOutputPort{}

			interactor.NewListContracts(query, output).Execute(context.Background(), port.ListContractsInput{
				Actor:      ownerActor(port.RoleEngineer),
				EngineerID: testEngineerID,
				YearMonth:  mustYearMonth(t, 2026, 10),
			})

			got := output.onlyPresented(t)
			if got.Items == nil {
				t.Errorf("Items が nil（0件でも空スライスであること。AC-7-21・AC-12-19-a ⑧）")
			}
			if len(got.Items) != 0 {
				t.Errorf("Items の件数 = %d, want 0", len(got.Items))
			}
		})
	}
}

// TestListContracts_QueryErrorPassesThrough は AC-12-19-a ⑨を検証する。
// Fake のエラー（ErrInvalidValue を含む）がそのまま PresentError へ渡る。
func TestListContracts_QueryErrorPassesThrough(t *testing.T) {
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
			query := &fakeContractListQuery{err: tt.err}
			output := &spyListContractsOutputPort{}

			interactor.NewListContracts(query, output).Execute(context.Background(), port.ListContractsInput{
				Actor:      ownerActor(port.RoleEngineer),
				EngineerID: testEngineerID,
				YearMonth:  mustYearMonth(t, 2026, 10),
			})

			if err := output.onlyPresentedError(t); !errors.Is(err, tt.err) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)（AC-7-19③）", err, tt.err)
			}
		})
	}
}
