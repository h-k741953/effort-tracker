package workmonth_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// 検証対象の受け入れ条件:
//   - docs/specs/daily-record-entry.md AC-2-1・AC-2-1-a・AC-2-6（時刻の保持と読み出し）
//   - 同 AC-3（時刻の検証と稼働時間の算出。入力と期待は仕様書の表から読む＝spec_table_test.go）
//   - docs/specs/workmonth-implementation-design.md AC-3-5〜AC-3-5-b・AC-3-12〜AC-3-15・
//     AC-11-4・AC-11-4-a・AC-12-18 ①
//
// 署名は AC-3-15 の表。期待の出所は daily-record-entry.md の表であり、テストコードへ手で写さない。

// ---- テスト補助 ----------------------------------------------------------

func mustTimeOfDay(t *testing.T, hour, minute int) workmonth.TimeOfDay {
	t.Helper()
	tod, err := workmonth.NewTimeOfDay(hour, minute)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewTimeOfDay(%d, %d): %v", hour, minute, err)
	}
	return tod
}

func mustBreak(t *testing.T, startHour, startMinute, endHour, endMinute int) workmonth.Break {
	t.Helper()
	b, err := workmonth.NewBreak(mustTimeOfDay(t, startHour, startMinute), mustTimeOfDay(t, endHour, endMinute))
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewBreak(%d:%02d, %d:%02d): %v", startHour, startMinute, endHour, endMinute, err)
	}
	return b
}

// mustDailyRecordAt は時刻から稼働実績を構築する。brk が nil なら休憩なし。
func mustDailyRecordAt(t *testing.T, date workmonth.Date, start, end [2]int, brk *workmonth.Break) workmonth.DailyRecord {
	t.Helper()
	r, err := workmonth.NewDailyRecord(date, mustTimeOfDay(t, start[0], start[1]), mustTimeOfDay(t, end[0], end[1]), brk)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewDailyRecord(%v〜%v): %v", start, end, err)
	}
	return r
}

type todView struct {
	Hour   int
	Minute int
}

func viewOfTimeOfDay(v workmonth.TimeOfDay) todView {
	return todView{Hour: v.Hour(), Minute: v.Minute()}
}

type breakView struct {
	Start todView
	End   todView
}

func viewOfBreak(b workmonth.Break) breakView {
	return breakView{Start: viewOfTimeOfDay(b.Start()), End: viewOfTimeOfDay(b.End())}
}

// recordTimesView は稼働実績が保持した時刻と、有無を判別できる休憩を go-cmp で比べる射影。
type recordTimesView struct {
	Start    todView
	End      todView
	HasBreak bool
	Break    breakView
}

func viewOfRecordTimes(r workmonth.DailyRecord) recordTimesView {
	b, ok := r.Break()
	return recordTimesView{
		Start:    viewOfTimeOfDay(r.StartTime()),
		End:      viewOfTimeOfDay(r.EndTime()),
		HasBreak: ok,
		Break:    viewOfBreak(b),
	}
}

// buildFromRow は仕様書の1行の入力から、NewTimeOfDay → NewBreak → NewDailyRecord の順に構築する。
// 失敗した段（"break" / "record"）を返す。
func buildFromRow(t *testing.T, row specRow) (rec workmonth.DailyRecord, stage string, err error) {
	t.Helper()
	date := mustDate(t, 2026, 7, 1)
	start := mustTimeOfDay(t, row.Start[0], row.Start[1])
	end := mustTimeOfDay(t, row.End[0], row.End[1])
	var brk *workmonth.Break
	if row.Break != nil {
		bs := mustTimeOfDay(t, row.Break[0][0], row.Break[0][1])
		be := mustTimeOfDay(t, row.Break[1][0], row.Break[1][1])
		b, berr := workmonth.NewBreak(bs, be)
		if berr != nil {
			return workmonth.DailyRecord{}, "break", berr
		}
		brk = &b
	}
	rec, err = workmonth.NewDailyRecord(date, start, end, brk)
	return rec, "record", err
}

// assertCombinationError は時刻の組み合わせの不正の番兵だけに当たることを検査する（AC-3-14 ②）。
// 両方向を検査する（片方向だけでは両方の番兵に当たるエラーを返す実装が Green になる）。
func assertCombinationError(t *testing.T, label string, err error) {
	t.Helper()
	if !errors.Is(err, workmonth.ErrInvalidTimeCombination) {
		t.Errorf("%s: エラー = %v, want errors.Is(err, ErrInvalidTimeCombination)", label, err)
	}
	if errors.Is(err, workmonth.ErrTimeOfDayOutOfRange) {
		t.Errorf("%s: エラー %v が ErrTimeOfDayOutOfRange にも当たる（番兵を兼ねない＝AC-3-14）", label, err)
	}
	if errors.Is(err, workmonth.ErrInvalidValue) {
		t.Errorf("%s: エラー %v が ErrInvalidValue にも当たる（永続化行の破損と区別する＝AC-9-15-h）", label, err)
	}
}

// ---- AC-3-12-a・3-12-b: TimeOfDay -----------------------------------------

// TestNewTimeOfDay は時刻の粒度と値域を検証する（daily-record-entry.md AC-3 の 3-A）。
// 許可・弾く行の値は仕様書の表の例示から読み、表が例示を持たない類（3-1-a の「分が負」、
// 3-2-a の「時が負、または 25 以上」）だけ、その類の代表値をここで補う。
func TestNewTimeOfDay(t *testing.T) {
	type tc struct {
		name         string
		hour, minute int
	}

	var allowed, rejected []tc
	for _, id := range []string{"3-1", "3-2"} {
		tokens := specTimeTokens(t, id)
		if len(tokens) == 0 {
			t.Fatalf("仕様書の表から行 %s の時刻を読めなかった", id)
		}
		for _, tk := range tokens {
			allowed = append(allowed, tc{name: fmt.Sprintf("%s: %d:%02d は許可", id, tk[0], tk[1]), hour: tk[0], minute: tk[1]})
		}
	}
	for _, id := range []string{"3-1-a", "3-2-b"} {
		tokens := specTimeTokens(t, id)
		if len(tokens) == 0 {
			t.Fatalf("仕様書の表から行 %s の時刻を読めなかった", id)
		}
		for _, tk := range tokens {
			rejected = append(rejected, tc{name: fmt.Sprintf("%s: %d:%d は弾く", id, tk[0], tk[1]), hour: tk[0], minute: tk[1]})
		}
	}
	rejected = append(rejected,
		// 3-1-a「分が負」。-15 は 15 の倍数であり、剰余だけで粒度を見る実装が通してしまう値
		tc{name: "3-1-a: 分が負 -1 は弾く", hour: 9, minute: -1},
		tc{name: "3-1-a: 分が負 -15 は弾く（15 の倍数でも負は弾く）", hour: 9, minute: -15},
		// 3-2-a「時が負、または 25 以上」
		tc{name: "3-2-a: 時が負 -1 は弾く", hour: -1, minute: 0},
		tc{name: "3-2-a: 時が 25 は弾く", hour: 25, minute: 0},
		tc{name: "3-2-a: 時が 26 は弾く", hour: 26, minute: 0},
		// 3-2-b「時 24 は分によらず弾く」
		tc{name: "3-2-b: 24:15 は弾く", hour: 24, minute: 15},
	)

	for _, tt := range allowed {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.NewTimeOfDay(tt.hour, tt.minute)
			if err != nil {
				t.Fatalf("NewTimeOfDay(%d, %d) が予期しないエラーを返した: %v", tt.hour, tt.minute, err)
			}
			if diff := cmp.Diff(todView{Hour: tt.hour, Minute: tt.minute}, viewOfTimeOfDay(got)); diff != "" {
				t.Errorf("NewTimeOfDay(%d, %d) の値が不一致 (-want +got):\n%s", tt.hour, tt.minute, diff)
			}
		})
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workmonth.NewTimeOfDay(tt.hour, tt.minute)
			if !errors.Is(err, workmonth.ErrTimeOfDayOutOfRange) {
				t.Fatalf("NewTimeOfDay(%d, %d) のエラー = %v, want errors.Is(err, ErrTimeOfDayOutOfRange)", tt.hour, tt.minute, err)
			}
			if errors.Is(err, workmonth.ErrInvalidTimeCombination) {
				t.Errorf("エラー %v が ErrInvalidTimeCombination にも当たる（番兵を兼ねない＝AC-3-14）", err)
			}
			if errors.Is(err, workmonth.ErrInvalidValue) {
				t.Errorf("エラー %v が ErrInvalidValue にも当たる（AC-3-14・AC-9-15-h）", err)
			}
			if got.Hour() != 0 || got.Minute() != 0 {
				t.Errorf("弾く入力で不正なインスタンスを返した: %d:%02d（AC-3 冒頭）", got.Hour(), got.Minute())
			}
		})
	}
}

// TestNewTimeOfDay_SameConstructionForAllFourTimes は、4つの時刻（開始・終了・休憩開始・休憩終了）の
// いずれに同じ不正な値を置いても同じ番兵で弾かれることを検査する（AC-3-12-b・AC-12-18 ① (i)）。
// 4つはすべて同じ型（TimeOfDay）を通る＝時刻ごとに検査を書き分けない。
// 公開の構築関数は NewTimeOfDay の1つだけなので、4つの位置の入力を同じ関数へ通して確かめる。
// 位置ごとの取り違え（片方の時刻だけを検査する実装）を interactor 経由で検出する対は
// interactor の TestEnterDailyRecord_InvalidTimeOfDayInEachOfFourTimes が持つ。
func TestNewTimeOfDay_SameConstructionForAllFourTimes(t *testing.T) {
	positions := []string{"開始時刻", "終了時刻", "休憩開始時刻", "休憩終了時刻"}
	// 3-1-a の `9:10`（分が粒度の外）と 3-2-b の `24:00`（時 24）
	invalids := []struct {
		name         string
		hour, minute int
	}{
		{name: "9:10", hour: 9, minute: 10},
		{name: "24:00", hour: 24, minute: 0},
	}
	for _, pos := range positions {
		for _, in := range invalids {
			t.Run(pos+"に "+in.name, func(t *testing.T) {
				_, err := workmonth.NewTimeOfDay(in.hour, in.minute)
				if !errors.Is(err, workmonth.ErrTimeOfDayOutOfRange) {
					t.Errorf("NewTimeOfDay(%d, %d) のエラー = %v, want errors.Is(err, ErrTimeOfDayOutOfRange)", in.hour, in.minute, err)
				}
			})
		}
	}
}

// ---- AC-3-12-c〜g・3-12-h: Break / DailyRecord（仕様書の表から駆動） -----------

func TestDailyRecord_FromSpecTable(t *testing.T) {
	rows := loadDailyRecordSpecRows(t)
	// AC-3-12 の表が参照する行がすべて読めたこと（解析が空振りして 0 件で緑になるのを防ぐ）。
	// 許可の行（3-12-e）・弾く行（3-12-f・3-12-g）・Break の行（3-12-c・3-12-d）。
	requireRowIDs(t, rows,
		"3-5", "3-5-d", "3-6", "3-7", "3-7-a", "3-9-b", "3-11", "3-11-a", "3-11-b", "3-11-c",
		"3-5-a", "3-5-b", "3-5-c", "3-5-e", "3-8", "3-8-a", "3-8-b", "3-9", "3-9-a",
		"3-6-a", "3-6-b",
	)
	// 3-5-e は例示が2つ（23:45〜23:45 と 23:45〜0:00）。両方が読めていること。
	count35e := 0
	for _, r := range rows {
		if r.ID == "3-5-e" {
			count35e++
		}
	}
	if count35e != 2 {
		t.Fatalf("3-5-e の例示を %d 件読んだ, want 2", count35e)
	}

	// 3-12-d: Break の構築で弾く行（休憩終了が休憩開始より後でない）。
	// 他の「弾く」行は Break は妥当で DailyRecord の構築（3-12-f・3-12-g）で弾く。
	breakRejectIDs := map[string]bool{"3-6-a": true, "3-6-b": true}

	for _, row := range rows {
		label := fmt.Sprintf("%s %v〜%v 休憩=%v", row.ID, row.Start, row.End, row.Break)
		t.Run(label, func(t *testing.T) {
			rec, stage, err := buildFromRow(t, row)

			if row.Reject {
				assertCombinationError(t, label, err)
				if want := map[bool]string{true: "break", false: "record"}[breakRejectIDs[row.ID]]; stage != want {
					t.Errorf("%s: 弾いた構築 = %s, want %s（AC-3-12 の表）", label, stage, want)
				}
				if rec.Date().Year() != 0 {
					t.Errorf("%s: 弾く行で不正な DailyRecord のインスタンスを返した（AC-3 冒頭）", label)
				}
				return
			}

			// 許可の行（3-12-c・3-12-e・3-12-h）
			if err != nil {
				t.Fatalf("%s: 予期しないエラー: %v", label, err)
			}
			if row.WantMinutes < 0 {
				t.Fatalf("%s: 許可の行の稼働時間を表から読めなかった", label)
			}
			want := hoursView{Hours: row.WantMinutes / 60, Minutes: row.WantMinutes % 60}
			if diff := cmp.Diff(want, viewOfHours(rec.WorkingHours())); diff != "" {
				t.Errorf("%s: 稼働時間が不一致 (-want +got):\n%s", label, diff)
			}

			// AC-3-13: 保持した時刻をそのまま返す。休憩の有無を判別できる。
			wantTimes := recordTimesView{
				Start: todView{Hour: row.Start[0], Minute: row.Start[1]},
				End:   todView{Hour: row.End[0], Minute: row.End[1]},
			}
			if row.Break != nil {
				wantTimes.HasBreak = true
				wantTimes.Break = breakView{
					Start: todView{Hour: row.Break[0][0], Minute: row.Break[0][1]},
					End:   todView{Hour: row.Break[1][0], Minute: row.Break[1][1]},
				}
			}
			if diff := cmp.Diff(wantTimes, viewOfRecordTimes(rec)); diff != "" {
				t.Errorf("%s: 保持した時刻が不一致 (-want +got):\n%s", label, diff)
			}
		})
	}
}

// 許可の行と弾く行を境界で対にして直接置く（AC-12-18 ① (ii)）。
// 表から読む上のテストが全行を走るが、対の相手が表から落ちても対が成立するよう、
// 対の行番号を明示して読み、両方が存在することを検査する。
func TestDailyRecord_BoundaryPairsFromSpecTable(t *testing.T) {
	rows := loadDailyRecordSpecRows(t)
	pairs := []struct{ allowed, rejected string }{
		{"3-5", "3-5-a"},
		{"3-6", "3-6-a"},
		{"3-7", "3-8"},
		{"3-7-a", "3-8-a"},
		{"3-9-b", "3-9"},
	}
	byID := func(id string) (specRow, bool) {
		for _, r := range rows {
			if r.ID == id {
				return r, true
			}
		}
		return specRow{}, false
	}
	for _, p := range pairs {
		t.Run(p.allowed+" と "+p.rejected, func(t *testing.T) {
			a, ok := byID(p.allowed)
			if !ok || a.Reject {
				t.Fatalf("許可の行 %s を表から読めない、または弾く行になっている", p.allowed)
			}
			r, ok := byID(p.rejected)
			if !ok || !r.Reject {
				t.Fatalf("弾く行 %s を表から読めない、または許可の行になっている", p.rejected)
			}
			if _, _, err := buildFromRow(t, a); err != nil {
				t.Errorf("許可の行 %s が弾かれた: %v", p.allowed, err)
			}
			if _, _, err := buildFromRow(t, r); err == nil {
				t.Errorf("弾く行 %s が構築できてしまった", p.rejected)
			}
		})
	}
}

// ---- AC-3-13: 休憩の有無の判別（AC-2-6-b・AC-2-6-c） ------------------------------

// TestDailyRecord_ReadBackFromSpecTable は保存した時刻の読み出し（daily-record-entry.md AC-2-6-a〜2-6-c）を、
// 仕様書の表から読んだ入力と期待で検査する。2-6-b（休憩なし）と 2-6-c（最小の時刻を持つ休憩）を対にする。
func TestDailyRecord_ReadBackFromSpecTable(t *testing.T) {
	rows := loadReadBackSpecRows(t)
	requireRowIDs(t, rows, "2-6-a", "2-6-b", "2-6-c")

	for _, row := range rows {
		t.Run(row.ID, func(t *testing.T) {
			rec, _, err := buildFromRow(t, row)
			if err != nil {
				t.Fatalf("%s: 予期しないエラー: %v", row.ID, err)
			}
			wantTimes := recordTimesView{
				Start: todView{Hour: row.Start[0], Minute: row.Start[1]},
				End:   todView{Hour: row.End[0], Minute: row.End[1]},
			}
			if row.Break != nil {
				wantTimes.HasBreak = true
				wantTimes.Break = breakView{
					Start: todView{Hour: row.Break[0][0], Minute: row.Break[0][1]},
					End:   todView{Hour: row.Break[1][0], Minute: row.Break[1][1]},
				}
			}
			if diff := cmp.Diff(wantTimes, viewOfRecordTimes(rec)); diff != "" {
				t.Errorf("%s: 読み出した時刻が不一致 (-want +got):\n%s", row.ID, diff)
			}
			if row.WantMinutes >= 0 {
				want := hoursView{Hours: row.WantMinutes / 60, Minutes: row.WantMinutes % 60}
				if diff := cmp.Diff(want, viewOfHours(rec.WorkingHours())); diff != "" {
					t.Errorf("%s: 稼働時間が不一致 (-want +got):\n%s", row.ID, diff)
				}
			}
		})
	}
}

// TestDailyRecord_NoBreakIsDistinguishedFromMinimumBreak は、休憩なしと「最小の時刻を持つ休憩」を
// 取り違えないことを対にして検査する（AC-3-13・daily-record-entry.md AC-2-6-b・AC-2-6-c）。
func TestDailyRecord_NoBreakIsDistinguishedFromMinimumBreak(t *testing.T) {
	date := mustDate(t, 2026, 7, 1)

	t.Run("休憩なし: 第2戻り値が false で、第1戻り値はゼロ値", func(t *testing.T) {
		rec := mustDailyRecordAt(t, date, [2]int{9, 0}, [2]int{18, 0}, nil)

		b, ok := rec.Break()

		if ok {
			t.Errorf("休憩なしの Break() の第2戻り値 = true, want false")
		}
		if diff := cmp.Diff(breakView{}, viewOfBreak(b)); diff != "" {
			t.Errorf("休憩なしの Break() の第1戻り値がゼロ値でない (-want +got):\n%s", diff)
		}
	})

	t.Run("0:00〜0:15 の休憩: 第2戻り値が true（ゼロ値の休憩を休憩なしと取り違えない）", func(t *testing.T) {
		brk := mustBreak(t, 0, 0, 0, 15)
		rec := mustDailyRecordAt(t, date, [2]int{0, 0}, [2]int{23, 45}, &brk)

		b, ok := rec.Break()

		if !ok {
			t.Fatalf("最小の時刻を持つ休憩の Break() の第2戻り値 = false, want true")
		}
		want := breakView{Start: todView{Hour: 0, Minute: 0}, End: todView{Hour: 0, Minute: 15}}
		if diff := cmp.Diff(want, viewOfBreak(b)); diff != "" {
			t.Errorf("休憩の時刻が不一致 (-want +got):\n%s", diff)
		}
	})
}

// TestDailyRecord_HoldsDateAndTimes は対象日・時刻を保持し、稼働時間を保持した時刻から算出することを検証する
// （AC-3-13。daily-record-entry.md AC-2-1・AC-2-1-a）。
func TestDailyRecord_HoldsDateAndTimes(t *testing.T) {
	date := mustDate(t, 2026, 7, 15)
	brk := mustBreak(t, 12, 30, 13, 15)

	rec := mustDailyRecordAt(t, date, [2]int{9, 15}, [2]int{17, 45}, &brk)

	if diff := cmp.Diff(dateView{Year: 2026, Month: 7, Day: 15}, viewOfDate(rec.Date())); diff != "" {
		t.Errorf("DailyRecord.Date() が不一致 (-want +got):\n%s", diff)
	}
	wantTimes := recordTimesView{
		Start:    todView{Hour: 9, Minute: 15},
		End:      todView{Hour: 17, Minute: 45},
		HasBreak: true,
		Break:    breakView{Start: todView{Hour: 12, Minute: 30}, End: todView{Hour: 13, Minute: 15}},
	}
	if diff := cmp.Diff(wantTimes, viewOfRecordTimes(rec)); diff != "" {
		t.Errorf("保持した時刻が不一致 (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(hoursView{Hours: 7, Minutes: 45}, viewOfHours(rec.WorkingHours())); diff != "" {
		t.Errorf("稼働時間が時刻からの算出値と不一致 (-want +got):\n%s", diff)
	}
}

// TestNewDailyRecord_CopiesBreak は、受け取った *Break の指す値を複写して保持することを検証する
// （AC-3-15。呼び出し側が後から指す先を書き換えても稼働実績が変わらない）。
func TestNewDailyRecord_CopiesBreak(t *testing.T) {
	date := mustDate(t, 2026, 7, 1)
	brk := mustBreak(t, 12, 0, 13, 0)
	rec := mustDailyRecordAt(t, date, [2]int{9, 0}, [2]int{18, 0}, &brk)

	brk = mustBreak(t, 10, 0, 10, 15) // 呼び出し側の指す先を書き換える

	got, ok := rec.Break()
	if !ok {
		t.Fatalf("Break() の第2戻り値 = false, want true")
	}
	want := breakView{Start: todView{Hour: 12, Minute: 0}, End: todView{Hour: 13, Minute: 0}}
	if diff := cmp.Diff(want, viewOfBreak(got)); diff != "" {
		t.Errorf("呼び出し側の書き換えが稼働実績に及んだ (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(hoursView{Hours: 8, Minutes: 0}, viewOfHours(rec.WorkingHours())); diff != "" {
		t.Errorf("呼び出し側の書き換えが稼働時間に及んだ (-want +got):\n%s", diff)
	}
}

// ---- AC-3-14: 番兵 ---------------------------------------------------------

// TestTimeSentinels_AreDistinct は、2つの時刻の番兵が互いに当たらず、ErrInvalidValue とも別であることを検査する
// （AC-3-14・AC-11-4・AC-11-4-a。契約の code と1対1）。
func TestTimeSentinels_AreDistinct(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrTimeOfDayOutOfRange", workmonth.ErrTimeOfDayOutOfRange},
		{"ErrInvalidTimeCombination", workmonth.ErrInvalidTimeCombination},
		{"ErrInvalidValue", workmonth.ErrInvalidValue},
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a.err, b.err) {
				t.Errorf("%s が %s に errors.Is で当たる（番兵は互いに兼ねない＝AC-3-14）", a.name, b.name)
			}
		}
	}
}

// TestNewWorkingHours_FailureIsInvalidValue は、WorkingHours の構築失敗が ErrInvalidValue であり、
// 時刻の番兵に当たらないことを検査する（AC-3-4・AC-12-18 ① (v)。決定16 の帰結）。
func TestNewWorkingHours_FailureIsInvalidValue(t *testing.T) {
	tests := []struct {
		name    string
		hours   int
		minutes int
	}{
		{name: "分が 60", hours: 8, minutes: 60},
		{name: "分が負", hours: 8, minutes: -1},
		{name: "時が負", hours: -1, minutes: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := workmonth.NewWorkingHours(tt.hours, tt.minutes)

			if !errors.Is(err, workmonth.ErrInvalidValue) {
				t.Fatalf("NewWorkingHours(%d, %d) のエラー = %v, want errors.Is(err, ErrInvalidValue)", tt.hours, tt.minutes, err)
			}
			if errors.Is(err, workmonth.ErrTimeOfDayOutOfRange) || errors.Is(err, workmonth.ErrInvalidTimeCombination) {
				t.Errorf("エラー %v が時刻の番兵に当たる（AC-3-4）", err)
			}
		})
	}
}
