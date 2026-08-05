package interactor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/interactor"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件:
//   - docs/specs/approval.md AC-1（状態別の可否）・AC-3（承認者ロール）・
//     AC-4（自己承認の排除）・AC-5（承認成立時の帰結）
//   - docs/specs/workmonth-implementation-design.md AC-7-12（入力と依存）・
//     AC-7-13（責務順序）・AC-7-14（承認の差分）・AC-8-11（認可の内訳と順序）
//   - docs/specs/domain-api-http-contract.md AC-7・AC-9（判定順序）

func (f *fixture) approve() *interactor.ApproveWorkMonth {
	return interactor.NewApproveWorkMonth(f.workMonths, f.contracts, f.output)
}

// putPendingApproval は締め済（PendingApproval）の勤務月を保存する。
//
// 確定済みの超過・不足には意図的に異なる値（超過=2時間30分・不足=0分）を与える。
// 超過・不足の一方は必ず0（`monthly-closing.md` AC-3-4）であり、Close() が
// 実際に到達しうる組み合わせはこれに限られる（超過・不足がともに正の値になる組は
// Close() では作れない。レビュー往復2 W-B）。それでも超過と不足を異なる値にするのは、
// 共有ヘルパー reconstructWorkMonth（enter_daily_record_test.go）・
// mustReconstructWorkMonth（workmonth_test.go）が超過・不足に同一の値を使っており、
// Approve() 内で両者を入れ替える変異を検出できないため（レビュー往復1 W-1）。
// 共有ヘルパーを変えると UC1・UC2 に波及するため、UC3 のテストが使う前提だけを
// workmonth.Reconstruct の直接呼び出しで作る。
func (f *fixture) putPendingApproval(t *testing.T) {
	t.Helper()
	excess := mustWorkingHours(t, 2, 30)
	shortfall := mustWorkingHours(t, 0, 0)
	target, err := workmonth.Reconstruct(
		f.contractID,
		f.yearMonth,
		mustSettlementRange(t, 140, 180),
		workmonth.StatePendingApproval,
		[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
		&excess,
		&shortfall,
	)
	if err != nil {
		t.Fatalf("前提の構築に失敗: Reconstruct: %v", err)
	}
	f.workMonths.put(target)
}

// ---- approval.md AC-1-1・AC-5 -----------------------------------------------

// TestApproveWorkMonth_StateTransition は承認の成立と、確定済みの超過／不足・
// 稼働実績・総稼働時間が締め時のまま出力・保存されることを検証する（AC-1-1・AC-5-1・
// AC-5-2。実装設計 AC-4-4・AC-7-14-c・AC-7-14-d）。
func TestApproveWorkMonth_StateTransition(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.putPendingApproval(t)

	f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
		Actor:      foreignActor(port.RoleApprover),
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	// 出力 DTO 全体を締め時の値と突き合わせる。超過・不足に加えて DailyRecords・
	// TotalHours・Generated・SettlementRange も締め時のまま（変化しない）ことを
	// 同時に固定する（AC-4-4。レビュー往復1 C-1・C-2）。
	wantExcess := port.Hours{Hours: 2, Minutes: 30}
	wantShortfall := port.Hours{Hours: 0, Minutes: 0}
	want := port.WorkMonthOutput{
		ContractID:          testContractID,
		ContractDisplayName: testDisplayName,
		YearMonth:           "2026-07",
		State:               workmonth.StateApproved.String(),
		Generated:           true,
		SettlementRange: port.SettlementRangeOutput{
			LowerBound: port.Hours{Hours: 140, Minutes: 0},
			UpperBound: port.Hours{Hours: 180, Minutes: 0},
		},
		TotalHours: port.Hours{Hours: 8, Minutes: 0},
		Excess:     &wantExcess,
		Shortfall:  &wantShortfall,
		DailyRecords: []port.DailyRecordOutput{
			{
				Date:                "2026-07-01",
				WorkingHours:        port.Hours{Hours: 8, Minutes: 0},
				RoundedWorkingHours: port.Hours{Hours: 8, Minutes: 0},
			},
		},
	}
	output := f.output.onlyPresented(t)
	if diff := cmp.Diff(want, output); diff != "" {
		t.Errorf("出力ポートへ渡された勤務月が不一致 (-want +got):\n%s（AC-5-2・AC-4-4）", diff)
	}
	if f.workMonths.saveCount != 1 {
		t.Errorf("Save の呼び出し回数 = %d, want 1", f.workMonths.saveCount)
	}
	saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
	if saved.State() != workmonth.StateApproved {
		t.Errorf("保存された状態 State() = %q, want %q", saved.State(), workmonth.StateApproved)
	}
}

// TestApproveWorkMonth_SavedWorkMonthRoundTrips は承認後に保存された勤務月が
// Approved かつ確定済みとして Fake の Reconstruct 経路で複製できることを検証する
// （実装設計 AC-12-7・AC-12-8・AC-5-9-c）。
func TestApproveWorkMonth_SavedWorkMonthRoundTrips(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.putPendingApproval(t)

	f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
		Actor:      foreignActor(port.RoleApprover),
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})
	if f.workMonths.saveCount != 1 {
		t.Fatalf("前提の構築に失敗: Save の呼び出し回数 = %d, want 1", f.workMonths.saveCount)
	}

	found, err := f.workMonths.Find(context.Background(), f.contractID, f.yearMonth)
	if err != nil {
		t.Fatalf("承認後の Find が失敗（ErrInvalidValue になっていないか。AC-5-9-c）: %v", err)
	}
	if found.State() != workmonth.StateApproved {
		t.Errorf("復元した State() = %q, want %q", found.State(), workmonth.StateApproved)
	}
	if _, ok := found.Excess(); !ok {
		t.Errorf("復元した Excess() の第2戻り値 = false, want true（承認後は確定済み）")
	}
}

// ---- approval.md AC-1-2・AC-1-3 ---------------------------------------------

// TestApproveWorkMonth_RejectsNonPendingApprovalState は PendingApproval 以外からの
// 承認を弾くことを検証する（AC-1-2 未締め・AC-1-3 二重承認。実装設計 AC-7-14-b の
// ErrNotApprovable）。
func TestApproveWorkMonth_RejectsNonPendingApprovalState(t *testing.T) {
	tests := []struct {
		name  string
		state workmonth.State
	}{
		{name: "下書き（Draft）からの承認は弾く（AC-1-2）", state: workmonth.StateDraft},
		{name: "承認済（Approved）からの承認は弾く（AC-1-3）", state: workmonth.StateApproved},
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

			f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
				Actor:      foreignActor(port.RoleApprover),
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, workmonth.ErrNotApprovable) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, ErrNotApprovable)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
		})
	}
}

// ---- 実装設計 AC-8-11（認可の内訳と順序） ------------------------------------

// TestApproveWorkMonth_Authorization は承認の認可（承認者ロール → 自己承認の2段）を
// 検証する（approval.md AC-3・AC-4／実装設計 AC-8-11）。
//
// 自己承認（Approver かつ本人）は、ErrNotApprover が先に返る組み合わせ
// （Engineer かつ本人）と対にして検査し、①承認者ロール→②自己承認の順序を固定する
// （AC-8-11「両方に該当する操作者には①の ErrNotApprover が返る」）。
func TestApproveWorkMonth_Authorization(t *testing.T) {
	tests := []struct {
		name    string
		actor   port.Actor
		wantErr error // nil なら許可
	}{
		{name: "承認者ロールを持ち本人でなければ許可（AC-3-1・AC-4-2）", actor: foreignActor(port.RoleApprover)},
		{
			name:    "承認者ロールを持ち本人なら自己承認で弾く（AC-4-1）",
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

			f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
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
				// 「認可で弾かれたのに保存済みの状態が変化していない」ことは、上の
				// saveCount == 0 の検査に尽きる。f.workMonths.Find は Reconstruct で
				// **複製**を返す（fake_test.go）ため、interactor が触る target は
				// 呼び出しごとの複製であり、f.stored（saved() が読む先）は Save を
				// 経ない限り変わらない。よって「弾かれた経路にだけ
				// _ = target.Approve() を挿入する」変異を入れても saved().State() は
				// 構造上ぜったいに変化せず、この形の追加検査は保護を提供しない
				// （レビュー往復2 W-C。往復1 C-1・C-2 と同じ「主張と実体の乖離」を
				// 残さないため、意図的に検査しない）。
				return
			}

			output := f.output.onlyPresented(t)
			if output.State != workmonth.StateApproved.String() {
				t.Errorf("State = %q, want %q", output.State, workmonth.StateApproved.String())
			}
		})
	}
}

// ---- 実装設計 AC-8-7（認証） ------------------------------------------------

// TestApproveWorkMonth_RejectsUnauthenticated は未認証（ゲスト）の承認を弾くことを
// 検証する（実装設計 AC-8-7・AC-7-13）。
func TestApproveWorkMonth_RejectsUnauthenticated(t *testing.T) {
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

			f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
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

// TestApproveWorkMonth_ContractNotFound は実在しない契約への承認を検証する。
func TestApproveWorkMonth_ContractNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
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

// TestApproveWorkMonth_UngeneratedWorkMonthNotFound は未生成の年月への承認が
// ErrWorkMonthNotFound で弾かれることを検証する（approval.md AC-9／実装設計 AC-7-9・
// AC-7-13。`domain-api-http-contract.md` AC-7-6）。
func TestApproveWorkMonth_UngeneratedWorkMonthNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
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

// TestApproveWorkMonth_JudgementOrder は承認の判定順序を検証する
// （実装設計 AC-7-13。①認証 → ②契約の実在 → ③勤務月の実在 → ④認可
// （承認者ロール→自己承認） → ⑤状態）。
// AC-12-8 が固定を求める3組（未生成×承認者でない、Draft×承認者でない、
// Draft×承認者ロールを持つ本人＝自己承認）を含む。3組目は④（認可）が⑤
// （target.Approve()）より先であることを、ロール判定の枝（2組目）だけでなく
// 自己承認の枝でも観測するために要る。状態に**成立する**組み合わせ
// （PendingApproval）で自己承認を検査すると、④と⑤の順序を入れ替えても
// 最終的に返るエラーは ErrSelfApproval のまま変わらず（⑤が先に成功しても
// ④が事後的に弾く）、順序の違いを検出できない（TestApproveWorkMonth_Authorization
// の自己承認ケースがこれに当たる）。**状態が不成立**の Draft を選ぶことで、
// ⑤が先に実行された場合は Approve() 自身のガード（ErrNotApprovable）が
// 先に返ってしまい、期待する ErrSelfApproval との差分として検出できる
// （レビュー往復2 C-1）。
func TestApproveWorkMonth_JudgementOrder(t *testing.T) {
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
			name:      "生成済み Draft で承認者ロールを持つ本人（自己承認）は ErrSelfApproval が先（順4。Approve() 自身の ErrNotApprovable ではない。AC-12-8 の3組目）",
			actor:     ownerActor(port.RoleApprover),
			generated: true,
			state:     workmonth.StateDraft,
			wantErr:   port.ErrSelfApproval,
		},
		{
			name:      "承認者だが Draft なら ErrNotApprovable（順5。認可を満たした後の状態）",
			actor:     foreignActor(port.RoleApprover),
			generated: true,
			state:     workmonth.StateDraft,
			wantErr:   workmonth.ErrNotApprovable,
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

			f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
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

// TestApproveWorkMonth_SaveFailure は保存の失敗が出力ポートへ渡ることを検証する。
func TestApproveWorkMonth_SaveFailure(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.putPendingApproval(t)
	f.workMonths.saveErr = errSaveFailed

	f.approve().Execute(context.Background(), port.ApproveWorkMonthInput{
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
