package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// DemoContract はデモの契約1件の値（docs/specs/demo-data-reset.md AC-15-2）。
// 契約期間の型に usecase/port の ContractPeriod を使わないのは、リセット用 cmd が
// usecase を import できないため（AC-12-11 ④）。
type DemoContract struct {
	ID              workmonth.ContractID
	DisplayName     string
	EngineerID      string
	SettlementRange workmonth.SettlementRange
	PeriodStart     workmonth.YearMonth
	// PeriodEnd が nil の契約は終了未定（終了の年・月がともに NULL＝AC-3-2）。
	PeriodEnd *workmonth.YearMonth
}

// DemoResetter はデモデータの日次リセットを1トランザクションで行う（AC-15）。
// usecase/port には載せない。リセット用 Lambda の cmd だけが呼ぶ（AC-15-1・AC-15-10）。
type DemoResetter struct {
	db DB
}

// NewDemoResetter は DemoResetter を構築する。
func NewDemoResetter(db DB) *DemoResetter {
	return &DemoResetter{db: db}
}

const demoContractEngineerSelectQuery = `
SELECT engineer_id FROM contracts WHERE id = $1
`

const demoContractInsertQuery = `
INSERT INTO contracts (
    id, display_name, engineer_id,
    lower_bound_hours, lower_bound_minutes, upper_bound_hours, upper_bound_minutes,
    period_start_year, period_start_month, period_end_year, period_end_month
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`

const demoContractPeriodUpdateQuery = `
UPDATE contracts
SET period_start_year = $2, period_start_month = $3, period_end_year = $4, period_end_month = $5
WHERE id = $1
`

// 勤務月と稼働実績の全行の破棄（AC-1-1・AC-1-2）。条件を持たず、引数も取らない。
const demoDailyRecordsDiscardQuery = `DELETE FROM daily_records`
const demoWorkMonthsDiscardQuery = `DELETE FROM work_months`

// Reset は契約の挿入または契約期間の書き換え → 勤務月と稼働実績の全行の破棄 →
// 各集約の保存を、ちょうど1つのトランザクションで行う（AC-15-3）。
// 失敗したらロールバックして以降の文を実行しない（AC-15-4）。
// 入力が不正なら何の文も実行せず（Begin も呼ばず）エラーを返す（AC-15-6〜15-8）。
// エラーの文言に SQL 文・取得した値を含めない（AC-15-11）。
func (r *DemoResetter) Reset(ctx context.Context, contracts []DemoContract, workMonths []*workmonth.WorkMonth) error {
	if err := validateDemoReset(contracts, workMonths); err != nil {
		return err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("demo reset: begin transaction: %w", err)
	}
	if err := r.resetWithinTx(ctx, tx, contracts, workMonths); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("demo reset: commit: %w", err)
	}
	return nil
}

func validateDemoReset(contracts []DemoContract, workMonths []*workmonth.WorkMonth) error {
	if len(workMonths) == 0 {
		return errors.New("demo reset: no work months given")
	}
	known := make(map[string]bool, len(contracts))
	for _, c := range contracts {
		known[c.ID.String()] = true
	}
	type key struct {
		contract    string
		year, month int
	}
	seen := make(map[key]bool, len(workMonths))
	for _, wm := range workMonths {
		id := wm.ContractID().String()
		if !known[id] {
			return errors.New("demo reset: work month belongs to a contract outside the demo contracts")
		}
		k := key{id, wm.YearMonth().Year(), wm.YearMonth().Month()}
		if seen[k] {
			return errors.New("demo reset: duplicate work month for the same contract and year-month")
		}
		seen[k] = true
	}
	return nil
}

func (r *DemoResetter) resetWithinTx(
	ctx context.Context, tx Tx, contracts []DemoContract, workMonths []*workmonth.WorkMonth,
) error {
	for _, c := range contracts {
		if err := writeDemoContract(ctx, tx, c); err != nil {
			return err
		}
	}
	if err := tx.Exec(ctx, demoDailyRecordsDiscardQuery); err != nil {
		return fmt.Errorf("demo reset: discard daily records: %w", err)
	}
	if err := tx.Exec(ctx, demoWorkMonthsDiscardQuery); err != nil {
		return fmt.Errorf("demo reset: discard work months: %w", err)
	}
	for _, wm := range workMonths {
		if err := writeWorkMonthWithinTx(ctx, tx, wm); err != nil {
			return fmt.Errorf("demo reset: save work month: %w", err)
		}
	}
	return nil
}

// writeDemoContract は契約が無ければ挿入し、あれば技術者識別子を照合して
// 契約期間の4列だけを書き換える（AC-6-2・AC-6-5・AC-6-7・AC-6-8）。
func writeDemoContract(ctx context.Context, tx Tx, c DemoContract) error {
	id := c.ID.String()
	rows, err := tx.Query(ctx, demoContractEngineerSelectQuery, id)
	if err != nil {
		return fmt.Errorf("demo reset: look up contract: %w", err)
	}
	exists := rows.Next()
	var engineerID string
	if exists {
		if err := rows.Scan(&engineerID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("demo reset: scan contract: %w", err)
		}
	}
	iterErr := rows.Err()
	_ = rows.Close()
	if iterErr != nil {
		return fmt.Errorf("demo reset: look up contract: %w", iterErr)
	}

	var endYear, endMonth *int
	if c.PeriodEnd != nil {
		endYear, endMonth = intPtr(c.PeriodEnd.Year()), intPtr(c.PeriodEnd.Month())
	}

	if !exists {
		lower, upper := c.SettlementRange.LowerBound(), c.SettlementRange.UpperBound()
		if err := tx.Exec(ctx, demoContractInsertQuery,
			id, c.DisplayName, c.EngineerID,
			lower.Hours(), lower.Minutes(), upper.Hours(), upper.Minutes(),
			c.PeriodStart.Year(), c.PeriodStart.Month(), endYear, endMonth,
		); err != nil {
			return fmt.Errorf("demo reset: insert contract: %w", err)
		}
		return nil
	}

	if engineerID != c.EngineerID {
		return errors.New("demo reset: existing demo contract belongs to a different engineer")
	}
	if err := tx.Exec(ctx, demoContractPeriodUpdateQuery,
		id, c.PeriodStart.Year(), c.PeriodStart.Month(), endYear, endMonth,
	); err != nil {
		return fmt.Errorf("demo reset: update contract period: %w", err)
	}
	return nil
}
