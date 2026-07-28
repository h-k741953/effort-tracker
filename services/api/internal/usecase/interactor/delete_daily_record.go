package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 【スタブ】本ファイルは tester 工程が置いたシグネチャのみの骨組みであり、
// 振る舞いは実装していない（TDD の Red を踏むための最小構成）。
// 中身は implementer 工程が埋める。

// DeleteDailyRecord は稼働実績の削除のユースケース（AC-7-1）。
type DeleteDailyRecord struct {
	workMonths port.WorkMonthRepository
	contracts  port.ContractRepository
	output     port.WorkMonthOutputPort
}

// NewDeleteDailyRecord は DeleteDailyRecord を組み立てる。
// 出力ポート（presenter）はリクエストごとに生成して渡す（AC-7-6）。
func NewDeleteDailyRecord(
	workMonths port.WorkMonthRepository,
	contracts port.ContractRepository,
	output port.WorkMonthOutputPort,
) *DeleteDailyRecord {
	return &DeleteDailyRecord{
		workMonths: workMonths,
		contracts:  contracts,
		output:     output,
	}
}

// Execute はユースケースを実行する。戻り値は返さず、出力ポートを呼ぶ（AC-7-3）。
func (i *DeleteDailyRecord) Execute(ctx context.Context, input port.DeleteDailyRecordInput) {
}
