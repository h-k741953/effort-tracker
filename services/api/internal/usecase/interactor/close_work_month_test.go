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
//   - docs/specs/monthly-closing.md AC-1（Draft のみ締められる）・AC-2（本人のみ）・
//     AC-3（超過／不足の算出・保存）・AC-4（境界を含む算出）・AC-5（状態遷移）・
//     AC-7（未生成は締められない。生成済みの空月・部分月は締められる）
//   - docs/specs/workmonth-implementation-design.md AC-7-10（CloseWorkMonth の
//     入力と責務順序）・AC-7-11（出力 DTO の超過／不足）・AC-8-2（認可）
//   - docs/specs/domain-api-http-contract.md AC-6-9（判定順序）

func (f *fixture) close() *interactor.CloseWorkMonth {
	return interactor.NewCloseWorkMonth(f.workMonths, f.contracts, f.output)
}

// recordsSummingTo は合計が totalMinutes 分になるよう、2026年7月の連続した日へ
// 分割した稼働実績を組み立てる。1日あたり20時間（1200分）以下に収め、
// DailyRecord の1日の上限（24時間）に抵触しないようにする。
// totalMinutes は15分単位でなければならない（呼び出し側が保証する）。
func recordsSummingTo(t *testing.T, totalMinutes int) []workmonth.DailyRecord {
	t.Helper()
	if totalMinutes%15 != 0 {
		t.Fatalf("前提の構築に失敗: %d分は15分単位ではない", totalMinutes)
	}

	const maxPerDayMinutes = 20 * 60

	var records []workmonth.DailyRecord
	day := 1
	remaining := totalMinutes
	for remaining > 0 {
		minutes := remaining
		if minutes > maxPerDayMinutes {
			minutes = maxPerDayMinutes
		}
		records = append(records, mustDailyRecord(t, 2026, 7, day, minutes/60, minutes%60))
		remaining -= minutes
		day++
	}
	return records
}

// ---- monthly-closing.md AC-1-1・AC-3・AC-4・AC-5-1 ------------------------

// TestCloseWorkMonth_ClosesDraftAndComputesExcessShortfall は Draft の勤務月を
// 締め、超過／不足を算出して出力 DTO へ詰めることを検証する
// （AC-1-1・AC-3-1・AC-4・AC-5-1。実装設計 AC-7-11）。
func TestCloseWorkMonth_ClosesDraftAndComputesExcessShortfall(t *testing.T) {
	tests := []struct {
		name          string
		totalMinutes  int
		wantExcess    port.Hours
		wantShortfall port.Hours
	}{
		{
			name:          "上限超過（AC-4-1 相当）",
			totalMinutes:  180*60 + 30,
			wantExcess:    port.Hours{Hours: 0, Minutes: 30},
			wantShortfall: port.Hours{Hours: 0, Minutes: 0},
		},
		{
			name:          "範囲内（AC-4-3 相当）",
			totalMinutes:  150 * 60,
			wantExcess:    port.Hours{Hours: 0, Minutes: 0},
			wantShortfall: port.Hours{Hours: 0, Minutes: 0},
		},
		{
			name:          "下限未達（AC-4-6 相当）",
			totalMinutes:  100 * 60,
			wantExcess:    port.Hours{Hours: 0, Minutes: 0},
			wantShortfall: port.Hours{Hours: 40, Minutes: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			f.workMonths.put(reconstructWorkMonth(
				t,
				f.contractID,
				f.yearMonth,
				mustSettlementRange(t, 140, 180),
				workmonth.StateDraft,
				recordsSummingTo(t, tt.totalMinutes),
			))

			f.close().Execute(context.Background(), port.CloseWorkMonthInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
			})

			output := f.output.onlyPresented(t)
			if output.State != workmonth.StatePendingApproval.String() {
				t.Errorf("State = %q, want %q", output.State, workmonth.StatePendingApproval.String())
			}
			if diff := cmp.Diff(&tt.wantExcess, output.Excess); diff != "" {
				t.Errorf("Excess が不一致 (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(&tt.wantShortfall, output.Shortfall); diff != "" {
				t.Errorf("Shortfall が不一致 (-want +got):\n%s", diff)
			}
			if f.workMonths.saveCount != 1 {
				t.Errorf("Save の呼び出し回数 = %d, want 1", f.workMonths.saveCount)
			}
			saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
			if saved.State() != workmonth.StatePendingApproval {
				t.Errorf("保存された状態 State() = %q, want %q", saved.State(), workmonth.StatePendingApproval)
			}
		})
	}
}

// ---- monthly-closing.md AC-1-2・AC-1-3 -------------------------------------

// TestCloseWorkMonth_RejectsNonDraftState は Draft 以外からの締めを弾くことを検証する
// （AC-1-2 二重締め・AC-1-3 終端状態。実装設計 AC-4-3 の ErrNotClosable）。
func TestCloseWorkMonth_RejectsNonDraftState(t *testing.T) {
	tests := []struct {
		name  string
		state workmonth.State
	}{
		{name: "締め済（PendingApproval）からの締めは弾く（AC-1-2）", state: workmonth.StatePendingApproval},
		{name: "承認済（Approved）からの締めは弾く（AC-1-3）", state: workmonth.StateApproved},
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

			f.close().Execute(context.Background(), port.CloseWorkMonthInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, workmonth.ErrNotClosable) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, ErrNotClosable)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
		})
	}
}

// ---- 実装設計 AC-8-2（認可） -----------------------------------------------

// TestCloseWorkMonth_Authorization は締めの認可を検証する。
// 実装設計 AC-8-2（本人のみ。ロールは問わない）／monthly-closing.md AC-2-2・AC-2-3
// （他の技術者・承認者の代行締めは無い）／HTTP 契約 AC-6-3。
func TestCloseWorkMonth_Authorization(t *testing.T) {
	tests := []struct {
		name    string
		actor   port.Actor
		wantErr error // nil なら許可
	}{
		{name: "本人（技術者ロール）は許可（AC-8-2）", actor: ownerActor(port.RoleEngineer)},
		{name: "本人なら承認者ロールでも許可（AC-8-2）", actor: ownerActor(port.RoleApprover)},
		{name: "他の技術者は弾く（代行締めは無い。AC-2-2）", actor: foreignActor(port.RoleEngineer), wantErr: port.ErrNotOwner},
		{name: "承認者でも代行締めは弾く（AC-2-3）", actor: foreignActor(port.RoleApprover), wantErr: port.ErrNotOwner},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			f.workMonths.put(reconstructWorkMonth(
				t,
				f.contractID,
				f.yearMonth,
				mustSettlementRange(t, 140, 180),
				workmonth.StateDraft,
				[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			))

			f.close().Execute(context.Background(), port.CloseWorkMonthInput{
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
				if saved.State() != workmonth.StateDraft {
					t.Errorf("認可で弾かれたのに状態が変化している State() = %q, want %q", saved.State(), workmonth.StateDraft)
				}
				return
			}

			output := f.output.onlyPresented(t)
			if output.State != workmonth.StatePendingApproval.String() {
				t.Errorf("State = %q, want %q", output.State, workmonth.StatePendingApproval.String())
			}
		})
	}
}

// ---- 実装設計 AC-8-7（認証） ------------------------------------------------

// TestCloseWorkMonth_RejectsUnauthenticated は未認証（ゲスト）の締めを弾くことを検証する。
// 実装設計 AC-8-7（更新操作はすべて弾く）。
func TestCloseWorkMonth_RejectsUnauthenticated(t *testing.T) {
	tests := []struct {
		name  string
		actor port.Actor
	}{
		{name: "操作者ヘッダを持たないゲストは弾く（AC-8-7）", actor: guestActor()},
		{
			name:  "識別子が本人と一致していても未認証なら弾く（AC-8-7）",
			actor: port.Actor{ID: testEngineerID, Role: port.RoleEngineer, Authenticated: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			f.workMonths.put(reconstructWorkMonth(
				t,
				f.contractID,
				f.yearMonth,
				mustSettlementRange(t, 140, 180),
				workmonth.StateDraft,
				[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
			))

			f.close().Execute(context.Background(), port.CloseWorkMonthInput{
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

// TestCloseWorkMonth_ContractNotFound は実在しない契約への締めを検証する。
func TestCloseWorkMonth_ContractNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.close().Execute(context.Background(), port.CloseWorkMonthInput{
		Actor:      f.actor,
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

// TestCloseWorkMonth_UngeneratedWorkMonthNotFound は、一度も入力されていない
// 未生成の年月への締めが ErrWorkMonthNotFound で弾かれることを検証する
// （monthly-closing.md AC-7-4／実装設計 AC-4-3・AC-7-9・AC-7-10）。
// 締めは勤務月の生成契機ではない（EnterDailyRecord とは異なり暗黙生成しない）。
func TestCloseWorkMonth_UngeneratedWorkMonthNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.close().Execute(context.Background(), port.CloseWorkMonthInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrWorkMonthNotFound) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrWorkMonthNotFound)", err)
	}
	if f.workMonths.saveCount != 0 {
		t.Errorf("未生成なのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
	}
	if len(f.workMonths.stored) != 0 {
		t.Errorf("締めが勤務月を生成している（件数 = %d, want 0）（AC-7-9）", len(f.workMonths.stored))
	}
}

// ---- domain-api-http-contract.md AC-6-9（判定順序） -------------------------

// TestCloseWorkMonth_JudgementOrder は締めの判定順序を検証する
// （AC-6-9／実装設計 AC-7-10。①認証 → ②契約の実在 → ③勤務月の実在 → ④認可 → ⑤状態）。
// 未生成かつ本人でない要求には ErrWorkMonthNotFound が先に返る
// （本人でなくても未生成であることは教える。AC-6-9「操作者が本人でなくても404が先に返る」）。
func TestCloseWorkMonth_JudgementOrder(t *testing.T) {
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
			name:                 "実在しない契約は ErrContractNotFound が先（順3。認可より先）",
			actor:                foreignActor(port.RoleEngineer),
			useUnknownContractID: true,
			wantErr:              port.ErrContractNotFound,
		},
		{
			name:      "未生成かつ本人でなくても ErrWorkMonthNotFound が先（順3）",
			actor:     foreignActor(port.RoleEngineer),
			generated: false,
			wantErr:   port.ErrWorkMonthNotFound,
		},
		{
			name:      "生成済みで本人でなければ ErrNotOwner（順4）",
			actor:     foreignActor(port.RoleEngineer),
			generated: true,
			state:     workmonth.StateDraft,
			wantErr:   port.ErrNotOwner,
		},
		{
			name:      "本人だが締め済なら ErrNotClosable（順5）",
			actor:     ownerActor(port.RoleEngineer),
			generated: true,
			state:     workmonth.StatePendingApproval,
			wantErr:   workmonth.ErrNotClosable,
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

			f.close().Execute(context.Background(), port.CloseWorkMonthInput{
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

// TestCloseWorkMonth_SaveFailure は保存の失敗が出力ポートへ渡ることを検証する。
func TestCloseWorkMonth_SaveFailure(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.workMonths.put(reconstructWorkMonth(
		t,
		f.contractID,
		f.yearMonth,
		mustSettlementRange(t, 140, 180),
		workmonth.StateDraft,
		[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
	))
	f.workMonths.saveErr = errSaveFailed

	f.close().Execute(context.Background(), port.CloseWorkMonthInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, errSaveFailed) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, errSaveFailed)", err)
	}
	if f.workMonths.saveCount != 1 {
		t.Errorf("Save の呼び出し回数 = %d, want 1（保存は試みられる）", f.workMonths.saveCount)
	}
	// 以下は Fake の忠実性を守るためのアサーション（検証対象は interactor ではない）。
	saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
	if saved.State() != workmonth.StateDraft {
		t.Errorf("Fake が保存失敗時に状態変化を反映している State() = %q, want %q", saved.State(), workmonth.StateDraft)
	}
}

// ---- 実装設計 AC-5-8（算出は保持する精算幅のみを使い、契約の現在値を参照しない） ----

// TestCloseWorkMonth_UsesStoredSettlementRangeNotCurrentContract は、超過／不足の
// 算出が勤務月の保持する精算幅スナップショットのみを使い、契約の現在値を
// 参照しないことを検証する（実装設計 AC-5-8）。
//
// 勤務月の精算幅（100〜200時間）と契約の現在の精算幅（140〜180時間。newFixture の既定値）を
// あえて食い違わせる。総稼働時間190時間は、勤務月側の幅（100〜200）では範囲内
// （超過・不足とも0）だが、契約の現在値（140〜180）を誤って参照すると超過10時間になる。
func TestCloseWorkMonth_UsesStoredSettlementRangeNotCurrentContract(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.workMonths.put(reconstructWorkMonth(
		t,
		f.contractID,
		f.yearMonth,
		mustSettlementRange(t, 100, 200),
		workmonth.StateDraft,
		recordsSummingTo(t, 190*60),
	))

	f.close().Execute(context.Background(), port.CloseWorkMonthInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})

	output := f.output.onlyPresented(t)
	wantExcess := port.Hours{Hours: 0, Minutes: 0}
	wantShortfall := port.Hours{Hours: 0, Minutes: 0}
	if diff := cmp.Diff(&wantExcess, output.Excess); diff != "" {
		t.Errorf("Excess が不一致 (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(&wantShortfall, output.Shortfall); diff != "" {
		t.Errorf("Shortfall が不一致 (-want +got):\n%s", diff)
	}
}

// ---- 実装設計 AC-12-7（Fake が集約を複製する経路） --------------------------

// TestFakeWorkMonthRepository_PreservesExcessShortfallOnFind は Fake の Find が
// 複製を返す際に、確定済みの超過／不足も引き継ぐことを検証する
// （実装設計 AC-12-7）。引き継がないと締め済の勤務月が Find のたびに未確定へ
// 戻ってしまい、テストが実装より弱くなる。
func TestFakeWorkMonthRepository_PreservesExcessShortfallOnFind(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))
	f.workMonths.put(reconstructWorkMonth(
		t,
		f.contractID,
		f.yearMonth,
		mustSettlementRange(t, 140, 180),
		workmonth.StateDraft,
		recordsSummingTo(t, 139*60+45),
	))

	f.close().Execute(context.Background(), port.CloseWorkMonthInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
	})
	if f.workMonths.saveCount != 1 {
		t.Fatalf("前提の構築に失敗: Save の呼び出し回数 = %d, want 1", f.workMonths.saveCount)
	}

	found, err := f.workMonths.Find(context.Background(), f.contractID, f.yearMonth)
	if err != nil {
		t.Fatalf("Find が失敗: %v", err)
	}

	gotShortfall, ok := found.Shortfall()
	if !ok {
		t.Fatalf("Find で復元した勤務月の Shortfall() ok = false, want true（締め済のはず）")
	}
	if diff := cmp.Diff(hoursView{Hours: 0, Minutes: 15}, viewOfHours(gotShortfall)); diff != "" {
		t.Errorf("Find で復元した Shortfall() が不一致 (-want +got):\n%s", diff)
	}
}
