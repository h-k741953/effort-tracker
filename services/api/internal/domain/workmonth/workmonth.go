package workmonth

// 集約ルート WorkMonth。保持するもの／保持しないものは
// docs/specs/workmonth-implementation-design.md AC-2、
// 状態と遷移メソッドは同 AC-4 が定める。
//
// 【スタブ】本ファイルの各関数・メソッドは tester 工程が置いたシグネチャのみの
// 骨組みであり、振る舞いは実装していない（TDD の Red を踏むための最小構成）。
// 中身は implementer 工程が埋める。

// WorkMonth は勤務月の集約ルート。フィールドはすべて非公開（AC-2-3）。
type WorkMonth struct {
	contractID      ContractID
	yearMonth       YearMonth
	settlementRange SettlementRange
	state           State
	dailyRecords    []DailyRecord
}

// New は勤務月を新規生成する。初期状態は Draft（AC-2-4）。
func New(contractID ContractID, yearMonth YearMonth, settlementRange SettlementRange) (*WorkMonth, error) {
	return &WorkMonth{}, nil
}

// Reconstruct は永続化された勤務月を再構築する（AC-2-5）。
// 状態遷移の検査は行わず、値オブジェクトの妥当性のみ検査する。
func Reconstruct(
	contractID ContractID,
	yearMonth YearMonth,
	settlementRange SettlementRange,
	state State,
	dailyRecords []DailyRecord,
) (*WorkMonth, error) {
	return &WorkMonth{}, nil
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
func (w *WorkMonth) DailyRecords() []DailyRecord { return w.dailyRecords }

// TotalHours は総稼働時間を都度算出して返す（AC-5-1）。
func (w *WorkMonth) TotalHours() WorkingHours { return WorkingHours{} }

// EnterDailyRecord は稼働実績を追加または上書きする（AC-4-1）。
// today は未来日判定の基準日であり、呼び出し側から受け取る（AC-4-7）。
func (w *WorkMonth) EnterDailyRecord(record DailyRecord, today Date) error {
	return nil
}

// DeleteDailyRecord は対象日の稼働実績を取り除く（AC-4-2）。
func (w *WorkMonth) DeleteDailyRecord(date Date) error {
	return nil
}
