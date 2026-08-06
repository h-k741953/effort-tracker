package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// WorkMonthQuery は port.WorkMonthQuery（AC-6-7）の実装。集約を経由せず、
// 行から直接リードモデルへ変換する（AC-9-18-a）。
type WorkMonthQuery struct {
	db DB
}

// NewWorkMonthQuery は WorkMonthQuery を構築する。
func NewWorkMonthQuery(db DB) *WorkMonthQuery {
	return &WorkMonthQuery{db: db}
}

// workMonthListFilters は条件から WHERE 句と引数を組み立てる。省略（空文字列）
// の条件は絞り込みに加えない（AC-9-18-g）。技術者識別子を省略した一覧は
// 技術者横断（承認待ち一覧）であり、行が0件になるのが正しい振る舞いではない。
func workMonthListFilters(condition port.WorkMonthQueryCondition) (string, []any) {
	var clauses []string
	var args []any
	if condition.EngineerID != "" {
		args = append(args, condition.EngineerID)
		clauses = append(clauses, fmt.Sprintf("c.engineer_id = $%d", len(args)))
	}
	if condition.State != "" {
		args = append(args, condition.State)
		clauses = append(clauses, fmt.Sprintf("wm.state = $%d", len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// List は条件に一致する行と総件数を返す（AC-6-7-e・AC-9-18）。生成済みの
// 勤務月のみを対象とし（AC-9-18-e。未生成の年月を行として作らない）、契約
// 表示名は勤務月と契約の結合で取る（N+1 を作らない＝AC-9-18-b）。並び順は
// 対象年月の降順・同一年月内は契約識別子の昇順で SQL の ORDER BY が与える
// （AC-9-18-c）。
//
// 総件数の取得手段は本仕様が固定しない（AC-13-20）。ここでは行を取得する
// クエリと総件数を取得するクエリを分けて発行する構成を選ぶ（tester が
// テストの前提として選んだ構成に合わせる。doubles_test.go 前書き）。
func (q *WorkMonthQuery) List(ctx context.Context, condition port.WorkMonthQueryCondition) ([]port.WorkMonthQueryRow, int, error) {
	where, filterArgs := workMonthListFilters(condition)

	listArgs := append(append([]any{}, filterArgs...), condition.Limit, condition.Offset)
	listQuery := fmt.Sprintf(`
SELECT wm.contract_id, c.display_name, wm.year, wm.month, wm.state
FROM work_months wm
JOIN contracts c ON c.id = wm.contract_id
%s
ORDER BY wm.year DESC, wm.month DESC, wm.contract_id ASC
LIMIT $%d OFFSET $%d
`, where, len(filterArgs)+1, len(filterArgs)+2)

	rows, err := q.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}

	result := make([]port.WorkMonthQueryRow, 0)
	for rows.Next() {
		var (
			contractID  string
			displayName string
			year        int
			month       int
			state       string
		)
		if err := rows.Scan(&contractID, &displayName, &year, &month, &state); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		result = append(result, port.WorkMonthQueryRow{
			ContractID:          contractID,
			ContractDisplayName: displayName,
			YearMonth:           fmt.Sprintf("%04d-%02d", year, month),
			State:               state,
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	_ = rows.Close()

	countQuery := fmt.Sprintf(`
SELECT COUNT(*)
FROM work_months wm
JOIN contracts c ON c.id = wm.contract_id
%s
`, where)
	countRows, err := q.db.Query(ctx, countQuery, filterArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = countRows.Close() }()

	total := 0
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	if err := countRows.Err(); err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

var _ port.WorkMonthQuery = (*WorkMonthQuery)(nil)
