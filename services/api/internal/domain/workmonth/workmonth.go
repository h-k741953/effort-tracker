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
// 状態遷移の検査は行わず、値オブジェクトの妥当性のみ検査する。
//
// 検査の非対称性について。ここでは「対象日で一意」（AC-2-6）は検査するが、
// 「対象日が当該年月に属すること」は検査しない。この非対称性をどう解消するか
// （当該年月に属さない対象日を持つ行の復元と、そのような対象日への削除の扱い）は
// **人間の決定を要する業務ルールであり、現時点では未決**である。
// 決定の材料として、現在の挙動を事実として記録しておく。
//
//   - 当該年月に属さない対象日を持つ行は、そのまま復元される（ここでは弾かない）。
//   - 当該年月に属さない対象日への削除は、一致するレコードが無いため何も起きず、
//     成功（HTTP 200）を返す（no-op）。「レコードのない日への削除は成功」
//     （docs/specs/daily-record-entry.md D-5）と観測上は区別が付かない。
//   - 入力側は集約が ErrDateOutOfMonth で弾く（AC-2-4）。削除側に対応する判定は無い。
//
// 決定が出るまで、この振る舞いを前提にした業務ルールを実装側で足さない。
func Reconstruct(
	contractID ContractID,
	yearMonth YearMonth,
	settlementRange SettlementRange,
	state State,
	dailyRecords []DailyRecord,
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
	}, nil
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
	if !w.yearMonth.contains(record.Date()) {
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
// レコードが無い日への削除は成功として扱う。「レコードのない日＝稼働なし」と
// 区別しないため（docs/specs/daily-record-entry.md D-5・AC-5-4）。
func (w *WorkMonth) DeleteDailyRecord(date Date) error {
	if err := w.EnsureEditable(); err != nil {
		return err
	}
	w.dailyRecords = slices.DeleteFunc(w.dailyRecords, func(record DailyRecord) bool {
		return record.Date().compare(date) == 0
	})
	return nil
}
