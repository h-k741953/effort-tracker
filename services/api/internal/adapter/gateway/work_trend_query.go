package gateway

import (
	"context"
	"fmt"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// WorkTrendQuery は port.WorkTrendQuery（AC-6-11）の実装（AC-9-21）。
// 契約の精算幅・契約期間・契約表示名は読まない（契約との結合は技術者識別子で
// 絞るためだけに使う。AC-9-21-j）。
type WorkTrendQuery struct {
	db DB
}

// NewWorkTrendQuery は WorkTrendQuery を構築する。
func NewWorkTrendQuery(db DB) *WorkTrendQuery {
	return &WorkTrendQuery{db: db}
}

// workTrendHeaderSelectQuery は技術者の勤務月の行を取得する SQL 文
// （AC-9-21-d。workMonthHeaderSelectQuery と同じ12列）。並びは契約識別子の昇順、
// 同じ契約の中は年・月の昇順。SQL 文そのものの正しさは検査対象ではない（AC-13-18）。
const workTrendHeaderSelectQuery = `
SELECT wm.contract_id, wm.year, wm.month, wm.lower_bound_hours, wm.lower_bound_minutes, wm.upper_bound_hours, wm.upper_bound_minutes,
       wm.state, wm.excess_hours, wm.excess_minutes, wm.shortfall_hours, wm.shortfall_minutes
FROM work_months wm
JOIN contracts c ON c.id = wm.contract_id
WHERE c.engineer_id = $1
ORDER BY wm.contract_id ASC, wm.year ASC, wm.month ASC
`

// workTrendDailyRecordsSelectQuery は技術者の稼働実績の行を取得する SQL 文
// （AC-9-21-e。先頭に契約識別子を持つ12列）。並びは固定しない（AC-9-21-b）。
const workTrendDailyRecordsSelectQuery = `
SELECT dr.contract_id, dr.year, dr.month, dr.day,
       dr.start_hour, dr.start_minute, dr.end_hour, dr.end_minute,
       dr.break_start_hour, dr.break_start_minute, dr.break_end_hour, dr.break_end_minute
FROM daily_records dr
JOIN contracts c ON c.id = dr.contract_id
WHERE c.engineer_id = $1
`

// trendHeader は勤務月の行（Reconstruct に渡す前の値）。
type trendHeader struct {
	contractID      workmonth.ContractID
	yearMonth       workmonth.YearMonth
	settlementRange workmonth.SettlementRange
	state           string
	excess          *workmonth.WorkingHours
	shortfall       *workmonth.WorkingHours
}

// trendKey は稼働実績を勤務月へ割り当てる鍵（契約識別子 × 年 × 月。AC-9-21-g）。
type trendKey struct {
	contractID string
	year       int
	month      int
}

// ListByEngineer は技術者の生成済みの勤務月を集約として返す（AC-9-21）。
// 1行でも構築・復元に失敗したら全体をそのエラーで返す（AC-9-21-i）。
func (q *WorkTrendQuery) ListByEngineer(ctx context.Context, engineerID string) ([]*workmonth.WorkMonth, error) {
	headers, err := q.queryHeaders(ctx, engineerID)
	if err != nil {
		return nil, err
	}

	recordsByKey, err := q.queryDailyRecords(ctx, engineerID)
	if err != nil {
		return nil, err
	}

	// 一致する勤務月の無い稼働実績の行は永続化行の破損（黙って捨てない。AC-9-21-g）。
	known := make(map[trendKey]struct{}, len(headers))
	for _, h := range headers {
		known[trendKey{h.contractID.String(), h.yearMonth.Year(), h.yearMonth.Month()}] = struct{}{}
	}
	for key := range recordsByKey {
		if _, ok := known[key]; !ok {
			return nil, fmt.Errorf("%w: daily record row has no matching work month: %s %04d-%02d",
				workmonth.ErrInvalidValue, key.contractID, key.year, key.month)
		}
	}

	result := make([]*workmonth.WorkMonth, 0, len(headers))
	for _, h := range headers {
		key := trendKey{h.contractID.String(), h.yearMonth.Year(), h.yearMonth.Month()}
		records := recordsByKey[key]
		if records == nil {
			records = []workmonth.DailyRecord{}
		}
		target, err := workmonth.Reconstruct(
			h.contractID, h.yearMonth, h.settlementRange, workmonth.State(h.state), records, h.excess, h.shortfall,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, target)
	}
	return result, nil
}

// queryHeaders は1回目の Query（勤務月の行）を走査し、全経路で閉じてから返す
// （AC-9-21-c）。
func (q *WorkTrendQuery) queryHeaders(ctx context.Context, engineerID string) ([]trendHeader, error) {
	rows, err := q.db.Query(ctx, workTrendHeaderSelectQuery, engineerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	headers := make([]trendHeader, 0)
	for rows.Next() {
		h, err := scanTrendHeader(rows)
		if err != nil {
			return nil, err
		}
		headers = append(headers, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return headers, nil
}

// queryDailyRecords は2回目の Query（稼働実績の行）を走査し、契約識別子 × 年 × 月
// ごとにまとめる。
func (q *WorkTrendQuery) queryDailyRecords(ctx context.Context, engineerID string) (map[trendKey][]workmonth.DailyRecord, error) {
	rows, err := q.db.Query(ctx, workTrendDailyRecordsSelectQuery, engineerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	byKey := make(map[trendKey][]workmonth.DailyRecord)
	for rows.Next() {
		var contractID string
		record, err := scanDailyRecord(rows, &contractID)
		if err != nil {
			return nil, err
		}
		key := trendKey{contractID, record.Date().Year(), record.Date().Month()}
		byKey[key] = append(byKey[key], record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return byKey, nil
}

// scanTrendHeader は勤務月の行（12列。AC-9-21-d）を構築する。
func scanTrendHeader(rows Rows) (trendHeader, error) {
	var (
		contractID     string
		year, month    int
		lowerH, lowerM int
		upperH, upperM int
		state          string
		excessH        *int
		excessM        *int
		shortfallH     *int
		shortfallM     *int
	)
	if err := rows.Scan(
		&contractID, &year, &month, &lowerH, &lowerM, &upperH, &upperM,
		&state, &excessH, &excessM, &shortfallH, &shortfallM,
	); err != nil {
		return trendHeader{}, err
	}

	id, err := workmonth.NewContractID(contractID)
	if err != nil {
		return trendHeader{}, err
	}
	ym, err := workmonth.NewYearMonth(year, month)
	if err != nil {
		return trendHeader{}, err
	}
	lower, err := workmonth.NewWorkingHours(lowerH, lowerM)
	if err != nil {
		return trendHeader{}, err
	}
	upper, err := workmonth.NewWorkingHours(upperH, upperM)
	if err != nil {
		return trendHeader{}, err
	}
	settlementRange, err := workmonth.NewSettlementRange(lower, upper)
	if err != nil {
		return trendHeader{}, err
	}
	excess, err := optionalWorkingHours(excessH, excessM)
	if err != nil {
		return trendHeader{}, err
	}
	shortfall, err := optionalWorkingHours(shortfallH, shortfallM)
	if err != nil {
		return trendHeader{}, err
	}

	return trendHeader{
		contractID:      id,
		yearMonth:       ym,
		settlementRange: settlementRange,
		state:           state,
		excess:          excess,
		shortfall:       shortfall,
	}, nil
}

var _ port.WorkTrendQuery = (*WorkTrendQuery)(nil)
