package workmonth

import (
	"fmt"
	"slices"
)

// 集約ルート WorkMonth。保持するもの／保持しないものは
// docs/specs/workmonth-implementation-design.md AC-2、
// 状態と遷移メソッドは同 AC-4 が定める。
//
// 集約が守るのは状態遷移の正当性と自身の不変条件に閉じる。
// 認可（本人・ロール・自己承認）は一切知らない（同 P-6・D-4）。

// WorkMonth は勤務月の集約ルート。フィールドはすべて非公開（AC-2-3）。
type WorkMonth struct {
	contractID      ContractID
	yearMonth       YearMonth
	settlementRange SettlementRange
	state           State
	dailyRecords    []DailyRecord
	// excess・shortfall は締め（Close）で確定する超過／不足（実装設計 AC-2-5・AC-5-9）。
	// nil は未確定（Draft）を表す。
	excess    *WorkingHours
	shortfall *WorkingHours
}

// New は勤務月を新規生成する。初期状態は Draft（AC-2-4）。
//
// 精算幅は契約から複写した値を受け取り、以降は契約の変更に追随しない
// （docs/specs/daily-record-entry.md AC-1-2）。
func New(contractID ContractID, yearMonth YearMonth, settlementRange SettlementRange) (*WorkMonth, error) {
	if err := validateIdentity(contractID, yearMonth); err != nil {
		return nil, err
	}
	return &WorkMonth{
		contractID:      contractID,
		yearMonth:       yearMonth,
		settlementRange: settlementRange,
		state:           StateDraft,
		dailyRecords:    []DailyRecord{},
	}, nil
}

// Reconstruct は永続化された勤務月を再構築する（AC-2-5）。
// 状態遷移の検査は行わず、値オブジェクトの妥当性と集約の不変条件のみ検査する。
//
// 検査する不変条件は「対象日で一意」（AC-2-6）と「すべての対象日が当該年月に属する」
// （AC-2-1・AC-2-6）の2つ。後者の違反は、正常な経路（EnterDailyRecord・
// DeleteDailyRecord）では作られない行に対する防御であり、利用者の要求の不正では
// ないため、`ErrDateOutOfMonth` ではなく `ErrInvalidValue` で失敗させる
// （AC-3-11。Issue #51、2026-07-28 に人間が確定）。
//
// excess・shortfall は確定済みの超過／不足（AC-5-9）。未確定は nil。
// 受け取った値と状態の整合（例: Draft なのに値がある／締め済なのに値が無い）を
// 検査するかどうかは Q-1 として人間へ確定を待っている未決の論点であり
// （実装設計「人間の決定を待っている論点」Q-1、2026-07-30）、確定するまで
// 3つ目の不変条件を足さない。受け取った値はそのまま複写して保持する。
func Reconstruct(
	contractID ContractID,
	yearMonth YearMonth,
	settlementRange SettlementRange,
	state State,
	dailyRecords []DailyRecord,
	excess *WorkingHours,
	shortfall *WorkingHours,
) (*WorkMonth, error) {
	if err := validateIdentity(contractID, yearMonth); err != nil {
		return nil, err
	}
	if !state.valid() {
		return nil, fmt.Errorf("%w: state %q is not defined", ErrInvalidValue, state)
	}

	records := make([]DailyRecord, len(dailyRecords))
	copy(records, dailyRecords)
	sortRecords(records)
	for _, record := range records {
		// すべての対象日が当該年月に属する（AC-2-1・AC-2-6）は集約の不変条件であり、
		// 復元によっても壊さない。番兵は ErrInvalidValue（AC-2-5・AC-3-11）。
		if !yearMonth.Contains(record.Date()) {
			return nil, fmt.Errorf(
				"%w: daily record for %04d-%02d-%02d does not belong to %04d-%02d",
				ErrInvalidValue,
				record.Date().Year(), record.Date().Month(), record.Date().Day(),
				yearMonth.Year(), yearMonth.Month(),
			)
		}
	}
	for i := 1; i < len(records); i++ {
		// 対象日で一意（1日最大1件。AC-2-6）は集約の不変条件であり、
		// 復元によっても壊さない。
		if records[i].Date().compare(records[i-1].Date()) == 0 {
			return nil, fmt.Errorf(
				"%w: duplicated daily record for %04d-%02d-%02d",
				ErrInvalidValue,
				records[i].Date().Year(), records[i].Date().Month(), records[i].Date().Day(),
			)
		}
	}

	return &WorkMonth{
		contractID:      contractID,
		yearMonth:       yearMonth,
		settlementRange: settlementRange,
		state:           state,
		dailyRecords:    records,
		excess:          copyWorkingHoursPointer(excess),
		shortfall:       copyWorkingHoursPointer(shortfall),
	}, nil
}

// copyWorkingHoursPointer は *WorkingHours を値として複写する。呼び出し側が
// 引数に渡したポインタの参照先を後から書き換えても集約が影響を受けないようにする
// （AC-2-3・AC-5-9）。nil はそのまま nil を返す（未確定）。
func copyWorkingHoursPointer(w *WorkingHours) *WorkingHours {
	if w == nil {
		return nil
	}
	copied := *w
	return &copied
}

// validateIdentity は勤務月を一意にする値（契約 × 年月）の妥当性を検査する。
func validateIdentity(contractID ContractID, yearMonth YearMonth) error {
	if contractID.String() == "" {
		return fmt.Errorf("%w: contract id must not be empty", ErrInvalidValue)
	}
	if yearMonth.Month() < 1 || yearMonth.Month() > 12 {
		return fmt.Errorf("%w: month %d is out of range", ErrInvalidValue, yearMonth.Month())
	}
	return nil
}

// sortRecords は稼働実績を対象日の昇順に並べ替える（AC-2-6）。
func sortRecords(records []DailyRecord) {
	slices.SortFunc(records, func(a, b DailyRecord) int {
		return a.Date().compare(b.Date())
	})
}

// ContractID は契約の識別子を返す。
func (w *WorkMonth) ContractID() ContractID { return w.contractID }

// YearMonth は対象年月を返す。
func (w *WorkMonth) YearMonth() YearMonth { return w.yearMonth }

// SettlementRange は生成時に複写した精算幅を返す。
func (w *WorkMonth) SettlementRange() SettlementRange { return w.settlementRange }

// State は状態を返す。
func (w *WorkMonth) State() State { return w.state }

// DailyRecords は稼働実績を対象日の昇順で返す（AC-2-6）。
//
// 内部のスライスをそのまま渡さない。集約の外から要素を差し替えられると
// 不変条件を集約の内側に閉じられなくなるため（AC-2-3）。
func (w *WorkMonth) DailyRecords() []DailyRecord {
	records := make([]DailyRecord, len(w.dailyRecords))
	copy(records, w.dailyRecords)
	return records
}

// Excess は確定済みの超過を返す。第2戻り値は確定済みか（AC-5-7）。
// 未確定（Draft）の間は第1戻り値をゼロ値とし、呼び出し側は第2戻り値でのみ判別する
// （AC-5-2・AC-5-7。0 と未確定を混同しない）。
func (w *WorkMonth) Excess() (WorkingHours, bool) {
	if w.excess == nil {
		return WorkingHours{}, false
	}
	return *w.excess, true
}

// Shortfall は確定済みの不足を返す。第2戻り値は確定済みか（AC-5-7）。
func (w *WorkMonth) Shortfall() (WorkingHours, bool) {
	if w.shortfall == nil {
		return WorkingHours{}, false
	}
	return *w.shortfall, true
}

// Close は勤務月を締める（monthly-closing.md AC-1・AC-3・AC-5、実装設計 AC-4-3）。
// Draft のみ許可し、超過／不足を算出して確定させたうえで PendingApproval へ
// 直接遷移する（中間状態を経ない。同 AC-5-1）。Draft 以外なら遷移も算出もせず
// ErrNotClosable（二重締め・終端状態。同 AC-1-2・AC-1-3。実装設計 AC-11-6）。
// 引数を取らない（締めに月末制約・「当日」を要さない。同 D-3・AC-1-4）。
func (w *WorkMonth) Close() error {
	if w.state != StateDraft {
		return fmt.Errorf("%w: state is %q", ErrNotClosable, w.state)
	}
	excess, shortfall := w.computeExcessShortfall()
	w.excess = &excess
	w.shortfall = &shortfall
	w.state = StatePendingApproval
	return nil
}

// computeExcessShortfall は超過／不足を算出する（実装設計 AC-5-6・AC-5-8）。
// 入力は TotalHours() と集約が保持する SettlementRange に限り、契約の現在値は
// 参照しない（monthly-closing.md AC-3-2・D-2）。両端は範囲内として扱う
// （同 AC-4。超過・不足の一方は必ず0であり、両方が同時に正になる戻り値を
// 構造上作らない＝実装設計 AC-5-6）。
func (w *WorkMonth) computeExcessShortfall() (excess, shortfall WorkingHours) {
	total := w.TotalHours().TotalMinutes()
	lower := w.settlementRange.LowerBound().TotalMinutes()
	upper := w.settlementRange.UpperBound().TotalMinutes()

	switch {
	case total > upper:
		excess = WorkingHours{minutes: total - upper}
	case total < lower:
		shortfall = WorkingHours{minutes: lower - total}
	}
	return excess, shortfall
}

// TotalHours は総稼働時間を都度算出して返す（AC-5-1）。
//
// 各日の稼働時間を15分単位で切り捨ててから合計する。合計してから丸めない
// （docs/specs/daily-record-entry.md AC-6-1、ユビキタス言語「丸め規則」）。
// レコードのない日は項に現れない（同 AC-6-3）。
func (w *WorkMonth) TotalHours() WorkingHours {
	var total WorkingHours
	for _, record := range w.dailyRecords {
		total = total.add(record.WorkingHours().TruncateTo15Minutes())
	}
	return total
}

// EnsureEditable は稼働実績を編集できる状態かを検査する。
// Draft 以外なら ErrNotEditable（docs/specs/daily-record-entry.md AC-5-1〜AC-5-3）。
//
// 状態の判定を業務バリデーション（値域）より先に行えるように公開している
// （判定順序は docs/specs/domain-api-http-contract.md AC-9）。
// 「Draft でのみ編集できる」という規則を集約の外へ書き写さないための入口であり、
// 遷移メソッド（EnterDailyRecord / DeleteDailyRecord）自身も同じ検査を通る。
func (w *WorkMonth) EnsureEditable() error {
	if w.state != StateDraft {
		return fmt.Errorf("%w: state is %q", ErrNotEditable, w.state)
	}
	return nil
}

// EnterDailyRecord は稼働実績を追加または上書きする（AC-4-1）。
// today は未来日判定の基準日であり、呼び出し側から受け取る（AC-4-7）。
//
// 同一日への入力は編集として上書きする（1日最大1件。
// docs/specs/daily-record-entry.md AC-2-2・AC-2-3）。
func (w *WorkMonth) EnterDailyRecord(record DailyRecord, today Date) error {
	if err := w.EnsureEditable(); err != nil {
		return err
	}
	if !w.yearMonth.Contains(record.Date()) {
		return fmt.Errorf(
			"%w: %04d-%02d-%02d does not belong to %04d-%02d",
			ErrDateOutOfMonth,
			record.Date().Year(), record.Date().Month(), record.Date().Day(),
			w.yearMonth.Year(), w.yearMonth.Month(),
		)
	}
	if record.Date().compare(today) > 0 {
		return fmt.Errorf(
			"%w: %04d-%02d-%02d is after today",
			ErrFutureDate,
			record.Date().Year(), record.Date().Month(), record.Date().Day(),
		)
	}

	for i, existing := range w.dailyRecords {
		if existing.Date().compare(record.Date()) == 0 {
			w.dailyRecords[i] = record
			return nil
		}
	}
	w.dailyRecords = append(w.dailyRecords, record)
	sortRecords(w.dailyRecords)
	return nil
}

// DeleteDailyRecord は対象日の稼働実績を取り除く（AC-4-2）。
//
// 検査順は ①状態（Draft か） → ②当該年月に属するか、とする
// （docs/specs/domain-api-http-contract.md AC-9 の順5→順6と一致させる）。
// 当該年月に属さない対象日への削除は、当該日にレコードがあるか否か・
// 他の日にレコードがあるか否かを問わず弾く（docs/specs/daily-record-entry.md
// AC-5-5・D-9。Issue #51、2026-07-28 に人間が確定）。
//
// 当該年月に属する対象日で、レコードが無い日への削除は成功として扱う。
// 「レコードのない日＝稼働なし」と区別しないため（同 D-5・AC-5-4）。
func (w *WorkMonth) DeleteDailyRecord(date Date) error {
	if err := w.EnsureEditable(); err != nil {
		return err
	}
	if !w.yearMonth.Contains(date) {
		return fmt.Errorf(
			"%w: %04d-%02d-%02d does not belong to %04d-%02d",
			ErrDateOutOfMonth,
			date.Year(), date.Month(), date.Day(),
			w.yearMonth.Year(), w.yearMonth.Month(),
		)
	}
	w.dailyRecords = slices.DeleteFunc(w.dailyRecords, func(record DailyRecord) bool {
		return record.Date().compare(date) == 0
	})
	return nil
}
