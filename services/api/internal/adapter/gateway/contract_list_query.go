package gateway

import (
	"context"
	"fmt"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// ContractListQuery は port.ContractListQuery（AC-6-10）の実装（AC-9-20）。
// 契約期間の列を読むのはここだけで、ContractRepository.Find は読まない
// （E-1〜E-7 の経路は契約期間の破損で失敗しない。AC-9-20-h）。
type ContractListQuery struct {
	db DB
}

// NewContractListQuery は ContractListQuery を構築する。
func NewContractListQuery(db DB) *ContractListQuery {
	return &ContractListQuery{db: db}
}

// contractListSelectQuery は契約一覧の行を取得する SQL 文（AC-9-20-c の11列）。
// 勤務月とは外部結合し、勤務月の有無・契約期間で契約を除外しない（AC-9-20-b）。
// 並びは契約識別子の昇順を ORDER BY で与える。
// SQL 文そのものの正しさは検査対象ではない（AC-13-18）。
const contractListSelectQuery = `
SELECT c.id, c.display_name,
       c.lower_bound_hours, c.lower_bound_minutes, c.upper_bound_hours, c.upper_bound_minutes,
       c.period_start_year, c.period_start_month, c.period_end_year, c.period_end_month,
       wm.state
FROM contracts c
LEFT JOIN work_months wm ON wm.contract_id = c.id AND wm.year = $2 AND wm.month = $3
WHERE c.engineer_id = $1
ORDER BY c.id ASC
`

// ListByEngineer は技術者の契約の行を契約識別子の昇順で返す（AC-9-20）。
// 1行でも破損していれば最初のエラーで打ち切り、一部の行を返さない。
func (q *ContractListQuery) ListByEngineer(
	ctx context.Context, engineerID string, yearMonth workmonth.YearMonth,
) ([]port.ContractListRow, error) {
	rows, err := q.db.Query(ctx, contractListSelectQuery, engineerID, yearMonth.Year(), yearMonth.Month())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]port.ContractListRow, 0)
	for rows.Next() {
		row, err := scanContractListRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// scanContractListRow は契約一覧の1行を判定の順（AC-9-20-d）に構築する。
func scanContractListRow(rows Rows) (port.ContractListRow, error) {
	var (
		id                    string
		displayName           string
		lowerH, lowerM        int
		upperH, upperM        int
		startYear, startMonth *int
		endYear, endMonth     *int
		workMonthState        *string
	)
	if err := rows.Scan(
		&id, &displayName, &lowerH, &lowerM, &upperH, &upperM,
		&startYear, &startMonth, &endYear, &endMonth, &workMonthState,
	); err != nil {
		return port.ContractListRow{}, err
	}

	// (i) 開始の年月を欠く行。
	if startYear == nil || startMonth == nil {
		return port.ContractListRow{}, fmt.Errorf("%w: contract row lacks period start", workmonth.ErrInvalidValue)
	}

	// (ii) 終了は2つとも nil（終了未定）か、2つとも非 nil のいずれか。
	if (endYear == nil) != (endMonth == nil) {
		return port.ContractListRow{}, fmt.Errorf("%w: contract row has a partial period end", workmonth.ErrInvalidValue)
	}

	// (iii) 開始・終了の年月を構築する。
	start, err := workmonth.NewYearMonth(*startYear, *startMonth)
	if err != nil {
		return port.ContractListRow{}, err
	}
	var end *workmonth.YearMonth
	if endYear != nil {
		e, err := workmonth.NewYearMonth(*endYear, *endMonth)
		if err != nil {
			return port.ContractListRow{}, err
		}
		end = &e

		// (iv) 開始が終了より後は破損（開始＝終了は通す）。
		if start.Year() > e.Year() || (start.Year() == e.Year() && start.Month() > e.Month()) {
			return port.ContractListRow{}, fmt.Errorf("%w: contract period start is after end", workmonth.ErrInvalidValue)
		}
	}

	// (v) 契約識別子・精算幅を構築する。
	contractID, err := workmonth.NewContractID(id)
	if err != nil {
		return port.ContractListRow{}, err
	}
	lower, err := workmonth.NewWorkingHours(lowerH, lowerM)
	if err != nil {
		return port.ContractListRow{}, err
	}
	upper, err := workmonth.NewWorkingHours(upperH, upperM)
	if err != nil {
		return port.ContractListRow{}, err
	}
	settlementRange, err := workmonth.NewSettlementRange(lower, upper)
	if err != nil {
		return port.ContractListRow{}, err
	}

	// (vi) 勤務月の状態は検査せず素通しする。
	return port.ContractListRow{
		ContractID:      contractID,
		DisplayName:     displayName,
		SettlementRange: settlementRange,
		Period:          port.ContractPeriod{Start: start, End: end},
		WorkMonthState:  workMonthState,
	}, nil
}

var _ port.ContractListQuery = (*ContractListQuery)(nil)
