package port

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// 本ファイルは契約一覧（E-8）の参照ポート ContractListQuery の具体形を置く
// （実装設計 AC-6-8・AC-6-10）。契約期間は契約一覧のリードモデルにだけ持たせ、
// port.Contract（AC-6-3）には足さない（決定20）。

// ContractPeriod は契約期間（ユビキタス言語「契約期間」ContractPeriod）。
type ContractPeriod struct {
	// Start は開始の年月。
	Start workmonth.YearMonth
	// End は終了の年月。nil は終了の年月が定まらない。
	End *workmonth.YearMonth
}

// ContractListRow は契約一覧の行のリードモデル。
type ContractListRow struct {
	ContractID  workmonth.ContractID
	DisplayName string
	// SettlementRange は契約がいま定める精算幅。
	SettlementRange workmonth.SettlementRange
	Period          ContractPeriod
	// WorkMonthState は指定した年月の勤務月の状態。nil は未生成。
	WorkMonthState *string
}

// ContractListQuery は契約一覧の参照ポート。ListContracts だけが使う。
type ContractListQuery interface {
	ListByEngineer(ctx context.Context, engineerID string, yearMonth workmonth.YearMonth) ([]ContractListRow, error)
}
