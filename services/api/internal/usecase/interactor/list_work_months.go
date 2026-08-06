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
// 技術者識別子が省略されている場合にのみ認可を判定する（承認待ち一覧＝
// AC-8-10）: 未認証なら ErrUnauthenticated、認証済みでロールが Approver
// でなければ ErrNotApprover（この順）。技術者識別子が指定されている場合は
// 認可を判定しない（未認証でも成功＝AC-8-9）。弾いた場合は WorkMonthQuery
// を呼ばない。入力の条件はそのまま WorkMonthQuery へ渡し、総件数・件数・
// 開始位置は出力 DTO へそのまま載せる（数え直さない＝AC-7-17）。
func (i *ListWorkMonths) Execute(ctx context.Context, input port.ListWorkMonthsInput) {
	if input.EngineerID == "" {
		if !input.Actor.Authenticated {
			i.output.PresentError(port.ErrUnauthenticated)
			return
		}
		if input.Actor.Role != port.RoleApprover {
			i.output.PresentError(port.ErrNotApprover)
			return
		}
	}

	rows, total, err := i.query.List(ctx, port.WorkMonthQueryCondition{
		EngineerID: input.EngineerID,
		State:      input.State,
		Limit:      input.Limit,
		Offset:     input.Offset,
	})
	if err != nil {
		i.output.PresentError(err)
		return
	}

	// WorkMonthQueryRow と ListWorkMonthsOutputRow は用途が異なる別の型
	// （AC-6-7-d）だが、フィールドの型・並びが一致するため変換で写す。
	items := make([]port.ListWorkMonthsOutputRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, port.ListWorkMonthsOutputRow(row))
	}

	i.output.Present(port.ListWorkMonthsOutput{
		Items:  items,
		Total:  total,
		Limit:  input.Limit,
		Offset: input.Offset,
	})
}
