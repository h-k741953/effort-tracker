package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// ApproveWorkMonth は承認（UC3）のユースケース（実装設計 AC-7-1）。
type ApproveWorkMonth struct {
	workMonths port.WorkMonthRepository
	contracts  port.ContractRepository
	output     port.WorkMonthOutputPort
}

// NewApproveWorkMonth は ApproveWorkMonth を組み立てる。
// 出力ポート（presenter）はリクエストごとに生成して渡す（AC-7-6）。
// Clock には依存しない（承認に「当日」を要さない。実装設計 AC-7-12・AC-4-4）。
func NewApproveWorkMonth(
	workMonths port.WorkMonthRepository,
	contracts port.ContractRepository,
	output port.WorkMonthOutputPort,
) *ApproveWorkMonth {
	return &ApproveWorkMonth{
		workMonths: workMonths,
		contracts:  contracts,
		output:     output,
	}
}

// Execute はユースケースを実行する。
//
// TODO(implementer): テスト工程が置いたスタブ（本体未実装）。
// docs/specs/workmonth-implementation-design.md AC-7-13・AC-7-14 に従い、
// ①操作者の認証済み確認 → ②契約の取得 → ③勤務月の取得（未生成なら
// ErrWorkMonthNotFound） → ④認可（承認者ロール → 自己承認の2段。AC-8-11） →
// ⑤ Approve()（AC-4-4） → ⑥保存 → ⑦出力ポート、の順序で実装する。
func (i *ApproveWorkMonth) Execute(ctx context.Context, input port.ApproveWorkMonthInput) {
}
