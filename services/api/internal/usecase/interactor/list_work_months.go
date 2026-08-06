package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// ListWorkMonths は勤務月一覧（E-2）のユースケース（実装設計 AC-7-1・AC-7-16）。
type ListWorkMonths struct {
	query  port.WorkMonthQuery
	output port.ListWorkMonthsOutputPort
}

// NewListWorkMonths は ListWorkMonths を組み立てる。依存は WorkMonthQuery と
// 一覧の出力ポートの2つだけ（ContractRepository・WorkMonthRepository・Clock
// には依存しない。実装設計 AC-7-16）。
func NewListWorkMonths(query port.WorkMonthQuery, output port.ListWorkMonthsOutputPort) *ListWorkMonths {
	return &ListWorkMonths{query: query, output: output}
}

// Execute はユースケースを実行する（実装設計 AC-7-16）。
//
// テスト工程時点では未実装（TDD の Red を確認するための宣言のみ）。実装は
// 次工程（implementer）が行う。
func (i *ListWorkMonths) Execute(ctx context.Context, input port.ListWorkMonthsInput) {
}
