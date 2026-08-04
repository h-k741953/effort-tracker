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

// Execute はユースケースを実行する。戻り値は返さず、出力ポートを呼ぶ（AC-7-3）。
//
// 責務の順序は実装設計 AC-7-13 に従う。
// ①操作者の認証済み確認 → ②契約の取得 → ③勤務月の取得（未生成ならここで打ち切り、
// ErrWorkMonthNotFound） → ④認可（承認者ロール → 自己承認の2段。AC-8-11） →
// ⑤ Approve()（状態の検査は集約が行う。AC-4-4） → ⑥保存 → ⑦出力ポート。
// 認可は2段で、①承認者ロールを持つか（ErrNotApprover）→②本人でないか
// （自己承認は ErrSelfApproval。approval.md AC-4・D-4）の順に判定する
// （両方に該当する操作者には①の ErrNotApprover が返る。AC-8-11）。
func (i *ApproveWorkMonth) Execute(ctx context.Context, input port.ApproveWorkMonthInput) {
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

	// ④ 認可。承認者ロール → 自己承認の2段（AC-8-11、approval.md AC-3・AC-4）。
	if input.Actor.Role != port.RoleApprover {
		i.output.PresentError(port.ErrNotApprover)
		return
	}
	if input.Actor.ID == contract.EngineerID {
		i.output.PresentError(port.ErrSelfApproval)
		return
	}

	// ⑤ 承認。状態の検査は集約が行う（AC-4-4）。
	if err := target.Approve(); err != nil {
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
