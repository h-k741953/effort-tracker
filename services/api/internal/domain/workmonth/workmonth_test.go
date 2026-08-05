package workmonth_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// 検証対象の受け入れ条件（docs/specs/daily-record-entry.md）:
//   - AC-1-3（生成された勤務月の初期状態は Draft）
//   - AC-2-2・AC-2-3（1日最大1レコード。同一日への入力は編集）
//   - AC-2-4（当該年月に属さない対象日は弾く）
//   - AC-4-1〜AC-4-3（未来日の制限。「当日」は引数で受け取る）
//   - AC-5-1〜AC-5-4（状態による可否と削除）
//   - AC-6-1〜AC-6-3（総稼働時間の算出）

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
		t.Fatalf("前提の構築に失敗: NewDailyRecord(%d-%02d-%02d, %d時間%d分): %v", year, month, day, hours, minutes, err)
	}
	return r
}

// mustNewWorkMonth は 2026年7月・精算幅 140〜180時間の下書きの勤務月を生成する。
func mustNewWorkMonth(t *testing.T) *workmonth.WorkMonth {
	t.Helper()
	w, err := workmonth.New(mustContractID(t, "ctr-0001"), mustYearMonth(t, 2026, 7), mustSettlementRange(t, 140, 180))
	if err != nil {
		t.Fatalf("前提の構築に失敗: New: %v", err)
	}
	return w
}

// mustReconstructWorkMonth は任意の状態の勤務月を復元する。
// 状態遷移メソッド（Close / Approve）は UC2・UC3 の関心事であるため、
// UC1 のテストでは永続化からの再構築（実装設計 AC-2-5）で前提を組み立てる。
//
// 確定済みの超過／不足（末尾の引数。実装設計 AC-2-5・AC-5-9）は、状態との整合
// （AC-2-5 の不変条件③・AC-5-9 の対応表 5-9-a〜5-9-c。決定9）を満たすように、
// Draft は (nil, nil)、それ以外は非nilの値を渡す。このヘルパーの呼び出し側は
// いずれも超過／不足そのものの値を検証する目的で使っていない（状態・稼働実績の
// 検証が主張であるため。AC-13-13）。超過／不足そのものの組み合わせの網羅は
// TestReconstruct_ExcessShortfallStateConsistency が、Reconstruct の往復は
// TestReconstruct_ExcessShortfall が個別に検証する。
func mustReconstructWorkMonth(t *testing.T, state workmonth.State, records []workmonth.DailyRecord) *workmonth.WorkMonth {
	t.Helper()
	var excess, shortfall *workmonth.WorkingHours
	if state != workmonth.StateDraft {
		determined := mustWorkingHours(t, 1, 0)
		excess = &determined
		shortfall = &determined
	}
	w, err := workmonth.Reconstruct(
		mustContractID(t, "ctr-0001"),
		mustYearMonth(t, 2026, 7),
		mustSettlementRange(t, 140, 180),
		state,
		records,
		excess,
		shortfall,
	)
	if err != nil {
		t.Fatalf("前提の構築に失敗: Reconstruct(%s): %v", state, err)
	}
	return w
}

// recordView は go-cmp で稼働実績の集合を比較するための表示用の射影。
type recordView struct {
	Date    string // YYYY-MM-DD
	Hours   int
	Minutes int
}

func viewOfRecords(records []workmonth.DailyRecord) []recordView {
	views := make([]recordView, 0, len(records))
	for _, r := range records {
		views = append(views, recordView{
			Date:    fmt.Sprintf("%04d-%02d-%02d", r.Date().Year(), r.Date().Month(), r.Date().Day()),
			Hours:   r.WorkingHours().Hours(),
			Minutes: r.WorkingHours().Minutes(),
		})
	}
	return views
}

// ---- AC-1-2・AC-1-3 ------------------------------------------------------

// TestNew_InitialState は新規生成した勤務月の初期状態と、複写された精算幅を検証する。
// AC-1-3（初期状態は Draft）・AC-1-2（生成時に精算幅を値オブジェクトとして保持）。
func TestNew_InitialState(t *testing.T) {
	contractID := mustContractID(t, "ctr-0001")
	yearMonth := mustYearMonth(t, 2026, 7)
	settlement := mustSettlementRange(t, 140, 180)

	got, err := workmonth.New(contractID, yearMonth, settlement)
	if err != nil {
		t.Fatalf("New が予期しないエラーを返した: %v", err)
	}

	if got.State() != workmonth.StateDraft {
		t.Errorf("生成直後の State() = %q, want %q（AC-1-3）", got.State(), workmonth.StateDraft)
	}
	if got.ContractID().String() != "ctr-0001" {
		t.Errorf("ContractID() = %q, want %q", got.ContractID().String(), "ctr-0001")
	}
	if got.YearMonth().Year() != 2026 || got.YearMonth().Month() != 7 {
		t.Errorf("YearMonth() = %d年%d月, want 2026年7月", got.YearMonth().Year(), got.YearMonth().Month())
	}
	wantSettlement := []hoursView{{Hours: 140}, {Hours: 180}}
	gotSettlement := []hoursView{
		viewOfHours(got.SettlementRange().LowerBound()),
		viewOfHours(got.SettlementRange().UpperBound()),
	}
	if diff := cmp.Diff(wantSettlement, gotSettlement); diff != "" {
		t.Errorf("SettlementRange() が不一致 (-want +got):\n%s（AC-1-2）", diff)
	}
	if diff := cmp.Diff([]recordView{}, viewOfRecords(got.DailyRecords())); diff != "" {
		t.Errorf("生成直後の DailyRecords() が不一致 (-want +got):\n%s", diff)
	}
}

// ---- AC-2-2・AC-2-3 ------------------------------------------------------

// TestWorkMonth_EnterDailyRecord_OneRecordPerDate は「1日1レコード」と
// 「同一日への入力は編集（上書き）」を検証する（AC-2-2・AC-2-3・D-1）。
func TestWorkMonth_EnterDailyRecord_OneRecordPerDate(t *testing.T) {
	today := mustDate(t, 2026, 8, 15)

	tests := []struct {
		name    string
		entries []workmonth.DailyRecord
		want    []recordView
	}{
		{
			name:    "1件の入力",
			entries: []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			want:    []recordView{{Date: "2026-07-01", Hours: 8, Minutes: 0}},
		},
		{
			name: "別の日への入力は別レコードになる",
			entries: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 7, 2, 7, 30),
			},
			want: []recordView{
				{Date: "2026-07-01", Hours: 8, Minutes: 0},
				{Date: "2026-07-02", Hours: 7, Minutes: 30},
			},
		},
		{
			name: "同一日への再入力は編集として上書きされる（AC-2-3）",
			entries: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 7, 1, 6, 30),
			},
			want: []recordView{{Date: "2026-07-01", Hours: 6, Minutes: 30}},
		},
		{
			name: "同一日へ3回入力しても1レコードのまま（AC-2-2）",
			entries: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 10, 8, 0),
				mustDailyRecord(t, 2026, 7, 10, 6, 30),
				mustDailyRecord(t, 2026, 7, 10, 0, 0),
			},
			want: []recordView{{Date: "2026-07-10", Hours: 0, Minutes: 0}},
		},
		{
			name: "入力順が降順でも対象日の昇順で返る（実装設計 AC-2-6）",
			entries: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 20, 8, 0),
				mustDailyRecord(t, 2026, 7, 3, 7, 0),
				mustDailyRecord(t, 2026, 7, 11, 6, 0),
			},
			want: []recordView{
				{Date: "2026-07-03", Hours: 7, Minutes: 0},
				{Date: "2026-07-11", Hours: 6, Minutes: 0},
				{Date: "2026-07-20", Hours: 8, Minutes: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustNewWorkMonth(t)

			for i, record := range tt.entries {
				if err := target.EnterDailyRecord(record, today); err != nil {
					t.Fatalf("EnterDailyRecord（%d件目）が予期しないエラーを返した: %v", i+1, err)
				}
			}

			if diff := cmp.Diff(tt.want, viewOfRecords(target.DailyRecords())); diff != "" {
				t.Errorf("DailyRecords() が不一致 (-want +got):\n%s", diff)
			}
		})
	}
}

// ---- AC-2-4 --------------------------------------------------------------

// TestWorkMonth_EnterDailyRecord_DateOutOfMonth は当該勤務月の年月に属さない対象日を弾くことを検証する
// （AC-2-4）。勤務月は 2026年7月、「当日」は 2026年9月15日。
func TestWorkMonth_EnterDailyRecord_DateOutOfMonth(t *testing.T) {
	today := mustDate(t, 2026, 9, 15)

	tests := []struct {
		name    string
		record  workmonth.DailyRecord
		wantErr error
	}{
		{name: "当該月の初日は許可", record: mustDailyRecord(t, 2026, 7, 1, 8, 0)},
		{name: "当該月の末日は許可", record: mustDailyRecord(t, 2026, 7, 31, 8, 0)},
		{name: "前月末日は弾く", record: mustDailyRecord(t, 2026, 6, 30, 8, 0), wantErr: workmonth.ErrDateOutOfMonth},
		{name: "翌月初日は弾く", record: mustDailyRecord(t, 2026, 8, 1, 8, 0), wantErr: workmonth.ErrDateOutOfMonth},
		{name: "同じ月でも年が違えば弾く", record: mustDailyRecord(t, 2025, 7, 15, 8, 0), wantErr: workmonth.ErrDateOutOfMonth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustNewWorkMonth(t)

			err := target.EnterDailyRecord(tt.record, today)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("EnterDailyRecord のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if diff := cmp.Diff([]recordView{}, viewOfRecords(target.DailyRecords())); diff != "" {
					t.Errorf("弾かれた入力が集約に残っている (-want +got):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnterDailyRecord が予期しないエラーを返した: %v", err)
			}
			if len(target.DailyRecords()) != 1 {
				t.Errorf("DailyRecords() の件数 = %d, want 1", len(target.DailyRecords()))
			}
		})
	}
}

// ---- AC-4 ----------------------------------------------------------------

// TestWorkMonth_EnterDailyRecord_FutureDate は未来日の制限を検証する
// （AC-4-1 当日は許可・AC-4-2 過去は許可・AC-4-3 未来は弾く）。
// 「当日」は引数として渡す（D-8 の JST 判定は Clock の実装＝driver の責務。実装設計 D-5・AC-4-7）。
func TestWorkMonth_EnterDailyRecord_FutureDate(t *testing.T) {
	today := mustDate(t, 2026, 7, 15)

	tests := []struct {
		name    string
		record  workmonth.DailyRecord
		wantErr error
	}{
		{name: "当日は許可（AC-4-1）", record: mustDailyRecord(t, 2026, 7, 15, 8, 0)},
		{name: "前日は許可（AC-4-2）", record: mustDailyRecord(t, 2026, 7, 14, 8, 0)},
		{name: "当月の初日は許可（AC-4-2）", record: mustDailyRecord(t, 2026, 7, 1, 8, 0)},
		{name: "翌日は弾く（AC-4-3）", record: mustDailyRecord(t, 2026, 7, 16, 8, 0), wantErr: workmonth.ErrFutureDate},
		{name: "当月の末日（未来）は弾く（AC-4-3）", record: mustDailyRecord(t, 2026, 7, 31, 8, 0), wantErr: workmonth.ErrFutureDate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustNewWorkMonth(t)

			err := target.EnterDailyRecord(tt.record, today)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("EnterDailyRecord のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if diff := cmp.Diff([]recordView{}, viewOfRecords(target.DailyRecords())); diff != "" {
					t.Errorf("弾かれた入力が集約に残っている (-want +got):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnterDailyRecord が予期しないエラーを返した: %v", err)
			}
			if len(target.DailyRecords()) != 1 {
				t.Errorf("DailyRecords() の件数 = %d, want 1", len(target.DailyRecords()))
			}
		})
	}
}

// ---- AC-5-1〜AC-5-3 ------------------------------------------------------

// TestWorkMonth_EnterDailyRecord_StateRestriction は状態による入力・編集の可否を検証する
// （AC-5-1 Draft は許可・AC-5-2 PendingApproval は弾く・AC-5-3 Approved は弾く。P-5）。
func TestWorkMonth_EnterDailyRecord_StateRestriction(t *testing.T) {
	today := mustDate(t, 2026, 8, 15)
	existing := []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)}

	tests := []struct {
		name    string
		state   workmonth.State
		wantErr error
	}{
		{name: "下書きは許可（AC-5-1）", state: workmonth.StateDraft},
		{name: "締め済は弾く（AC-5-2）", state: workmonth.StatePendingApproval, wantErr: workmonth.ErrNotEditable},
		{name: "承認済は弾く（AC-5-3）", state: workmonth.StateApproved, wantErr: workmonth.ErrNotEditable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, tt.state, existing)

			err := target.EnterDailyRecord(mustDailyRecord(t, 2026, 7, 2, 7, 30), today)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("EnterDailyRecord のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				want := []recordView{{Date: "2026-07-01", Hours: 8, Minutes: 0}}
				if diff := cmp.Diff(want, viewOfRecords(target.DailyRecords())); diff != "" {
					t.Errorf("弾かれたのに稼働実績が変化している (-want +got):\n%s", diff)
				}
				if target.State() != tt.state {
					t.Errorf("State() = %q, want %q（状態は変化しない）", target.State(), tt.state)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnterDailyRecord が予期しないエラーを返した: %v", err)
			}
			want := []recordView{
				{Date: "2026-07-01", Hours: 8, Minutes: 0},
				{Date: "2026-07-02", Hours: 7, Minutes: 30},
			}
			if diff := cmp.Diff(want, viewOfRecords(target.DailyRecords())); diff != "" {
				t.Errorf("DailyRecords() が不一致 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestWorkMonth_DeleteDailyRecord_StateRestriction は状態による削除の可否を検証する
// （AC-5-2・AC-5-3。P-5）。
func TestWorkMonth_DeleteDailyRecord_StateRestriction(t *testing.T) {
	existing := []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)}

	tests := []struct {
		name    string
		state   workmonth.State
		wantErr error
	}{
		{name: "下書きは許可（AC-5-1）", state: workmonth.StateDraft},
		{name: "締め済は弾く（AC-5-2）", state: workmonth.StatePendingApproval, wantErr: workmonth.ErrNotEditable},
		{name: "承認済は弾く（AC-5-3）", state: workmonth.StateApproved, wantErr: workmonth.ErrNotEditable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, tt.state, existing)

			err := target.DeleteDailyRecord(mustDate(t, 2026, 7, 1))

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("DeleteDailyRecord のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				want := []recordView{{Date: "2026-07-01", Hours: 8, Minutes: 0}}
				if diff := cmp.Diff(want, viewOfRecords(target.DailyRecords())); diff != "" {
					t.Errorf("弾かれたのに稼働実績が削除されている (-want +got):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeleteDailyRecord が予期しないエラーを返した: %v", err)
			}
			if diff := cmp.Diff([]recordView{}, viewOfRecords(target.DailyRecords())); diff != "" {
				t.Errorf("DailyRecords() が不一致 (-want +got):\n%s", diff)
			}
		})
	}
}

// ---- AC-5-5・D-9（Issue #51, 2026-07-28 人間確定） -----------------------

// TestWorkMonth_DeleteDailyRecord_DateOutOfMonth は、当該勤務月の年月に属さない
// 対象日への削除を弾くことを検証する（daily-record-entry.md AC-5-5・D-9。
// 実装設計 AC-4-2・AC-3-11）。
//
// 「当該日にレコードがあるか否かを問わず弾く」（AC-5-5）を、集約が既に保持している
// 他の日のレコードの有無を変えて検証する。対象日自体（年月外）にレコードを持たせる
// ことはできない。「すべての対象日が当該年月に属する」という不変条件（AC-2-6）により、
// 年月外の対象日は構造上レコードを持ち得ないため。
func TestWorkMonth_DeleteDailyRecord_DateOutOfMonth(t *testing.T) {
	tests := []struct {
		name       string
		existing   []workmonth.DailyRecord
		deleteDate workmonth.Date
	}{
		{
			name:       "レコードが1件も無くても前月末日の削除は弾く（AC-5-5・D-9）",
			existing:   nil,
			deleteDate: mustDate(t, 2026, 6, 30),
		},
		{
			name:       "レコードが1件も無くても翌月初日の削除は弾く（AC-5-5・D-9）",
			existing:   nil,
			deleteDate: mustDate(t, 2026, 8, 1),
		},
		{
			name: "当該年月に他のレコードがあっても翌月初日の削除は弾く（AC-5-5・D-9）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
			},
			deleteDate: mustDate(t, 2026, 8, 1),
		},
		{
			name:       "同じ月でも年が違えば弾く（AC-5-5・D-9）",
			existing:   nil,
			deleteDate: mustDate(t, 2025, 7, 15),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, workmonth.StateDraft, tt.existing)

			err := target.DeleteDailyRecord(tt.deleteDate)

			if !errors.Is(err, workmonth.ErrDateOutOfMonth) {
				t.Fatalf("DeleteDailyRecord のエラー = %v, want errors.Is(err, ErrDateOutOfMonth)（AC-5-5・D-9）", err)
			}
			if diff := cmp.Diff(viewOfRecords(tt.existing), viewOfRecords(target.DailyRecords())); diff != "" {
				t.Errorf("弾かれたのに稼働実績が変化している (-want +got):\n%s", diff)
			}
		})
	}
}

// TestWorkMonth_DeleteDailyRecord_StateCheckedBeforeDateOutOfMonth は、
// DeleteDailyRecord の検査順序が「①状態 → ②当該年月に属するか」であることを検証する
// （実装設計 AC-4-2。docs/specs/domain-api-http-contract.md AC-9 の順 5 → 順 6 と一致させる）。
//
// Draft 以外の状態で当該年月外の対象日を削除しようとしても、ErrDateOutOfMonth ではなく
// ErrNotEditable が先に返ることを固定する。
func TestWorkMonth_DeleteDailyRecord_StateCheckedBeforeDateOutOfMonth(t *testing.T) {
	existing := []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)}
	dateOutOfMonth := mustDate(t, 2026, 8, 1)

	tests := []struct {
		name    string
		state   workmonth.State
		wantErr error
	}{
		{
			name:    "下書きなら年月外の判定に進み ErrDateOutOfMonth（AC-4-2 順②）",
			state:   workmonth.StateDraft,
			wantErr: workmonth.ErrDateOutOfMonth,
		},
		{
			name:    "締め済は年月外でもまず状態で弾く（AC-4-2 順①優先）",
			state:   workmonth.StatePendingApproval,
			wantErr: workmonth.ErrNotEditable,
		},
		{
			name:    "承認済は年月外でもまず状態で弾く（AC-4-2 順①優先）",
			state:   workmonth.StateApproved,
			wantErr: workmonth.ErrNotEditable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, tt.state, existing)

			err := target.DeleteDailyRecord(dateOutOfMonth)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DeleteDailyRecord のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if tt.wantErr == workmonth.ErrNotEditable && errors.Is(err, workmonth.ErrDateOutOfMonth) {
				t.Fatalf("DeleteDailyRecord のエラー = %v が ErrDateOutOfMonth にも該当している。"+
					"状態の検査（①）が年月外の判定（②）より先であるべき（AC-4-2）", err)
			}
		})
	}
}

// ---- AC-5-4 --------------------------------------------------------------

// TestWorkMonth_DeleteDailyRecord は下書きでの削除の振る舞いを検証する。
// AC-5-4（削除した日は以降「レコードのない日＝稼働なし」として扱う。明示的なゼロ記録を残さない）。
// レコードの無い日への削除は成功として扱う（実装設計 AC-4-2・D-5）。
func TestWorkMonth_DeleteDailyRecord(t *testing.T) {
	tests := []struct {
		name           string
		existing       []workmonth.DailyRecord
		deleteYear     int
		deleteMonth    int
		deleteDay      int
		want           []recordView
		wantTotalHours hoursView
	}{
		{
			name: "レコードのある日を削除するとその日は消える（AC-5-4）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 7, 2, 7, 0),
			},
			deleteYear: 2026, deleteMonth: 7, deleteDay: 1,
			want:           []recordView{{Date: "2026-07-02", Hours: 7, Minutes: 0}},
			wantTotalHours: hoursView{Hours: 7, Minutes: 0},
		},
		{
			name: "削除してもゼロ記録は残らない（AC-5-4・D-5）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
			},
			deleteYear: 2026, deleteMonth: 7, deleteDay: 1,
			want:           []recordView{},
			wantTotalHours: hoursView{Hours: 0, Minutes: 0},
		},
		{
			name: "レコードの無い日の削除は成功し、他の日に影響しない（実装設計 AC-4-2）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 2, 7, 0),
			},
			deleteYear: 2026, deleteMonth: 7, deleteDay: 1,
			want:           []recordView{{Date: "2026-07-02", Hours: 7, Minutes: 0}},
			wantTotalHours: hoursView{Hours: 7, Minutes: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, workmonth.StateDraft, tt.existing)

			if err := target.DeleteDailyRecord(mustDate(t, tt.deleteYear, tt.deleteMonth, tt.deleteDay)); err != nil {
				t.Fatalf("DeleteDailyRecord が予期しないエラーを返した: %v", err)
			}

			if diff := cmp.Diff(tt.want, viewOfRecords(target.DailyRecords())); diff != "" {
				t.Errorf("DailyRecords() が不一致 (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantTotalHours, viewOfHours(target.TotalHours())); diff != "" {
				t.Errorf("TotalHours() が不一致 (-want +got):\n%s", diff)
			}
		})
	}
}

// ---- 実装設計 AC-2-4・AC-3-1・AC-3-2・AC-11-5 -----------------------------

// TestNew_Invariants は新規生成時の不変条件を検証する。
// 勤務月を一意にする値（契約 × 年月。実装設計 P-5）が不正なら生成させない
// （AC-3-1 契約識別子／AC-3-2 対象年月／AC-11-5 ErrInvalidValue）。
func TestNew_Invariants(t *testing.T) {
	tests := []struct {
		name       string
		contractID workmonth.ContractID
		yearMonth  workmonth.YearMonth
		wantErr    error
	}{
		{
			name:       "契約識別子と対象年月が妥当なら生成できる",
			contractID: mustContractID(t, "ctr-0001"),
			yearMonth:  mustYearMonth(t, 2026, 7),
		},
		{
			name:       "ゼロ値の契約識別子（空文字）は弾く（AC-3-1）",
			contractID: workmonth.ContractID{},
			yearMonth:  mustYearMonth(t, 2026, 7),
			wantErr:    workmonth.ErrInvalidValue,
		},
		{
			name:       "ゼロ値の対象年月（月が範囲外）は弾く（AC-3-2）",
			contractID: mustContractID(t, "ctr-0001"),
			yearMonth:  workmonth.YearMonth{},
			wantErr:    workmonth.ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.New(tt.contractID, tt.yearMonth, mustSettlementRange(t, 140, 180))

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("New のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if got != nil {
					t.Errorf("不正な値から勤務月が生成されている: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("New が予期しないエラーを返した: %v", err)
			}
			if got == nil {
				t.Fatal("New が nil を返した")
			}
		})
	}
}

// ---- 実装設計 AC-2-5・AC-2-6・AC-3-7・AC-11-5 -----------------------------

// TestReconstruct_Invariants は永続化からの再構築時の不変条件を検証する（AC-2-5）。
//
// Reconstruct は adapter/gateway が集約を組み立てる唯一の入口であり、
// ここを通り抜けた不正な値はそのまま集約の不変条件の破れになる。
// 状態遷移の検査は行わない（保存済みの事実の復元であり遷移ではない。AC-2-5）が、
// 値オブジェクトの妥当性（AC-3-1・AC-3-2・AC-3-7）と
// 「対象日で一意」（AC-2-6）は検査する。
func TestReconstruct_Invariants(t *testing.T) {
	validContractID := mustContractID(t, "ctr-0001")
	validYearMonth := mustYearMonth(t, 2026, 7)

	// 締め済・承認済の復元には確定済みの超過／不足が要る（AC-2-5 の不変条件③・
	// AC-5-9 の対応表 5-9-b・5-9-c。決定9）。本テストが検証する主張は
	// 「状態遷移を検査しない」ことであり、超過／不足そのものの値は主張に含まない
	// （AC-13-13）ため、任意の非nil値を使う。組み合わせの網羅は
	// TestReconstruct_ExcessShortfallStateConsistency が別途固定する。
	determined := mustWorkingHours(t, 1, 0)

	tests := []struct {
		name       string
		contractID workmonth.ContractID
		yearMonth  workmonth.YearMonth
		state      workmonth.State
		records    []workmonth.DailyRecord
		excess     *workmonth.WorkingHours
		shortfall  *workmonth.WorkingHours
		wantErr    error
	}{
		{
			name:       "下書きは復元できる",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
		},
		{
			name:       "締め済は復元できる（状態遷移を検査しない。AC-2-5）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StatePendingApproval,
			records:   []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			excess:    &determined,
			shortfall: &determined,
		},
		{
			name:       "承認済は復元できる（状態遷移を検査しない。AC-2-5）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateApproved,
			records:   []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			excess:    &determined,
			shortfall: &determined,
		},
		{
			name:       "実績0件でも復元できる",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: nil,
		},
		{
			name:       "定義に無い状態は弾く（AC-3-7）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.State("Closed"),
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "空の状態は弾く（AC-3-7）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.State(""),
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "大文字小文字が違う状態は弾く（AC-3-7 の英語名一致）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.State("draft"),
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "同一の対象日が2件あるものは弾く（AC-2-6）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 7, 1, 6, 30),
			},
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "重複が入力順で隣り合っていなくても弾く（AC-2-6）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 7, 20, 7, 0),
				mustDailyRecord(t, 2026, 7, 1, 6, 30),
			},
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "対象日が重複していなければ復元できる（AC-2-6）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 7, 2, 7, 0),
			},
		},
		{
			// AC-2-5: 「すべての対象日が当該年月に属する」（AC-2-1・AC-2-6）の違反は
			// ErrInvalidValue で弾く。利用者の要求に対する ErrDateOutOfMonth
			// （AC-11-2）とは番兵を使い分ける（Reconstruct は保存済みの事実を
			// 復元する操作であり、遷移でも利用者の要求でもないため）。
			name:       "翌月初日を含む行は弾く（AC-2-1・AC-2-5・AC-2-6・AC-3-11）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 8, 1, 8, 0),
			},
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "前月末日を含む行は弾く（AC-2-1・AC-2-5・AC-2-6・AC-3-11）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 6, 30, 8, 0),
			},
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			// 当該年月に属する行と属さない行が混在していても、1件でも違反があれば弾く。
			name:       "当該年月に属する行と属さない行が混在していても弾く（AC-2-1・AC-2-5・AC-2-6）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateDraft,
			records: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 8, 1, 7, 0),
			},
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "ゼロ値の契約識別子（空文字）は弾く（AC-3-1・AC-11-5）",
			contractID: workmonth.ContractID{}, yearMonth: validYearMonth, state: workmonth.StateDraft,
			wantErr: workmonth.ErrInvalidValue,
		},
		{
			name:       "ゼロ値の対象年月（月が範囲外）は弾く（AC-3-2・AC-11-5）",
			contractID: validContractID, yearMonth: workmonth.YearMonth{}, state: workmonth.StateDraft,
			wantErr: workmonth.ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.Reconstruct(
				tt.contractID,
				tt.yearMonth,
				mustSettlementRange(t, 140, 180),
				tt.state,
				tt.records,
				tt.excess,
				tt.shortfall,
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Reconstruct のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if got != nil {
					t.Errorf("不正な値から勤務月が復元されている: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Reconstruct が予期しないエラーを返した: %v", err)
			}
			if got.State() != tt.state {
				t.Errorf("State() = %q, want %q", got.State(), tt.state)
			}
			if len(got.DailyRecords()) != len(tt.records) {
				t.Errorf("DailyRecords() の件数 = %d, want %d", len(got.DailyRecords()), len(tt.records))
			}
		})
	}
}

// TestReconstruct_DateOutOfMonthUsesInvalidValueNotDateOutOfMonth は、
// 「対象日が当該年月に属さない」という同じ種類の違反であっても、
// Reconstruct では ErrDateOutOfMonth ではなく ErrInvalidValue を返すことを検証する
// （実装設計 AC-2-5・AC-3-11、AC-11-2「Reconstruct は ErrInvalidValue」）。
//
// ErrDateOutOfMonth は利用者の要求（EnterDailyRecord・DeleteDailyRecord）に対する
// 判定であり、Reconstruct は保存済みの事実を復元する操作であって利用者の要求では
// ないため、番兵を使い分ける。
func TestReconstruct_DateOutOfMonthUsesInvalidValueNotDateOutOfMonth(t *testing.T) {
	_, err := workmonth.Reconstruct(
		mustContractID(t, "ctr-0001"),
		mustYearMonth(t, 2026, 7),
		mustSettlementRange(t, 140, 180),
		workmonth.StateDraft,
		[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 8, 1, 8, 0)},
		nil,
		nil,
	)

	if !errors.Is(err, workmonth.ErrInvalidValue) {
		t.Fatalf("Reconstruct のエラー = %v, want errors.Is(err, ErrInvalidValue)（AC-2-5・AC-11-2）", err)
	}
	if errors.Is(err, workmonth.ErrDateOutOfMonth) {
		t.Fatalf("Reconstruct のエラー = %v が ErrDateOutOfMonth に該当している。"+
			"Reconstruct は利用者の要求ではないため ErrDateOutOfMonth を使わない（AC-2-5・AC-11-2）", err)
	}
}

// TestReconstruct_OrdersAndCopiesRecords は復元時に対象日の昇順へ整列すること（AC-2-6）と、
// 呼び出し側が渡したスライスを書き換えないこと（AC-2-3。不変条件を集約の内側に閉じる）を検証する。
func TestReconstruct_OrdersAndCopiesRecords(t *testing.T) {
	given := []workmonth.DailyRecord{
		mustDailyRecord(t, 2026, 7, 20, 8, 0),
		mustDailyRecord(t, 2026, 7, 3, 7, 0),
		mustDailyRecord(t, 2026, 7, 11, 6, 0),
	}
	before := viewOfRecords(given)

	target, err := workmonth.Reconstruct(
		mustContractID(t, "ctr-0001"),
		mustYearMonth(t, 2026, 7),
		mustSettlementRange(t, 140, 180),
		workmonth.StateDraft,
		given,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Reconstruct が予期しないエラーを返した: %v", err)
	}

	want := []recordView{
		{Date: "2026-07-03", Hours: 7, Minutes: 0},
		{Date: "2026-07-11", Hours: 6, Minutes: 0},
		{Date: "2026-07-20", Hours: 8, Minutes: 0},
	}
	if diff := cmp.Diff(want, viewOfRecords(target.DailyRecords())); diff != "" {
		t.Errorf("復元後の DailyRecords() が対象日の昇順になっていない (-want +got):\n%s（AC-2-6）", diff)
	}
	if diff := cmp.Diff(before, viewOfRecords(given)); diff != "" {
		t.Errorf("Reconstruct が呼び出し側のスライスを書き換えている (-want +got):\n%s（AC-2-3）", diff)
	}
}

// TestWorkMonth_DailyRecords_ReturnsCopy は DailyRecords() が内部のスライスを
// そのまま渡さないこと（AC-2-3。不変条件を集約の内側に閉じる）を検証する。
// TestReconstruct_OrdersAndCopiesRecords は「入力」スライスの複製を固定したもので、
// ここで固定するのは「出力」スライスの複製という別の主張である。
func TestWorkMonth_DailyRecords_ReturnsCopy(t *testing.T) {
	target := mustReconstructWorkMonth(t, workmonth.StateDraft, []workmonth.DailyRecord{
		mustDailyRecord(t, 2026, 7, 3, 7, 0),
		mustDailyRecord(t, 2026, 7, 11, 6, 0),
	})

	got := target.DailyRecords()
	// 集約の外から返却スライスの要素を差し替える。
	got[0] = mustDailyRecord(t, 2026, 7, 3, 1, 0)

	want := []recordView{
		{Date: "2026-07-03", Hours: 7, Minutes: 0},
		{Date: "2026-07-11", Hours: 6, Minutes: 0},
	}
	if diff := cmp.Diff(want, viewOfRecords(target.DailyRecords())); diff != "" {
		t.Errorf("返却スライスへの書き換えが集約の状態に及んでいる (-want +got):\n%s（AC-2-3）", diff)
	}
	if diff := cmp.Diff(hoursView{Hours: 13, Minutes: 0}, viewOfHours(target.TotalHours())); diff != "" {
		t.Errorf("返却スライスへの書き換えが総稼働時間に及んでいる (-want +got):\n%s（AC-2-3）", diff)
	}
}

// ---- AC-6 ----------------------------------------------------------------

// TestWorkMonth_TotalHours は総稼働時間の算出を検証する。
// AC-6-1（各日を15分単位で切り捨ててから合計する。合計してから丸めない）
// AC-6-2（8時間50分の日の寄与は8時間45分）
// AC-6-3（レコードのない日は項に現れない）。
func TestWorkMonth_TotalHours(t *testing.T) {
	tests := []struct {
		name     string
		existing []workmonth.DailyRecord
		want     hoursView
	}{
		{
			name:     "レコードが1件（AC-6-2）",
			existing: []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 50)},
			want:     hoursView{Hours: 8, Minutes: 45},
		},
		{
			name: "各日を切り捨ててから合計する（AC-6-1）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 50),
				mustDailyRecord(t, 2026, 7, 2, 7, 20),
			},
			want: hoursView{Hours: 16, Minutes: 0},
		},
		{
			name: "合計してから丸めた値にはならない（AC-6-1）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 10),
				mustDailyRecord(t, 2026, 7, 2, 8, 10),
			},
			// 各日切り捨て: 8:00 + 8:00 = 16:00。合計してから丸めると 16:15 になり不一致。
			want: hoursView{Hours: 16, Minutes: 0},
		},
		{
			name: "レコードのある日だけを合計する（AC-6-3）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
				mustDailyRecord(t, 2026, 7, 20, 8, 0),
			},
			want: hoursView{Hours: 16, Minutes: 0},
		},
		{
			name: "稼働ゼロの日も項として0を加えるだけ（AC-6-3・D-5）",
			existing: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 0, 0),
				mustDailyRecord(t, 2026, 7, 2, 8, 0),
			},
			want: hoursView{Hours: 8, Minutes: 0},
		},
		{
			name:     "レコードが無ければ0時間0分（AC-6-3）",
			existing: nil,
			want:     hoursView{Hours: 0, Minutes: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, workmonth.StateDraft, tt.existing)

			if diff := cmp.Diff(tt.want, viewOfHours(target.TotalHours())); diff != "" {
				t.Errorf("TotalHours() が不一致 (-want +got):\n%s", diff)
			}
		})
	}
}

// ---- UC2: 月次締め --------------------------------------------------------
//
// 検証対象の受け入れ条件:
//   - docs/specs/monthly-closing.md AC-1（Draft のみ締められる）
//   - 同 AC-3（超過／不足は締め時に確定し、以後再計算しない）
//   - 同 AC-4（境界を含む算出ロジック）
//   - 同 AC-5（PendingApproval へ直接遷移。中間状態を経ない）
//   - 同 AC-7-1・AC-7-2（一部未入力・空月も締められる）
//   - docs/specs/workmonth-implementation-design.md AC-4-3（Close() のシグネチャと番兵）
//   - 同 AC-5-2・AC-5-6・AC-5-7・AC-5-9（超過／不足のアクセサと Reconstruct の往復）

// recordsSummingTo は合計が totalMinutes 分になるよう、2026年7月の連続した日へ
// 分割した稼働実績を組み立てる。1日あたり20時間（1200分）以下に収め、
// DailyRecord の1日の上限（24時間）に抵触しないようにする。
// totalMinutes は15分単位でなければならない（呼び出し側が保証する）。
func recordsSummingTo(t *testing.T, totalMinutes int) []workmonth.DailyRecord {
	t.Helper()
	if totalMinutes%15 != 0 {
		t.Fatalf("前提の構築に失敗: %d分は15分単位ではない", totalMinutes)
	}

	const maxPerDayMinutes = 20 * 60

	var records []workmonth.DailyRecord
	day := 1
	remaining := totalMinutes
	for remaining > 0 {
		minutes := remaining
		if minutes > maxPerDayMinutes {
			minutes = maxPerDayMinutes
		}
		records = append(records, mustDailyRecord(t, 2026, 7, day, minutes/60, minutes%60))
		remaining -= minutes
		day++
	}
	return records
}

// determinedHoursView は go-cmp で「確定済みか」を含めて超過／不足を比較するための
// 表示用の射影（実装設計 AC-5-7・AC-12-4）。
type determinedHoursView struct {
	Determined bool
	Hours      int
	Minutes    int
}

func viewOfDetermined(w workmonth.WorkingHours, ok bool) determinedHoursView {
	// !ok でも w を捨てない。未確定のとき第1戻り値がゼロ値であること自体を
	// 検証対象にするため（AC-5-2・AC-5-7。レビュー往復2 W-A）。
	return determinedHoursView{Determined: ok, Hours: w.Hours(), Minutes: w.Minutes()}
}

// TestWorkMonth_ExcessShortfall_UndeterminedInDraft は Draft の間、超過／不足が
// 未確定であることを検証する（実装設計 AC-5-2）。0 と未確定を混同しない。
func TestWorkMonth_ExcessShortfall_UndeterminedInDraft(t *testing.T) {
	target := mustNewWorkMonth(t)

	want := determinedHoursView{Determined: false}

	gotExcess, gotExcessOK := target.Excess()
	if diff := cmp.Diff(want, viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
		t.Errorf("Excess() が不一致 (-want +got):\n%s", diff)
	}

	gotShortfall, gotShortfallOK := target.Shortfall()
	if diff := cmp.Diff(want, viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
		t.Errorf("Shortfall() が不一致 (-want +got):\n%s", diff)
	}
}

// TestWorkMonth_Close_StateTransition は Close の状態遷移を検証する。
// Draft から PendingApproval へ直接遷移し、中間状態を経ない（monthly-closing.md AC-5-1）。
func TestWorkMonth_Close_StateTransition(t *testing.T) {
	target := mustReconstructWorkMonth(t, workmonth.StateDraft, recordsSummingTo(t, 160*60))

	if err := target.Close(); err != nil {
		t.Fatalf("Close() が失敗: %v", err)
	}

	if got := target.State(); got != workmonth.StatePendingApproval {
		t.Errorf("State() = %q, want %q", got, workmonth.StatePendingApproval)
	}
}

// TestWorkMonth_Close_RejectsNonDraftState は Draft 以外からの締めを弾くことを検証する
// （monthly-closing.md AC-1-2・AC-1-3。実装設計 AC-4-3 の ErrNotClosable）。
func TestWorkMonth_Close_RejectsNonDraftState(t *testing.T) {
	tests := []struct {
		name  string
		state workmonth.State
	}{
		{name: "PendingApproval からの締めは弾く（二重締め）", state: workmonth.StatePendingApproval},
		{name: "Approved からの締めは弾く（終端状態）", state: workmonth.StateApproved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, tt.state, nil)

			err := target.Close()
			if !errors.Is(err, workmonth.ErrNotClosable) {
				t.Errorf("Close() error = %v, want errors.Is ErrNotClosable", err)
			}
			if got := target.State(); got != tt.state {
				t.Errorf("State() = %q, want unchanged %q", got, tt.state)
			}
		})
	}
}

// TestWorkMonth_Close_ExcessAndShortfall は超過／不足の算出ロジックと境界を検証する
// （monthly-closing.md AC-4-1〜AC-4-6。精算幅は下限140時間0分／上限180時間0分の
// 具体例で固定されている。両端は「含む」）。
func TestWorkMonth_Close_ExcessAndShortfall(t *testing.T) {
	tests := []struct {
		name          string
		totalMinutes  int
		wantExcess    determinedHoursView
		wantShortfall determinedHoursView
	}{
		{
			name:          "180時間15分は上限超過（AC-4-1）",
			totalMinutes:  180*60 + 15,
			wantExcess:    determinedHoursView{Determined: true, Hours: 0, Minutes: 15},
			wantShortfall: determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
		},
		{
			name:          "180時間0分は上限ちょうどで範囲内（AC-4-2）",
			totalMinutes:  180 * 60,
			wantExcess:    determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
			wantShortfall: determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
		},
		{
			name:          "160時間0分は範囲内・中間（AC-4-3）",
			totalMinutes:  160 * 60,
			wantExcess:    determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
			wantShortfall: determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
		},
		{
			name:          "140時間0分は下限ちょうどで範囲内（AC-4-4）",
			totalMinutes:  140 * 60,
			wantExcess:    determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
			wantShortfall: determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
		},
		{
			name:          "139時間45分は下限未達（AC-4-5）",
			totalMinutes:  139*60 + 45,
			wantExcess:    determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
			wantShortfall: determinedHoursView{Determined: true, Hours: 0, Minutes: 15},
		},
		{
			name:          "0時間0分（空月）は不足が下限そのものに一致する（AC-4-6・AC-7-2）",
			totalMinutes:  0,
			wantExcess:    determinedHoursView{Determined: true, Hours: 0, Minutes: 0},
			wantShortfall: determinedHoursView{Determined: true, Hours: 140, Minutes: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, workmonth.StateDraft, recordsSummingTo(t, tt.totalMinutes))

			if err := target.Close(); err != nil {
				t.Fatalf("Close() が失敗: %v", err)
			}

			gotExcess, gotExcessOK := target.Excess()
			if diff := cmp.Diff(tt.wantExcess, viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
				t.Errorf("Excess() が不一致 (-want +got):\n%s", diff)
			}

			gotShortfall, gotShortfallOK := target.Shortfall()
			if diff := cmp.Diff(tt.wantShortfall, viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
				t.Errorf("Shortfall() が不一致 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestWorkMonth_Close_PartialMonth は一部の日が未入力の勤務月・稼働実績が
// 1件も無い勤務月（空月）も締められることを検証する
// （monthly-closing.md AC-7-1・AC-7-2。「全日入力済み」を締め条件にしない）。
func TestWorkMonth_Close_PartialMonth(t *testing.T) {
	tests := []struct {
		name    string
		records []workmonth.DailyRecord
	}{
		{
			name: "一部の日が未入力でも締められる（AC-7-1）",
			records: []workmonth.DailyRecord{
				mustDailyRecord(t, 2026, 7, 1, 8, 0),
			},
		},
		{
			name:    "稼働実績が1件も無い空月も締められる（AC-7-2）",
			records: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustReconstructWorkMonth(t, workmonth.StateDraft, tt.records)

			if err := target.Close(); err != nil {
				t.Fatalf("Close() が失敗: %v", err)
			}
			if got := target.State(); got != workmonth.StatePendingApproval {
				t.Errorf("State() = %q, want %q", got, workmonth.StatePendingApproval)
			}
		})
	}
}

// TestWorkMonth_Close_DoesNotRecomputeOnRepeatedAttempt は二重締めの試行が
// 確定済みの超過／不足を変えないことを検証する
// （monthly-closing.md AC-1-2・AC-3-3。実装設計 AC-5-4）。
//
// 1つ目のサブテストは実際に Close() した後の再試行を検証するが、総稼働・精算幅を
// 変えずに2回目を試みるため「再算出しても同じ値になる」。この前提だけでは、
// 超過／不足の算出・代入を状態番兵より前へ移す変異があっても構造上失敗しえず、
// 偽 Green を許した（レビュー往復1, C-1, Issue #51）。2つ目のサブテストは
// 「保存済みの値」と「再算出したら出る値」が食い違う前提を追加し、同じ主張
// （AC-3-3）を実際に検出できる形へ強化する。
func TestWorkMonth_Close_DoesNotRecomputeOnRepeatedAttempt(t *testing.T) {
	t.Run("実際に締めた後の再試行では値が変わらない", func(t *testing.T) {
		target := mustReconstructWorkMonth(t, workmonth.StateDraft, recordsSummingTo(t, 139*60+45))
		if err := target.Close(); err != nil {
			t.Fatalf("前提の構築に失敗: 1回目の Close(): %v", err)
		}

		wantExcess, wantExcessOK := target.Excess()
		wantShortfall, wantShortfallOK := target.Shortfall()

		if err := target.Close(); !errors.Is(err, workmonth.ErrNotClosable) {
			t.Fatalf("2回目の Close() error = %v, want errors.Is ErrNotClosable", err)
		}

		gotExcess, gotExcessOK := target.Excess()
		gotShortfall, gotShortfallOK := target.Shortfall()

		if diff := cmp.Diff(viewOfDetermined(wantExcess, wantExcessOK), viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
			t.Errorf("2回目の Close() 試行で Excess() が変化した (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(viewOfDetermined(wantShortfall, wantShortfallOK), viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
			t.Errorf("2回目の Close() 試行で Shortfall() が変化した (-want +got):\n%s", diff)
		}
	})

	t.Run("保存済みの値が再算出したら出る値と食い違っていても変わらない（C-1）", func(t *testing.T) {
		// 総稼働（160時間）は精算幅（140〜180時間）の内側 → 再算出すれば超過・不足とも0。
		// しかし復元時に渡した超過1時間0分（食い違う値）をそのまま保持することを固定する。
		records := recordsSummingTo(t, 160*60)
		storedExcess := mustWorkingHours(t, 1, 0)
		storedShortfall := mustWorkingHours(t, 0, 0)

		target, err := workmonth.Reconstruct(
			mustContractID(t, "ctr-0001"),
			mustYearMonth(t, 2026, 7),
			mustSettlementRange(t, 140, 180),
			workmonth.StatePendingApproval,
			records,
			&storedExcess,
			&storedShortfall,
		)
		if err != nil {
			t.Fatalf("前提の構築に失敗: Reconstruct: %v", err)
		}

		if err := target.Close(); !errors.Is(err, workmonth.ErrNotClosable) {
			t.Fatalf("Close() error = %v, want errors.Is ErrNotClosable", err)
		}

		gotExcess, gotExcessOK := target.Excess()
		wantExcess := determinedHoursView{Determined: true, Hours: 1, Minutes: 0}
		if diff := cmp.Diff(wantExcess, viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
			t.Errorf("弾かれた Close() 試行で Excess() が保存済みの値から変化した (-want +got):\n%s", diff)
		}

		gotShortfall, gotShortfallOK := target.Shortfall()
		wantShortfall := determinedHoursView{Determined: true, Hours: 0, Minutes: 0}
		if diff := cmp.Diff(wantShortfall, viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
			t.Errorf("弾かれた Close() 試行で Shortfall() が保存済みの値から変化した (-want +got):\n%s", diff)
		}
	})
}

// ---- UC3（承認・差戻し）実装設計 AC-4-4・AC-4-5・AC-5-10・AC-12-8 -----------
//
// 集約のテストは Approve() / Reject() を3状態（Draft / PendingApproval /
// Approved。AC-3-7）× 2メソッドで網羅する。弾かれた場合は状態も確定済みの
// 超過／不足も動かないことを、State() とアクセサ（AC-5-7）の双方で確認する
// （Close() の同種のテストと同じ形。AC-12-8）。

// mustReconstructApproveFixture は TestWorkMonth_Approve_StateTransition の前提を
// 組み立てる。PendingApproval には超過・不足に異なる値（超過=2時間30分・不足=0分。
// 一方は必ず0という Close() 到達可能な組。`monthly-closing.md` AC-3-4）を与える。
// 共有ヘルパー mustReconstructWorkMonth は超過・不足に同一の値（1時間0分・1時間0分）を
// 使っており、Approve() 内で両者を入れ替える変異を domain パッケージ単体では
// 検出できない（レビュー往復2 W-A）。共有ヘルパーを変えると UC1・UC2 に波及するため、
// この UC3 のテストが使う前提だけをローカルに作る（interactor 側の
// putPendingApproval と同じ方針。approve_work_month_test.go）。
// Draft・Approved は Approve() が成功しない（値を触らない）ため、従来どおり
// mustReconstructWorkMonth を使う。
func mustReconstructApproveFixture(t *testing.T, state workmonth.State, records []workmonth.DailyRecord) *workmonth.WorkMonth {
	t.Helper()
	if state != workmonth.StatePendingApproval {
		return mustReconstructWorkMonth(t, state, records)
	}
	excess := mustWorkingHours(t, 2, 30)
	shortfall := mustWorkingHours(t, 0, 0)
	w, err := workmonth.Reconstruct(
		mustContractID(t, "ctr-0001"),
		mustYearMonth(t, 2026, 7),
		mustSettlementRange(t, 140, 180),
		state,
		records,
		&excess,
		&shortfall,
	)
	if err != nil {
		t.Fatalf("前提の構築に失敗: Reconstruct(%s): %v", state, err)
	}
	return w
}

// TestWorkMonth_Approve_StateTransition は Approve() の状態遷移と、成立した場合に
// 確定済みの超過・不足・稼働実績が締め時のまま保たれることを検証する
// （approval.md AC-1・AC-5-1・AC-5-2。実装設計 AC-4-4・AC-11-6 の ErrNotApprovable）。
func TestWorkMonth_Approve_StateTransition(t *testing.T) {
	tests := []struct {
		name    string
		state   workmonth.State
		wantErr error // nil なら成功
	}{
		{name: "PendingApproval からは許可（AC-1-1）", state: workmonth.StatePendingApproval},
		{name: "Draft からは弾く（まだ締められていない。AC-1-2）", state: workmonth.StateDraft, wantErr: workmonth.ErrNotApprovable},
		{name: "Approved からは弾く（二重承認・終端状態。AC-1-3）", state: workmonth.StateApproved, wantErr: workmonth.ErrNotApprovable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := recordsSummingTo(t, 160*60)
			target := mustReconstructApproveFixture(t, tt.state, records)
			wantExcess, wantExcessOK := target.Excess()
			wantShortfall, wantShortfallOK := target.Shortfall()

			err := target.Approve()

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Approve() error = %v, want errors.Is %v", err, tt.wantErr)
				}
				if got := target.State(); got != tt.state {
					t.Errorf("弾かれたのに State() が変化した = %q, want unchanged %q", got, tt.state)
				}
				gotExcess, gotExcessOK := target.Excess()
				gotShortfall, gotShortfallOK := target.Shortfall()
				if diff := cmp.Diff(viewOfDetermined(wantExcess, wantExcessOK), viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
					t.Errorf("弾かれたのに Excess() が変化した (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(viewOfDetermined(wantShortfall, wantShortfallOK), viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
					t.Errorf("弾かれたのに Shortfall() が変化した (-want +got):\n%s", diff)
				}
				return
			}

			if err != nil {
				t.Fatalf("Approve() が失敗: %v", err)
			}
			if got := target.State(); got != workmonth.StateApproved {
				t.Errorf("State() = %q, want %q", got, workmonth.StateApproved)
			}
			// 承認では確定値が締め時のまま（再計算・変更しない。AC-4-4・AC-5-4）。
			gotExcess, gotExcessOK := target.Excess()
			gotShortfall, gotShortfallOK := target.Shortfall()
			if diff := cmp.Diff(viewOfDetermined(wantExcess, wantExcessOK), viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
				t.Errorf("承認で Excess() が変化した (-want +got):\n%s（AC-4-4・AC-5-4）", diff)
			}
			if diff := cmp.Diff(viewOfDetermined(wantShortfall, wantShortfallOK), viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
				t.Errorf("承認で Shortfall() が変化した (-want +got):\n%s（AC-4-4・AC-5-4）", diff)
			}
			// 承認では稼働実績も締め時のまま（再計算・変更しない。AC-4-4・実装設計 AC-4-4）。
			if diff := cmp.Diff(viewOfRecords(records), viewOfRecords(target.DailyRecords())); diff != "" {
				t.Errorf("承認で DailyRecords() が変化した (-want +got):\n%s（AC-4-4）", diff)
			}
		})
	}
}

// TestWorkMonth_Reject_StateTransition は Reject() の状態遷移と、成立した場合に
// 確定済みの超過／不足が未確定へ戻ることを検証する（approval.md AC-2・AC-6-1・
// AC-6-3。実装設計 AC-4-5・AC-5-10・AC-11-6 の ErrNotRejectable）。
func TestWorkMonth_Reject_StateTransition(t *testing.T) {
	tests := []struct {
		name    string
		state   workmonth.State
		wantErr error // nil なら成功
	}{
		{name: "PendingApproval からは許可（AC-2-1）", state: workmonth.StatePendingApproval},
		{name: "Draft からは弾く（差戻す対象が無い。AC-2-2）", state: workmonth.StateDraft, wantErr: workmonth.ErrNotRejectable},
		{name: "Approved からは弾く（終端状態。AC-2-3）", state: workmonth.StateApproved, wantErr: workmonth.ErrNotRejectable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := recordsSummingTo(t, 160*60)
			target := mustReconstructWorkMonth(t, tt.state, records)
			wantExcess, wantExcessOK := target.Excess()
			wantShortfall, wantShortfallOK := target.Shortfall()

			err := target.Reject()

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Reject() error = %v, want errors.Is %v", err, tt.wantErr)
				}
				if got := target.State(); got != tt.state {
					t.Errorf("弾かれたのに State() が変化した = %q, want unchanged %q", got, tt.state)
				}
				gotExcess, gotExcessOK := target.Excess()
				gotShortfall, gotShortfallOK := target.Shortfall()
				if diff := cmp.Diff(viewOfDetermined(wantExcess, wantExcessOK), viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
					t.Errorf("弾かれたのに Excess() が変化した (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(viewOfDetermined(wantShortfall, wantShortfallOK), viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
					t.Errorf("弾かれたのに Shortfall() が変化した (-want +got):\n%s", diff)
				}
				return
			}

			if err != nil {
				t.Fatalf("Reject() が失敗: %v", err)
			}
			if got := target.State(); got != workmonth.StateDraft {
				t.Errorf("State() = %q, want %q", got, workmonth.StateDraft)
			}
			// 差戻しでは両アクセサの第2戻り値が false（未確定）になる（AC-5-10）。
			want := determinedHoursView{Determined: false}
			gotExcess, gotExcessOK := target.Excess()
			if diff := cmp.Diff(want, viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
				t.Errorf("差戻し後の Excess() が未確定になっていない (-want +got):\n%s（AC-5-10）", diff)
			}
			gotShortfall, gotShortfallOK := target.Shortfall()
			if diff := cmp.Diff(want, viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
				t.Errorf("差戻し後の Shortfall() が未確定になっていない (-want +got):\n%s（AC-5-10）", diff)
			}
			// 稼働実績は取り除かない。Draft へ戻って再び編集の対象になるだけ
			// （実装設計 AC-4-5、approval.md AC-6-2・AC-6-3）。
			if diff := cmp.Diff(viewOfRecords(records), viewOfRecords(target.DailyRecords())); diff != "" {
				t.Errorf("差戻しで稼働実績が変化した (-want +got):\n%s（AC-4-5）", diff)
			}
		})
	}
}

// TestReconstruct_ExcessShortfall は永続化からの往復における確定済みの
// 超過／不足の復元を検証する（実装設計 AC-5-9）。
func TestReconstruct_ExcessShortfall(t *testing.T) {
	t.Run("Draft へ nil を渡すと未確定として復元される", func(t *testing.T) {
		w, err := workmonth.Reconstruct(
			mustContractID(t, "ctr-0001"),
			mustYearMonth(t, 2026, 7),
			mustSettlementRange(t, 140, 180),
			workmonth.StateDraft,
			nil,
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("Reconstruct が失敗: %v", err)
		}
		want := determinedHoursView{Determined: false}

		gotExcess, gotExcessOK := w.Excess()
		if diff := cmp.Diff(want, viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
			t.Errorf("Excess() が不一致 (-want +got):\n%s", diff)
		}

		gotShortfall, gotShortfallOK := w.Shortfall()
		if diff := cmp.Diff(want, viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
			t.Errorf("Shortfall() が不一致 (-want +got):\n%s", diff)
		}
	})

	t.Run("締め済は渡した値をそのまま返し、再計算しない", func(t *testing.T) {
		// 精算幅（100〜200時間）と総稼働時間（190時間）の組み合わせを
		// 現在の SettlementRange で計算し直すと範囲内（超過・不足とも0）に
		// なるはずだが、復元時に渡した超過15分・不足0分をそのまま返すことを
		// 検証する（実装設計 AC-5-8 の「再構築時は再計算しない」側の裏付け）。
		excess := mustWorkingHours(t, 0, 15)
		shortfall := mustWorkingHours(t, 0, 0)
		w, err := workmonth.Reconstruct(
			mustContractID(t, "ctr-0001"),
			mustYearMonth(t, 2026, 7),
			mustSettlementRange(t, 100, 200),
			workmonth.StatePendingApproval,
			recordsSummingTo(t, 190*60),
			&excess,
			&shortfall,
		)
		if err != nil {
			t.Fatalf("Reconstruct が失敗: %v", err)
		}

		gotExcess, gotExcessOK := w.Excess()
		if diff := cmp.Diff(determinedHoursView{Determined: true, Hours: 0, Minutes: 15}, viewOfDetermined(gotExcess, gotExcessOK)); diff != "" {
			t.Errorf("Excess() が不一致 (-want +got):\n%s", diff)
		}

		gotShortfall, gotShortfallOK := w.Shortfall()
		if diff := cmp.Diff(determinedHoursView{Determined: true, Hours: 0, Minutes: 0}, viewOfDetermined(gotShortfall, gotShortfallOK)); diff != "" {
			t.Errorf("Shortfall() が不一致 (-want +got):\n%s", diff)
		}
	})

	t.Run("渡したポインタの参照先を外から書き換えても集約は影響を受けない", func(t *testing.T) {
		excess := mustWorkingHours(t, 1, 0)
		shortfall := mustWorkingHours(t, 0, 0)
		w, err := workmonth.Reconstruct(
			mustContractID(t, "ctr-0001"),
			mustYearMonth(t, 2026, 7),
			mustSettlementRange(t, 140, 180),
			workmonth.StatePendingApproval,
			nil,
			&excess,
			&shortfall,
		)
		if err != nil {
			t.Fatalf("Reconstruct が失敗: %v", err)
		}

		excess = mustWorkingHours(t, 99, 0) // 呼び出し側の変数を書き換える

		gotExcess, ok := w.Excess()
		if !ok {
			t.Fatalf("Excess() ok = false, want true")
		}
		if diff := cmp.Diff(hoursView{Hours: 1, Minutes: 0}, viewOfHours(gotExcess)); diff != "" {
			t.Errorf("呼び出し側の変数の書き換えの影響を受けた (-want +got):\n%s", diff)
		}
	})
}

// ---- 実装設計 AC-2-5 の不変条件③・AC-5-9（対応表 5-9-a〜5-9-c）・AC-11-5（決定9）---

// TestReconstruct_ExcessShortfallStateConsistency は、Reconstruct が受け取る
// 確定済みの超過／不足（excess・shortfall）と状態の整合を検証する
// （実装設計 AC-2-5 の不変条件③・AC-5-9 の対応表 5-9-a〜5-9-c。決定9）。
//
// 状態は Draft・PendingApproval・Approved の3値がすべてであり（AC-3-7。
// 差戻しは状態ではなく操作でその結果は Draft）、対応表は全状態を尽くす:
//   - Draft: 復元に成功するのは (nil, nil) のみ。(値, nil)・(nil, 値)・(値, 値) は
//     いずれも ErrInvalidValue（5-9-a）。
//   - PendingApproval・Approved: 復元に成功するのは双方が非nilのみ。
//     (nil, nil)・(値, nil)・(nil, 値) はいずれも ErrInvalidValue（5-9-b・5-9-c）。
//
// 値そのもの（ゼロか正か、超過と不足が同時に正か）は本 AC の検査対象に含まない
// （AC-13-13）ため、失敗しない組み合わせに使う値は任意の非nil値でよい。
func TestReconstruct_ExcessShortfallStateConsistency(t *testing.T) {
	determined := mustWorkingHours(t, 1, 0)

	tests := []struct {
		name                    string
		state                   workmonth.State
		excess                  *workmonth.WorkingHours
		shortfall               *workmonth.WorkingHours
		wantErr                 error
		wantExcessDetermined    bool
		wantShortfallDetermined bool
	}{
		// AC-5-9-a: Draft
		{
			name:                 "Draftは(nil, nil)なら復元でき、両アクセサとも未確定になる（AC-5-9-a）",
			state:                workmonth.StateDraft,
			excess:               nil,
			shortfall:            nil,
			wantExcessDetermined: false, wantShortfallDetermined: false,
		},
		{
			name:      "Draftで超過だけ確定済みの行は弾く（AC-5-9-a）",
			state:     workmonth.StateDraft,
			excess:    &determined,
			shortfall: nil,
			wantErr:   workmonth.ErrInvalidValue,
		},
		{
			name:      "Draftで不足だけ確定済みの行は弾く（AC-5-9-a）",
			state:     workmonth.StateDraft,
			excess:    nil,
			shortfall: &determined,
			wantErr:   workmonth.ErrInvalidValue,
		},
		{
			name:      "Draftで超過・不足とも確定済みの行は弾く（AC-5-9-a）",
			state:     workmonth.StateDraft,
			excess:    &determined,
			shortfall: &determined,
			wantErr:   workmonth.ErrInvalidValue,
		},
		// AC-5-9-b: PendingApproval
		{
			name:                 "PendingApprovalは双方非nilなら復元でき、両アクセサとも確定済みになる（AC-5-9-b）",
			state:                workmonth.StatePendingApproval,
			excess:               &determined,
			shortfall:            &determined,
			wantExcessDetermined: true, wantShortfallDetermined: true,
		},
		{
			name:      "PendingApprovalで双方nilの行は弾く（AC-5-9-b）",
			state:     workmonth.StatePendingApproval,
			excess:    nil,
			shortfall: nil,
			wantErr:   workmonth.ErrInvalidValue,
		},
		{
			name:      "PendingApprovalで超過だけnilの行は弾く（AC-5-9-b）",
			state:     workmonth.StatePendingApproval,
			excess:    nil,
			shortfall: &determined,
			wantErr:   workmonth.ErrInvalidValue,
		},
		{
			name:      "PendingApprovalで不足だけnilの行は弾く（AC-5-9-b）",
			state:     workmonth.StatePendingApproval,
			excess:    &determined,
			shortfall: nil,
			wantErr:   workmonth.ErrInvalidValue,
		},
		// AC-5-9-c: Approved
		{
			name:                 "Approvedは双方非nilなら復元でき、両アクセサとも確定済みになる（AC-5-9-c）",
			state:                workmonth.StateApproved,
			excess:               &determined,
			shortfall:            &determined,
			wantExcessDetermined: true, wantShortfallDetermined: true,
		},
		{
			name:      "Approvedで双方nilの行は弾く（AC-5-9-c）",
			state:     workmonth.StateApproved,
			excess:    nil,
			shortfall: nil,
			wantErr:   workmonth.ErrInvalidValue,
		},
		{
			name:      "Approvedで超過だけnilの行は弾く（AC-5-9-c）",
			state:     workmonth.StateApproved,
			excess:    nil,
			shortfall: &determined,
			wantErr:   workmonth.ErrInvalidValue,
		},
		{
			name:      "Approvedで不足だけnilの行は弾く（AC-5-9-c）",
			state:     workmonth.StateApproved,
			excess:    &determined,
			shortfall: nil,
			wantErr:   workmonth.ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.Reconstruct(
				mustContractID(t, "ctr-0001"),
				mustYearMonth(t, 2026, 7),
				mustSettlementRange(t, 140, 180),
				tt.state,
				nil,
				tt.excess,
				tt.shortfall,
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Reconstruct のエラー = %v, want errors.Is(err, %v)（AC-11-5・決定9）", err, tt.wantErr)
				}
				if got != nil {
					t.Errorf("不正な組み合わせから勤務月が復元されている: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Reconstruct が予期しないエラーを返した: %v", err)
			}

			_, excessOK := got.Excess()
			if excessOK != tt.wantExcessDetermined {
				t.Errorf("Excess() の確定済みフラグ = %v, want %v（AC-5-7）", excessOK, tt.wantExcessDetermined)
			}
			_, shortfallOK := got.Shortfall()
			if shortfallOK != tt.wantShortfallDetermined {
				t.Errorf("Shortfall() の確定済みフラグ = %v, want %v（AC-5-7）", shortfallOK, tt.wantShortfallDetermined)
			}
		})
	}
}
