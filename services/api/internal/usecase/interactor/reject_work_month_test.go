package interactor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/interactor"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件:
//   - docs/specs/approval.md AC-2（状態別の可否）・AC-3（承認者ロール）・
//     AC-4（自己承認の排除。差戻しも同様に扱う。D-4）・AC-6（差戻し成立時の帰結）
//   - docs/specs/workmonth-implementation-design.md AC-7-12（入力と依存）・
//     AC-7-13（責務順序。承認と完全に同一）・AC-7-14（差戻しの差分）・AC-8-11（認可の内訳と順序）
//   - docs/specs/domain-api-http-contract.md AC-8・AC-9（判定順序）
//
// 判定順序（AC-7-13）は ApproveWorkMonth と完全に同一であり、判定順序の網羅は
// approve_work_month_test.go の TestApproveWorkMonth_JudgementOrder が持つ。
// 本ファイルは AC-12-8 が固定を求める2組を含む最小限の判定順序テストを別途持ち、
// 差戻しにだけ判定を足さない・省かないこと（AC-7-14）を独立に検証する。

func (f *fixture) reject() *interactor.RejectWorkMonth {
	return interactor.NewRejectWorkMonth(f.workMonths, f.contracts, f.output)
}

// ---- approval.md AC-2-1・AC-6 -----------------------------------------------

// TestRejectWorkMonth_StateTransition は差戻しの成立と、確定済みの超過／不足が
// 未確定（値なし）へ戻って出力・保存されることを検証する（AC-2-1・AC-6-1・AC-6-3。
// 実装設計 AC-4-5・AC-5-10・AC-7-14-c・AC-7-14-d）。
func TestRejectWorkMonth_StateTransition(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.putPendingApproval(t)

	f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
		Actor:      foreignActor(port.RoleApprover),
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	output := f.output.onlyPresented(t)
	if output.State != workmonth.StateDraft.String() {
		t.Errorf("State = %q, want %q", output.State, workmonth.StateDraft.String())
	}
	if output.Excess != nil {
		t.Errorf("Excess = %+v, want nil（値なし。AC-7-14-d）", output.Excess)
	}
	if output.Shortfall != nil {
		t.Errorf("Shortfall = %+v, want nil（値なし。AC-7-14-d）", output.Shortfall)
	}
	if f.workMonths.saveCount != 1 {
		t.Errorf("Save の呼び出し回数 = %d, want 1", f.workMonths.saveCount)
	}
	saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
	if saved.State() != workmonth.StateDraft {
		t.Errorf("保存された状態 State() = %q, want %q", saved.State(), workmonth.StateDraft)
	}
	if _, ok := saved.Excess(); ok {
		t.Errorf("保存された Excess() の第2戻り値 = true, want false（未確定へ戻る。AC-5-10）")
	}
	if _, ok := saved.Shortfall(); ok {
		t.Errorf("保存された Shortfall() の第2戻り値 = true, want false（未確定へ戻る。AC-5-10）")
	}
}

// TestRejectWorkMonth_SavedWorkMonthRoundTrips は差戻し後に保存された勤務月が
// Draft かつ未確定として Fake の Reconstruct 経路で複製できることを検証する
// （実装設計 AC-12-7・AC-12-8・AC-5-9-a。差戻し後は (nil, nil) のみが復元を通る）。
func TestRejectWorkMonth_SavedWorkMonthRoundTrips(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.putPendingApproval(t)

	f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
		Actor:      foreignActor(port.RoleApprover),
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})
	if f.workMonths.saveCount != 1 {
		t.Fatalf("前提の構築に失敗: Save の呼び出し回数 = %d, want 1", f.workMonths.saveCount)
	}

	found, err := f.workMonths.Find(context.Background(), f.contractID, f.yearMonth)
	if err != nil {
		t.Fatalf("差戻し後の Find が失敗（ErrInvalidValue になっていないか。AC-5-9-a）: %v", err)
	}
	if found.State() != workmonth.StateDraft {
		t.Errorf("復元した State() = %q, want %q", found.State(), workmonth.StateDraft)
	}
	if _, ok := found.Excess(); ok {
		t.Errorf("復元した Excess() の第2戻り値 = true, want false（差戻し後は未確定）")
	}
}

// ---- approval.md AC-2-2・AC-2-3 ---------------------------------------------

// TestRejectWorkMonth_RejectsNonPendingApprovalState は PendingApproval 以外からの
// 差戻しを弾くことを検証する（AC-2-2 下書き・AC-2-3 終端状態。実装設計 AC-7-14-b の
// ErrNotRejectable）。
func TestRejectWorkMonth_RejectsNonPendingApprovalState(t *testing.T) {
	tests := []struct {
		name  string
		state workmonth.State
	}{
		{name: "下書き（Draft）からの差戻しは弾く（AC-2-2）", state: workmonth.StateDraft},
		{name: "承認済（Approved）からの差戻しは弾く（AC-2-3）", state: workmonth.StateApproved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			f.workMonths.put(reconstructWorkMonth(
				t,
				f.contractID,
				f.yearMonth,
				mustSettlementRange(t, 140, 180),
				tt.state,
				[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			))

			f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
				Actor:      foreignActor(port.RoleApprover),
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, workmonth.ErrNotRejectable) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, ErrNotRejectable)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
		})
	}
}

// ---- 実装設計 AC-8-11（認可の内訳と順序） ------------------------------------

// TestRejectWorkMonth_Authorization は差戻しの認可（承認者ロール → 自己承認の2段）を
// 検証する（approval.md AC-3・AC-4「差戻しも同様に扱う」／実装設計 AC-8-11）。
// 自己承認と同じ組み合わせを差戻しにも独立に固定し、判定を省いていないことを示す
// （実装設計 AC-7-14「差戻しにだけ判定を足さない・省かない」）。
func TestRejectWorkMonth_Authorization(t *testing.T) {
	tests := []struct {
		name    string
		actor   port.Actor
		wantErr error // nil なら許可
	}{
		{name: "承認者ロールを持ち本人でなければ許可（AC-3-1・AC-4-2）", actor: foreignActor(port.RoleApprover)},
		{
			name:    "承認者ロールを持ち本人なら自己差戻しで弾く（AC-4-1・D-4）",
			actor:   ownerActor(port.RoleApprover),
			wantErr: port.ErrSelfApproval,
		},
		{
			name:    "本人でも承認者ロールが無ければ ErrNotApprover が先（AC-8-11 の順序）",
			actor:   ownerActor(port.RoleEngineer),
			wantErr: port.ErrNotApprover,
		},
		{
			name:    "承認者ロールを持たない者は本人でなくても弾く（AC-3-2）",
			actor:   foreignActor(port.RoleEngineer),
			wantErr: port.ErrNotApprover,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			f.putPendingApproval(t)

			f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
				Actor:      tt.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
			})

			if tt.wantErr != nil {
				if err := f.output.onlyPresentedError(t); !errors.Is(err, tt.wantErr) {
					t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if f.workMonths.saveCount != 0 {
					t.Errorf("認可で弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
				}
				saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
				if saved.State() != workmonth.StatePendingApproval {
					t.Errorf("認可で弾かれたのに状態が変化している State() = %q, want %q", saved.State(), workmonth.StatePendingApproval)
				}
				return
			}

			output := f.output.onlyPresented(t)
			if output.State != workmonth.StateDraft.String() {
				t.Errorf("State = %q, want %q", output.State, workmonth.StateDraft.String())
			}
		})
	}
}

// ---- 実装設計 AC-8-7（認証） ------------------------------------------------

// TestRejectWorkMonth_RejectsUnauthenticated は未認証（ゲスト）の差戻しを弾くことを
// 検証する（実装設計 AC-8-7・AC-7-13）。
func TestRejectWorkMonth_RejectsUnauthenticated(t *testing.T) {
	tests := []struct {
		name  string
		actor port.Actor
	}{
		{name: "操作者ヘッダを持たないゲストは弾く（AC-8-7）", actor: guestActor()},
		{
			name:  "承認者ロールの識別子でも未認証なら弾く（AC-8-7）",
			actor: port.Actor{ID: testOtherEngineerID, Role: port.RoleApprover, Authenticated: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			f.putPendingApproval(t)

			f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
				Actor:      tt.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrUnauthenticated) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("未認証なのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
		})
	}
}

// ---- 実装設計 AC-11-7（対象の実在） -----------------------------------------

// TestRejectWorkMonth_ContractNotFound は実在しない契約への差戻しを検証する。
func TestRejectWorkMonth_ContractNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
		Actor:      foreignActor(port.RoleApprover),
		ContractID: mustContractID(t, "ctr-unknown"),
		YearMonth:  f.yearMonth,
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrContractNotFound) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrContractNotFound)", err)
	}
	if f.workMonths.saveCount != 0 {
		t.Errorf("契約が実在しないのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
	}
}

// TestRejectWorkMonth_UngeneratedWorkMonthNotFound は未生成の年月への差戻しが
// ErrWorkMonthNotFound で弾かれることを検証する（approval.md AC-9／実装設計 AC-7-9・
// AC-7-13。`domain-api-http-contract.md` AC-8-8）。
func TestRejectWorkMonth_UngeneratedWorkMonthNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
		Actor:      foreignActor(port.RoleApprover),
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrWorkMonthNotFound) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrWorkMonthNotFound)", err)
	}
	if f.workMonths.saveCount != 0 {
		t.Errorf("未生成なのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
	}
}

// ---- 実装設計 AC-7-13・AC-8-11（判定順序） -----------------------------------

// TestRejectWorkMonth_JudgementOrder は差戻しの判定順序を検証する
// （実装設計 AC-7-13。承認と完全に同一の順序。①認証 → ②契約の実在 → ③勤務月の実在 →
// ④認可（承認者ロール→自己承認） → ⑤状態）。
// AC-12-8 が固定を求める2組（未生成×承認者でない、Draft×承認者でない）を含む。
func TestRejectWorkMonth_JudgementOrder(t *testing.T) {
	tests := []struct {
		name                 string
		actor                port.Actor
		useUnknownContractID bool
		generated            bool
		state                workmonth.State
		wantErr              error
	}{
		{
			name:    "ゲストは未認証が先（順1）",
			actor:   guestActor(),
			wantErr: port.ErrUnauthenticated,
		},
		{
			name:                 "ゲスト かつ 実在しない契約IDでも未認証が先（順1。認証は契約の Find より前）",
			actor:                guestActor(),
			useUnknownContractID: true,
			wantErr:              port.ErrUnauthenticated,
		},
		{
			name:                 "実在しない契約は ErrContractNotFound が先（順3。認可より先）",
			actor:                foreignActor(port.RoleEngineer),
			useUnknownContractID: true,
			wantErr:              port.ErrContractNotFound,
		},
		{
			name:      "未生成かつ承認者でなくても ErrWorkMonthNotFound が先（順3）",
			actor:     foreignActor(port.RoleEngineer),
			generated: false,
			wantErr:   port.ErrWorkMonthNotFound,
		},
		{
			name:      "生成済み Draft で承認者でなければ ErrNotApprover が先（順4。状態エラーではない）",
			actor:     foreignActor(port.RoleEngineer),
			generated: true,
			state:     workmonth.StateDraft,
			wantErr:   port.ErrNotApprover,
		},
		{
			name:      "承認者だが Draft なら ErrNotRejectable（順5。認可を満たした後の状態）",
			actor:     foreignActor(port.RoleApprover),
			generated: true,
			state:     workmonth.StateDraft,
			wantErr:   workmonth.ErrNotRejectable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			contractID := f.contractID
			if tt.useUnknownContractID {
				contractID = mustContractID(t, "ctr-unknown")
			}
			if tt.generated {
				f.workMonths.put(reconstructWorkMonth(
					t,
					f.contractID,
					f.yearMonth,
					mustSettlementRange(t, 140, 180),
					tt.state,
					[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
				))
			}

			f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
				Actor:      tt.actor,
				ContractID: contractID,
				YearMonth:  f.yearMonth,
			})

			err := f.output.onlyPresentedError(t)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
		})
	}
}

// ---- 実装設計 AC-11-11（保存の失敗） ----------------------------------------

// TestRejectWorkMonth_SaveFailure は保存の失敗が出力ポートへ渡ることを検証する。
func TestRejectWorkMonth_SaveFailure(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.putPendingApproval(t)
	f.workMonths.saveErr = errSaveFailed

	f.reject().Execute(context.Background(), port.RejectWorkMonthInput{
		Actor:      foreignActor(port.RoleApprover),
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, errSaveFailed) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, errSaveFailed)", err)
	}
	if f.workMonths.saveCount != 1 {
		t.Errorf("Save の呼び出し回数 = %d, want 1（保存は試みられる）", f.workMonths.saveCount)
	}
}
