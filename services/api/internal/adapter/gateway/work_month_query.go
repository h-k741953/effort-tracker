package gateway

import (
	"context"

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

// List はテスト工程時点では未実装（TDD の Red を確認するための宣言のみ。
// docs/rules/development-process.md）。実装は次工程（implementer）が行う
// （AC-9-18）。
func (q *WorkMonthQuery) List(ctx context.Context, condition port.WorkMonthQueryCondition) ([]port.WorkMonthQueryRow, int, error) {
	return nil, 0, nil
}

var _ port.WorkMonthQuery = (*WorkMonthQuery)(nil)
