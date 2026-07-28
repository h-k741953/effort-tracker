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

// 検証対象の受け入れ条件（docs/specs/daily-record-entry.md）:
//   - AC-1-1・AC-1-2・AC-1-3（初回入力時の暗黙生成と精算幅の複写）
//   - AC-1-4（既存の勤務月への追加。再生成しない）
//   - AC-2-3・AC-2-4（同一日は編集／当該年月外は弾く）
//   - AC-3（稼働時間の値域）
//   - AC-4（未来日の制限。「当日」は Clock ポートから取る）
//   - AC-5-2・AC-5-3（Draft 以外は弾く）

const (
	testContractID  = "ctr-0001"
	testEngineerID  = "eng-0001"
	testDisplayName = "サンプル株式会社 / 基幹システム保守"
)

// ---- テスト補助 ----------------------------------------------------------

func mustWorkingHours(t *testing.T, hours, minutes int) workmonth.WorkingHours {
	t.Helper()
	w, err := workmonth.NewWorkingHours(hours, minutes)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewWorkingHours(%d, %d): %v", hours, minutes, err)
	}
	return w
}

func mustDate(t *testing.T, year, month, day int) workmonth.Date {
	t.Helper()
	d, err := workmonth.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewDate(%d, %d, %d): %v", year, month, day, err)
	}
	return d
}

func mustYearMonth(t *testing.T, year, month int) workmonth.YearMonth {
	t.Helper()
	ym, err := workmonth.NewYearMonth(year, month)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewYearMonth(%d, %d): %v", year, month, err)
	}
	return ym
}

func mustContractID(t *testing.T, value string) workmonth.ContractID {
	t.Helper()
	id, err := workmonth.NewContractID(value)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewContractID(%q): %v", value, err)
	}
	return id
}

func mustSettlementRange(t *testing.T, lowerHours, upperHours int) workmonth.SettlementRange {
	t.Helper()
	s, err := workmonth.NewSettlementRange(mustWorkingHours(t, lowerHours, 0), mustWorkingHours(t, upperHours, 0))
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewSettlementRange(%d, %d): %v", lowerHours, upperHours, err)
	}
	return s
}

func mustDailyRecord(t *testing.T, year, month, day, hours, minutes int) workmonth.DailyRecord {
	t.Helper()
	r, err := workmonth.NewDailyRecord(mustDate(t, year, month, day), mustWorkingHours(t, hours, minutes))
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewDailyRecord(%d-%02d-%02d): %v", year, month, day, err)
	}
	return r
}

// fixture は UC1 のユースケースを実行するための最小の前提一式。
type fixture struct {
	workMonths *fakeWorkMonthRepository
	contracts  *fakeContractRepository
	clock      fakeClock
	output     *spyWorkMonthOutputPort
	contractID workmonth.ContractID
	yearMonth  workmonth.YearMonth
	actor      port.Actor
}

// newFixture は 2026年7月・精算幅 140〜180時間の契約と、その技術者本人の操作者を用意する。
func newFixture(t *testing.T, today workmonth.Date) *fixture {
	t.Helper()

	contractID := mustContractID(t, testContractID)
	contracts := newFakeContractRepository()
	contracts.put(port.Contract{
		ID:              contractID,
		DisplayName:     testDisplayName,
		EngineerID:      testEngineerID,
		SettlementRange: mustSettlementRange(t, 140, 180),
	})

	return &fixture{
		workMonths: newFakeWorkMonthRepository(),
		contracts:  contracts,
		clock:      fakeClock{today: today},
		output:     &spyWorkMonthOutputPort{},
		contractID: contractID,
		yearMonth:  mustYearMonth(t, 2026, 7),
		actor:      port.Actor{ID: testEngineerID, Role: port.RoleEngineer, Authenticated: true},
	}
}

func (f *fixture) enter() *interactor.EnterDailyRecord {
	return interactor.NewEnterDailyRecord(f.workMonths, f.contracts, f.clock, f.output)
}

// reconstructWorkMonth は保存済みの勤務月を組み立てる（前提投入用）。
func reconstructWorkMonth(
	t *testing.T,
	contractID workmonth.ContractID,
	yearMonth workmonth.YearMonth,
	settlement workmonth.SettlementRange,
	state workmonth.State,
	records []workmonth.DailyRecord,
) *workmonth.WorkMonth {
	t.Helper()
	target, err := workmonth.Reconstruct(contractID, yearMonth, settlement, state, records)
	if err != nil {
		t.Fatalf("前提の構築に失敗: Reconstruct: %v", err)
	}
	return target
}

// ---- AC-1-1・AC-1-2・AC-1-3 ----------------------------------------------

// TestEnterDailyRecord_GeneratesWorkMonthImplicitly は初回入力時の暗黙生成を検証する。
// AC-1-1（勤務月が無ければ新規生成して実績を追加する）
// AC-1-2（生成時に契約から精算幅を複写する）
// AC-1-3（生成された勤務月の初期状態は Draft）。
func TestEnterDailyRecord_GeneratesWorkMonthImplicitly(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
		Hours:      8,
		Minutes:    50,
	})

	if f.workMonths.saveCount != 1 {
		t.Fatalf("Save の呼び出し回数 = %d, want 1（AC-1-1）", f.workMonths.saveCount)
	}
	saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
	if saved.State() != workmonth.StateDraft {
		t.Errorf("生成された勤務月の State() = %q, want %q（AC-1-3）", saved.State(), workmonth.StateDraft)
	}
	wantSettlement := []hoursView{{Hours: 140}, {Hours: 180}}
	gotSettlement := []hoursView{
		viewOfHours(saved.SettlementRange().LowerBound()),
		viewOfHours(saved.SettlementRange().UpperBound()),
	}
	if diff := cmp.Diff(wantSettlement, gotSettlement); diff != "" {
		t.Errorf("契約から複写した精算幅が不一致 (-want +got):\n%s（AC-1-2）", diff)
	}

	want := port.WorkMonthOutput{
		ContractID:          testContractID,
		ContractDisplayName: testDisplayName,
		YearMonth:           "2026-07",
		State:               "Draft",
		Generated:           true,
		SettlementRange: port.SettlementRangeOutput{
			LowerBound: port.Hours{Hours: 140, Minutes: 0},
			UpperBound: port.Hours{Hours: 180, Minutes: 0},
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
	}
	if diff := cmp.Diff(want, f.output.onlyPresented(t)); diff != "" {
		t.Errorf("出力ポートへ渡された勤務月が不一致 (-want +got):\n%s", diff)
	}
}

// ---- AC-1-4 --------------------------------------------------------------

// TestEnterDailyRecord_UsesExistingWorkMonth は既存の勤務月へ追加することを検証する（AC-1-4）。
// 既存の精算幅（生成時の複写）を契約の現在値で上書きしないことも併せて検証する（AC-1-2）。
func TestEnterDailyRecord_UsesExistingWorkMonth(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.workMonths.put(reconstructWorkMonth(
		t,
		f.contractID,
		f.yearMonth,
		mustSettlementRange(t, 100, 200), // 生成時に複写された値。契約の現在値（140〜180）とは異なる
		workmonth.StateDraft,
		[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
	))

	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 2),
		Hours:      7,
		Minutes:    30,
	})

	output := f.output.onlyPresented(t)
	wantRecords := []port.DailyRecordOutput{
		{
			Date:                "2026-07-01",
			WorkingHours:        port.Hours{Hours: 8, Minutes: 0},
			RoundedWorkingHours: port.Hours{Hours: 8, Minutes: 0},
		},
		{
			Date:                "2026-07-02",
			WorkingHours:        port.Hours{Hours: 7, Minutes: 30},
			RoundedWorkingHours: port.Hours{Hours: 7, Minutes: 30},
		},
	}
	if diff := cmp.Diff(wantRecords, output.DailyRecords); diff != "" {
		t.Errorf("既存の勤務月へ追加されていない (-want +got):\n%s（AC-1-4）", diff)
	}
	wantSettlement := port.SettlementRangeOutput{
		LowerBound: port.Hours{Hours: 100, Minutes: 0},
		UpperBound: port.Hours{Hours: 200, Minutes: 0},
	}
	if diff := cmp.Diff(wantSettlement, output.SettlementRange); diff != "" {
		t.Errorf("既存の勤務月の精算幅が契約の現在値で上書きされている (-want +got):\n%s（AC-1-2・AC-1-4）", diff)
	}
	if diff := cmp.Diff(port.Hours{Hours: 15, Minutes: 30}, output.TotalHours); diff != "" {
		t.Errorf("TotalHours が不一致 (-want +got):\n%s", diff)
	}
}

// TestEnterDailyRecord_ContractChangeDoesNotRetroact は、勤務月の生成後に契約が変わっても
// 当該勤務月へ遡及しないことを検証する（AC-1-2）。
func TestEnterDailyRecord_ContractChangeDoesNotRetroact(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
		Hours:      8,
		Minutes:    0,
	})
	if len(f.output.errs) != 0 {
		t.Fatalf("前提の実行に失敗: PresentError = %v", f.output.errs)
	}

	// 契約の精算幅を変更する。
	f.contracts.put(port.Contract{
		ID:              f.contractID,
		DisplayName:     testDisplayName,
		EngineerID:      testEngineerID,
		SettlementRange: mustSettlementRange(t, 100, 200),
	})

	f.output = &spyWorkMonthOutputPort{}
	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 2),
		Hours:      7,
		Minutes:    0,
	})

	saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
	wantSettlement := []hoursView{{Hours: 140}, {Hours: 180}}
	gotSettlement := []hoursView{
		viewOfHours(saved.SettlementRange().LowerBound()),
		viewOfHours(saved.SettlementRange().UpperBound()),
	}
	if diff := cmp.Diff(wantSettlement, gotSettlement); diff != "" {
		t.Errorf("契約の変更が生成済みの勤務月へ遡及している (-want +got):\n%s（AC-1-2）", diff)
	}
}

// ---- AC-2-4・AC-3・AC-4 --------------------------------------------------

// TestEnterDailyRecord_RejectsInvalidEntry は業務バリデーションで弾かれる入力を検証する。
// AC-2-4（当該年月に属さない対象日）／AC-3-3・AC-3-5（稼働時間の値域）／AC-4-3（未来日）。
// 「当日」は Clock ポートから取る（実装設計 AC-6-5。time.Now を使わない）。
func TestEnterDailyRecord_RejectsInvalidEntry(t *testing.T) {
	tests := []struct {
		name    string
		date    [3]int // 年・月・日
		hours   int
		minutes int
		wantErr error
	}{
		{name: "翌月の日は弾く（AC-2-4）", date: [3]int{2026, 8, 1}, hours: 8, wantErr: workmonth.ErrDateOutOfMonth},
		{name: "前月の日は弾く（AC-2-4）", date: [3]int{2026, 6, 30}, hours: 8, wantErr: workmonth.ErrDateOutOfMonth},
		{name: "24時間1分は弾く（AC-3-3）", date: [3]int{2026, 7, 1}, hours: 24, minutes: 1, wantErr: workmonth.ErrWorkingHoursOutOfRange},
		{name: "分が60は弾く（AC-3-5）", date: [3]int{2026, 7, 1}, hours: 8, minutes: 60, wantErr: workmonth.ErrWorkingHoursOutOfRange},
		{name: "負の稼働時間は弾く（AC-3-4）", date: [3]int{2026, 7, 1}, hours: -1, wantErr: workmonth.ErrWorkingHoursOutOfRange},
		{name: "当日より後の日は弾く（AC-4-3）", date: [3]int{2026, 7, 11}, hours: 8, wantErr: workmonth.ErrFutureDate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 「当日」は 2026-07-10。
			f := newFixture(t, mustDate(t, 2026, 7, 10))

			f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, tt.date[0], tt.date[1], tt.date[2]),
				Hours:      tt.hours,
				Minutes:    tt.minutes,
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれた入力で Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
			if len(f.workMonths.stored) != 0 {
				t.Errorf("弾かれた入力で勤務月が生成されている（件数 = %d, want 0）", len(f.workMonths.stored))
			}
		})
	}
}

// TestEnterDailyRecord_AcceptsToday は「当日」の入力が許可されることを検証する（AC-4-1）。
func TestEnterDailyRecord_AcceptsToday(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 7, 10))

	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 10),
		Hours:      8,
		Minutes:    0,
	})

	output := f.output.onlyPresented(t)
	wantRecords := []port.DailyRecordOutput{
		{
			Date:                "2026-07-10",
			WorkingHours:        port.Hours{Hours: 8, Minutes: 0},
			RoundedWorkingHours: port.Hours{Hours: 8, Minutes: 0},
		},
	}
	if diff := cmp.Diff(wantRecords, output.DailyRecords); diff != "" {
		t.Errorf("当日の入力が反映されていない (-want +got):\n%s（AC-4-1）", diff)
	}
}

// ---- AC-2-3 --------------------------------------------------------------

// TestEnterDailyRecord_EditsSameDate は同一日への再入力が編集（上書き）になることを検証する（AC-2-3）。
func TestEnterDailyRecord_EditsSameDate(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.workMonths.put(reconstructWorkMonth(
		t,
		f.contractID,
		f.yearMonth,
		mustSettlementRange(t, 140, 180),
		workmonth.StateDraft,
		[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
	))

	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
		Hours:      6,
		Minutes:    30,
	})

	output := f.output.onlyPresented(t)
	wantRecords := []port.DailyRecordOutput{
		{
			Date:                "2026-07-01",
			WorkingHours:        port.Hours{Hours: 6, Minutes: 30},
			RoundedWorkingHours: port.Hours{Hours: 6, Minutes: 30},
		},
	}
	if diff := cmp.Diff(wantRecords, output.DailyRecords); diff != "" {
		t.Errorf("同一日への入力が編集として扱われていない (-want +got):\n%s（AC-2-3）", diff)
	}
}

// ---- AC-5-2・AC-5-3 ------------------------------------------------------

// TestEnterDailyRecord_RejectsNotEditableState は Draft 以外の状態で入力を弾くことを検証する
// （AC-5-2 締め済・AC-5-3 承認済）。弾かれたときに保存が行われないことも検証する。
func TestEnterDailyRecord_RejectsNotEditableState(t *testing.T) {
	tests := []struct {
		name  string
		state workmonth.State
	}{
		{name: "締め済は弾く（AC-5-2）", state: workmonth.StatePendingApproval},
		{name: "承認済は弾く（AC-5-3）", state: workmonth.StateApproved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			f.workMonths.put(reconstructWorkMonth(
				t,
				f.contractID,
				f.yearMonth,
				mustSettlementRange(t, 140, 180),
				tt.state,
				[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			))

			f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, 2026, 7, 2),
				Hours:      7,
				Minutes:    30,
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, workmonth.ErrNotEditable) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, ErrNotEditable)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
			saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
			if len(saved.DailyRecords()) != 1 {
				t.Errorf("弾かれたのに稼働実績が変化している（件数 = %d, want 1）", len(saved.DailyRecords()))
			}
		})
	}
}

// hoursView は go-cmp で稼働時間を比較するための表示用の射影。
type hoursView struct {
	Hours   int
	Minutes int
}

func viewOfHours(w workmonth.WorkingHours) hoursView {
	return hoursView{Hours: w.Hours(), Minutes: w.Minutes()}
}
