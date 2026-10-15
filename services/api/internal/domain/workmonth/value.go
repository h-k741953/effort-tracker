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

// Contains は暦日が当該年月に属するかを返す
// （docs/specs/daily-record-entry.md AC-2-4・AC-5-5・D-9 の判定に使う）。
//
// 「対象日が当該年月に属するか」の判定はここ1箇所に置き、規則を複数箇所へ
// 実装しない（実装設計 AC-3-11）。利用者は集約の EnterDailyRecord・
// DeleteDailyRecord、Reconstruct、未生成の勤務月に対する削除の interactor の4つ。
// 返すエラーは利用者側で使い分ける（利用者の要求に対しては ErrDateOutOfMonth、
// Reconstruct は ErrInvalidValue）。
func (ym YearMonth) Contains(d Date) bool {
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
// 時が負／分が 0〜59 の範囲外なら ErrInvalidValue（AC-3-4・AC-11-4）。
// 1日の上限（24時間）はここでは検査しない（AC-3-8）。
func NewWorkingHours(hours, minutes int) (WorkingHours, error) {
	if hours < 0 {
		return WorkingHours{}, fmt.Errorf("%w: hours %d must not be negative", ErrInvalidValue, hours)
	}
	if minutes < 0 || minutes >= minutesPerHour {
		return WorkingHours{}, fmt.Errorf(
			"%w: minutes %d must be in 0..%d", ErrInvalidValue, minutes, minutesPerHour-1,
		)
	}
	return WorkingHours{minutes: hours*minutesPerHour + minutes}, nil
}

// Hours は時の部分を返す。
func (w WorkingHours) Hours() int { return w.minutes / minutesPerHour }

// Minutes は分の部分を返す。値域の出典は NewWorkingHours と同じ。
func (w WorkingHours) Minutes() int { return w.minutes % minutesPerHour }

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

// 時刻の粒度と値域（daily-record-entry.md D-10・D-11）。値そのものの正解は出典が持つ。
const (
	minutesPerDay    = 24 * minutesPerHour
	timeOfDayStepMin = 15
)

// 1日の稼働時間の下限・上限（daily-record-entry.md D-7）。分で保持する。
const (
	minDailyMinutes = 15
	maxDailyMinutes = minutesPerDay - 15
)

// TimeOfDay は時刻（1日の中の時点）を表す値オブジェクト（AC-3-5-a）。
// 対象日の始まりからの経過分の整数で保持する（AC-3-10）。
type TimeOfDay struct {
	minutes int
}

// NewTimeOfDay は時刻を構築する。時が 0〜23、分が 0・15・30・45 でなければ
// ErrTimeOfDayOutOfRange（AC-3-14 ①）。4つの時刻すべてが同じ構築を通る。
func NewTimeOfDay(hour, minute int) (TimeOfDay, error) {
	if hour < 0 || hour > 23 {
		return TimeOfDay{}, fmt.Errorf("%w: hour %d must be in 0..23", ErrTimeOfDayOutOfRange, hour)
	}
	if minute < 0 || minute >= minutesPerHour || minute%timeOfDayStepMin != 0 {
		return TimeOfDay{}, fmt.Errorf("%w: minute %d must be one of 0, 15, 30, 45", ErrTimeOfDayOutOfRange, minute)
	}
	return TimeOfDay{minutes: hour*minutesPerHour + minute}, nil
}

// Hour は時を返す。
func (t TimeOfDay) Hour() int { return t.minutes / minutesPerHour }

// Minute は分を返す。
func (t TimeOfDay) Minute() int { return t.minutes % minutesPerHour }

// Break は休憩（休憩開始時刻と休憩終了時刻の組）を表す値オブジェクト（AC-3-5-b）。
type Break struct {
	start TimeOfDay
	end   TimeOfDay
}

// NewBreak は休憩を構築する。終了が開始より後でなければ ErrInvalidTimeCombination（AC-3-14 ②）。
func NewBreak(start, end TimeOfDay) (Break, error) {
	if end.minutes <= start.minutes {
		return Break{}, fmt.Errorf("%w: break end must be after break start", ErrInvalidTimeCombination)
	}
	return Break{start: start, end: end}, nil
}

// Start は休憩開始時刻を返す。
func (b Break) Start() TimeOfDay { return b.start }

// End は休憩終了時刻を返す。
func (b Break) End() TimeOfDay { return b.end }

// DailyRecord は1日分の稼働の記録を表す値オブジェクト（AC-3-5）。
// 稼働時間は保持した時刻から算出し、別に保持しない（AC-3-13）。
type DailyRecord struct {
	date      Date
	startTime TimeOfDay
	endTime   TimeOfDay
	brk       Break
	hasBreak  bool
}

// NewDailyRecord は稼働実績を構築する。brk が nil なら休憩なし。
// 終了＞開始、休憩が勤務時間に収まること（端点を含む）、算出した稼働時間が
// D-7 の範囲にあることを検査し、満たさなければ ErrInvalidTimeCombination（AC-3-14 ②）。
//
// 稼働時間の上限側の分岐は、時刻の値域と前後関係を満たす入力からは到達しない防御であり、
// テスト不足ではない（AC-3-12-j）。
func NewDailyRecord(date Date, startTime, endTime TimeOfDay, brk *Break) (DailyRecord, error) {
	if endTime.minutes <= startTime.minutes {
		return DailyRecord{}, fmt.Errorf("%w: end time must be after start time", ErrInvalidTimeCombination)
	}
	r := DailyRecord{date: date, startTime: startTime, endTime: endTime}
	if brk != nil {
		if brk.start.minutes < startTime.minutes || brk.end.minutes > endTime.minutes {
			return DailyRecord{}, fmt.Errorf("%w: break must be within working time", ErrInvalidTimeCombination)
		}
		r.brk = *brk
		r.hasBreak = true
	}
	worked := r.WorkingHours().TotalMinutes()
	if worked < minDailyMinutes || worked > maxDailyMinutes {
		return DailyRecord{}, fmt.Errorf(
			"%w: daily working hours %d minutes must be in %d..%d minutes",
			ErrInvalidTimeCombination, worked, minDailyMinutes, maxDailyMinutes,
		)
	}
	return r, nil
}

// Date は対象日を返す。
func (r DailyRecord) Date() Date { return r.date }

// StartTime は開始時刻を返す。
func (r DailyRecord) StartTime() TimeOfDay { return r.startTime }

// EndTime は終了時刻を返す。
func (r DailyRecord) EndTime() TimeOfDay { return r.endTime }

// Break は休憩を返す。第2戻り値が休憩ありか。休憩なしのとき第1戻り値はゼロ値で、
// 判別は第2戻り値でのみ行う（AC-3-15）。
func (r DailyRecord) Break() (Break, bool) {
	if !r.hasBreak {
		return Break{}, false
	}
	return r.brk, true
}

// WorkingHours は時刻から算出した稼働時間を返す（AC-3-13）。
func (r DailyRecord) WorkingHours() WorkingHours {
	minutes := r.endTime.minutes - r.startTime.minutes
	if r.hasBreak {
		minutes -= r.brk.end.minutes - r.brk.start.minutes
	}
	return WorkingHours{minutes: minutes}
}

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
