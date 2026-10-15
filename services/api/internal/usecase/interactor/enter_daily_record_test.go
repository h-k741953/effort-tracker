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
	testContractID = "ctr-0001"
	testEngineerID = "eng-0001"
	// testOtherEngineerID は当該契約の技術者ではない操作者の識別子。
	testOtherEngineerID = "eng-9999"
	testDisplayName     = "サンプル株式会社 / 基幹システム保守"
)

// errSaveFailed は永続化の失敗を注入するための番兵値。
// 番兵として定義済みのいずれにも該当しないエラー（実装設計 AC-11-11）を表す。
var errSaveFailed = errors.New("fake: save failed")

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

// mustDailyRecord は「開始 0:00・終了 = 稼働時間・休憩なし」の稼働実績を構築する
// （前提投入用。稼働時間は時刻から算出される値であり入力ではない＝daily-record-entry.md D-2）。
// 稼働時間は 15 分の倍数で 0時間15分以上 23時間45分以下でなければならない（同 AC-3 の 3-A・3-B）。
func mustDailyRecord(t *testing.T, year, month, day, hours, minutes int) workmonth.DailyRecord {
	t.Helper()
	start, err := workmonth.NewTimeOfDay(0, 0)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewTimeOfDay(0, 0): %v", err)
	}
	end, err := workmonth.NewTimeOfDay(hours, minutes)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewTimeOfDay(%d, %d): %v", hours, minutes, err)
	}
	r, err := workmonth.NewDailyRecord(mustDate(t, year, month, day), start, end, nil)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewDailyRecord(%d-%02d-%02d, 0:00〜%d:%02d): %v", year, month, day, hours, minutes, err)
	}
	return r
}

// tod は入力・出力 DTO の時刻（素の整数。実装設計 AC-7-18-a）を組み立てる。
func tod(hour, minute int) port.TimeOfDay {
	return port.TimeOfDay{Hour: hour, Minute: minute}
}

// recordOutput は mustDailyRecord が作る稼働実績（開始 0:00・終了 = 稼働時間・休憩なし）の
// 出力 DTO を組み立てる。稼働時間は時刻から算出した値で、15 分の倍数なので丸め後の寄与と等しい
// （daily-record-entry.md AC-6-2-a）。
func recordOutput(date string, hours, minutes int) port.DailyRecordOutput {
	return port.DailyRecordOutput{
		Date:                date,
		StartTime:           tod(0, 0),
		EndTime:             tod(hours, minutes),
		Break:               nil,
		WorkingHours:        port.Hours{Hours: hours, Minutes: minutes},
		RoundedWorkingHours: port.Hours{Hours: hours, Minutes: minutes},
	}
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

// ---- 操作者（port.Actor）の組み立て --------------------------------------
//
// ゲストは「未認証の操作者」として表す。ロール値 Guest は存在しない
// （実装設計 D-9・AC-8-7／HTTP 契約 AC-1-5）。

// ownerActor は当該契約の技術者本人。
func ownerActor(role port.Role) port.Actor {
	return port.Actor{ID: testEngineerID, Role: role, Authenticated: true}
}

// foreignActor は当該契約の技術者ではない認証済みの操作者。
func foreignActor(role port.Role) port.Actor {
	return port.Actor{ID: testOtherEngineerID, Role: role, Authenticated: true}
}

// guestActor は操作者ヘッダを持たないゲスト（HTTP 契約 AC-1-6）。
func guestActor() port.Actor {
	return port.Actor{Authenticated: false}
}

// reconstructWorkMonth は保存済みの勤務月を組み立てる（前提投入用）。
//
// 確定済みの超過／不足（実装設計 AC-2-5・AC-5-9）は、状態との整合
// （AC-2-5 の不変条件③・AC-5-9 の対応表 5-9-a〜5-9-c。決定9）を満たすように、
// Draft は (nil, nil)、PendingApproval・Approved は非nilの値を渡す。
// UC1・UC2 のこのファイル群のテストはいずれも超過／不足そのものの値を検証する
// 目的では使っていない（状態・稼働実績・判定順序の検証が主張であるため。
// AC-13-13）。超過／不足そのものの組み合わせの網羅は workmonth 側の
// TestReconstruct_ExcessShortfallStateConsistency が、Reconstruct の往復は
// 同 TestReconstruct_ExcessShortfall が個別に検証する。
func reconstructWorkMonth(
	t *testing.T,
	contractID workmonth.ContractID,
	yearMonth workmonth.YearMonth,
	settlement workmonth.SettlementRange,
	state workmonth.State,
	records []workmonth.DailyRecord,
) *workmonth.WorkMonth {
	t.Helper()
	var excess, shortfall *workmonth.WorkingHours
	if state != workmonth.StateDraft {
		determined := mustWorkingHours(t, 1, 0)
		excess = &determined
		shortfall = &determined
	}
	target, err := workmonth.Reconstruct(contractID, yearMonth, settlement, state, records, excess, shortfall)
	if err != nil {
		t.Fatalf("前提の構築に失敗: Reconstruct(%s): %v", state, err)
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
		// daily-record-entry.md AC-2-6-a の入力（稼働時間は 7時間45分）。
		StartTime: tod(9, 15),
		EndTime:   tod(17, 45),
		Break:     &port.Break{Start: tod(12, 30), End: tod(13, 15)},
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
		TotalHours: port.Hours{Hours: 7, Minutes: 45},
		Excess:     nil,
		Shortfall:  nil,
		DailyRecords: []port.DailyRecordOutput{
			{
				Date:                "2026-07-01",
				StartTime:           tod(9, 15),
				EndTime:             tod(17, 45),
				Break:               &port.Break{Start: tod(12, 30), End: tod(13, 15)},
				WorkingHours:        port.Hours{Hours: 7, Minutes: 45},
				RoundedWorkingHours: port.Hours{Hours: 7, Minutes: 45},
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
		StartTime:  tod(0, 0),
		EndTime:    tod(7, 30),
	})

	output := f.output.onlyPresented(t)
	wantRecords := []port.DailyRecordOutput{
		recordOutput("2026-07-01", 8, 0),
		recordOutput("2026-07-02", 7, 30),
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
		StartTime:  tod(0, 0),
		EndTime:    tod(8, 0),
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
		StartTime:  tod(0, 0),
		EndTime:    tod(7, 0),
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

// timesCase は1つの入力の時刻の組。値は daily-record-entry.md AC-3 の行の例を使う
// （行番号を各ケースの名前に付ける）。
type timesCase struct {
	start, end port.TimeOfDay
	brk        *port.Break
}

// validTimes は AC-3-11 の入力（`9:00`〜`18:00`／休憩なし。稼働時間 9時間0分）。
// 他の条件を満たす「妥当な時刻」として、日付・状態・認可の検査を単独で当てる行に使う。
var validTimes = timesCase{start: tod(9, 0), end: tod(18, 0)}

func (c timesCase) input(f *fixture, date workmonth.Date) port.EnterDailyRecordInput {
	return port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       date,
		StartTime:  c.start,
		EndTime:    c.end,
		Break:      c.brk,
	}
}

// TestEnterDailyRecord_RejectsInvalidEntry は業務バリデーションで弾かれる入力を検証する。
// AC-2-4（当該年月に属さない対象日）／AC-4-3（未来日）／
// daily-record-entry.md AC-3（時刻の値域外は ErrTimeOfDayOutOfRange、
// 時刻の組み合わせの不正は ErrInvalidTimeCombination。実装設計 AC-7-18・AC-3-14）。
// 「当日」は Clock ポートから取る（実装設計 AC-6-5。time.Now を使わない）。
func TestEnterDailyRecord_RejectsInvalidEntry(t *testing.T) {
	tests := []struct {
		name    string
		date    [3]int // 年・月・日
		times   timesCase
		wantErr error
	}{
		{name: "翌月の日は弾く（AC-2-4）", date: [3]int{2026, 8, 1}, times: validTimes, wantErr: workmonth.ErrDateOutOfMonth},
		{name: "前月の日は弾く（AC-2-4）", date: [3]int{2026, 6, 30}, times: validTimes, wantErr: workmonth.ErrDateOutOfMonth},
		{name: "当日より後の日は弾く（AC-4-3）", date: [3]int{2026, 7, 11}, times: validTimes, wantErr: workmonth.ErrFutureDate},
		{name: "3-1-a: 開始の分が 10 は値域外", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(9, 10), end: tod(18, 0)}, wantErr: workmonth.ErrTimeOfDayOutOfRange},
		{name: "3-2-a: 開始の時が負は値域外", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(-1, 0), end: tod(18, 0)}, wantErr: workmonth.ErrTimeOfDayOutOfRange},
		{name: "3-2-b: 終了 24:00 は値域外", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(9, 0), end: tod(24, 0)}, wantErr: workmonth.ErrTimeOfDayOutOfRange},
		{name: "3-5-a: 終了＝開始は組み合わせの不正", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(9, 0), end: tod(9, 0)}, wantErr: workmonth.ErrInvalidTimeCombination},
		{name: "3-5-b: 終了が開始より前は組み合わせの不正", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(18, 0), end: tod(9, 0)}, wantErr: workmonth.ErrInvalidTimeCombination},
		{name: "3-6-a: 休憩終了＝休憩開始は組み合わせの不正", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(9, 0), end: tod(18, 0), brk: &port.Break{Start: tod(12, 0), End: tod(12, 0)}}, wantErr: workmonth.ErrInvalidTimeCombination},
		{name: "3-8: 休憩開始が開始より前は組み合わせの不正", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(9, 0), end: tod(18, 0), brk: &port.Break{Start: tod(8, 45), End: tod(10, 0)}}, wantErr: workmonth.ErrInvalidTimeCombination},
		{name: "3-9: 稼働 0 になる休憩は組み合わせの不正", date: [3]int{2026, 7, 1}, times: timesCase{start: tod(9, 0), end: tod(18, 0), brk: &port.Break{Start: tod(9, 0), End: tod(18, 0)}}, wantErr: workmonth.ErrInvalidTimeCombination},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 「当日」は 2026-07-10。
			f := newFixture(t, mustDate(t, 2026, 7, 10))

			f.enter().Execute(context.Background(), tt.times.input(f, mustDate(t, tt.date[0], tt.date[1], tt.date[2])))

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

// TestEnterDailyRecord_InvalidTimeOfDayInEachOfFourTimes は、同じ不正な時刻（3-1-a の分 10）が
// 4つの時刻（開始・終了・休憩開始・休憩終了）のどこに現れても ErrTimeOfDayOutOfRange で弾かれる
// ことを検証する（実装設計 AC-12-18 ① (i)・⑤。契約 AC-4-5「いずれに現れても同じ」）。
// 片方の時刻だけを検査する実装を Red にする。他の3つは妥当な値にして、値域の違反だけを当てる。
func TestEnterDailyRecord_InvalidTimeOfDayInEachOfFourTimes(t *testing.T) {
	// 妥当な基準: 9:00〜18:00／休憩 12:00〜13:00
	base := func() (start, end, bs, be port.TimeOfDay) { return tod(9, 0), tod(18, 0), tod(12, 0), tod(13, 0) }
	positions := []struct {
		name  string
		build func(bad port.TimeOfDay) timesCase
	}{
		{"開始時刻", func(bad port.TimeOfDay) timesCase {
			_, e, bs, be := base()
			return timesCase{start: bad, end: e, brk: &port.Break{Start: bs, End: be}}
		}},
		{"終了時刻", func(bad port.TimeOfDay) timesCase {
			s, _, bs, be := base()
			return timesCase{start: s, end: bad, brk: &port.Break{Start: bs, End: be}}
		}},
		{"休憩開始時刻", func(bad port.TimeOfDay) timesCase {
			s, e, _, be := base()
			return timesCase{start: s, end: e, brk: &port.Break{Start: bad, End: be}}
		}},
		{"休憩終了時刻", func(bad port.TimeOfDay) timesCase {
			s, e, bs, _ := base()
			return timesCase{start: s, end: e, brk: &port.Break{Start: bs, End: bad}}
		}},
	}
	bads := []struct {
		name string
		tod  port.TimeOfDay
	}{
		{"分が粒度の外 (3-1-a: 9:10)", tod(9, 10)},
		{"時 24 (3-2-b: 24:00)", tod(24, 0)},
	}

	for _, pos := range positions {
		for _, bad := range bads {
			t.Run(pos.name+"に"+bad.name, func(t *testing.T) {
				f := newFixture(t, mustDate(t, 2026, 7, 10))

				f.enter().Execute(context.Background(), pos.build(bad.tod).input(f, mustDate(t, 2026, 7, 1)))

				err := f.output.onlyPresentedError(t)
				if !errors.Is(err, workmonth.ErrTimeOfDayOutOfRange) {
					t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, ErrTimeOfDayOutOfRange)", err)
				}
				if errors.Is(err, workmonth.ErrInvalidTimeCombination) {
					t.Errorf("エラー %v が ErrInvalidTimeCombination にも当たる（番兵を兼ねない＝AC-3-14）", err)
				}
				if f.workMonths.saveCount != 0 || len(f.workMonths.stored) != 0 {
					t.Errorf("弾かれた入力で保存・生成が行われた（Save %d 回・勤務月 %d 件）", f.workMonths.saveCount, len(f.workMonths.stored))
				}
			})
		}
	}
}

// TestEnterDailyRecord_TimeValidationOrder は、時刻の検証の中の順序を検証する
// （実装設計 AC-7-18。契約 AC-4-13-b〜AC-4-13-h・D-17）。
//
//	状態(5) → ① 4つの時刻すべての値域 → ② Break と DailyRecord の組み合わせ → 当該月外 → 未来日
//
// いずれの組でも Save が呼ばれず、未生成の勤務月を生成しないこと（4-13-h）まで固定する。
func TestEnterDailyRecord_TimeValidationOrder(t *testing.T) {
	// 3-5-a: 9:00〜9:00（終了＝開始）
	combination := timesCase{start: tod(9, 0), end: tod(9, 0)}
	tests := []struct {
		name      string
		times     timesCase
		date      [3]int
		state     workmonth.State
		generated bool
		actor     func() port.Actor // nil なら本人
		wantErr   error
	}{
		{
			// 4-13-b: 3-5-a に当たり、かつ break.start の minute が 3-1-a。①が②より先。
			// 休憩の時刻が値域外のとき、開始・終了の組み合わせの不正より値域外が返る。
			name: "4-13-b: 値域外(休憩開始 12:10)が組み合わせの不正(9:00〜9:00)より先",
			times: timesCase{start: tod(9, 0), end: tod(9, 0),
				brk: &port.Break{Start: tod(12, 10), End: tod(13, 0)}},
			date: [3]int{2026, 7, 1}, state: workmonth.StateDraft, generated: true,
			wantErr: workmonth.ErrTimeOfDayOutOfRange,
		},
		{
			// 4-13-c: 3-5-a に当たり、かつ当該月外。②が当該月外より先。
			name: "4-13-c: 組み合わせの不正が当該月外より先", times: combination,
			date: [3]int{2026, 8, 1}, state: workmonth.StateDraft, generated: true,
			wantErr: workmonth.ErrInvalidTimeCombination,
		},
		{
			// 4-13-d: startTime が 3-1-a に当たり、かつ未来日。①が未来日より先。
			name: "4-13-d: 値域外が未来日より先", times: timesCase{start: tod(9, 10), end: tod(18, 0)},
			date: [3]int{2026, 7, 11}, state: workmonth.StateDraft, generated: true,
			wantErr: workmonth.ErrTimeOfDayOutOfRange,
		},
		{
			// 4-13-e: 時刻は妥当で、当該月外かつ未来日。当該月外が未来日より先。
			name: "4-13-e: 当該月外が未来日より先", times: validTimes,
			date: [3]int{2026, 8, 1}, state: workmonth.StateDraft, generated: true,
			wantErr: workmonth.ErrDateOutOfMonth,
		},
		{
			// 4-13-f: 3-5-a に当たり、かつ PendingApproval。状態(5)が順6より先。
			name: "4-13-f: 状態が組み合わせの不正より先", times: combination,
			date: [3]int{2026, 7, 1}, state: workmonth.StatePendingApproval, generated: true,
			wantErr: workmonth.ErrNotEditable,
		},
		{
			// 4-13-g: startTime が 3-2-b に当たり、かつ本人でない。認可(4)が順6より先。
			name: "4-13-g: 認可が値域外より先", times: timesCase{start: tod(24, 0), end: tod(18, 0)},
			date: [3]int{2026, 7, 1}, state: workmonth.StateDraft, generated: true,
			actor:   func() port.Actor { return foreignActor(port.RoleEngineer) },
			wantErr: port.ErrNotOwner,
		},
		{
			// 4-13-h: 3-9 に当たり、かつ未生成。勤務月を生成しない。
			name: "4-13-h: 稼働 0(3-9)で未生成の勤務月は生成されない",
			times: timesCase{start: tod(9, 0), end: tod(18, 0),
				brk: &port.Break{Start: tod(9, 0), End: tod(18, 0)}},
			date: [3]int{2026, 7, 1}, generated: false,
			wantErr: workmonth.ErrInvalidTimeCombination,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 「当日」は 2026-07-10。2026-07-11 は未来日、2026-08-01 は当該月外かつ未来日。
			f := newFixture(t, mustDate(t, 2026, 7, 10))
			if tt.generated {
				f.workMonths.put(reconstructWorkMonth(
					t, f.contractID, f.yearMonth, mustSettlementRange(t, 140, 180),
					tt.state, []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
				))
			}
			in := tt.times.input(f, mustDate(t, tt.date[0], tt.date[1], tt.date[2]))
			if tt.actor != nil {
				in.Actor = tt.actor()
			}

			f.enter().Execute(context.Background(), in)

			if err := f.output.onlyPresentedError(t); !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれた入力で Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
			if !tt.generated && len(f.workMonths.stored) != 0 {
				t.Errorf("弾かれた入力で未生成の勤務月が生成された（件数 = %d, want 0）", len(f.workMonths.stored))
			}
		})
	}
}

// TestEnterDailyRecord_NoBreakAndMinimumBreak は、休憩なし（Break が nil）と
// 「最小の時刻を持つ休憩」（&Break{0:00〜0:15}）を取り違えずに保存・出力することを対にして検証する
// （実装設計 AC-7-18・AC-7-18-a。daily-record-entry.md AC-2-6-b・AC-2-6-c）。
func TestEnterDailyRecord_NoBreakAndMinimumBreak(t *testing.T) {
	tests := []struct {
		name      string
		times     timesCase
		wantBreak *port.Break
		wantHours port.Hours
		wantEnd   port.TimeOfDay
	}{
		{
			name:      "2-6-b: 休憩なしは Break が nil",
			times:     timesCase{start: tod(9, 0), end: tod(18, 0)},
			wantBreak: nil,
			wantHours: port.Hours{Hours: 9, Minutes: 0},
			wantEnd:   tod(18, 0),
		},
		{
			name:      "2-6-c: 0:00〜0:15 の休憩は nil でない",
			times:     timesCase{start: tod(0, 0), end: tod(23, 45), brk: &port.Break{Start: tod(0, 0), End: tod(0, 15)}},
			wantBreak: &port.Break{Start: tod(0, 0), End: tod(0, 15)},
			wantHours: port.Hours{Hours: 23, Minutes: 30},
			wantEnd:   tod(23, 45),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 7, 10))

			f.enter().Execute(context.Background(), tt.times.input(f, mustDate(t, 2026, 7, 1)))

			output := f.output.onlyPresented(t)
			if len(output.DailyRecords) != 1 {
				t.Fatalf("DailyRecords の件数 = %d, want 1", len(output.DailyRecords))
			}
			got := output.DailyRecords[0]
			want := port.DailyRecordOutput{
				Date:                "2026-07-01",
				StartTime:           tt.times.start,
				EndTime:             tt.wantEnd,
				Break:               tt.wantBreak,
				WorkingHours:        tt.wantHours,
				RoundedWorkingHours: tt.wantHours,
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("出力の稼働実績が不一致 (-want +got):\n%s", diff)
			}
			// 保存された集約も同じ時刻・休憩の有無を持つ（Save に渡された値の検査）。
			saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
			records := saved.DailyRecords()
			if len(records) != 1 {
				t.Fatalf("保存された稼働実績の件数 = %d, want 1", len(records))
			}
			brk, ok := records[0].Break()
			if ok != (tt.wantBreak != nil) {
				t.Errorf("保存された稼働実績の Break() の第2戻り値 = %v, want %v", ok, tt.wantBreak != nil)
			}
			if ok && tt.wantBreak != nil {
				gotBreak := port.Break{
					Start: tod(brk.Start().Hour(), brk.Start().Minute()),
					End:   tod(brk.End().Hour(), brk.End().Minute()),
				}
				if diff := cmp.Diff(*tt.wantBreak, gotBreak); diff != "" {
					t.Errorf("保存された休憩が不一致 (-want +got):\n%s", diff)
				}
			}
		})
	}
}

// TestEnterDailyRecord_ReplacesBreakWhenEditedToNoBreak は、休憩ありの日を休憩なしで再入力（編集）
// すると編集前の休憩の時刻を残さないことを検証する（daily-record-entry.md AC-2-6-d）。
func TestEnterDailyRecord_ReplacesBreakWhenEditedToNoBreak(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	first := timesCase{start: tod(9, 15), end: tod(17, 45), brk: &port.Break{Start: tod(12, 30), End: tod(13, 15)}} // 2-6-a
	date := mustDate(t, 2026, 7, 1)

	f.enter().Execute(context.Background(), first.input(f, date))
	f.output = &spyWorkMonthOutputPort{}
	f.enter().Execute(context.Background(), validTimes.input(f, date)) // 2-6-d: 9:00〜18:00／休憩なし

	output := f.output.onlyPresented(t)
	want := []port.DailyRecordOutput{{
		Date:                "2026-07-01",
		StartTime:           tod(9, 0),
		EndTime:             tod(18, 0),
		Break:               nil,
		WorkingHours:        port.Hours{Hours: 9, Minutes: 0},
		RoundedWorkingHours: port.Hours{Hours: 9, Minutes: 0},
	}}
	if diff := cmp.Diff(want, output.DailyRecords); diff != "" {
		t.Errorf("編集後の稼働実績が不一致 (-want +got):\n%s（AC-2-6-d）", diff)
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
		StartTime:  tod(0, 0),
		EndTime:    tod(8, 0),
	})

	output := f.output.onlyPresented(t)
	wantRecords := []port.DailyRecordOutput{
		recordOutput("2026-07-10", 8, 0),
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
		StartTime:  tod(0, 0),
		EndTime:    tod(6, 30),
	})

	output := f.output.onlyPresented(t)
	wantRecords := []port.DailyRecordOutput{
		recordOutput("2026-07-01", 6, 30),
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
				StartTime:  tod(0, 0),
				EndTime:    tod(7, 30),
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

// ---- 実装設計 AC-8-7（認証） ---------------------------------------------

// TestEnterDailyRecord_RejectsUnauthenticated は未認証（ゲスト）の入力を弾くことを検証する。
// 実装設計 AC-8-7（更新操作はすべて弾く）／HTTP 契約 AC-1-6（401 UNAUTHENTICATED）。
// 認証は判定順序の1番目であり、対象の実在（3）より先に判定する（HTTP 契約 AC-9）。
func TestEnterDailyRecord_RejectsUnauthenticated(t *testing.T) {
	tests := []struct {
		name       string
		actor      port.Actor
		contractID string // 空なら fixture の契約
	}{
		{
			name:  "操作者ヘッダを持たないゲストは弾く（AC-8-7）",
			actor: guestActor(),
		},
		{
			name:  "識別子が本人と一致していても未認証なら弾く（AC-8-7）",
			actor: port.Actor{ID: testEngineerID, Role: port.RoleEngineer, Authenticated: false},
		},
		{
			name:       "契約が実在しなくても認証を先に判定する（判定順序 1 → 3）",
			actor:      guestActor(),
			contractID: "ctr-unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 7, 10))
			contractID := f.contractID
			if tt.contractID != "" {
				contractID = mustContractID(t, tt.contractID)
			}

			f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
				Actor:      tt.actor,
				ContractID: contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, 2026, 7, 1),
				StartTime:  tod(0, 0),
				EndTime:    tod(8, 0),
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrUnauthenticated) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("未認証なのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
			if len(f.workMonths.stored) != 0 {
				t.Errorf("未認証なのに勤務月が生成されている（件数 = %d, want 0）", len(f.workMonths.stored))
			}
		})
	}
}

// ---- 実装設計 AC-8-1（認可） ---------------------------------------------

// TestEnterDailyRecord_Authorization は入力・編集の認可を検証する。
// 実装設計 AC-8-1（本人のみ。ロールは問わない）／HTTP 契約 AC-4-3
// （本人でなければ 403 FORBIDDEN_NOT_OWNER。本人であれば Approver でも許可）。
//
// 未生成の年月（暗黙生成の経路）でも同じ判定になることを併せて固定する。
func TestEnterDailyRecord_Authorization(t *testing.T) {
	actors := []struct {
		name    string
		actor   port.Actor
		wantErr error // nil なら許可
	}{
		{name: "本人（技術者ロール）は許可（AC-8-1）", actor: ownerActor(port.RoleEngineer)},
		{name: "本人なら承認者ロールでも許可（AC-8-1・HTTP AC-4-3）", actor: ownerActor(port.RoleApprover)},
		{name: "他の技術者は弾く（HTTP AC-4-3）", actor: foreignActor(port.RoleEngineer), wantErr: port.ErrNotOwner},
		{name: "他人が承認者ロールでも弾く（ロールは問わない。AC-8-1）", actor: foreignActor(port.RoleApprover), wantErr: port.ErrNotOwner},
	}
	states := []struct {
		name      string
		generated bool
	}{
		{name: "生成済みの勤務月", generated: true},
		{name: "未生成の年月（暗黙生成の経路）", generated: false},
	}

	for _, ac := range actors {
		for _, st := range states {
			t.Run(ac.name+"/"+st.name, func(t *testing.T) {
				f := newFixture(t, mustDate(t, 2026, 8, 15))
				if st.generated {
					f.workMonths.put(reconstructWorkMonth(
						t,
						f.contractID,
						f.yearMonth,
						mustSettlementRange(t, 140, 180),
						workmonth.StateDraft,
						[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
					))
				}

				f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
					Actor:      ac.actor,
					ContractID: f.contractID,
					YearMonth:  f.yearMonth,
					Date:       mustDate(t, 2026, 7, 2),
					StartTime:  tod(0, 0),
					EndTime:    tod(7, 30),
				})

				if ac.wantErr != nil {
					if err := f.output.onlyPresentedError(t); !errors.Is(err, ac.wantErr) {
						t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, ac.wantErr)
					}
					if f.workMonths.saveCount != 0 {
						t.Errorf("認可で弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
					}
					if st.generated {
						saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
						if len(saved.DailyRecords()) != 1 {
							t.Errorf("認可で弾かれたのに稼働実績が変化している（件数 = %d, want 1）", len(saved.DailyRecords()))
						}
						return
					}
					if len(f.workMonths.stored) != 0 {
						t.Errorf("認可で弾かれたのに勤務月が生成されている（件数 = %d, want 0）", len(f.workMonths.stored))
					}
					return
				}

				output := f.output.onlyPresented(t)
				if f.workMonths.saveCount != 1 {
					t.Errorf("Save の呼び出し回数 = %d, want 1", f.workMonths.saveCount)
				}
				want := recordOutput("2026-07-02", 7, 30)
				got := output.DailyRecords[len(output.DailyRecords)-1]
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("許可された入力が反映されていない (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// ---- 判定順序（HTTP 契約 AC-9） ------------------------------------------

// TestEnterDailyRecord_JudgementOrder は複数の条件に該当する入力で
// 「先に該当したもの」が返ることを検証する（HTTP 契約 AC-9 の判定順序表・実装設計 AC-7-7）。
//
//	4 認可 → 5 状態 → 6 業務バリデーション（時刻の値域・組み合わせ・未来日・当該月外）
//
// 状態を時刻の検証より先に判定するため、締め済の勤務月へ値域外の時刻を送っても
// 400 TIME_OF_DAY_OUT_OF_RANGE ではなく 409 WORK_MONTH_NOT_EDITABLE になる
// （HTTP 契約 AC-4-4）。時刻の検証の中の順序は TestEnterDailyRecord_TimeValidationOrder が持つ。
func TestEnterDailyRecord_JudgementOrder(t *testing.T) {
	outOfRange := timesCase{start: tod(24, 0), end: tod(18, 0)} // 3-2-b
	combination := timesCase{start: tod(9, 0), end: tod(9, 0)}  // 3-5-a
	tests := []struct {
		name    string
		actor   port.Actor
		state   workmonth.State
		date    [3]int
		times   timesCase
		wantErr error
	}{
		{
			name:  "状態(5)は時刻の値域(6)より先 — 締め済へ時 24",
			actor: ownerActor(port.RoleEngineer), state: workmonth.StatePendingApproval,
			date: [3]int{2026, 7, 2}, times: outOfRange,
			wantErr: workmonth.ErrNotEditable,
		},
		{
			name:  "状態(5)は時刻の組み合わせ(6)より先 — 締め済へ終了＝開始",
			actor: ownerActor(port.RoleEngineer), state: workmonth.StatePendingApproval,
			date: [3]int{2026, 7, 2}, times: combination,
			wantErr: workmonth.ErrNotEditable,
		},
		{
			name:  "状態(5)は未来日(6)より先 — 承認済へ未来日",
			actor: ownerActor(port.RoleEngineer), state: workmonth.StateApproved,
			date: [3]int{2026, 7, 31}, times: validTimes,
			wantErr: workmonth.ErrNotEditable,
		},
		{
			name:  "状態(5)は当該月外(6)より先 — 承認済へ翌月の日",
			actor: ownerActor(port.RoleEngineer), state: workmonth.StateApproved,
			date: [3]int{2026, 8, 1}, times: validTimes,
			wantErr: workmonth.ErrNotEditable,
		},
		{
			name:  "認可(4)は状態(5)より先 — 他人が締め済へ入力",
			actor: foreignActor(port.RoleApprover), state: workmonth.StatePendingApproval,
			date: [3]int{2026, 7, 2}, times: validTimes,
			wantErr: port.ErrNotOwner,
		},
		{
			name:  "認可(4)は時刻の値域(6)より先 — 他人が下書きへ時 24",
			actor: foreignActor(port.RoleEngineer), state: workmonth.StateDraft,
			date: [3]int{2026, 7, 2}, times: outOfRange,
			wantErr: port.ErrNotOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 「当日」は 2026-07-10。2026-07-31 は未来日にあたる。
			f := newFixture(t, mustDate(t, 2026, 7, 10))
			f.workMonths.put(reconstructWorkMonth(
				t,
				f.contractID,
				f.yearMonth,
				mustSettlementRange(t, 140, 180),
				tt.state,
				[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			))
			in := tt.times.input(f, mustDate(t, tt.date[0], tt.date[1], tt.date[2]))
			in.Actor = tt.actor

			f.enter().Execute(context.Background(), in)

			if err := f.output.onlyPresentedError(t); !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
		})
	}
}

// ---- 実装設計 AC-11-7・AC-11-11（対象の実在・想定外のエラー） -------------

// TestEnterDailyRecord_ContractNotFound は実在しない契約への入力を検証する
// （HTTP 契約 AC-4-11。404 CONTRACT_NOT_FOUND）。
func TestEnterDailyRecord_ContractNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 7, 10))

	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: mustContractID(t, "ctr-unknown"),
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
		StartTime:  tod(0, 0),
		EndTime:    tod(8, 0),
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrContractNotFound) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrContractNotFound)", err)
	}
	if f.workMonths.saveCount != 0 {
		t.Errorf("契約が実在しないのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
	}
	if len(f.workMonths.stored) != 0 {
		t.Errorf("契約が実在しないのに勤務月が生成されている（件数 = %d, want 0）", len(f.workMonths.stored))
	}
}

// TestEnterDailyRecord_SaveFailure は保存の失敗が出力ポートへ渡ることを検証する。
// 番兵として定義済みのいずれにも該当しないエラーはそのまま伝播させ、interactor で握り潰さない
// （実装設計 AC-11-11／HTTP 契約 AC-9 の INTERNAL_ERROR）。
func TestEnterDailyRecord_SaveFailure(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 7, 10))
	f.workMonths.saveErr = errSaveFailed

	f.enter().Execute(context.Background(), port.EnterDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
		StartTime:  tod(0, 0),
		EndTime:    tod(8, 0),
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, errSaveFailed) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, errSaveFailed)", err)
	}
	if f.workMonths.saveCount != 1 {
		t.Errorf("Save の呼び出し回数 = %d, want 1（保存は試みられる）", f.workMonths.saveCount)
	}
	// 以下は Fake の忠実性を守るためのアサーション（検証対象は interactor ではない）。
	// Save がエラーを返したのに Fake 側に書き込まれていると、後続のテストが
	// 「保存されたこと」を偽って観測できてしまう。
	if len(f.workMonths.stored) != 0 {
		t.Errorf("Fake が保存失敗時に勤務月を残している（件数 = %d, want 0）", len(f.workMonths.stored))
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
