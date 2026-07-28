package interactor

import (
	"context"
	"errors"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

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
//
// 責務の順序は AC-7-7 に従う（判定順序は docs/specs/domain-api-http-contract.md AC-9）。
// 未生成の年月への削除は勤務月を生成せず、空の表現を返して成功とする
// （入力以外の生成契機を設けない。docs/specs/daily-record-entry.md AC-1-5・D-6。
// 実装設計 AC-7-9／HTTP 契約 AC-5-3）。
func (i *DeleteDailyRecord) Execute(ctx context.Context, input port.DeleteDailyRecordInput) {
	// ① 操作者の認証済み確認（AC-8-7）。
	if !input.Actor.Authenticated {
		i.output.PresentError(port.ErrUnauthenticated)
		return
	}

	// ② 対象の取得。契約は認可の判定材料でもある（AC-8-6）。
	contract, err := i.contracts.Find(ctx, input.ContractID)
	if err != nil {
		i.output.PresentError(err)
		return
	}

	target, err := i.workMonths.Find(ctx, input.ContractID, input.YearMonth)
	generated := true
	switch {
	case err == nil:
	case errors.Is(err, port.ErrWorkMonthNotFound):
		generated = false
	default:
		i.output.PresentError(err)
		return
	}

	// ③ 認可。本人のみが削除できる。ロールは問わない（AC-8-1）。
	// 未生成でも認可は判定する（本人以外に成功を返さない。HTTP 契約 AC-5-4）。
	if input.Actor.ID != contract.EngineerID {
		i.output.PresentError(port.ErrNotOwner)
		return
	}

	if !generated {
		// 未生成でも、対象日が当該年月に属さない場合は no-op（AC-5-3）より優先して弾く
		// （HTTP 契約 AC-5-8・AC-5-9・D-13／実装設計 AC-7-9）。
		if !input.YearMonth.Contains(input.Date) {
			i.output.PresentError(workmonth.ErrDateOutOfMonth)
			return
		}
		i.output.Present(newEmptyWorkMonthOutput(contract, input.YearMonth))
		return
	}

	// ④ 状態（Draft 以外は弾く。AC-5-2・AC-5-3）。
	// レコードの無い日への削除は成功として扱う（同 D-5。集約が判定する）。
	if err := target.DeleteDailyRecord(input.Date); err != nil {
		i.output.PresentError(err)
		return
	}

	// ⑤ 保存。
	if err := i.workMonths.Save(ctx, target); err != nil {
		i.output.PresentError(err)
		return
	}

	// ⑥ 更新後の勤務月を出力ポートへ渡す（AC-7-5）。
	i.output.Present(newWorkMonthOutput(target, contract))
}
