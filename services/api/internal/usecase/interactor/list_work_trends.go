package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// ListWorkTrends は稼働の推移（E-9）のユースケース（実装設計 AC-7-20）。
// 平均・確定した月の数・精算幅に収まったかは算出しない（契約 D-22 ⑥）。
type ListWorkTrends struct {
	query  port.WorkTrendQuery
	output port.ListWorkTrendsOutputPort
}

// NewListWorkTrends は ListWorkTrends を組み立てる。依存は WorkTrendQuery と
// 出力ポートの2つだけ。
func NewListWorkTrends(query port.WorkTrendQuery, output port.ListWorkTrendsOutputPort) *ListWorkTrends {
	return &ListWorkTrends{query: query, output: output}
}

// Execute はユースケースを実行する。
func (i *ListWorkTrends) Execute(ctx context.Context, input port.ListWorkTrendsInput) {
	if err := authorizeOwnerEngineer(input.Actor, input.EngineerID); err != nil {
		i.output.PresentError(err)
		return
	}

	months, err := i.query.ListByEngineer(ctx, input.EngineerID)
	if err != nil {
		i.output.PresentError(err)
		return
	}

	items := make([]port.WorkTrendOutputRow, 0, len(months))
	for _, m := range months {
		items = append(items, port.WorkTrendOutputRow{
			ContractID: m.ContractID().String(),
			YearMonth:  formatYearMonth(m.YearMonth()),
			State:      m.State().String(),
			// 集約が保持するスナップショット（契約の現在値を読まない）。
			SettlementRange: toSettlementRangeOutput(m.SettlementRange()),
			TotalHours:      toHours(m.TotalHours()),
			Excess:          excessOutput(m),
			Shortfall:       shortfallOutput(m),
		})
	}

	i.output.Present(port.ListWorkTrendsOutput{Items: items})
}
