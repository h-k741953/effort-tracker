package interactor

import (
	"fmt"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 出力 DTO の組み立て。domain の型をそのまま外へ出さず、表示に依らない素の値へ落とす
// （docs/specs/workmonth-implementation-design.md AC-7-4）。
// JSON の形は adapter/presenter が決めるため、ここでは決めない。

// newWorkMonthOutput は生成済みの勤務月から出力 DTO を組み立てる。
//
// 契約表示名は集約が持たないため、リードモデル（port.Contract）から取る（AC-2-2・AC-6-3）。
// 精算幅は生成時に複写した集約側の値を使い、契約の現在値で上書きしない
// （docs/specs/daily-record-entry.md AC-1-2）。
func newWorkMonthOutput(target *workmonth.WorkMonth, contract port.Contract) port.WorkMonthOutput {
	records := target.DailyRecords()
	outputs := make([]port.DailyRecordOutput, 0, len(records))
	for _, record := range records {
		outputs = append(outputs, port.DailyRecordOutput{
			Date:                formatDate(record.Date()),
			WorkingHours:        toHours(record.WorkingHours()),
			RoundedWorkingHours: toHours(record.WorkingHours().TruncateTo15Minutes()),
		})
	}

	return port.WorkMonthOutput{
		ContractID:          target.ContractID().String(),
		ContractDisplayName: contract.DisplayName,
		YearMonth:           formatYearMonth(target.YearMonth()),
		State:               target.State().String(),
		Generated:           true,
		SettlementRange:     toSettlementRangeOutput(target.SettlementRange()),
		TotalHours:          toHours(target.TotalHours()),
		// 超過／不足は締め時に確定する（AC-5-2・AC-5-3）。UC1 の経路では常に未確定。
		Excess:       nil,
		Shortfall:    nil,
		DailyRecords: outputs,
	}
}

// newEmptyWorkMonthOutput は未生成の年月の出力 DTO を組み立てる（AC-7-8・AC-7-9）。
//
// 勤務月を生成しないため、精算幅は契約が現在定める値を返す
// （docs/specs/domain-api-http-contract.md AC-5-3・AC-2-2）。
func newEmptyWorkMonthOutput(contract port.Contract, yearMonth workmonth.YearMonth) port.WorkMonthOutput {
	return port.WorkMonthOutput{
		ContractID:          contract.ID.String(),
		ContractDisplayName: contract.DisplayName,
		YearMonth:           formatYearMonth(yearMonth),
		State:               workmonth.StateDraft.String(),
		Generated:           false,
		SettlementRange:     toSettlementRangeOutput(contract.SettlementRange),
		TotalHours:          port.Hours{},
		Excess:              nil,
		Shortfall:           nil,
		DailyRecords:        []port.DailyRecordOutput{},
	}
}

// toSettlementRangeOutput は精算幅を出力 DTO へ落とす。
func toSettlementRangeOutput(settlement workmonth.SettlementRange) port.SettlementRangeOutput {
	return port.SettlementRangeOutput{
		LowerBound: toHours(settlement.LowerBound()),
		UpperBound: toHours(settlement.UpperBound()),
	}
}

// toHours は稼働の量を時・分の素の値へ落とす。
func toHours(w workmonth.WorkingHours) port.Hours {
	return port.Hours{Hours: w.Hours(), Minutes: w.Minutes()}
}

// formatYearMonth は年月を YYYY-MM で表す。
func formatYearMonth(yearMonth workmonth.YearMonth) string {
	return fmt.Sprintf("%04d-%02d", yearMonth.Year(), yearMonth.Month())
}

// formatDate は暦日を YYYY-MM-DD で表す。
func formatDate(date workmonth.Date) string {
	return fmt.Sprintf("%04d-%02d-%02d", date.Year(), date.Month(), date.Day())
}
