package workmonth

// 値オブジェクトと不変条件の置き場所は
// docs/specs/workmonth-implementation-design.md AC-3 が定める。
//
// 【スタブ】本ファイルの各関数・メソッドは tester 工程が置いたシグネチャのみの
// 骨組みであり、振る舞いは実装していない（TDD の Red を踏むための最小構成）。
// 中身は implementer 工程が埋める。

// ContractID は契約の識別子を表す値オブジェクト（AC-3-1）。
type ContractID struct {
	value string
}

// NewContractID は契約識別子を構築する。空文字は ErrInvalidValue。
func NewContractID(value string) (ContractID, error) {
	return ContractID{}, nil
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
	return YearMonth{}, nil
}

// Year は年を返す。
func (ym YearMonth) Year() int { return ym.year }

// Month は月（1〜12）を返す。
func (ym YearMonth) Month() int { return ym.month }

// Date は暦日を表す値オブジェクト（AC-3-3）。
type Date struct {
	year  int
	month int
	day   int
}

// NewDate は暦日を構築する。暦上実在しない日付は ErrInvalidValue。
func NewDate(year, month, day int) (Date, error) {
	return Date{}, nil
}

// Year は年を返す。
func (d Date) Year() int { return d.year }

// Month は月を返す。
func (d Date) Month() int { return d.month }

// Day は日を返す。
func (d Date) Day() int { return d.day }

// WorkingHours は稼働の量を表す値オブジェクト（AC-3-4）。内部表現は分の整数（AC-3-10）。
type WorkingHours struct {
	minutes int
}

// NewWorkingHours は稼働の量を構築する。
// 時が負／分が 0〜59 の範囲外なら ErrWorkingHoursOutOfRange（AC-11-4）。
// 1日の上限（24時間）はここでは検査しない（AC-3-8）。
func NewWorkingHours(hours, minutes int) (WorkingHours, error) {
	return WorkingHours{}, nil
}

// Hours は時の部分を返す。
func (w WorkingHours) Hours() int { return w.minutes / 60 }

// Minutes は分の部分（0〜59）を返す。
func (w WorkingHours) Minutes() int { return w.minutes % 60 }

// TotalMinutes は合計を分で返す。
func (w WorkingHours) TotalMinutes() int { return w.minutes }

// TruncateTo15Minutes は 15 分単位で切り捨てた稼働の量を返す（AC-3-9）。
func (w WorkingHours) TruncateTo15Minutes() WorkingHours {
	return WorkingHours{}
}

// DailyRecord は1日分の稼働の記録を表す値オブジェクト（AC-3-5）。
type DailyRecord struct {
	date         Date
	workingHours WorkingHours
}

// NewDailyRecord は稼働実績を構築する。
// 稼働時間が 0時間0分以上 24時間0分以下でなければ ErrWorkingHoursOutOfRange。
func NewDailyRecord(date Date, workingHours WorkingHours) (DailyRecord, error) {
	return DailyRecord{}, nil
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
	return SettlementRange{}, nil
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
