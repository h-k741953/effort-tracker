package gateway

import (
	"context"
	"fmt"

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

// contractSelectQuery は契約1件を取得する SQL 文（識別子・契約表示名・
// 技術者識別子・精算幅下限（時・分）・精算幅上限（時・分）の7列。AC-9-17-a）。
// SQL 文そのものの正しさは検査対象ではない（AC-13-18）。
const contractSelectQuery = `
SELECT id, display_name, engineer_id, lower_bound_hours, lower_bound_minutes, upper_bound_hours, upper_bound_minutes
FROM contracts
WHERE id = $1
`

// Find は契約を1件取得する（AC-9-17）。行が無ければ port.ErrContractNotFound
// を返す（AC-9-17-b）。それ以外のドライバ由来のエラーは変換せずそのまま返す
// （AC-9-19-a）。精算幅は workmonth.NewSettlementRange で構築し、失敗
// （下限 > 上限）は ErrInvalidValue のまま返す（AC-9-17-d）。
func (r *ContractRepository) Find(ctx context.Context, contractID workmonth.ContractID) (port.Contract, error) {
	rows, err := r.db.Query(ctx, contractSelectQuery, contractID.String())
	if err != nil {
		return port.Contract{}, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return port.Contract{}, err
		}
		return port.Contract{}, fmt.Errorf("%w: contract %s", port.ErrContractNotFound, contractID.String())
	}

	var (
		id          string
		displayName string
		engineerID  string
		lowerH      int
		lowerM      int
		upperH      int
		upperM      int
	)
	if err := rows.Scan(&id, &displayName, &engineerID, &lowerH, &lowerM, &upperH, &upperM); err != nil {
		return port.Contract{}, err
	}

	parsedID, err := workmonth.NewContractID(id)
	if err != nil {
		return port.Contract{}, err
	}

	lower, err := workmonth.NewWorkingHours(lowerH, lowerM)
	if err != nil {
		return port.Contract{}, err
	}
	upper, err := workmonth.NewWorkingHours(upperH, upperM)
	if err != nil {
		return port.Contract{}, err
	}
	settlementRange, err := workmonth.NewSettlementRange(lower, upper)
	if err != nil {
		return port.Contract{}, err
	}

	return port.Contract{
		ID:              parsedID,
		DisplayName:     displayName,
		EngineerID:      engineerID,
		SettlementRange: settlementRange,
	}, nil
}

var _ port.ContractRepository = (*ContractRepository)(nil)
