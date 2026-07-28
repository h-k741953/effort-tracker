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

// 検証対象の受け入れ条件（docs/specs/daily-record-entry.md）:
//   - AC-5-4（削除。以降その日は「レコードのない日＝稼働なし」）
//   - AC-5-2・AC-5-3（Draft 以外の削除は弾く）
//   - AC-1-5（入力以外の生成契機を設けない。削除は勤務月を生成しない）

func (f *fixture) delete() *interactor.DeleteDailyRecord {
	return interactor.NewDeleteDailyRecord(f.workMonths, f.contracts, f.output)
}

// ---- AC-5-4 --------------------------------------------------------------

// TestDeleteDailyRecord_RemovesRecord は下書きの勤務月からレコードを取り除くことを検証する。
// AC-5-4（削除後はレコードのない日として扱う。明示的なゼロ記録を残さない）。
// レコードの無い日への削除は成功として扱う（実装設計 AC-4-2・D-5）。
func TestDeleteDailyRecord_RemovesRecord(t *testing.T) {
	tests := []struct {
		name       string
		deleteDate [3]int
		want       []port.DailyRecordOutput
		wantTotal  port.Hours
	}{
		{
			name:       "レコードのある日を削除するとその日は消える（AC-5-4）",
			deleteDate: [3]int{2026, 7, 1},
			want: []port.DailyRecordOutput{
				{
					Date:                "2026-07-02",
					WorkingHours:        port.Hours{Hours: 7, Minutes: 0},
					RoundedWorkingHours: port.Hours{Hours: 7, Minutes: 0},
				},
			},
			wantTotal: port.Hours{Hours: 7, Minutes: 0},
		},
		{
			name:       "レコードの無い日の削除は成功し、他の日に影響しない（実装設計 AC-4-2）",
			deleteDate: [3]int{2026, 7, 10},
			want: []port.DailyRecordOutput{
				{
					Date:                "2026-07-01",
					WorkingHours:        port.Hours{Hours: 8, Minutes: 0},
					RoundedWorkingHours: port.Hours{Hours: 8, Minutes: 0},
				},
				{
					Date:                "2026-07-02",
					WorkingHours:        port.Hours{Hours: 7, Minutes: 0},
					RoundedWorkingHours: port.Hours{Hours: 7, Minutes: 0},
				},
			},
			wantTotal: port.Hours{Hours: 15, Minutes: 0},
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
				[]workmonth.DailyRecord{
					mustDailyRecord(t, 2026, 7, 1, 8, 0),
					mustDailyRecord(t, 2026, 7, 2, 7, 0),
				},
			))

			f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, tt.deleteDate[0], tt.deleteDate[1], tt.deleteDate[2]),
			})

			output := f.output.onlyPresented(t)
			if diff := cmp.Diff(tt.want, output.DailyRecords); diff != "" {
				t.Errorf("削除後の稼働実績が不一致 (-want +got):\n%s（AC-5-4）", diff)
			}
			if diff := cmp.Diff(tt.wantTotal, output.TotalHours); diff != "" {
				t.Errorf("削除後の TotalHours が不一致 (-want +got):\n%s", diff)
			}
			if !output.Generated {
				t.Errorf("Generated = false, want true（生成済みの勤務月に対する削除）")
			}
		})
	}
}

// ---- AC-1-5 --------------------------------------------------------------

// TestDeleteDailyRecord_DoesNotGenerateWorkMonth は、未生成の年月への削除が
// 勤務月を生成しないことを検証する（AC-1-5・D-6。実装設計 AC-7-9）。
// 応答は「未生成の空の表現」であり、精算幅は契約が現在定める値を返す
// （docs/specs/domain-api-http-contract.md AC-5-3・AC-2-2）。
func TestDeleteDailyRecord_DoesNotGenerateWorkMonth(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
	})

	if f.workMonths.saveCount != 0 {
		t.Errorf("Save の呼び出し回数 = %d, want 0（AC-1-5）", f.workMonths.saveCount)
	}
	if len(f.workMonths.stored) != 0 {
		t.Errorf("削除操作で勤務月が生成されている（件数 = %d, want 0）（AC-1-5）", len(f.workMonths.stored))
	}

	want := port.WorkMonthOutput{
		ContractID:          testContractID,
		ContractDisplayName: testDisplayName,
		YearMonth:           "2026-07",
		State:               "Draft",
		Generated:           false,
		SettlementRange: port.SettlementRangeOutput{
			LowerBound: port.Hours{Hours: 140, Minutes: 0},
			UpperBound: port.Hours{Hours: 180, Minutes: 0},
		},
		TotalHours:   port.Hours{Hours: 0, Minutes: 0},
		Excess:       nil,
		Shortfall:    nil,
		DailyRecords: []port.DailyRecordOutput{},
	}
	if diff := cmp.Diff(want, f.output.onlyPresented(t)); diff != "" {
		t.Errorf("未生成の年月に対する出力が不一致 (-want +got):\n%s", diff)
	}
}

// ---- 実装設計 AC-8-7（認証） ---------------------------------------------

// TestDeleteDailyRecord_RejectsUnauthenticated は未認証（ゲスト）の削除を弾くことを検証する。
// 実装設計 AC-8-7（更新操作はすべて弾く）／HTTP 契約 AC-1-6（401 UNAUTHENTICATED）。
func TestDeleteDailyRecord_RejectsUnauthenticated(t *testing.T) {
	tests := []struct {
		name      string
		actor     port.Actor
		generated bool
	}{
		{name: "操作者ヘッダを持たないゲストは弾く（AC-8-7）", actor: guestActor(), generated: true},
		{
			name:      "識別子が本人と一致していても未認証なら弾く（AC-8-7）",
			actor:     port.Actor{ID: testEngineerID, Role: port.RoleEngineer, Authenticated: false},
			generated: true,
		},
		{name: "未生成の年月でも未認証は弾く（AC-8-7）", actor: guestActor(), generated: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			if tt.generated {
				f.workMonths.put(reconstructWorkMonth(
					t,
					f.contractID,
					f.yearMonth,
					mustSettlementRange(t, 140, 180),
					workmonth.StateDraft,
					[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
				))
			}

			f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
				Actor:      tt.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, 2026, 7, 1),
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrUnauthenticated) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("未認証なのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
			if tt.generated {
				saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
				if len(saved.DailyRecords()) != 1 {
					t.Errorf("未認証なのに稼働実績が削除されている（件数 = %d, want 1）", len(saved.DailyRecords()))
				}
			}
		})
	}
}

// ---- 実装設計 AC-8-1（認可） ---------------------------------------------

// TestDeleteDailyRecord_Authorization は削除の認可を検証する。
// 実装設計 AC-8-1（本人のみ。ロールは問わない）／HTTP 契約 AC-5-4（403 FORBIDDEN_NOT_OWNER）。
//
// **未生成の年月でも認可を判定する**（HTTP 契約 AC-5-4）。未生成判定より先に認可を置くのは、
// 本人以外に「その年月が未生成である」ことを 200 で教えないためであり、
// この順序が崩れると他人へ成功が返る。
func TestDeleteDailyRecord_Authorization(t *testing.T) {
	actors := []struct {
		name    string
		actor   port.Actor
		wantErr error // nil なら許可
	}{
		{name: "本人（技術者ロール）は許可（AC-8-1）", actor: ownerActor(port.RoleEngineer)},
		{name: "本人なら承認者ロールでも許可（AC-8-1）", actor: ownerActor(port.RoleApprover)},
		{name: "他の技術者は弾く（HTTP AC-5-4）", actor: foreignActor(port.RoleEngineer), wantErr: port.ErrNotOwner},
		{name: "他人が承認者ロールでも弾く（ロールは問わない。AC-8-1）", actor: foreignActor(port.RoleApprover), wantErr: port.ErrNotOwner},
	}
	states := []struct {
		name      string
		generated bool
	}{
		{name: "生成済みの勤務月", generated: true},
		{name: "未生成の年月（HTTP AC-5-3・AC-5-4）", generated: false},
	}

	for _, ac := range actors {
		for _, st := range states {
			t.Run(ac.name+"/"+st.name, func(t *testing.T) {
				f := newFixture(t, mustDate(t, 2026, 8, 15))
				if st.generated {
					f.workMonths.put(reconstructWorkMonth(
						t,
						f.contractID,
						f.yearMonth,
						mustSettlementRange(t, 140, 180),
						workmonth.StateDraft,
						[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
					))
				}

				f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
					Actor:      ac.actor,
					ContractID: f.contractID,
					YearMonth:  f.yearMonth,
					Date:       mustDate(t, 2026, 7, 1),
				})

				if ac.wantErr != nil {
					if err := f.output.onlyPresentedError(t); !errors.Is(err, ac.wantErr) {
						t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, ac.wantErr)
					}
					if f.workMonths.saveCount != 0 {
						t.Errorf("認可で弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
					}
					if st.generated {
						saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
						if len(saved.DailyRecords()) != 1 {
							t.Errorf("認可で弾かれたのに稼働実績が削除されている（件数 = %d, want 1）", len(saved.DailyRecords()))
						}
					}
					return
				}

				output := f.output.onlyPresented(t)
				if diff := cmp.Diff([]port.DailyRecordOutput{}, output.DailyRecords); diff != "" {
					t.Errorf("削除後の稼働実績が不一致 (-want +got):\n%s", diff)
				}
				if output.Generated != st.generated {
					t.Errorf("Generated = %v, want %v", output.Generated, st.generated)
				}
			})
		}
	}
}

// ---- 実装設計 AC-11-7・AC-11-11（対象の実在・想定外のエラー） -------------

// TestDeleteDailyRecord_ContractNotFound は実在しない契約への削除を検証する
// （HTTP 契約 AC-5-6。404 CONTRACT_NOT_FOUND）。
func TestDeleteDailyRecord_ContractNotFound(t *testing.T) {
	f := newFixture(t, mustDate(t, 2026, 8, 15))

	f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
		Actor:      f.actor,
		ContractID: mustContractID(t, "ctr-unknown"),
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, port.ErrContractNotFound) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrContractNotFound)", err)
	}
	if f.workMonths.saveCount != 0 {
		t.Errorf("契約が実在しないのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
	}
}

// TestDeleteDailyRecord_SaveFailure は保存の失敗が出力ポートへ渡ることを検証する
// （実装設計 AC-11-11／HTTP 契約 AC-9 の INTERNAL_ERROR）。
func TestDeleteDailyRecord_SaveFailure(t *testing.T) {
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

	f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
		Actor:      f.actor,
		ContractID: f.contractID,
		YearMonth:  f.yearMonth,
		Date:       mustDate(t, 2026, 7, 1),
	})

	if err := f.output.onlyPresentedError(t); !errors.Is(err, errSaveFailed) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, errSaveFailed)", err)
	}
	if f.workMonths.saveCount != 1 {
		t.Errorf("Save の呼び出し回数 = %d, want 1（保存は試みられる）", f.workMonths.saveCount)
	}
	// 以下は Fake の忠実性を守るためのアサーション（検証対象は interactor ではない）。
	// Save がエラーを返したのに Fake 側へ反映されていると、後続のテストが
	// 「保存されたこと」を偽って観測できてしまう。
	saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
	if len(saved.DailyRecords()) != 1 {
		t.Errorf("Fake が保存失敗時に削除を反映している（件数 = %d, want 1）", len(saved.DailyRecords()))
	}
}

// ---- AC-5-2・AC-5-3 ------------------------------------------------------

// ---- daily-record-entry.md AC-5-5・D-9／HTTP 契約 AC-5-8・AC-5-9・D-13（Issue #51） --

// TestDeleteDailyRecord_RejectsDateOutOfMonth は、当該勤務月の年月に属さない対象日への
// 削除が、勤務月の生成有無を問わず弾かれることを検証する
// （daily-record-entry.md AC-5-5・D-9／HTTP 契約 AC-5-8・D-13／実装設計 AC-7-9）。
//
// 未生成の年月への削除は本来 200 no-op（HTTP 契約 AC-5-3・実装設計 AC-7-9 前段）だが、
// 対象日が当該年月に属さない場合はその扱いより優先して弾く（HTTP 契約 D-13
// 「未生成の年月への削除を 200 とする AC-5-3 の扱いより優先する」）。
func TestDeleteDailyRecord_RejectsDateOutOfMonth(t *testing.T) {
	tests := []struct {
		name       string
		generated  bool
		deleteDate [3]int
	}{
		{name: "生成済みの勤務月で翌月初日は弾く（AC-5-5・D-9）", generated: true, deleteDate: [3]int{2026, 8, 1}},
		{name: "生成済みの勤務月で前月末日は弾く（AC-5-5・D-9）", generated: true, deleteDate: [3]int{2026, 6, 30}},
		{
			name:       "未生成の年月でも翌月初日は弾く（生成有無によらない。AC-7-9・D-13）",
			generated:  false,
			deleteDate: [3]int{2026, 8, 1},
		},
		{
			name:       "未生成の年月でも前月末日は弾く（生成有無によらない。AC-7-9・D-13）",
			generated:  false,
			deleteDate: [3]int{2026, 6, 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, mustDate(t, 2026, 8, 15))
			if tt.generated {
				f.workMonths.put(reconstructWorkMonth(
					t,
					f.contractID,
					f.yearMonth,
					mustSettlementRange(t, 140, 180),
					workmonth.StateDraft,
					[]workmonth.DailyRecord{mustDailyRecord(t, 2026, 7, 1, 8, 0)},
				))
			}

			f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, tt.deleteDate[0], tt.deleteDate[1], tt.deleteDate[2]),
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, workmonth.ErrDateOutOfMonth) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, workmonth.ErrDateOutOfMonth)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
			if tt.generated {
				saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
				if len(saved.DailyRecords()) != 1 {
					t.Errorf("弾かれたのに稼働実績が変化している（件数 = %d, want 1）", len(saved.DailyRecords()))
				}
			}
		})
	}
}

// TestDeleteDailyRecord_DateOutOfMonthJudgedLast は、当該年月外への削除（400 相当。
// HTTP 契約 AC-9 の順 6「業務バリデーション」）が、認証・存在・認可・状態の判定より
// 後に判定されることを検証する（HTTP 契約 AC-5-9・AC-9 判定順序表）。
//
// いずれのケースも削除対象日は当該年月外（2026-08-01）に固定し、他の条件が
// ErrDateOutOfMonth より先に返ることを確認する。
func TestDeleteDailyRecord_DateOutOfMonthJudgedLast(t *testing.T) {
	dateOutOfMonth := [3]int{2026, 8, 1}

	tests := []struct {
		name                 string
		actor                port.Actor
		useUnknownContractID bool
		generated            bool
		state                workmonth.State
		wantErr              error
	}{
		{
			name:      "ゲストは 401 相当（ErrUnauthenticated）が先（AC-9 順1）",
			actor:     guestActor(),
			generated: true,
			state:     workmonth.StateDraft,
			wantErr:   port.ErrUnauthenticated,
		},
		{
			name:      "未生成でもゲストは 401 相当が先（AC-9 順1）",
			actor:     guestActor(),
			generated: false,
			wantErr:   port.ErrUnauthenticated,
		},
		{
			name:                 "実在しない契約は 404 相当（ErrContractNotFound）が先（AC-9 順3）",
			actor:                ownerActor(port.RoleEngineer),
			useUnknownContractID: true,
			wantErr:              port.ErrContractNotFound,
		},
		{
			name:      "本人でなければ 403 相当（ErrNotOwner）が先（AC-9 順4）",
			actor:     foreignActor(port.RoleEngineer),
			generated: true,
			state:     workmonth.StateDraft,
			wantErr:   port.ErrNotOwner,
		},
		{
			name:      "未生成でも本人でなければ 403 相当が先（AC-9 順4）",
			actor:     foreignActor(port.RoleEngineer),
			generated: false,
			wantErr:   port.ErrNotOwner,
		},
		{
			name:      "締め済（PendingApproval）は 409 相当（ErrNotEditable）が先（AC-9 順5）",
			actor:     ownerActor(port.RoleEngineer),
			generated: true,
			state:     workmonth.StatePendingApproval,
			wantErr:   workmonth.ErrNotEditable,
		},
		{
			name:      "承認済（Approved）は 409 相当が先（AC-9 順5）",
			actor:     ownerActor(port.RoleEngineer),
			generated: true,
			state:     workmonth.StateApproved,
			wantErr:   workmonth.ErrNotEditable,
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

			f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
				Actor:      tt.actor,
				ContractID: contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, dateOutOfMonth[0], dateOutOfMonth[1], dateOutOfMonth[2]),
			})

			err := f.output.onlyPresentedError(t)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if errors.Is(err, workmonth.ErrDateOutOfMonth) {
				t.Fatalf("PresentError に渡されたエラー = %v が ErrDateOutOfMonth にも該当している。"+
					"業務バリデーション（順6）より優先度の高い判定が先に返るべき（HTTP 契約 AC-9）", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
		})
	}
}

// TestDeleteDailyRecord_RejectsNotEditableState は Draft 以外の状態で削除を弾くことを検証する
// （AC-5-2 締め済・AC-5-3 承認済）。
func TestDeleteDailyRecord_RejectsNotEditableState(t *testing.T) {
	tests := []struct {
		name  string
		state workmonth.State
	}{
		{name: "締め済は弾く（AC-5-2）", state: workmonth.StatePendingApproval},
		{name: "承認済は弾く（AC-5-3）", state: workmonth.StateApproved},
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

			f.delete().Execute(context.Background(), port.DeleteDailyRecordInput{
				Actor:      f.actor,
				ContractID: f.contractID,
				YearMonth:  f.yearMonth,
				Date:       mustDate(t, 2026, 7, 1),
			})

			if err := f.output.onlyPresentedError(t); !errors.Is(err, workmonth.ErrNotEditable) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, ErrNotEditable)", err)
			}
			if f.workMonths.saveCount != 0 {
				t.Errorf("弾かれたのに Save が呼ばれた（回数 = %d, want 0）", f.workMonths.saveCount)
			}
			saved := f.workMonths.saved(t, f.contractID, f.yearMonth)
			if len(saved.DailyRecords()) != 1 {
				t.Errorf("弾かれたのに稼働実績が削除されている（件数 = %d, want 1）", len(saved.DailyRecords()))
			}
		})
	}
}
