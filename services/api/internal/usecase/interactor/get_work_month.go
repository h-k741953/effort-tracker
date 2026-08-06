package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// GetWorkMonth は勤務月1件の取得（E-1）のユースケース（実装設計 AC-7-1・AC-7-15）。
type GetWorkMonth struct {
	workMonths port.WorkMonthRepository
	contracts  port.ContractRepository
	output     port.WorkMonthOutputPort
}

// NewGetWorkMonth は GetWorkMonth を組み立てる。
// Clock・WorkMonthQuery には依存しない（実装設計 AC-7-15）。
func NewGetWorkMonth(
	workMonths port.WorkMonthRepository,
	contracts port.ContractRepository,
	output port.WorkMonthOutputPort,
) *GetWorkMonth {
	return &GetWorkMonth{
		workMonths: workMonths,
		contracts:  contracts,
		output:     output,
	}
}

// Execute はユースケースを実行する（実装設計 AC-7-15）。
//
// テスト工程時点では未実装（TDD の Red を確認するための宣言のみ。
// docs/rules/development-process.md）。実装は次工程（implementer）が行う。
func (i *GetWorkMonth) Execute(ctx context.Context, input port.GetWorkMonthInput) {
}
