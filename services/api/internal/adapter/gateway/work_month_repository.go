package gateway

import (
	"context"
	"fmt"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// WorkMonthRepository は port.WorkMonthRepository（AC-6-1）の実装。
// SQL の実行手段は自身が宣言した DB（AC-9-14）越しに受け取る。
type WorkMonthRepository struct {
	db DB
}

// NewWorkMonthRepository は WorkMonthRepository を構築する。
func NewWorkMonthRepository(db DB) *WorkMonthRepository {
	return &WorkMonthRepository{db: db}
}

// workMonthHeaderSelectQuery は勤務月のヘッダ行を取得する SQL 文（契約識別子・
// 年・月・精算幅下限（時・分）・精算幅上限（時・分）・状態・超過（時・分。
// NULL 許容）・不足（時・分。NULL 許容）の12列。AC-9-15-a）。
// SQL 文そのものの正しさは検査対象ではない（AC-13-18）。
const workMonthHeaderSelectQuery = `
SELECT contract_id, year, month, lower_bound_hours, lower_bound_minutes, upper_bound_hours, upper_bound_minutes,
       state, excess_hours, excess_minutes, shortfall_hours, shortfall_minutes
FROM work_months
WHERE contract_id = $1 AND year = $2 AND month = $3
`

// workMonthDailyRecordsSelectQuery は稼働実績の行を取得する SQL 文
// （年・月・日・稼働時間（時・分）の5列。AC-9-15-a）。
const workMonthDailyRecordsSelectQuery = `
SELECT year, month, day, hours, minutes
FROM daily_records
WHERE contract_id = $1 AND year = $2 AND month = $3
`

// workMonthHeaderUpsertQuery は勤務月のヘッダ行を書き込む SQL 文（AC-9-16）。
const workMonthHeaderUpsertQuery = `
INSERT INTO work_months (
    contract_id, year, month, lower_bound_hours, lower_bound_minutes, upper_bound_hours, upper_bound_minutes,
    state, excess_hours, excess_minutes, shortfall_hours, shortfall_minutes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (contract_id, year, month) DO UPDATE SET
    lower_bound_hours = EXCLUDED.lower_bound_hours,
    lower_bound_minutes = EXCLUDED.lower_bound_minutes,
    upper_bound_hours = EXCLUDED.upper_bound_hours,
    upper_bound_minutes = EXCLUDED.upper_bound_minutes,
    state = EXCLUDED.state,
    excess_hours = EXCLUDED.excess_hours,
    excess_minutes = EXCLUDED.excess_minutes,
    shortfall_hours = EXCLUDED.shortfall_hours,
    shortfall_minutes = EXCLUDED.shortfall_minutes
`

// workMonthDailyRecordsDeleteQuery は稼働実績の行を全削除する SQL 文
// （削除された対象日を残さないため、書き込み前に対象の勤務月分を消す。AC-9-16-b）。
const workMonthDailyRecordsDeleteQuery = `
DELETE FROM daily_records WHERE contract_id = $1 AND year = $2 AND month = $3
`

// workMonthDailyRecordInsertQuery は稼働実績を1件挿入する SQL 文（AC-9-16-c。
// 入力された稼働時間のみを書き、丸め値・総稼働時間の列は持たない）。
const workMonthDailyRecordInsertQuery = `
INSERT INTO daily_records (year, month, day, hours, minutes) VALUES ($1, $2, $3, $4, $5)
`

// Find は勤務月を1件取得し、workmonth.Reconstruct（AC-2-5）で集約へ組み立てる
// （AC-9-15）。行が無ければ port.ErrWorkMonthNotFound を返す（AC-9-15-e）。
// 「行が無い」以外のドライバ由来のエラーは変換せずそのまま返す（AC-9-19-a）。
// Reconstruct の失敗は握り潰さずそのまま返す（AC-9-15-d）。
func (r *WorkMonthRepository) Find(
	ctx context.Context, contractID workmonth.ContractID, yearMonth workmonth.YearMonth,
) (*workmonth.WorkMonth, error) {
	headerRows, err := r.db.Query(ctx, workMonthHeaderSelectQuery, contractID.String(), yearMonth.Year(), yearMonth.Month())
	if err != nil {
		return nil, err
	}

	if !headerRows.Next() {
		iterErr := headerRows.Err()
		_ = headerRows.Close()
		if iterErr != nil {
			return nil, iterErr
		}
		return nil, fmt.Errorf("%w: contract %s %04d-%02d", port.ErrWorkMonthNotFound, contractID.String(), yearMonth.Year(), yearMonth.Month())
	}

	var (
		rowContractID string
		rowYear       int
		rowMonth      int
		lowerH        int
		lowerM        int
		upperH        int
		upperM        int
		state         string
		excessH       *int
		excessM       *int
		shortfallH    *int
		shortfallM    *int
	)
	if err := headerRows.Scan(
		&rowContractID, &rowYear, &rowMonth, &lowerH, &lowerM, &upperH, &upperM,
		&state, &excessH, &excessM, &shortfallH, &shortfallM,
	); err != nil {
		_ = headerRows.Close()
		return nil, err
	}
	_ = headerRows.Close()

	lower, err := workmonth.NewWorkingHours(lowerH, lowerM)
	if err != nil {
		return nil, err
	}
	upper, err := workmonth.NewWorkingHours(upperH, upperM)
	if err != nil {
		return nil, err
	}
	settlementRange, err := workmonth.NewSettlementRange(lower, upper)
	if err != nil {
		return nil, err
	}

	excess, err := optionalWorkingHours(excessH, excessM)
	if err != nil {
		return nil, err
	}
	shortfall, err := optionalWorkingHours(shortfallH, shortfallM)
	if err != nil {
		return nil, err
	}

	recordRows, err := r.db.Query(ctx, workMonthDailyRecordsSelectQuery, contractID.String(), yearMonth.Year(), yearMonth.Month())
	if err != nil {
		return nil, err
	}
	defer func() { _ = recordRows.Close() }()

	records := make([]workmonth.DailyRecord, 0)
	for recordRows.Next() {
		var (
			year    int
			month   int
			day     int
			hours   int
			minutes int
		)
		if err := recordRows.Scan(&year, &month, &day, &hours, &minutes); err != nil {
			return nil, err
		}
		date, err := workmonth.NewDate(year, month, day)
		if err != nil {
			return nil, err
		}
		workingHours, err := workmonth.NewWorkingHours(hours, minutes)
		if err != nil {
			return nil, err
		}
		record, err := workmonth.NewDailyRecord(date, workingHours)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := recordRows.Err(); err != nil {
		return nil, err
	}

	return workmonth.Reconstruct(contractID, yearMonth, settlementRange, workmonth.State(state), records, excess, shortfall)
}

// optionalWorkingHours は NULL 許容の時・分の列を *workmonth.WorkingHours へ写す。
// 両方 nil なら未確定として nil を返す（AC-9-15-c）。
func optionalWorkingHours(hours, minutes *int) (*workmonth.WorkingHours, error) {
	if hours == nil || minutes == nil {
		return nil, nil
	}
	wh, err := workmonth.NewWorkingHours(*hours, *minutes)
	if err != nil {
		return nil, err
	}
	return &wh, nil
}

// hoursMinutesPtr は確定済みの稼働の量を NULL 許容の時・分の列の値へ写す。
// 未確定（ok=false）なら両方 nil（AC-9-16-d）。
func hoursMinutesPtr(h workmonth.WorkingHours, ok bool) (*int, *int) {
	if !ok {
		return nil, nil
	}
	hours := h.Hours()
	minutes := h.Minutes()
	return &hours, &minutes
}

// Save は勤務月と稼働実績を1トランザクションで書き込む（AC-9-16）。
// ヘッダ行の upsert → 稼働実績の全削除 → 現在保持する稼働実績の挿入の順に
// 書き込み、途中の失敗では取消し（Rollback）を呼び確定（Commit）を呼ばない
// （AC-9-16-a・AC-10-7）。
func (r *WorkMonthRepository) Save(ctx context.Context, target *workmonth.WorkMonth) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	if err := r.writeWithinTx(ctx, tx, target); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

// writeWithinTx は Save の書き込み本体。Tx を使ってヘッダ行・稼働実績の行を
// 書く。呼び出し順の取り決めは doubles_test.go の前書きを参照。
func (r *WorkMonthRepository) writeWithinTx(ctx context.Context, tx Tx, target *workmonth.WorkMonth) error {
	contractID := target.ContractID().String()
	year := target.YearMonth().Year()
	month := target.YearMonth().Month()

	lower := target.SettlementRange().LowerBound()
	upper := target.SettlementRange().UpperBound()
	excessH, excessM := hoursMinutesPtr(target.Excess())
	shortfallH, shortfallM := hoursMinutesPtr(target.Shortfall())

	if err := tx.Exec(ctx, workMonthHeaderUpsertQuery,
		contractID, year, month,
		lower.Hours(), lower.Minutes(), upper.Hours(), upper.Minutes(),
		string(target.State()),
		excessH, excessM, shortfallH, shortfallM,
	); err != nil {
		return err
	}

	if err := tx.Exec(ctx, workMonthDailyRecordsDeleteQuery, contractID, year, month); err != nil {
		return err
	}

	for _, record := range target.DailyRecords() {
		if err := tx.Exec(ctx, workMonthDailyRecordInsertQuery,
			record.Date().Year(), record.Date().Month(), record.Date().Day(),
			record.WorkingHours().Hours(), record.WorkingHours().Minutes(),
		); err != nil {
			return err
		}
	}

	return nil
}

var _ port.WorkMonthRepository = (*WorkMonthRepository)(nil)
