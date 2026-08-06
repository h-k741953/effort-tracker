package interactor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/interactor"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-7-16（ListWorkMonths の
//     入力と責務順序）・AC-8-9・AC-8-10（認可）・AC-12-12（ListWorkMonths の
//     テスト）
//   - docs/specs/domain-api-http-contract.md AC-9（判定順序。順1「認証」→順4「認可」）

// TestListWorkMonths_Authorization は AC-8-9・AC-8-10 の認可判定と、契約 AC-9
// の順1（認証）→順4（認可）の順序を検証する（AC-12-12。前2組を対にして
// 判定順序を固定する）。
func TestListWorkMonths_Authorization(t *testing.T) {
	tests := []struct {
		name       string
		engineerID string
		state      string
		actor      port.Actor
		wantErr    error // nil なら成功
	}{
		{
			name:       "省略×未認証は ErrUnauthenticated（順1が先。AC-8-10）",
			engineerID: "",
			state:      "PendingApproval",
			actor:      guestActor(),
			wantErr:    port.ErrUnauthenticated,
		},
		{
			name:       "省略×Engineer ロールは ErrNotApprover（順4。順1 とペアで判定順序を固定）",
			engineerID: "",
			state:      "PendingApproval",
			actor:      ownerActor(port.RoleEngineer),
			wantErr:    port.ErrNotApprover,
		},
		{
			name:       "省略×Approver ロールは成功（承認待ち一覧。AC-8-10）",
			engineerID: "",
			state:      "PendingApproval",
			actor:      ownerActor(port.RoleApprover),
		},
		{
			name:       "指定あり×未認証は成功（操作者で絞らない。AC-8-9）",
			engineerID: testEngineerID,
			state:      "",
			actor:      guestActor(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &fakeWorkMonthQuery{}
			output := &spyListWorkMonthsOutputPort{}

			interactor.NewListWorkMonths(query, output).Execute(context.Background(), port.ListWorkMonthsInput{
				Actor:      tt.actor,
				EngineerID: tt.engineerID,
				State:      tt.state,
				Limit:      20,
				Offset:     0,
			})

			if tt.wantErr != nil {
				err := output.onlyPresentedError(t)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if len(query.calls) != 0 {
					t.Errorf("弾かれたのに WorkMonthQuery が呼ばれた（回数 = %d, want 0）", len(query.calls))
				}
				return
			}

			output.onlyPresented(t)
			if len(query.calls) != 1 {
				t.Errorf("WorkMonthQuery の呼び出し回数 = %d, want 1", len(query.calls))
			}
		})
	}
}

// TestListWorkMonths_PassesConditionThroughAndCopiesResult は、入力の条件が
// そのまま WorkMonthQuery へ渡ることと、総件数・件数・開始位置が出力 DTO へ
// そのまま載ることを検証する（AC-7-16②・AC-7-17・AC-12-12）。
func TestListWorkMonths_PassesConditionThroughAndCopiesResult(t *testing.T) {
	query := &fakeWorkMonthQuery{
		rows: []port.WorkMonthQueryRow{
			{ContractID: testContractID, ContractDisplayName: testDisplayName, YearMonth: "2026-07", State: "PendingApproval"},
		},
		total: 42,
	}
	output := &spyListWorkMonthsOutputPort{}

	input := port.ListWorkMonthsInput{
		Actor:      ownerActor(port.RoleApprover),
		EngineerID: "",
		State:      "PendingApproval",
		Limit:      10,
		Offset:     5,
	}
	interactor.NewListWorkMonths(query, output).Execute(context.Background(), input)

	if len(query.calls) != 1 {
		t.Fatalf("WorkMonthQuery の呼び出し回数 = %d, want 1", len(query.calls))
	}
	wantCondition := port.WorkMonthQueryCondition{EngineerID: "", State: "PendingApproval", Limit: 10, Offset: 5}
	if diff := cmp.Diff(wantCondition, query.calls[0]); diff != "" {
		t.Errorf("WorkMonthQuery へ渡った条件が入力と不一致 (-want +got):\n%s（AC-7-16②）", diff)
	}

	got := output.onlyPresented(t)
	want := port.ListWorkMonthsOutput{
		Items: []port.ListWorkMonthsOutputRow{
			{ContractID: testContractID, ContractDisplayName: testDisplayName, YearMonth: "2026-07", State: "PendingApproval"},
		},
		Total:  42,
		Limit:  10,
		Offset: 5,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("出力 DTO が不一致 (-want +got):\n%s（AC-7-17）", diff)
	}
}

// TestListWorkMonths_QueryErrorIsNotConvertedToEmptyOutput は、WorkMonthQuery
// が返したエラーが空の一覧へ変換されず、そのまま PresentError へ渡ることを
// 検証する（永続化の失敗を 200 で隠さない。AC-7-3・AC-7-16・AC-11-11）。
// GetWorkMonth 側の TestGetWorkMonth_OtherRepositoryErrorIsNotConvertedToEmptyOutput
// と同じ形に揃える。
func TestListWorkMonths_QueryErrorIsNotConvertedToEmptyOutput(t *testing.T) {
	driverErr := errors.New("fake: query failed unexpectedly")
	query := &fakeWorkMonthQuery{err: driverErr}
	output := &spyListWorkMonthsOutputPort{}

	interactor.NewListWorkMonths(query, output).Execute(context.Background(), port.ListWorkMonthsInput{
		Actor:      guestActor(),
		EngineerID: testEngineerID,
		Limit:      20,
		Offset:     0,
	})

	if err := output.onlyPresentedError(t); !errors.Is(err, driverErr) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, driverErr)（AC-7-3・AC-11-11）", err)
	}
}

// TestListWorkMonths_EmptyResultIsEmptySliceNotNil は該当0件でも行が空スライス
// （nil でない）であることを検証する（AC-7-17・AC-12-12）。
func TestListWorkMonths_EmptyResultIsEmptySliceNotNil(t *testing.T) {
	query := &fakeWorkMonthQuery{rows: []port.WorkMonthQueryRow{}, total: 0}
	output := &spyListWorkMonthsOutputPort{}

	interactor.NewListWorkMonths(query, output).Execute(context.Background(), port.ListWorkMonthsInput{
		Actor:      guestActor(),
		EngineerID: testEngineerID,
		Limit:      20,
		Offset:     0,
	})

	got := output.onlyPresented(t)
	if got.Items == nil {
		t.Errorf("Items が nil（該当0件でも空スライスであること。AC-7-17・AC-12-12）")
	}
	if len(got.Items) != 0 {
		t.Errorf("Items の件数 = %d, want 0", len(got.Items))
	}
	if got.Total != 0 {
		t.Errorf("Total = %d, want 0", got.Total)
	}
}
