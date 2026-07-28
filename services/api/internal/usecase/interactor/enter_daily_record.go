// Package interactor はユースケースの実装を置く。
//
// 依存の向きは docs/specs/workmonth-implementation-design.md AC-1-4 に従い、
// domain と usecase/port のみを import する。
package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 【スタブ】本ファイルは tester 工程が置いたシグネチャのみの骨組みであり、
// 振る舞いは実装していない（TDD の Red を踏むための最小構成）。
// 中身は implementer 工程が埋める。

// EnterDailyRecord は稼働実績の入力・編集のユースケース（AC-7-1）。
type EnterDailyRecord struct {
	workMonths port.WorkMonthRepository
	contracts  port.ContractRepository
	clock      port.Clock
	output     port.WorkMonthOutputPort
}

// NewEnterDailyRecord は EnterDailyRecord を組み立てる。
// 出力ポート（presenter）はリクエストごとに生成して渡す（AC-7-6）。
func NewEnterDailyRecord(
	workMonths port.WorkMonthRepository,
	contracts port.ContractRepository,
	clock port.Clock,
	output port.WorkMonthOutputPort,
) *EnterDailyRecord {
	return &EnterDailyRecord{
		workMonths: workMonths,
		contracts:  contracts,
		clock:      clock,
		output:     output,
	}
}

// Execute はユースケースを実行する。戻り値は返さず、出力ポートを呼ぶ（AC-7-3）。
func (i *EnterDailyRecord) Execute(ctx context.Context, input port.EnterDailyRecordInput) {
}
