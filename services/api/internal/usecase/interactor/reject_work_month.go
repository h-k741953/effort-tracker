package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// RejectWorkMonth は差戻し（UC3）のユースケース（実装設計 AC-7-1）。
type RejectWorkMonth struct {
	workMonths port.WorkMonthRepository
	contracts  port.ContractRepository
	output     port.WorkMonthOutputPort
}

// NewRejectWorkMonth は RejectWorkMonth を組み立てる。
// 出力ポート（presenter）はリクエストごとに生成して渡す（AC-7-6）。
// Clock には依存しない（差戻しに「当日」を要さない。実装設計 AC-7-12・AC-4-5）。
func NewRejectWorkMonth(
	workMonths port.WorkMonthRepository,
	contracts port.ContractRepository,
	output port.WorkMonthOutputPort,
) *RejectWorkMonth {
	return &RejectWorkMonth{
		workMonths: workMonths,
		contracts:  contracts,
		output:     output,
	}
}

// Execute はユースケースを実行する。戻り値は返さず、出力ポートを呼ぶ（AC-7-3）。
//
// 責務の順序は ApproveWorkMonth と完全に同一（実装設計 AC-7-13）。
// ①操作者の認証済み確認 → ②契約の取得 → ③勤務月の取得（未生成ならここで打ち切り、
// ErrWorkMonthNotFound） → ④認可（承認者ロール → 自己承認の2段。AC-8-11。差戻しも
// 自己承認と同様に扱う。approval.md D-4） → ⑤ Reject()（状態の検査は集約が行う。
// AC-4-5） → ⑥保存 → ⑦出力ポート。
func (i *RejectWorkMonth) Execute(ctx context.Context, input port.RejectWorkMonthInput) {
	// ① 操作者の認証済み確認（AC-8-7。ゲストは未認証の Actor として表れる）。
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

	// ③ 勤務月の取得。未生成ならここで打ち切る（AC-7-9）。
	target, err := i.workMonths.Find(ctx, input.ContractID, input.YearMonth)
	if err != nil {
		i.output.PresentError(err)
		return
	}

	// ④ 認可。承認者ロール → 自己承認の2段（AC-8-11、approval.md AC-3・AC-4・D-4）。
	if input.Actor.Role != port.RoleApprover {
		i.output.PresentError(port.ErrNotApprover)
		return
	}
	if input.Actor.ID == contract.EngineerID {
		i.output.PresentError(port.ErrSelfApproval)
		return
	}

	// ⑤ 差戻し。状態の検査は集約が行う（AC-4-5）。
	if err := target.Reject(); err != nil {
		i.output.PresentError(err)
		return
	}

	// ⑥ 保存。弾かれた要求では Save を呼ばない（AC-9-4）。
	if err := i.workMonths.Save(ctx, target); err != nil {
		i.output.PresentError(err)
		return
	}

	// ⑦ 更新後の勤務月を出力ポートへ渡す（AC-7-5）。
	i.output.Present(newWorkMonthOutput(target, contract))
}
