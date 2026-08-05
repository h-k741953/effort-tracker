package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-17・AC-9-19-a・AC-12-11③。

var contractCmpOpts = cmp.AllowUnexported(workmonth.ContractID{}, workmonth.SettlementRange{}, workmonth.WorkingHours{})

// TestContractRepository_Find_BuildsContractFromRow は、行から
// port.Contract（識別子・契約表示名・技術者識別子・精算幅）を組み立てることを
// 検証する（AC-9-17-a）。精算幅は workmonth.NewSettlementRange で構築する
// （AC-9-17-d）。
func TestContractRepository_Find_BuildsContractFromRow(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		contractRow("contract-1", "Acme Corp", "engineer-1", 8, 0, 20, 0),
	), nil)

	repo := gateway.NewContractRepository(db)
	got, err := repo.Find(context.Background(), mustContractID(t, "contract-1"))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	want := port.Contract{
		ID:              mustContractID(t, "contract-1"),
		DisplayName:     "Acme Corp",
		EngineerID:      "engineer-1",
		SettlementRange: mustSettlementRange(t, 8, 0, 20, 0),
	}
	if diff := cmp.Diff(want, got, contractCmpOpts); diff != "" {
		t.Errorf("Find が返す契約が不一致 (-want +got):\n%s（AC-9-17-a）", diff)
	}
}

// TestContractRepository_Find_NotFoundReturnsSentinel は、行が無いとき
// port.ErrContractNotFound を返すことを検証する（AC-9-17-b・AC-9-19-a）。
func TestContractRepository_Find_NotFoundReturnsSentinel(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(), nil)

	repo := gateway.NewContractRepository(db)
	_, err := repo.Find(context.Background(), mustContractID(t, "contract-1"))
	if !errors.Is(err, port.ErrContractNotFound) {
		t.Fatalf("Find のエラー = %v, want errors.Is(err, port.ErrContractNotFound)（AC-9-17-b）", err)
	}
}

// TestContractRepository_Find_OtherDriverErrorPassedThrough は、「行が無い」
// 以外のドライバ由来のエラーを port の番兵へ変換せずそのまま返すことを検証する
// （AC-9-19-a）。
func TestContractRepository_Find_OtherDriverErrorPassedThrough(t *testing.T) {
	driverErr := errors.New("connection reset by peer")
	db := newFakeDB()
	db.pushQuery(nil, driverErr)

	repo := gateway.NewContractRepository(db)
	_, err := repo.Find(context.Background(), mustContractID(t, "contract-1"))
	if !errors.Is(err, driverErr) {
		t.Fatalf("Find のエラー = %v, want errors.Is(err, driverErr)（AC-9-19-a: それ以外はそのまま返す）", err)
	}
	if errors.Is(err, port.ErrContractNotFound) {
		t.Errorf("Find のエラーが port.ErrContractNotFound に化けている（AC-9-19-a 違反）: %v", err)
	}
}

// TestContractRepository_Find_InvalidSettlementRangePropagatesInvalidValue は、
// 精算幅の構築失敗（下限 > 上限）を ErrInvalidValue のまま返すことを検証する
// （AC-9-17-d）。
func TestContractRepository_Find_InvalidSettlementRangePropagatesInvalidValue(t *testing.T) {
	db := newFakeDB()
	db.pushQuery(newFakeRows(
		contractRow("contract-1", "Acme Corp", "engineer-1", 20, 0, 8, 0), // 下限 > 上限
	), nil)

	repo := gateway.NewContractRepository(db)
	_, err := repo.Find(context.Background(), mustContractID(t, "contract-1"))
	if !errors.Is(err, workmonth.ErrInvalidValue) {
		t.Fatalf("Find のエラー = %v, want errors.Is(err, workmonth.ErrInvalidValue)（AC-9-17-d）", err)
	}
}
