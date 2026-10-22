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
// NULL 許容）・不足（時・分。NULL 許容）の12列。AC-9-15 の勤務月の行で、並びと型は AC-9-21-d が列挙する）。
// SQL 文そのものの正しさは検査対象ではない（AC-13-18）。
const workMonthHeaderSelectQuery = `
SELECT contract_id, year, month, lower_bound_hours, lower_bound_minutes, upper_bound_hours, upper_bound_minutes,
       state, excess_hours, excess_minutes, shortfall_hours, shortfall_minutes
FROM work_months
WHERE contract_id = $1 AND year = $2 AND month = $3
`

// workMonthDailyRecordsSelectQuery は稼働実績の行を取得する SQL 文
// （年・月・日・開始時刻・終了時刻・休憩開始時刻・休憩終了時刻（いずれも時・分）の11列。
// AC-9-15-i。稼働時間の列は読まない）。
const workMonthDailyRecordsSelectQuery = `
SELECT year, month, day,
       start_hour, start_minute, end_hour, end_minute,
       break_start_hour, break_start_minute, break_end_hour, break_end_minute
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

// workMonthDailyRecordInsertQuery は稼働実績を1件挿入する SQL 文（AC-9-16-c・AC-9-16-h。
// 保存する時刻のみを書き、稼働時間・丸め値・総稼働時間の列は持たない。
// 休憩なしは休憩の4値をすべて NULL で書く）。
const workMonthDailyRecordInsertQuery = `
INSERT INTO daily_records (
    contract_id, year, month, day,
    start_hour, start_minute, end_hour, end_minute,
    break_start_hour, break_start_minute, break_end_hour, break_end_minute
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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

	row, err := scanWorkMonthHeaderRow(headerRows)
	_ = headerRows.Close()
	if err != nil {
		return nil, err
	}
	settlementRange, excess, shortfall, err := row.values()
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
		record, err := scanDailyRecord(recordRows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := recordRows.Err(); err != nil {
		return nil, err
	}

	return workmonth.Reconstruct(contractID, yearMonth, settlementRange, workmonth.State(row.state), records, excess, shortfall)
}

// workMonthHeaderRow は勤務月の行（12列。AC-9-15。並びと型は AC-9-21-d）を Scan した生の値。
type workMonthHeaderRow struct {
	contractID     string
	year, month    int
	lowerH, lowerM int
	upperH, upperM int
	state          string
	excessH        *int
	excessM        *int
	shortfallH     *int
	shortfallM     *int
}

// scanWorkMonthHeaderRow は勤務月の行（12列）を Scan する。
func scanWorkMonthHeaderRow(rows Rows) (workMonthHeaderRow, error) {
	var r workMonthHeaderRow
	if err := rows.Scan(
		&r.contractID, &r.year, &r.month, &r.lowerH, &r.lowerM, &r.upperH, &r.upperM,
		&r.state, &r.excessH, &r.excessM, &r.shortfallH, &r.shortfallM,
	); err != nil {
		return workMonthHeaderRow{}, err
	}
	return r, nil
}

// values は行から精算幅・超過・不足を組み立てる（下限 → 上限 → 精算幅 → 超過 → 不足の順）。
func (r workMonthHeaderRow) values() (workmonth.SettlementRange, *workmonth.WorkingHours, *workmonth.WorkingHours, error) {
	lower, err := workmonth.NewWorkingHours(r.lowerH, r.lowerM)
	if err != nil {
		return workmonth.SettlementRange{}, nil, nil, err
	}
	upper, err := workmonth.NewWorkingHours(r.upperH, r.upperM)
	if err != nil {
		return workmonth.SettlementRange{}, nil, nil, err
	}
	settlementRange, err := workmonth.NewSettlementRange(lower, upper)
	if err != nil {
		return workmonth.SettlementRange{}, nil, nil, err
	}
	excess, err := optionalWorkingHours(r.excessH, r.excessM)
	if err != nil {
		return workmonth.SettlementRange{}, nil, nil, err
	}
	shortfall, err := optionalWorkingHours(r.shortfallH, r.shortfallM)
	if err != nil {
		return workmonth.SettlementRange{}, nil, nil, err
	}
	return settlementRange, excess, shortfall, nil
}

// scanDailyRecord は稼働実績の1行（11列。AC-9-15-i）を DailyRecord へ構築する。
// 判定の順は AC-9-15-i: (i) 開始・終了の欠落 → (ii) 休憩の4値の揃い → (iii) 構築。
// 構築の失敗は利用者向けの番兵ではなく ErrInvalidValue として返す（AC-9-15-h）。
//
// leading は先頭の列へ渡す Scan の宛先（稼働の推移の2回目の Query が先頭に持つ
// 契約識別子の列。AC-9-21-e）。通常の11列だけの走査では渡さない。
func scanDailyRecord(rows Rows, leading ...any) (workmonth.DailyRecord, error) {
	var (
		year, month, day           int
		startH, startM, endH, endM *int
		breakStartH, breakStartM   *int
		breakEndH, breakEndM       *int
	)
	dest := append(append([]any{}, leading...),
		&year, &month, &day, &startH, &startM, &endH, &endM,
		&breakStartH, &breakStartM, &breakEndH, &breakEndM,
	)
	if err := rows.Scan(dest...); err != nil {
		return workmonth.DailyRecord{}, err
	}
	if startH == nil || startM == nil || endH == nil || endM == nil {
		return workmonth.DailyRecord{}, fmt.Errorf("%w: daily record row lacks start or end time", workmonth.ErrInvalidValue)
	}
	date, err := workmonth.NewDate(year, month, day)
	if err != nil {
		return workmonth.DailyRecord{}, err
	}
	startTime, err := corruptedAsInvalid(workmonth.NewTimeOfDay(*startH, *startM))
	if err != nil {
		return workmonth.DailyRecord{}, err
	}
	endTime, err := corruptedAsInvalid(workmonth.NewTimeOfDay(*endH, *endM))
	if err != nil {
		return workmonth.DailyRecord{}, err
	}

	breakNils := countNil(breakStartH, breakStartM, breakEndH, breakEndM)
	var brk *workmonth.Break
	switch breakNils {
	case 4:
		// 休憩なし。
	case 0:
		breakStart, err := corruptedAsInvalid(workmonth.NewTimeOfDay(*breakStartH, *breakStartM))
		if err != nil {
			return workmonth.DailyRecord{}, err
		}
		breakEnd, err := corruptedAsInvalid(workmonth.NewTimeOfDay(*breakEndH, *breakEndM))
		if err != nil {
			return workmonth.DailyRecord{}, err
		}
		b, err := corruptedAsInvalid(workmonth.NewBreak(breakStart, breakEnd))
		if err != nil {
			return workmonth.DailyRecord{}, err
		}
		brk = &b
	default:
		return workmonth.DailyRecord{}, fmt.Errorf("%w: daily record row has a partial break", workmonth.ErrInvalidValue)
	}

	record, err := corruptedAsInvalid(workmonth.NewDailyRecord(date, startTime, endTime, brk))
	if err != nil {
		return workmonth.DailyRecord{}, err
	}
	return record, nil
}

// corruptedAsInvalid は永続化行からの構築失敗を ErrInvalidValue へそろえる（AC-9-15-h）。
// 利用者の要求に対する時刻の検証の番兵には当たらない形にし、行の破損を 400 として外へ出さない。
// 元のエラーの文言は残すが、番兵としてはラップしない。
func corruptedAsInvalid[T any](v T, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, fmt.Errorf("%w: stored row is corrupted: %s", workmonth.ErrInvalidValue, err.Error())
	}
	return v, nil
}

// intPtr は整数のポインタを返す。
func intPtr(v int) *int { return &v }

// countNil は nil のポインタの個数を返す。
func countNil(values ...*int) int {
	n := 0
	for _, v := range values {
		if v == nil {
			n++
		}
	}
	return n
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
		// 休憩なしは4値とも (*int)(nil)（AC-9-16-g）。ゼロ値で埋めない。
		var breakStartH, breakStartM, breakEndH, breakEndM *int
		if brk, ok := record.Break(); ok {
			breakStartH, breakStartM = intPtr(brk.Start().Hour()), intPtr(brk.Start().Minute())
			breakEndH, breakEndM = intPtr(brk.End().Hour()), intPtr(brk.End().Minute())
		}
		if err := tx.Exec(ctx, workMonthDailyRecordInsertQuery,
			contractID, record.Date().Year(), record.Date().Month(), record.Date().Day(),
			record.StartTime().Hour(), record.StartTime().Minute(),
			record.EndTime().Hour(), record.EndTime().Minute(),
			breakStartH, breakStartM, breakEndH, breakEndM,
		); err != nil {
			return err
		}
	}

	return nil
}

var _ port.WorkMonthRepository = (*WorkMonthRepository)(nil)
