package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// authorizeOwnerEngineer は「Engineer かつ本人」を要求する（実装設計 AC-8-12）。
// 未認証は ErrUnauthenticated、ロールが Engineer でない／識別子不一致は ErrNotOwner。
func authorizeOwnerEngineer(actor port.Actor, engineerID string) error {
	if !actor.Authenticated {
		return port.ErrUnauthenticated
	}
	if actor.Role != port.RoleEngineer || actor.ID != engineerID {
		return port.ErrNotOwner
	}
	return nil
}

// ListContracts は契約一覧（E-8）のユースケース（実装設計 AC-7-19）。
type ListContracts struct {
	query  port.ContractListQuery
	output port.ListContractsOutputPort
}

// NewListContracts は ListContracts を組み立てる。依存は ContractListQuery と
// 出力ポートの2つだけ。
func NewListContracts(query port.ContractListQuery, output port.ListContractsOutputPort) *ListContracts {
	return &ListContracts{query: query, output: output}
}

// Execute はユースケースを実行する。勤務月は生成しない。
func (i *ListContracts) Execute(ctx context.Context, input port.ListContractsInput) {
	if err := authorizeOwnerEngineer(input.Actor, input.EngineerID); err != nil {
		i.output.PresentError(err)
		return
	}

	rows, err := i.query.ListByEngineer(ctx, input.EngineerID, input.YearMonth)
	if err != nil {
		i.output.PresentError(err)
		return
	}

	requested := formatYearMonth(input.YearMonth)
	items := make([]port.ContractOutputRow, 0, len(rows))
	for _, row := range rows {
		state := workmonth.StateDraft.String()
		generated := false
		if row.WorkMonthState != nil {
			state = *row.WorkMonthState
			generated = true
		}

		var end *string
		if row.Period.End != nil {
			s := formatYearMonth(*row.Period.End)
			end = &s
		}

		items = append(items, port.ContractOutputRow{
			ContractID:          row.ContractID.String(),
			ContractDisplayName: row.DisplayName,
			SettlementRange:     toSettlementRangeOutput(row.SettlementRange),
			ContractPeriod: port.ContractPeriodOutput{
				Start: formatYearMonth(row.Period.Start),
				End:   end,
			},
			WorkMonth: port.ContractWorkMonthOutput{
				YearMonth: requested,
				State:     state,
				Generated: generated,
			},
		})
	}

	i.output.Present(port.ListContractsOutput{Items: items})
}
