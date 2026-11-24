package interactor_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-12-20 ①②（本人かの真偽の検査）・
//     AC-7-22（出力 DTO の IsOwnWorkMonth）・AC-8-13（求めることは認可ではない）・
//     AC-7-15（GetWorkMonth の組み立て）
//   - docs/specs/domain-api-http-contract.md AC-10-6（10-6-a〜10-6-i）

// ---- AC-12-20 ①: GetWorkMonth ---------------------------------------------

// TestGetWorkMonth_IsOwnWorkMonth は 10-6-a〜10-6-e と (iii) の各操作者について、
// 生成済み・未生成の両方で出力 DTO の IsOwnWorkMonth を固定する（10-6-f）。
// 真と偽を対で置き、固定値を返す実装・ロールを見る実装・Authenticated を見ない実装を
// それぞれ Red にする。いずれの組でも PresentError は呼ばれない（AC-8-13。
// onlyPresented が PresentError の呼び出しを失敗にする）。
func TestGetWorkMonth_IsOwnWorkMonth(t *testing.T) {
	actors := []struct {
		name  string
		actor port.Actor
		want  bool
	}{
		{name: "10-6-a Engineer・一致 → 真", actor: ownerActor(port.RoleEngineer), want: true},
		{name: "10-6-b Engineer・不一致 → 偽", actor: foreignActor(port.RoleEngineer), want: false},
		{name: "10-6-c Approver・不一致 → 偽", actor: foreignActor(port.RoleApprover), want: false},
		{name: "10-6-d Approver・一致 → 真（ロールを問わない）", actor: ownerActor(port.RoleApprover), want: true},
		{name: "10-6-e ゲスト → 偽", actor: guestActor(), want: false},
		{
			name:  "(iii) 未認証で Actor.ID が契約の技術者識別子と同じ値 → 偽（Authenticated を見る）",
			actor: port.Actor{ID: testEngineerID, Role: port.RoleEngineer, Authenticated: false},
			want:  false,
		},
	}

	for _, generated := range []bool{true, false} {
		for _, a := range actors {
			name := a.name
			if generated {
				name += "（生成済み）"
			} else {
				name += "（未生成。10-6-f）"
			}
			t.Run(name, func(t *testing.T) {
				f := newFixture(t, mustDate(t, 2026, 8, 15))
				if generated {
					f.workMonths.put(reconstructWorkMonth(
						t, f.contractID, f.yearMonth,
						mustSettlementRange(t, 140, 180),
						workmonth.StateDraft,
						[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
					))
				}

				f.get().Execute(context.Background(), port.GetWorkMonthInput{
					Actor:      a.actor,
					ContractID: f.contractID,
					YearMonth:  f.yearMonth,
				})

				got := f.output.onlyPresented(t)
				if got.Generated != generated {
					t.Fatalf("前提: Generated = %v, want %v", got.Generated, generated)
				}
				if got.IsOwnWorkMonth != a.want {
					t.Errorf("IsOwnWorkMonth = %v, want %v（AC-7-22・AC-10-6）", got.IsOwnWorkMonth, a.want)
				}
			})
		}
	}
}

// TestGetWorkMonth_OwnAndForeignDifferOnlyInIsOwnWorkMonth は 10-6-i を固定する。
// 同じ契約 × 年月を 10-6-a と 10-6-b の操作者で取得した出力 DTO は、
// IsOwnWorkMonth を除いて一致する（操作者で絞り込まない。AC-8-8）。
func TestGetWorkMonth_OwnAndForeignDifferOnlyInIsOwnWorkMonth(t *testing.T) {
	for _, generated := range []bool{true, false} {
		name := "未生成"
		if generated {
			name = "生成済み"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			if generated {
				f.workMonths.put(reconstructWorkMonth(
					t, f.contractID, f.yearMonth,
					mustSettlementRange(t, 140, 180),
					workmonth.StateDraft,
					[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
				))
			}
			present := func(actor port.Actor) port.WorkMonthOutput {
				f.output = &spyWorkMonthOutputPort{}
				f.get().Execute(context.Background(), port.GetWorkMonthInput{
					Actor:      actor,
					ContractID: f.contractID,
					YearMonth:  f.yearMonth,
				})
				return f.output.onlyPresented(t)
			}

			own := present(ownerActor(port.RoleEngineer))
			foreign := present(foreignActor(port.RoleEngineer))
			if own.IsOwnWorkMonth == foreign.IsOwnWorkMonth {
				t.Fatalf("前提: 10-6-a と 10-6-b で IsOwnWorkMonth が同じ（%v）", own.IsOwnWorkMonth)
			}
			own.IsOwnWorkMonth = false
			foreign.IsOwnWorkMonth = false
			if diff := cmp.Diff(own, foreign); diff != "" {
				t.Errorf("IsOwnWorkMonth 以外の出力が操作者で異なる (-own +foreign):\n%s", diff)
			}
		})
	}
}

// ---- AC-12-20 ②: 更新系5つの成功時の真偽 ---------------------------------

// TestUpdateUseCases_IsOwnWorkMonthOnSuccess は更新系5つの成功の経路の IsOwnWorkMonth を固定する
// （10-6-g・10-6-h）。EnterDailyRecord・DeleteDailyRecord・CloseWorkMonth は真、
// ApproveWorkMonth・RejectWorkMonth は偽。EnterDailyRecord には Approver ロールの本人の組も置き、
// 真であることを固定する（AC-8-1。ロールで真偽を決める実装を Red にする）。
func TestUpdateUseCases_IsOwnWorkMonthOnSuccess(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		want bool
		run  func(t *testing.T, f *fixture)
	}{
		{
			name: "EnterDailyRecord（Engineer の本人）→ 真（10-6-g）",
			want: true,
			run: func(t *testing.T, f *fixture) {
				f.enter().Execute(ctx, port.EnterDailyRecordInput{
					Actor: ownerActor(port.RoleEngineer), ContractID: f.contractID, YearMonth: f.yearMonth,
					Date: mustDate(t, 2026, 7, 1), StartTime: tod(0, 0), EndTime: tod(8, 0),
				})
			},
		},
		{
			name: "EnterDailyRecord（Approver ロールの本人）→ 真（AC-8-1。ロールを問わない）",
			want: true,
			run: func(t *testing.T, f *fixture) {
				f.enter().Execute(ctx, port.EnterDailyRecordInput{
					Actor: ownerActor(port.RoleApprover), ContractID: f.contractID, YearMonth: f.yearMonth,
					Date: mustDate(t, 2026, 7, 1), StartTime: tod(0, 0), EndTime: tod(8, 0),
				})
			},
		},
		{
			name: "DeleteDailyRecord（本人）→ 真（10-6-g）",
			want: true,
			run: func(t *testing.T, f *fixture) {
				f.workMonths.put(reconstructWorkMonth(
					t, f.contractID, f.yearMonth, mustSettlementRange(t, 140, 180),
					workmonth.StateDraft, []workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
				))
				f.delete().Execute(ctx, port.DeleteDailyRecordInput{
					Actor: ownerActor(port.RoleEngineer), ContractID: f.contractID, YearMonth: f.yearMonth,
					Date: mustDate(t, 2026, 7, 1),
				})
			},
		},
		{
			name: "CloseWorkMonth（本人）→ 真（10-6-g）",
			want: true,
			run: func(t *testing.T, f *fixture) {
				f.workMonths.put(reconstructWorkMonth(
					t, f.contractID, f.yearMonth, mustSettlementRange(t, 140, 180),
					workmonth.StateDraft, recordsSummingTo(t, 150*60),
				))
				f.close().Execute(ctx, port.CloseWorkMonthInput{
					Actor: ownerActor(port.RoleEngineer), ContractID: f.contractID, YearMonth: f.yearMonth,
				})
			},
		},
		{
			name: "ApproveWorkMonth（本人でない Approver）→ 偽（10-6-h）",
			want: false,
			run: func(t *testing.T, f *fixture) {
				f.putPendingApproval(t)
				f.approve().Execute(ctx, port.ApproveWorkMonthInput{
					Actor: foreignActor(port.RoleApprover), ContractID: f.contractID, YearMonth: f.yearMonth,
				})
			},
		},
		{
			name: "RejectWorkMonth（本人でない Approver）→ 偽（10-6-h）",
			want: false,
			run: func(t *testing.T, f *fixture) {
				f.putPendingApproval(t)
				f.reject().Execute(ctx, port.RejectWorkMonthInput{
					Actor: foreignActor(port.RoleApprover), ContractID: f.contractID, YearMonth: f.yearMonth,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			tt.run(t, f)
			got := f.output.onlyPresented(t)
			if got.IsOwnWorkMonth != tt.want {
				t.Errorf("IsOwnWorkMonth = %v, want %v（AC-7-22・AC-10-6）", got.IsOwnWorkMonth, tt.want)
			}
		})
	}
}
