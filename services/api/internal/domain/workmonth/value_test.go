package workmonth_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// 検証対象の受け入れ条件:
//   - docs/specs/daily-record-entry.md AC-2（稼働実績が持つもの）
//   - 同 AC-3（稼働時間の値域バリデーション）
//   - 同 AC-6（15分切り捨て）
//   - docs/specs/workmonth-implementation-design.md AC-3（値オブジェクトと不変条件）

// hoursView は go-cmp で稼働時間を比較するための表示用の射影。
// 値オブジェクトは非公開フィールドを持つため、公開アクセサ経由で比較する
// （実装設計 AC-12-4。reflect.DeepEqual は使わない）。
type hoursView struct {
	Hours   int
	Minutes int
}

func viewOfHours(w workmonth.WorkingHours) hoursView {
	return hoursView{Hours: w.Hours(), Minutes: w.Minutes()}
}

// TestNewWorkingHours は稼働時間の値域を検証する。
// AC-3-1（0時間0分は許可）・AC-3-4（負の値は弾く）・AC-3-5（分が 0〜59 の範囲外は弾く）。
// 1日の上限（24時間）は WorkingHours では検査しない（実装設計 AC-3-8）。
func TestNewWorkingHours(t *testing.T) {
	tests := []struct {
		name    string
		hours   int
		minutes int
		want    hoursView
		wantErr error
	}{
		{name: "0時間0分は許可（AC-3-1）", hours: 0, minutes: 0, want: hoursView{Hours: 0, Minutes: 0}},
		{name: "8時間50分は許可", hours: 8, minutes: 50, want: hoursView{Hours: 8, Minutes: 50}},
		{name: "分の上端 59 は許可（AC-3-5）", hours: 8, minutes: 59, want: hoursView{Hours: 8, Minutes: 59}},
		{name: "24時間0分は許可（AC-3-2）", hours: 24, minutes: 0, want: hoursView{Hours: 24, Minutes: 0}},
		{name: "24時間を超える値も WorkingHours 自体は許可（実装設計 AC-3-8）", hours: 140, minutes: 0, want: hoursView{Hours: 140, Minutes: 0}},
		{name: "分が 60 は弾く（AC-3-5）", hours: 8, minutes: 60, wantErr: workmonth.ErrWorkingHoursOutOfRange},
		{name: "分が負は弾く（AC-3-5）", hours: 8, minutes: -1, wantErr: workmonth.ErrWorkingHoursOutOfRange},
		{name: "時が負は弾く（AC-3-4）", hours: -1, minutes: 0, wantErr: workmonth.ErrWorkingHoursOutOfRange},
		{name: "時も分も負は弾く（AC-3-4）", hours: -1, minutes: -30, wantErr: workmonth.ErrWorkingHoursOutOfRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.NewWorkingHours(tt.hours, tt.minutes)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewWorkingHours(%d, %d) のエラー = %v, want errors.Is(err, %v)", tt.hours, tt.minutes, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewWorkingHours(%d, %d) が予期しないエラーを返した: %v", tt.hours, tt.minutes, err)
			}
			if diff := cmp.Diff(tt.want, viewOfHours(got)); diff != "" {
				t.Errorf("NewWorkingHours(%d, %d) の値が不一致 (-want +got):\n%s", tt.hours, tt.minutes, diff)
			}
		})
	}
}

// TestWorkingHours_TruncateTo15Minutes は15分単位の切り捨てを検証する。
// AC-6-1・AC-6-2（8時間50分 → 8時間45分）。丸め規則の出典はユビキタス言語「丸め規則」。
func TestWorkingHours_TruncateTo15Minutes(t *testing.T) {
	tests := []struct {
		name    string
		hours   int
		minutes int
		want    hoursView
	}{
		{name: "8時間50分は8時間45分になる（AC-6-2）", hours: 8, minutes: 50, want: hoursView{Hours: 8, Minutes: 45}},
		{name: "8時間45分はそのまま（15分の倍数）", hours: 8, minutes: 45, want: hoursView{Hours: 8, Minutes: 45}},
		{name: "0時間14分は0時間0分になる", hours: 0, minutes: 14, want: hoursView{Hours: 0, Minutes: 0}},
		{name: "0時間15分は0時間15分のまま", hours: 0, minutes: 15, want: hoursView{Hours: 0, Minutes: 15}},
		{name: "8時間59分は8時間45分になる", hours: 8, minutes: 59, want: hoursView{Hours: 8, Minutes: 45}},
		{name: "8時間0分はそのまま", hours: 8, minutes: 0, want: hoursView{Hours: 8, Minutes: 0}},
		{name: "24時間0分はそのまま", hours: 24, minutes: 0, want: hoursView{Hours: 24, Minutes: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := mustWorkingHours(t, tt.hours, tt.minutes)

			if diff := cmp.Diff(tt.want, viewOfHours(w.TruncateTo15Minutes())); diff != "" {
				t.Errorf("(%d時間%d分).TruncateTo15Minutes() が不一致 (-want +got):\n%s", tt.hours, tt.minutes, diff)
			}
		})
	}
}

// TestNewDailyRecord_WorkingHoursRange は1日の稼働時間の上限を検証する。
// AC-3-1（0時間0分は許可）・AC-3-2（24時間0分は許可・上限を含む）・AC-3-3（24時間超は弾く）。
// 検査位置は DailyRecord の構築時（実装設計 AC-3-5・AC-3-8）。
func TestNewDailyRecord_WorkingHoursRange(t *testing.T) {
	tests := []struct {
		name    string
		hours   int
		minutes int
		wantErr error
	}{
		{name: "0時間0分は許可（AC-3-1）", hours: 0, minutes: 0},
		{name: "8時間50分は許可", hours: 8, minutes: 50},
		{name: "24時間0分は許可・上限を含む（AC-3-2）", hours: 24, minutes: 0},
		{name: "23時間59分は許可", hours: 23, minutes: 59},
		{name: "24時間1分は弾く（AC-3-3）", hours: 24, minutes: 1, wantErr: workmonth.ErrWorkingHoursOutOfRange},
		{name: "25時間0分は弾く（AC-3-3）", hours: 25, minutes: 0, wantErr: workmonth.ErrWorkingHoursOutOfRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := mustDate(t, 2026, 7, 1)
			hours := mustWorkingHours(t, tt.hours, tt.minutes)

			got, err := workmonth.NewDailyRecord(date, hours)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewDailyRecord(%d時間%d分) のエラー = %v, want errors.Is(err, %v)", tt.hours, tt.minutes, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewDailyRecord(%d時間%d分) が予期しないエラーを返した: %v", tt.hours, tt.minutes, err)
			}
			want := hoursView{Hours: tt.hours, Minutes: tt.minutes}
			if diff := cmp.Diff(want, viewOfHours(got.WorkingHours())); diff != "" {
				t.Errorf("NewDailyRecord(%d時間%d分) の稼働時間が不一致 (-want +got):\n%s", tt.hours, tt.minutes, diff)
			}
		})
	}
}

// TestNewDailyRecord_HoldsDateAndWorkingHours は稼働実績が対象日と稼働時間を持つことを検証する（AC-2-1）。
// 開始・終了時刻や休憩時間は持たない（AC-2-1・D-2・D-3）。これは型の定義そのもので担保する。
func TestNewDailyRecord_HoldsDateAndWorkingHours(t *testing.T) {
	date := mustDate(t, 2026, 7, 15)
	hours := mustWorkingHours(t, 8, 50)

	record, err := workmonth.NewDailyRecord(date, hours)
	if err != nil {
		t.Fatalf("NewDailyRecord が予期しないエラーを返した: %v", err)
	}

	wantDate := dateView{Year: 2026, Month: 7, Day: 15}
	if diff := cmp.Diff(wantDate, viewOfDate(record.Date())); diff != "" {
		t.Errorf("DailyRecord.Date() が不一致 (-want +got):\n%s", diff)
	}
	wantHours := hoursView{Hours: 8, Minutes: 50}
	if diff := cmp.Diff(wantHours, viewOfHours(record.WorkingHours())); diff != "" {
		t.Errorf("DailyRecord.WorkingHours() が不一致 (-want +got):\n%s", diff)
	}
}

// dateView は go-cmp で暦日を比較するための表示用の射影。
type dateView struct {
	Year  int
	Month int
	Day   int
}

func viewOfDate(d workmonth.Date) dateView {
	return dateView{Year: d.Year(), Month: d.Month(), Day: d.Day()}
}

// TestNewDate は暦日の妥当性を検証する（実装設計 AC-3-3）。
// 未来日判定（AC-4）・当該月の判定（AC-2-4）はこの型を前提とする。
func TestNewDate(t *testing.T) {
	tests := []struct {
		name             string
		year, month, day int
		wantErr          error
	}{
		{name: "実在する日付", year: 2026, month: 7, day: 15},
		{name: "うるう年の2月29日は実在する", year: 2024, month: 2, day: 29},
		{name: "月末（31日）", year: 2026, month: 7, day: 31},
		{name: "2月30日は実在しない", year: 2026, month: 2, day: 30, wantErr: workmonth.ErrInvalidValue},
		{name: "うるう年でない年の2月29日は実在しない", year: 2026, month: 2, day: 29, wantErr: workmonth.ErrInvalidValue},
		{name: "4月31日は実在しない", year: 2026, month: 4, day: 31, wantErr: workmonth.ErrInvalidValue},
		{name: "0月は実在しない", year: 2026, month: 0, day: 1, wantErr: workmonth.ErrInvalidValue},
		{name: "13月は実在しない", year: 2026, month: 13, day: 1, wantErr: workmonth.ErrInvalidValue},
		{name: "0日は実在しない", year: 2026, month: 7, day: 0, wantErr: workmonth.ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.NewDate(tt.year, tt.month, tt.day)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewDate(%d, %d, %d) のエラー = %v, want errors.Is(err, %v)", tt.year, tt.month, tt.day, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewDate(%d, %d, %d) が予期しないエラーを返した: %v", tt.year, tt.month, tt.day, err)
			}
			want := dateView{Year: tt.year, Month: tt.month, Day: tt.day}
			if diff := cmp.Diff(want, viewOfDate(got)); diff != "" {
				t.Errorf("NewDate(%d, %d, %d) の値が不一致 (-want +got):\n%s", tt.year, tt.month, tt.day, diff)
			}
		})
	}
}

// TestNewYearMonth は対象年月の妥当性を検証する（実装設計 AC-3-2）。
func TestNewYearMonth(t *testing.T) {
	tests := []struct {
		name        string
		year, month int
		wantErr     error
	}{
		{name: "1月は許可", year: 2026, month: 1},
		{name: "12月は許可", year: 2026, month: 12},
		{name: "0月は弾く", year: 2026, month: 0, wantErr: workmonth.ErrInvalidValue},
		{name: "13月は弾く", year: 2026, month: 13, wantErr: workmonth.ErrInvalidValue},
		{name: "負の月は弾く", year: 2026, month: -1, wantErr: workmonth.ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.NewYearMonth(tt.year, tt.month)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewYearMonth(%d, %d) のエラー = %v, want errors.Is(err, %v)", tt.year, tt.month, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewYearMonth(%d, %d) が予期しないエラーを返した: %v", tt.year, tt.month, err)
			}
			if got.Year() != tt.year || got.Month() != tt.month {
				t.Errorf("NewYearMonth(%d, %d) = %d年%d月, want %d年%d月", tt.year, tt.month, got.Year(), got.Month(), tt.year, tt.month)
			}
		})
	}
}

// TestNewContractID は契約識別子の不変条件を検証する（実装設計 AC-3-1）。
func TestNewContractID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "非空文字は許可", value: "ctr-0001"},
		{name: "空文字は弾く", value: "", wantErr: workmonth.ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.NewContractID(tt.value)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewContractID(%q) のエラー = %v, want errors.Is(err, %v)", tt.value, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewContractID(%q) が予期しないエラーを返した: %v", tt.value, err)
			}
			if got.String() != tt.value {
				t.Errorf("NewContractID(%q).String() = %q, want %q", tt.value, got.String(), tt.value)
			}
		})
	}
}

// TestNewSettlementRange は精算幅の不変条件（下限 ≤ 上限）を検証する（実装設計 AC-3-6）。
// 勤務月の生成時に複写される値であり、AC-1-2 の前提となる。
func TestNewSettlementRange(t *testing.T) {
	tests := []struct {
		name         string
		lowerHours   int
		lowerMinutes int
		upperHours   int
		upperMinutes int
		wantErr      error
	}{
		{name: "下限 < 上限は許可", lowerHours: 140, upperHours: 180},
		{name: "下限 = 上限は許可", lowerHours: 160, upperHours: 160},
		{name: "下限 > 上限は弾く", lowerHours: 180, upperHours: 140, wantErr: workmonth.ErrInvalidValue},
		{name: "分の差で下限 > 上限になる場合も弾く", lowerHours: 160, lowerMinutes: 1, upperHours: 160, upperMinutes: 0, wantErr: workmonth.ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower := mustWorkingHours(t, tt.lowerHours, tt.lowerMinutes)
			upper := mustWorkingHours(t, tt.upperHours, tt.upperMinutes)

			got, err := workmonth.NewSettlementRange(lower, upper)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewSettlementRange のエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewSettlementRange が予期しないエラーを返した: %v", err)
			}
			want := []hoursView{
				{Hours: tt.lowerHours, Minutes: tt.lowerMinutes},
				{Hours: tt.upperHours, Minutes: tt.upperMinutes},
			}
			gotView := []hoursView{viewOfHours(got.LowerBound()), viewOfHours(got.UpperBound())}
			if diff := cmp.Diff(want, gotView); diff != "" {
				t.Errorf("SettlementRange の下限・上限が不一致 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStateStringRepresentation は状態の文字列表現がユビキタス言語の英語名と一致することを検証する
// （実装設計 AC-3-7）。
func TestStateStringRepresentation(t *testing.T) {
	tests := []struct {
		name  string
		state workmonth.State
		want  string
	}{
		{name: "下書き", state: workmonth.StateDraft, want: "Draft"},
		{name: "締め済（承認待ち）", state: workmonth.StatePendingApproval, want: "PendingApproval"},
		{name: "承認済", state: workmonth.StateApproved, want: "Approved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
