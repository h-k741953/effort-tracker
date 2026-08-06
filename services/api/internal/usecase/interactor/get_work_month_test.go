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
//   - docs/specs/workmonth-implementation-design.md AC-7-15（GetWorkMonth の
//     入力・依存・責務順序）・AC-8-8（認可を判定しない）・AC-12-12（GetWorkMonth
//     のテスト。①〜⑥）

func (f *fixture) get() *interactor.GetWorkMonth {
	return interactor.NewGetWorkMonth(f.workMonths, f.contracts, f.output)
}

// TestGetWorkMonth_GuestCanRead はゲスト（未認証）でも Present が呼ばれる
// ことを検証する（AC-8-8・AC-12-12①）。GetWorkMonth は認証・認可のいずれも
// 判定しない。
func TestGetWorkMonth_GuestCanRead(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.workMonths.put(reconstructWorkMonth(
		t, f.contractID, f.yearMonth,
		mustSettlementRange(t, 140, 180),
		workmonth.StateDraft,
		[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
	))

	f.get().Execute(context.Background(), port.GetWorkMonthInput{
		Actor:      guestActor(),
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	f.output.onlyPresented(t)
	if f.workMonths.saveCount != 0 {
		t.Errorf("参照なのに Save が呼ばれた（回数 = %d, want 0）（AC-7-15⑥・AC-12-12⑥）", f.workMonths.saveCount)
	}
}

// TestGetWorkMonth_ContractNotFound は契約が実在しない場合 ErrContractNotFound
// が PresentError へ渡ることを検証する（AC-12-12②）。
func TestGetWorkMonth_ContractNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.get().Execute(context.Background(), port.GetWorkMonthInput{
		Actor:      f.actor,
		ContractID: mustContractID(t, "ctr-unknown"),
		YearMonth:  f.yearMonth,
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrContractNotFound) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrContractNotFound)（AC-12-12②）", err)
	}
	if f.workMonths.saveCount != 0 {
		t.Errorf("参照なのに Save が呼ばれた（回数 = %d, want 0）（AC-7-15⑥）", f.workMonths.saveCount)
	}
}

// TestGetWorkMonth_GeneratedVsUngenerated は生成済み・未生成の両方の出力を
// 対にして検証する（AC-12-12③④）。
//
//   - 生成済み: Generated=真。契約表示名はリードモデルの値。精算幅は集約側の
//     スナップショット（契約の現在値では上書きしない）。
//   - 未生成: Generated=偽。State=Draft。稼働実績は空スライス。超過／不足は
//     値なし（nil）。精算幅は契約の現在値。
func TestGetWorkMonth_GeneratedVsUngenerated(t *testing.T) {
	tests := []struct {
		name      string
		generated bool
		want      port.WorkMonthOutput
	}{
		{
			name:      "生成済み：契約表示名はリードモデルの値・精算幅は集約側のスナップショット（AC-12-12③）",
			generated: true,
			want: port.WorkMonthOutput{
				ContractID:          testContractID,
				ContractDisplayName: testDisplayName,
				YearMonth:           "2026-07",
				State:               "Draft",
				Generated:           true,
				SettlementRange: port.SettlementRangeOutput{
					LowerBound: port.Hours{Hours: 100, Minutes: 0},
					UpperBound: port.Hours{Hours: 200, Minutes: 0},
				},
				TotalHours: port.Hours{Hours: 8, Minutes: 45},
				Excess:     nil,
				Shortfall:  nil,
				DailyRecords: []port.DailyRecordOutput{
					{
						Date:                "2026-07-01",
						WorkingHours:        port.Hours{Hours: 8, Minutes: 50},
						RoundedWorkingHours: port.Hours{Hours: 8, Minutes: 45},
					},
				},
			},
		},
		{
			name:      "未生成：Generated偽・State=Draft・稼働実績空スライス・超過／不足は値なし・精算幅は契約の現在値（AC-12-12④）",
			generated: false,
			want: port.WorkMonthOutput{
				ContractID:          testContractID,
				ContractDisplayName: testDisplayName,
				YearMonth:           "2026-07",
				State:               "Draft",
				Generated:           false,
				SettlementRange: port.SettlementRangeOutput{
					LowerBound: port.Hours{Hours: 140, Minutes: 0},
					UpperBound: port.Hours{Hours: 180, Minutes: 0},
				},
				TotalHours:   port.Hours{},
				Excess:       nil,
				Shortfall:    nil,
				DailyRecords: []port.DailyRecordOutput{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			if tt.generated {
				// 契約の現在値（140〜180）とあえて食い違わせ、上書きされないことを示す。
				f.workMonths.put(reconstructWorkMonth(
					t, f.contractID, f.yearMonth,
					mustSettlementRange(t, 100, 200),
					workmonth.StateDraft,
					[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 50)},
				))
			}

			f.get().Execute(context.Background(), port.GetWorkMonthInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
			})

			got := f.output.onlyPresented(t)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("出力 DTO が不一致 (-want +got):\n%s", diff)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("参照なのに Save が呼ばれた（回数 = %d, want 0）（AC-7-15⑥）", f.workMonths.saveCount)
			}
		})
	}
}

// TestGetWorkMonth_OtherRepositoryErrorIsNotConvertedToEmptyOutput は、
// WorkMonthRepository が ErrWorkMonthNotFound 以外のエラーを返した場合、
// そのエラーがそのまま PresentError へ渡ることを検証する（未生成の空の下書き
// 出力へ変換しない。AC-12-12⑤）。
func TestGetWorkMonth_OtherRepositoryErrorIsNotConvertedToEmptyOutput(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	driverErr := errors.New("fake: find failed unexpectedly")
	f.workMonths.findErr = driverErr

	f.get().Execute(context.Background(), port.GetWorkMonthInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, driverErr) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, driverErr)（AC-12-12⑤: ErrWorkMonthNotFound 以外は素通しする）", err)
	}
	if f.workMonths.saveCount != 0 {
		t.Errorf("参照なのに Save が呼ばれた（回数 = %d, want 0）（AC-7-15⑥）", f.workMonths.saveCount)
	}
}
