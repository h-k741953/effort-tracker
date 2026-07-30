package interactor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// テストダブルはすべて手書きのインメモリ Fake / spy とする
// （ADR 0007・docs/specs/workmonth-implementation-design.md AC-12-2・AC-12-3）。
// モックライブラリは使わない。

// ---- 勤務月リポジトリの Fake ---------------------------------------------

type fakeWorkMonthRepository struct {
	stored    map[string]*workmonth.WorkMonth
	saveCount int
	saveErr   error
}

func newFakeWorkMonthRepository() *fakeWorkMonthRepository {
	return &fakeWorkMonthRepository{stored: map[string]*workmonth.WorkMonth{}}
}

func workMonthKey(contractID workmonth.ContractID, yearMonth workmonth.YearMonth) string {
	return fmt.Sprintf("%s|%04d-%02d", contractID.String(), yearMonth.Year(), yearMonth.Month())
}

// Find は保存済みの勤務月の**複製**を返す。実装（gateway）は行から集約を組み立てるため、
// 呼び出し側が受け取った集約を変更しても Save を経ない限り保存済みの状態は変わらない。
// 同じ性質を Fake でも再現しないと、「Save を呼ばずに反映されている」誤りを検知できない。
//
// 確定済みの超過／不足も複製へ引き継ぐ（実装設計 AC-12-7）。引き継がないと
// 締め済・承認済の勤務月が未確定として復元され、テストが実装より弱くなる。
func (f *fakeWorkMonthRepository) Find(_ context.Context, contractID workmonth.ContractID, yearMonth workmonth.YearMonth) (*workmonth.WorkMonth, error) {
	target, ok := f.stored[workMonthKey(contractID, yearMonth)]
	if !ok {
		return nil, port.ErrWorkMonthNotFound
	}
	excess, excessOK := target.Excess()
	shortfall, shortfallOK := target.Shortfall()
	return workmonth.Reconstruct(
		target.ContractID(),
		target.YearMonth(),
		target.SettlementRange(),
		target.State(),
		target.DailyRecords(),
		workingHoursPointer(excess, excessOK),
		workingHoursPointer(shortfall, shortfallOK),
	)
}

// workingHoursPointer は (WorkingHours, bool) の組を Reconstruct が受け取る
// *WorkingHours へ変換する。未確定（ok=false）なら nil。
func workingHoursPointer(w workmonth.WorkingHours, ok bool) *workmonth.WorkingHours {
	if !ok {
		return nil
	}
	return &w
}

// Save は保存を記録する。saveCount は「Save が呼ばれた回数」であり、
// 失敗を注入した場合も回数に数える（呼ばれたか否かと成否を混同しないため）。
func (f *fakeWorkMonthRepository) Save(_ context.Context, target *workmonth.WorkMonth) error {
	f.saveCount++
	if f.saveErr != nil {
		return f.saveErr
	}
	f.stored[workMonthKey(target.ContractID(), target.YearMonth())] = target
	return nil
}

// put は保存済みの勤務月をテストの前提として投入する（saveCount を増やさない）。
func (f *fakeWorkMonthRepository) put(target *workmonth.WorkMonth) {
	f.stored[workMonthKey(target.ContractID(), target.YearMonth())] = target
}

// saved は保存されている勤務月を取り出す。
func (f *fakeWorkMonthRepository) saved(t *testing.T, contractID workmonth.ContractID, yearMonth workmonth.YearMonth) *workmonth.WorkMonth {
	t.Helper()
	target, ok := f.stored[workMonthKey(contractID, yearMonth)]
	if !ok {
		t.Fatalf("勤務月が保存されていない: key=%s", workMonthKey(contractID, yearMonth))
	}
	return target
}

// ---- 契約リポジトリの Fake -----------------------------------------------

type fakeContractRepository struct {
	contracts map[string]port.Contract
}

func newFakeContractRepository() *fakeContractRepository {
	return &fakeContractRepository{contracts: map[string]port.Contract{}}
}

func (f *fakeContractRepository) Find(_ context.Context, contractID workmonth.ContractID) (port.Contract, error) {
	contract, ok := f.contracts[contractID.String()]
	if !ok {
		return port.Contract{}, port.ErrContractNotFound
	}
	return contract, nil
}

func (f *fakeContractRepository) put(contract port.Contract) {
	f.contracts[contract.ID.String()] = contract
}

// ---- 時計の Fake ---------------------------------------------------------

// fakeClock は「当日」を固定して返す。JST への変換は driver の責務であり
// （実装設計 D-5・AC-6-5）、ユースケースのテストは time.Now を使わない。
type fakeClock struct {
	today workmonth.Date
}

func (c fakeClock) Today() workmonth.Date { return c.today }

// ---- 出力ポートの spy -----------------------------------------------------

type spyWorkMonthOutputPort struct {
	presented []port.WorkMonthOutput
	errs      []error
}

func (s *spyWorkMonthOutputPort) Present(output port.WorkMonthOutput) {
	s.presented = append(s.presented, output)
}

func (s *spyWorkMonthOutputPort) PresentError(err error) {
	s.errs = append(s.errs, err)
}

// onlyPresented は Present が1度だけ呼ばれたことを確認して、その出力を返す。
func (s *spyWorkMonthOutputPort) onlyPresented(t *testing.T) port.WorkMonthOutput {
	t.Helper()
	if len(s.errs) != 0 {
		t.Fatalf("PresentError が呼ばれた: %v", s.errs)
	}
	if len(s.presented) != 1 {
		t.Fatalf("Present の呼び出し回数 = %d, want 1", len(s.presented))
	}
	return s.presented[0]
}

// onlyPresentedError は PresentError が1度だけ呼ばれたことを確認して、そのエラーを返す。
func (s *spyWorkMonthOutputPort) onlyPresentedError(t *testing.T) error {
	t.Helper()
	if len(s.presented) != 0 {
		t.Fatalf("Present が呼ばれた: %+v", s.presented)
	}
	if len(s.errs) != 1 {
		t.Fatalf("PresentError の呼び出し回数 = %d, want 1", len(s.errs))
	}
	return s.errs[0]
}
