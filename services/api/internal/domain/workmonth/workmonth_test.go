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
func mustReconstructWorkMonth(t *testing.T, state workmonth.State, records []workmonth.DailyRecord) *workmonth.WorkMonth {
	t.Helper()
	w, err := workmonth.Reconstruct(
		mustContractID(t, "ctr-0001"),
		mustYearMonth(t, 2026, 7),
		mustSettlementRange(t, 140, 180),
		state,
		records,
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

	tests := []struct {
		name       string
		contractID workmonth.ContractID
		yearMonth  workmonth.YearMonth
		state      workmonth.State
		records    []workmonth.DailyRecord
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
			records: []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
		},
		{
			name:       "承認済は復元できる（状態遷移を検査しない。AC-2-5）",
			contractID: validContractID, yearMonth: validYearMonth, state: workmonth.StateApproved,
			records: []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
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
