// Package interactor はユースケースの実装を置く。
//
// 依存の向きは docs/specs/workmonth-implementation-design.md AC-1-4 に従い、
// domain と usecase/port のみを import する。
package interactor

import (
	"context"
	"errors"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

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
//
// 責務の順序は AC-7-7 に従う。
// ①認証 → ②対象の取得（契約 → 勤務月） → ③認可 → ④状態 → ⑤業務バリデーション
// → ⑥保存 → ⑦出力。判定順序は docs/specs/domain-api-http-contract.md AC-9 と一致させる。
func (i *EnterDailyRecord) Execute(ctx context.Context, input port.EnterDailyRecordInput) {
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

	target, err := i.workMonths.Find(ctx, input.ContractID, input.YearMonth)
	switch {
	case err == nil:
		// 既存の勤務月へ追加する。再生成しない（docs/specs/daily-record-entry.md AC-1-4）。
	case errors.Is(err, port.ErrWorkMonthNotFound):
		// 未生成なら暗黙生成する（同 AC-1-1・D-6。実装設計 AC-7-9）。
		// 精算幅はこの時点の契約から複写する（同 AC-1-2）。
		target, err = workmonth.New(input.ContractID, input.YearMonth, contract.SettlementRange)
		if err != nil {
			i.output.PresentError(err)
			return
		}
	default:
		i.output.PresentError(err)
		return
	}

	// ③ 認可。本人のみが入力・編集できる。ロールは問わない（AC-8-1）。
	if input.Actor.ID != contract.EngineerID {
		i.output.PresentError(port.ErrNotOwner)
		return
	}

	// ④ 状態。値域の検査より先に行う（判定順序 AC-9）。
	if err := target.EnsureEditable(); err != nil {
		i.output.PresentError(err)
		return
	}

	// ⑤ 業務バリデーション（値域 → 当該月外・未来日は集約が判定する）。
	hours, err := workmonth.NewWorkingHours(input.Hours, input.Minutes)
	if err != nil {
		i.output.PresentError(err)
		return
	}
	record, err := workmonth.NewDailyRecord(input.Date, hours)
	if err != nil {
		i.output.PresentError(err)
		return
	}
	// 「当日」は Clock から取る。JST への変換は driver の責務（AC-6-5・D-5）。
	if err := target.EnterDailyRecord(record, i.clock.Today()); err != nil {
		i.output.PresentError(err)
		return
	}

	// ⑥ 保存。弾かれた入力では Save を呼ばない（部分的な更新を残さない。AC-9-4）。
	if err := i.workMonths.Save(ctx, target); err != nil {
		i.output.PresentError(err)
		return
	}

	// ⑦ 更新後の勤務月を出力ポートへ渡す（AC-7-5）。
	i.output.Present(newWorkMonthOutput(target, contract))
}
