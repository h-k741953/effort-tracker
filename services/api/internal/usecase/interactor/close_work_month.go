package interactor

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// CloseWorkMonth は月次締め（UC2）のユースケース（実装設計 AC-7-1）。
type CloseWorkMonth struct {
	workMonths port.WorkMonthRepository
	contracts  port.ContractRepository
	output     port.WorkMonthOutputPort
}

// NewCloseWorkMonth は CloseWorkMonth を組み立てる。
// 出力ポート（presenter）はリクエストごとに生成して渡す（AC-7-6）。
// Clock には依存しない（締めに「当日」を要さない。実装設計 AC-7-10・AC-4-3）。
func NewCloseWorkMonth(
	workMonths port.WorkMonthRepository,
	contracts port.ContractRepository,
	output port.WorkMonthOutputPort,
) *CloseWorkMonth {
	return &CloseWorkMonth{
		workMonths: workMonths,
		contracts:  contracts,
		output:     output,
	}
}

// Execute はユースケースを実行する。戻り値は返さず、出力ポートを呼ぶ（AC-7-3）。
//
// 責務の順序は実装設計 AC-7-10 に従う。
// ①操作者の認証済み確認 → ②契約の取得 → ③勤務月の取得（未生成ならここで打ち切り、
// ErrWorkMonthNotFound。締めは生成契機ではない＝AC-7-9） → ④認可（本人のみ。
// ロールは問わない。AC-8-2） → ⑤ Close()（状態の検査と超過／不足の確定は集約が行う。
// AC-4-3） → ⑥保存 → ⑦出力ポート。**③が④より先**であるのは
// docs/specs/domain-api-http-contract.md AC-6-9 の順序（未生成かつ本人でない要求にも
// ErrWorkMonthNotFound が先に返る）と一致させるため。弾かれた要求では Save を呼ばない。
func (i *CloseWorkMonth) Execute(ctx context.Context, input port.CloseWorkMonthInput) {
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

	// ③ 勤務月の取得。未生成ならここで打ち切る（締めは生成契機ではない。AC-7-9）。
	target, err := i.workMonths.Find(ctx, input.ContractID, input.YearMonth)
	if err != nil {
		i.output.PresentError(err)
		return
	}

	// ④ 認可。本人のみが締められる。ロールは問わない（AC-8-2）。
	if input.Actor.ID != contract.EngineerID {
		i.output.PresentError(port.ErrNotOwner)
		return
	}

	// ⑤ 締め。状態の検査と超過／不足の確定は集約が行う（AC-4-3）。
	if err := target.Close(); err != nil {
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
