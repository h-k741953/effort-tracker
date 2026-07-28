package workmonth

import "fmt"

// 値オブジェクトと不変条件の置き場所は
// docs/specs/workmonth-implementation-design.md AC-3 が定める。
//
// 構築時に検証し、不正な値のインスタンスを存在させない（AC-3 柱書）。
// 稼働の量は分の整数で保持し、浮動小数点を使わない（AC-3-10）。

const (
	// minutesPerHour は1時間の分数。
	minutesPerHour = 60

	// truncationUnitMinutes は日次の稼働時間の丸め単位（15分切り捨て。AC-3-9）。
	truncationUnitMinutes = 15

	// maxDailyMinutes は1日の稼働時間の上限（24時間0分。上限を含む）。
	// 検査位置は DailyRecord の構築時（AC-3-8）。
	maxDailyMinutes = 24 * minutesPerHour
)

// ContractID は契約の識別子を表す値オブジェクト（AC-3-1）。
type ContractID struct {
	value string
}

// NewContractID は契約識別子を構築する。空文字は ErrInvalidValue。
func NewContractID(value string) (ContractID, error) {
	if value == "" {
		return ContractID{}, fmt.Errorf("%w: contract id must not be empty", ErrInvalidValue)
	}
	return ContractID{value: value}, nil
}

// String は契約識別子の文字列表現を返す。
func (c ContractID) String() string {
	return c.value
}

// YearMonth は勤務月の対象年月を表す値オブジェクト（AC-3-2）。
type YearMonth struct {
	year  int
	month int
}

// NewYearMonth は対象年月を構築する。月が 1〜12 の範囲外なら ErrInvalidValue。
func NewYearMonth(year, month int) (YearMonth, error) {
	if month < 1 || month > 12 {
		return YearMonth{}, fmt.Errorf("%w: month %d is out of range", ErrInvalidValue, month)
	}
	return YearMonth{year: year, month: month}, nil
}

// Year は年を返す。
func (ym YearMonth) Year() int { return ym.year }

// Month は月（1〜12）を返す。
func (ym YearMonth) Month() int { return ym.month }

// contains は暦日が当該年月に属するかを返す
// （docs/specs/daily-record-entry.md AC-2-4 の判定に使う）。
func (ym YearMonth) contains(d Date) bool {
	return ym.year == d.year && ym.month == d.month
}

// Date は暦日を表す値オブジェクト（AC-3-3）。
type Date struct {
	year  int
	month int
	day   int
}

// NewDate は暦日を構築する。暦上実在しない日付は ErrInvalidValue。
//
// 暦の判定はここで完結させる。集約は時計もタイムゾーンも持たず、
// 「当日」は呼び出し側から引数として受け取る（実装設計 D-5・AC-4-7）。
func NewDate(year, month, day int) (Date, error) {
	if month < 1 || month > 12 {
		return Date{}, fmt.Errorf("%w: month %d is out of range", ErrInvalidValue, month)
	}
	if day < 1 || day > daysInMonth(year, month) {
		return Date{}, fmt.Errorf("%w: %04d-%02d-%02d does not exist", ErrInvalidValue, year, month, day)
	}
	return Date{year: year, month: month, day: day}, nil
}

// daysInMonth は当該年月の日数を返す。
func daysInMonth(year, month int) int {
	switch month {
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

// isLeapYear はグレゴリオ暦のうるう年かを返す。
func isLeapYear(year int) bool {
	if year%400 == 0 {
		return true
	}
	if year%100 == 0 {
		return false
	}
	return year%4 == 0
}

// Year は年を返す。
func (d Date) Year() int { return d.year }

// Month は月を返す。
func (d Date) Month() int { return d.month }

// Day は日を返す。
func (d Date) Day() int { return d.day }

// compare は暦日の前後を比較する。d が other より前なら負、同じなら 0、後なら正。
func (d Date) compare(other Date) int {
	if d.year != other.year {
		return d.year - other.year
	}
	if d.month != other.month {
		return d.month - other.month
	}
	return d.day - other.day
}

// WorkingHours は稼働の量を表す値オブジェクト（AC-3-4）。内部表現は分の整数（AC-3-10）。
type WorkingHours struct {
	minutes int
}

// NewWorkingHours は稼働の量を構築する。
// 時が負／分が 0〜59 の範囲外なら ErrWorkingHoursOutOfRange（AC-11-4）。
// 1日の上限（24時間）はここでは検査しない（AC-3-8）。
func NewWorkingHours(hours, minutes int) (WorkingHours, error) {
	if hours < 0 {
		return WorkingHours{}, fmt.Errorf("%w: hours %d must not be negative", ErrWorkingHoursOutOfRange, hours)
	}
	if minutes < 0 || minutes >= minutesPerHour {
		return WorkingHours{}, fmt.Errorf("%w: minutes %d must be in 0..59", ErrWorkingHoursOutOfRange, minutes)
	}
	return WorkingHours{minutes: hours*minutesPerHour + minutes}, nil
}

// Hours は時の部分を返す。
func (w WorkingHours) Hours() int { return w.minutes / 60 }

// Minutes は分の部分（0〜59）を返す。
func (w WorkingHours) Minutes() int { return w.minutes % 60 }

// TotalMinutes は合計を分で返す。
func (w WorkingHours) TotalMinutes() int { return w.minutes }

// TruncateTo15Minutes は 15 分単位で切り捨てた稼働の量を返す（AC-3-9）。
//
// 日次の値に対して適用し、合計に対しては適用しない
// （docs/specs/daily-record-entry.md AC-6-1）。
func (w WorkingHours) TruncateTo15Minutes() WorkingHours {
	return WorkingHours{minutes: w.minutes - w.minutes%truncationUnitMinutes}
}

// add は稼働の量を加算する。合計の算出にのみ使う（分単位の整数演算。AC-3-10）。
func (w WorkingHours) add(other WorkingHours) WorkingHours {
	return WorkingHours{minutes: w.minutes + other.minutes}
}

// DailyRecord は1日分の稼働の記録を表す値オブジェクト（AC-3-5）。
type DailyRecord struct {
	date         Date
	workingHours WorkingHours
}

// NewDailyRecord は稼働実績を構築する。
// 稼働時間が 0時間0分以上 24時間0分以下でなければ ErrWorkingHoursOutOfRange。
func NewDailyRecord(date Date, workingHours WorkingHours) (DailyRecord, error) {
	if workingHours.TotalMinutes() < 0 || workingHours.TotalMinutes() > maxDailyMinutes {
		return DailyRecord{}, fmt.Errorf(
			"%w: daily working hours %d minutes must be in 0..%d minutes",
			ErrWorkingHoursOutOfRange, workingHours.TotalMinutes(), maxDailyMinutes,
		)
	}
	return DailyRecord{date: date, workingHours: workingHours}, nil
}

// Date は対象日を返す。
func (r DailyRecord) Date() Date { return r.date }

// WorkingHours は入力された稼働時間を返す。
func (r DailyRecord) WorkingHours() WorkingHours { return r.workingHours }

// SettlementRange は精算幅を表す値オブジェクト（AC-3-6）。
type SettlementRange struct {
	lowerBound WorkingHours
	upperBound WorkingHours
}

// NewSettlementRange は精算幅を構築する。下限 > 上限なら ErrInvalidValue。
func NewSettlementRange(lowerBound, upperBound WorkingHours) (SettlementRange, error) {
	if lowerBound.TotalMinutes() > upperBound.TotalMinutes() {
		return SettlementRange{}, fmt.Errorf(
			"%w: settlement lower bound %d minutes exceeds upper bound %d minutes",
			ErrInvalidValue, lowerBound.TotalMinutes(), upperBound.TotalMinutes(),
		)
	}
	return SettlementRange{lowerBound: lowerBound, upperBound: upperBound}, nil
}

// LowerBound は下限を返す。
func (s SettlementRange) LowerBound() WorkingHours { return s.lowerBound }

// UpperBound は上限を返す。
func (s SettlementRange) UpperBound() WorkingHours { return s.upperBound }

// State は勤務月の状態を表す（AC-3-7）。文字列表現はユビキタス言語の英語名と一致させる。
type State string

const (
	// StateDraft は下書き。勤務月の初期状態。
	StateDraft State = "Draft"
	// StatePendingApproval は締め済（承認待ちを兼ねる）。
	StatePendingApproval State = "PendingApproval"
	// StateApproved は承認済（終端状態）。
	StateApproved State = "Approved"
)

// String は状態の文字列表現を返す。
func (s State) String() string { return string(s) }

// valid は状態が定義済みの3値のいずれかかを返す（AC-3-7）。
func (s State) valid() bool {
	switch s {
	case StateDraft, StatePendingApproval, StateApproved:
		return true
	default:
		return false
	}
}
