package gateway

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// ContractRepository は port.ContractRepository（AC-6-2）の実装。
// 契約は読み取り専用の与件であり、書き込みは提供しない（AC-9-17-c）。
type ContractRepository struct {
	db DB
}

// NewContractRepository は ContractRepository を構築する。
func NewContractRepository(db DB) *ContractRepository {
	return &ContractRepository{db: db}
}

// Find は契約を1件取得する（AC-9-17）。行が無ければ port.ErrContractNotFound
// を返す（AC-9-17-b）。精算幅は workmonth.NewSettlementRange で構築し、失敗
// （下限 > 上限）は ErrInvalidValue のまま返す（AC-9-17-d）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない。db を一切呼ばない）。
func (r *ContractRepository) Find(_ context.Context, _ workmonth.ContractID) (port.Contract, error) {
	// TODO(implementer): AC-9-17・AC-9-19-a を実装する。
	return port.Contract{}, nil
}

var _ port.ContractRepository = (*ContractRepository)(nil)
