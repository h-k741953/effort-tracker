package gateway

import (
	"context"

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

// Find は勤務月を1件取得し、workmonth.Reconstruct（AC-2-5）で集約へ組み立てる
// （AC-9-15）。行が無ければ port.ErrWorkMonthNotFound を返す（AC-9-15-e）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない。db を一切呼ばない）。
func (r *WorkMonthRepository) Find(_ context.Context, _ workmonth.ContractID, _ workmonth.YearMonth) (*workmonth.WorkMonth, error) {
	// TODO(implementer): AC-9-15・AC-9-19-a を実装する。
	return nil, nil
}

// Save は勤務月と稼働実績を1トランザクションで書き込む（AC-9-16）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない。db を一切呼ばない）。
func (r *WorkMonthRepository) Save(_ context.Context, _ *workmonth.WorkMonth) error {
	// TODO(implementer): AC-9-16・AC-9-16-a（原子性）を実装する。
	return nil
}

var _ port.WorkMonthRepository = (*WorkMonthRepository)(nil)
