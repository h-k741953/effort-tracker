package interactor

import (
	"context"
	"errors"

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
// 認証・認可のいずれも判定しない（ゲストでも成功し、操作者で絞らない＝
// AC-8-8）。順序は ①契約の取得（不在は ErrContractNotFound） → ②勤務月の
// 取得（生成済みなら集約と Contract から出力 DTO を組み立て、
// ErrWorkMonthNotFound なら空の下書き相当の出力） → ③出力ポート。
// Save は呼ばない（参照は生成契機にしない＝AC-7-8）。ErrWorkMonthNotFound
// 以外のエラーは空の出力へ変換せずそのまま PresentError へ渡す（AC-11-11）。
func (i *GetWorkMonth) Execute(ctx context.Context, input port.GetWorkMonthInput) {
	contract, err := i.contracts.Find(ctx, input.ContractID)
	if err != nil {
		i.output.PresentError(err)
		return
	}

	target, err := i.workMonths.Find(ctx, input.ContractID, input.YearMonth)
	switch {
	case err == nil:
		i.output.Present(newWorkMonthOutput(target, contract))
	case errors.Is(err, port.ErrWorkMonthNotFound):
		i.output.Present(newEmptyWorkMonthOutput(contract, input.YearMonth))
	default:
		i.output.PresentError(err)
	}
}
