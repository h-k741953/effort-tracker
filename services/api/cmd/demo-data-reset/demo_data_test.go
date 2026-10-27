package main

// 検証対象: docs/specs/demo-data-reset.md の単位1「デモデータの組み立て」
// （AC-14-1・AC-14-6）。AC-2・AC-3-1〜3-3・AC-3-5・AC-4・AC-6-1・AC-6-4（組み立てが
// 契約の識別子で勤務月を作ること）・AC-6-7〜6-7-c・AC-9（9-4・9-6〜9-10）と、
// 「デモデータの値」節（V-2〜V-6・V-8）。
//
// リセット日を与えて組み立てた結果だけを検査する（実 DB・時計・環境変数を使わない）。
// 期待値は V-2〜V-8 の表から読み、実装の式を写さない。4-4 の判定は、組み立てた各勤務月の
// 稼働実績から本ファイルが別に求めた値と、集約のアクセサの値を突き合わせる（AC-4-4-a）。
//
// 本ファイルが tester として定めた署名（実装が満たす）:
//
//	func buildDemoData(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error)
//
// 引数の engineerID は、契約の技術者（デモの技術者識別子＝AC-7-1）。組み立ての内側で環境変数を
// 読まないため（AC-9-5）、呼び出し側が渡す。error は AC-9-3（集約の操作の失敗を握り潰さない）を
// 戻り値で表すために要る。
//
// 担保しないもの（AC-14-7）: AC-9-1 の「Reconstruct を投入に使わない」「Approve に承認者を要さない」
// （署名・呼び出しの有無はテストで観測できない）、AC-9-5 の「時計・乱数・環境変数を内側で読まない」
// （入力を渡して同じ結果になること＝AC-9-4 は検査する）。AC-9-3 の失敗の伝播は、Run が組み立ての関数を
// 引数に取る差し替え点（AC-9-3-a）を通じて main_test.go が検査する。

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

const (
	testEngineerID = "demo-engineer-not-a-uuid"
	displayNameA   = "基幹システム更改" // V-2 の契約 A
	displayNameB   = "運用保守"     // V-2 の契約 B
)

// ---- ヘルパ ----------------------------------------------------------------

func mustResetDate(t *testing.T, year, month, day int) workmonth.Date {
	t.Helper()
	d, err := workmonth.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("NewDate(%d, %d, %d) failed: %v", year, month, day, err)
	}
	return d
}

func mustBuild(t *testing.T, resetDate workmonth.Date) ([]gateway.DemoContract, []*workmonth.WorkMonth) {
	t.Helper()
	contracts, workMonths, err := buildDemoData(resetDate, testEngineerID)
	if err != nil {
		t.Fatalf("buildDemoData(%d-%02d-%02d) failed: %v", resetDate.Year(), resetDate.Month(), resetDate.Day(), err)
	}
	return contracts, workMonths
}

// monthsBefore は (year, month) の n か月前の年月を返す（AC-9-9。年をまたぐ）。
func monthsBefore(year, month, n int) (int, int) {
	index := year*12 + (month - 1) - n
	return index / 12, index%12 + 1
}

func ymKey(year, month int) string { return fmt.Sprintf("%04d-%02d", year, month) }

func weekdayOf(year, month, day int) time.Weekday {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Weekday()
}

func daysIn(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// weekdaysInMonth は月の平日（月〜金）の数、saturdaysInMonth は土曜の数（V-4・V-6）。
func weekdaysInMonth(year, month int) int {
	n := 0
	for d := 1; d <= daysIn(year, month); d++ {
		if w := weekdayOf(year, month, d); w >= time.Monday && w <= time.Friday {
			n++
		}
	}
	return n
}

func saturdaysInMonth(year, month int) int {
	n := 0
	for d := 1; d <= daysIn(year, month); d++ {
		if weekdayOf(year, month, d) == time.Saturday {
			n++
		}
	}
	return n
}

func hm(h, m int) int { return h*60 + m }

// minutesView は WorkingHours を分で表す。
func minutesOf(h workmonth.WorkingHours) int { return h.TotalMinutes() }

// recordMinutes は稼働実績から、日の稼働時間を15分単位で切り捨てた分を別に求める
// （daily-record-entry.md AC-6-1。集約の TotalHours を使わない）。
func recordMinutes(r workmonth.DailyRecord) int {
	worked := hm(r.EndTime().Hour(), r.EndTime().Minute()) - hm(r.StartTime().Hour(), r.StartTime().Minute())
	if b, ok := r.Break(); ok {
		worked -= hm(b.End().Hour(), b.End().Minute()) - hm(b.Start().Hour(), b.Start().Minute())
	}
	return worked / 15 * 15
}

func totalMinutes(records []workmonth.DailyRecord) int {
	total := 0
	for _, r := range records {
		total += recordMinutes(r)
	}
	return total
}

// contractByName は契約表示名（V-2）で契約を1件引く。
func contractByName(t *testing.T, contracts []gateway.DemoContract, name string) gateway.DemoContract {
	t.Helper()
	var found []gateway.DemoContract
	for _, c := range contracts {
		if c.DisplayName == name {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("契約表示名 %q の契約が %d 件ある, want 1", name, len(found))
	}
	return found[0]
}

type periodView struct {
	Start  string
	HasEnd bool
	End    string
}

func periodOf(c gateway.DemoContract) (start string, end *string) {
	start = ymKey(c.PeriodStart.Year(), c.PeriodStart.Month())
	if c.PeriodEnd != nil {
		e := ymKey(c.PeriodEnd.Year(), c.PeriodEnd.Month())
		end = &e
	}
	return start, end
}

// ---- 契約（AC-6-1・AC-6-7-a〜6-7-c・AC-3-1〜3-3・AC-3-5・V-2） ------------------

// TestBuildDemoData_Contracts は、リセット日ごとの契約の値を 6-7-a の表で固定する。
// 年月の境（月末と翌月1日＝6-7-b）と、年をまたぐ（6-7-c）を含む。
func TestBuildDemoData_Contracts(t *testing.T) {
	tests := []struct {
		name             string
		year, month, day int
		aStart           string
		bStart, bEnd     string
	}{
		{"2026-10-27", 2026, 10, 27, "2026-04", "2026-07", "2026-09"},
		{"2026-10-31（月末）", 2026, 10, 31, "2026-04", "2026-07", "2026-09"},
		{"2026-11-01（翌月1日）", 2026, 11, 1, "2026-05", "2026-08", "2026-10"},
		{"2026-03-15（N か月前が前年）", 2026, 3, 15, "2025-09", "2025-12", "2026-02"},
		{"2027-01-01（年をまたぐ）", 2027, 1, 1, "2026-07", "2026-10", "2026-12"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contracts, _ := mustBuild(t, mustResetDate(t, tt.year, tt.month, tt.day))
			if len(contracts) != 2 {
				t.Fatalf("契約の数 = %d, want 2（V-2）", len(contracts))
			}
			a := contractByName(t, contracts, displayNameA)
			b := contractByName(t, contracts, displayNameB)

			// AC-6-1: 技術者はデモの技術者識別子。V-2: 精算幅。
			for _, c := range []gateway.DemoContract{a, b} {
				if c.EngineerID != testEngineerID {
					t.Errorf("%s の技術者識別子 = %q, want %q（AC-6-1）", c.DisplayName, c.EngineerID, testEngineerID)
				}
			}
			if a.ID.String() == b.ID.String() || a.ID.String() == "" || b.ID.String() == "" {
				t.Errorf("契約の識別子が空または重複している: A=%q B=%q", a.ID.String(), b.ID.String())
			}
			rangeOf := func(c gateway.DemoContract) [4]int {
				lo, up := c.SettlementRange.LowerBound(), c.SettlementRange.UpperBound()
				return [4]int{lo.Hours(), lo.Minutes(), up.Hours(), up.Minutes()}
			}
			if diff := cmp.Diff([4]int{140, 0, 180, 0}, rangeOf(a)); diff != "" {
				t.Errorf("A の精算幅 (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([4]int{20, 0, 40, 0}, rangeOf(b)); diff != "" {
				t.Errorf("B の精算幅 (-want +got):\n%s", diff)
			}

			// 6-7-a の表。
			aStart, aEnd := periodOf(a)
			if aStart != tt.aStart || aEnd != nil {
				t.Errorf("A の契約期間 = %s〜%v, want %s〜（終了未定）", aStart, aEnd, tt.aStart)
			}
			bStart, bEnd := periodOf(b)
			if diff := cmp.Diff(&periodView{Start: tt.bStart, HasEnd: true, End: tt.bEnd}, &periodView{Start: bStart, HasEnd: bEnd != nil, End: derefOr(bEnd)}); diff != "" {
				t.Errorf("B の契約期間 (-want +got):\n%s", diff)
			}
		})
	}
}

func derefOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// TestBuildDemoData_ContractPeriodInvariants は AC-3-1〜3-3・3-5 を、日々のリセット日に
// 渡って固定する（AC-14-6）。開始は必ず持つ（型で NULL を取れない）ので、終了の組と前後、
// 終了未定と終了ありの両方を含むことを見る。
func TestBuildDemoData_ContractPeriodInvariants(t *testing.T) {
	for _, d := range eachDay(2025, 1, 1, 2028, 12, 31) {
		contracts, _ := mustBuild(t, mustResetDate(t, d[0], d[1], d[2]))
		open, closed := 0, 0
		for _, c := range contracts {
			if c.PeriodEnd == nil {
				open++
				continue
			}
			closed++
			// 3-3: 開始の年月は終了の年月より後でない（同じ年月は可）。
			if c.PeriodStart.Year()*12+c.PeriodStart.Month() > c.PeriodEnd.Year()*12+c.PeriodEnd.Month() {
				t.Fatalf("%v: %s の開始が終了より後: %s > %s", d, c.DisplayName,
					ymKey(c.PeriodStart.Year(), c.PeriodStart.Month()), ymKey(c.PeriodEnd.Year(), c.PeriodEnd.Month()))
			}
		}
		if open < 1 || closed < 1 {
			t.Fatalf("%v: 終了未定 %d 件・終了あり %d 件, want 各1件以上（AC-3-5）", d, open, closed)
		}
	}
}

// eachDay は [from, to] の暦日を [年, 月, 日] で返す。
func eachDay(fy, fm, fd, ty, tm, td int) [][3]int {
	var out [][3]int
	cur := time.Date(fy, time.Month(fm), fd, 0, 0, 0, 0, time.UTC)
	end := time.Date(ty, time.Month(tm), td, 0, 0, 0, 0, time.UTC)
	for !cur.After(end) {
		out = append(out, [3]int{cur.Year(), int(cur.Month()), cur.Day()})
		cur = cur.AddDate(0, 0, 1)
	}
	return out
}

// ---- 勤務月の配置（V-3・AC-4-1・AC-4-6・AC-9-7・AC-9-8） --------------------------

// V-3 の配置。n か月前（0 が当月）→ 状態と日の型。
type slot struct {
	monthsAgo int
	state     workmonth.State
	pattern   string // "standard" / "over" / "short" / "b"
	// outcome は確定した月の判定: "fit"（超過0・不足0）/ "excess" / "short" / ""（Draft は未確定）。
	outcome string
}

var slotsA = []slot{
	{5, workmonth.StateApproved, "standard", "fit"},
	{4, workmonth.StateApproved, "over", "excess"},
	{3, workmonth.StateApproved, "standard", "fit"},
	{2, workmonth.StateApproved, "short", "short"},
	{1, workmonth.StatePendingApproval, "standard", "fit"},
	{0, workmonth.StateDraft, "standard", ""},
}

var slotsB = []slot{
	{3, workmonth.StateApproved, "b", "fit"},
	{2, workmonth.StateApproved, "b", "fit"},
	{1, workmonth.StateApproved, "b", "fit"},
}

// TestBuildDemoData_MonthPlacement は、勤務月の集合が V-3 のとおりであり（6か月前は置かない・
// 列挙に無い月を足さない）、状態と判定の別が 4-4 の独立計算で一致することを、日々のリセット日に
// 渡って固定する。平日の数が 20・21・22・23 の月で別が入れ替わらないこと（AC-4-6）の
// 陽性対照として、各型で4通りの平日の数を実際に踏んだことも確かめる。
func TestBuildDemoData_MonthPlacement(t *testing.T) {
	seenWeekdays := map[string]map[int]bool{"fit": {}, "excess": {}, "short": {}}
	seenSaturdays := map[int]bool{}

	for _, d := range eachDay(2025, 1, 1, 2028, 12, 31) {
		resetDate := mustResetDate(t, d[0], d[1], d[2])
		contracts, workMonths := mustBuild(t, resetDate)
		a := contractByName(t, contracts, displayNameA)
		b := contractByName(t, contracts, displayNameB)

		type key struct{ contract, ym string }
		got := map[key]*workmonth.WorkMonth{}
		for _, wm := range workMonths {
			k := key{wm.ContractID().String(), ymKey(wm.YearMonth().Year(), wm.YearMonth().Month())}
			if _, dup := got[k]; dup {
				t.Fatalf("%v: 勤務月が重複している: %v（勤務月は契約 × 年月で一意）", d, k)
			}
			got[k] = wm
		}
		wantCount := len(slotsA) + len(slotsB)
		if len(workMonths) != wantCount {
			t.Fatalf("%v: 勤務月の数 = %d, want %d（V-3。列挙に無い月を足さない）", d, len(workMonths), wantCount)
		}

		check := func(c gateway.DemoContract, slots []slot) {
			for _, s := range slots {
				y, m := monthsBefore(d[0], d[1], s.monthsAgo)
				wm, ok := got[key{c.ID.String(), ymKey(y, m)}]
				if !ok {
					t.Fatalf("%v: %s の %s（%d か月前）の勤務月が無い", d, c.DisplayName, ymKey(y, m), s.monthsAgo)
				}
				if wm.State() != s.state {
					t.Fatalf("%v: %s %s の状態 = %s, want %s", d, c.DisplayName, ymKey(y, m), wm.State(), s.state)
				}
				// 4-5: 勤務月が写し取る精算幅は契約と同じ値。
				if wm.SettlementRange() != c.SettlementRange {
					t.Fatalf("%v: %s %s の精算幅が契約と違う", d, c.DisplayName, ymKey(y, m))
				}
				records := wm.DailyRecords()
				excess, hasExcess := wm.Excess()
				shortfall, hasShortfall := wm.Shortfall()

				if s.state == workmonth.StateDraft {
					// 4-2: Draft は超過／不足が未確定。
					if hasExcess || hasShortfall {
						t.Fatalf("%v: Draft の %s %s に超過／不足が確定している", d, c.DisplayName, ymKey(y, m))
					}
					continue
				}
				// 4-4: 別に求めた総稼働時間と判定規則（境界は両端とも含む）。
				total := totalMinutes(records)
				if accessor := minutesOf(wm.TotalHours()); accessor != total {
					t.Fatalf("%v: %s %s の総稼働時間 = %d分, 稼働実績から求めた値 = %d分", d, c.DisplayName, ymKey(y, m), accessor, total)
				}
				lower, upper := minutesOf(c.SettlementRange.LowerBound()), minutesOf(c.SettlementRange.UpperBound())
				wantExcess, wantShort := 0, 0
				if total > upper {
					wantExcess = total - upper
				} else if total < lower {
					wantShort = lower - total
				}
				if !hasExcess || !hasShortfall || minutesOf(excess) != wantExcess || minutesOf(shortfall) != wantShort {
					t.Fatalf("%v: %s %s の超過／不足 = (%v,%d分)/(%v,%d分), want (%d分)/(%d分)",
						d, c.DisplayName, ymKey(y, m), hasExcess, minutesOf(excess), hasShortfall, minutesOf(shortfall), wantExcess, wantShort)
				}
				// 4-6: 割り当てた別（fit/excess/short）は、どのリセット日でも変わらない。
				gotOutcome := "fit"
				if wantExcess > 0 {
					gotOutcome = "excess"
				} else if wantShort > 0 {
					gotOutcome = "short"
				}
				if gotOutcome != s.outcome {
					t.Fatalf("%v: %s %s の別 = %s, want %s（AC-4-6。総稼働時間 %d分）", d, c.DisplayName, ymKey(y, m), gotOutcome, s.outcome, total)
				}
				if c.DisplayName == displayNameA {
					seenWeekdays[s.outcome][weekdaysInMonth(y, m)] = true
				} else {
					seenSaturdays[saturdaysInMonth(y, m)] = true
				}
			}
		}
		check(a, slotsA)
		check(b, slotsB)
	}

	// 陽性対照: 4通りの平日の数（20〜23）を、各型で踏んでいる。踏んでいなければ上の検査は空洞。
	for outcome, seen := range seenWeekdays {
		for _, n := range []int{20, 21, 22, 23} {
			if !seen[n] {
				t.Errorf("別 %q で平日の数 %d の月を一度も踏んでいない（AC-4-6 の検査が空洞）", outcome, n)
			}
		}
	}
	for _, n := range []int{4, 5} {
		if !seenSaturdays[n] {
			t.Errorf("B で土曜の数 %d の月を一度も踏んでいない", n)
		}
	}
}

// ---- 稼働実績の形と日の型（V-4・V-5・AC-2・AC-9-6・AC-9-7・AC-9-10） --------------

type dayPattern struct {
	start, end [2]int
	brk        *[4]int
}

var lunch = &[4]int{12, 0, 13, 0}

func standardPattern(k int) dayPattern {
	switch k % 5 {
	case 1:
		return dayPattern{[2]int{9, 0}, [2]int{18, 0}, lunch}
	case 2:
		return dayPattern{[2]int{9, 0}, [2]int{18, 30}, lunch}
	case 3:
		return dayPattern{[2]int{8, 45}, [2]int{17, 45}, lunch}
	case 4:
		return dayPattern{[2]int{9, 0}, [2]int{17, 30}, lunch}
	default:
		return dayPattern{[2]int{9, 0}, [2]int{15, 0}, nil}
	}
}

func overPattern(k int) dayPattern {
	if k%2 == 1 {
		return dayPattern{[2]int{9, 0}, [2]int{19, 0}, lunch}
	}
	return dayPattern{[2]int{9, 0}, [2]int{19, 30}, lunch}
}

func shortPattern(k int) dayPattern {
	if k%2 == 1 {
		return dayPattern{[2]int{9, 0}, [2]int{15, 45}, lunch}
	}
	return dayPattern{[2]int{9, 0}, [2]int{16, 0}, lunch}
}

func patternFor(kind string, k int) dayPattern {
	switch kind {
	case "standard":
		return standardPattern(k)
	case "over":
		return overPattern(k)
	case "short":
		return shortPattern(k)
	default: // "b"
		return dayPattern{[2]int{10, 0}, [2]int{17, 0}, lunch}
	}
}

type recordShape struct {
	Day        int
	Start, End [2]int
	Break      *[4]int
}

func shapeOf(r workmonth.DailyRecord) recordShape {
	s := recordShape{
		Day:   r.Date().Day(),
		Start: [2]int{r.StartTime().Hour(), r.StartTime().Minute()},
		End:   [2]int{r.EndTime().Hour(), r.EndTime().Minute()},
	}
	if b, ok := r.Break(); ok {
		s.Break = &[4]int{b.Start().Hour(), b.Start().Minute(), b.End().Hour(), b.End().Minute()}
	}
	return s
}

// expectedRecords は V-4・V-5 から、ある月の稼働実績の期待を求める。
// A は平日（k は月の中で何番目の平日か）、B は土曜。lastDay はその月の最終の対象日
// （過去の月は月末、当月はリセット日の前日）。
func expectedRecords(year, month, lastDay int, kind string) []recordShape {
	var out []recordShape
	k := 0
	for day := 1; day <= daysIn(year, month); day++ {
		w := weekdayOf(year, month, day)
		isWorkday := w >= time.Monday && w <= time.Friday
		if kind == "b" {
			isWorkday = w == time.Saturday
		}
		if !isWorkday {
			continue
		}
		k++
		if day > lastDay {
			continue
		}
		p := patternFor(kind, k)
		out = append(out, recordShape{Day: day, Start: p.start, End: p.end, Break: p.brk})
	}
	return out
}

// TestBuildDemoData_DailyRecords は、各月の稼働実績が V-4・V-5 のとおりであることを、
// 日々のリセット日に渡って固定する。過去の月は稼働日のすべてに置き（日曜に置かない）、
// 当月は1日からリセット日の前日までの稼働日（0件でもよい＝AC-9-8）に置く。
func TestBuildDemoData_DailyRecords(t *testing.T) {
	for _, d := range eachDay(2025, 1, 1, 2028, 12, 31) {
		resetDate := mustResetDate(t, d[0], d[1], d[2])
		contracts, workMonths := mustBuild(t, resetDate)
		byContract := map[string]string{}
		for _, c := range contracts {
			byContract[c.ID.String()] = c.DisplayName
		}

		for _, wm := range workMonths {
			year, month := wm.YearMonth().Year(), wm.YearMonth().Month()
			name := byContract[wm.ContractID().String()]
			var slotList []slot
			if name == displayNameA {
				slotList = slotsA
			} else {
				slotList = slotsB
			}
			var s slot
			for _, cand := range slotList {
				y, m := monthsBefore(d[0], d[1], cand.monthsAgo)
				if y == year && m == month {
					s = cand
				}
			}
			lastDay := daysIn(year, month)
			isCurrent := year == d[0] && month == d[1]
			if isCurrent {
				lastDay = d[2] - 1 // 9-7: 当月は1日からリセット日の前日まで
			}
			want := expectedRecords(year, month, lastDay, s.pattern)

			var got []recordShape
			for _, r := range wm.DailyRecords() {
				got = append(got, shapeOf(r))
				// 9-6: 対象日はリセット日より前。当該年月に属する。
				rd := r.Date()
				if rd.Year() != year || rd.Month() != month {
					t.Fatalf("%v: %s %s に別の月の稼働実績 %d-%02d-%02d", d, name, ymKey(year, month), rd.Year(), rd.Month(), rd.Day())
				}
				if rd.Year()*10000+rd.Month()*100+rd.Day() >= d[0]*10000+d[1]*100+d[2] {
					t.Fatalf("%v: %s %s の対象日 %d-%02d-%02d がリセット日以降（AC-9-6）", d, name, ymKey(year, month), rd.Year(), rd.Month(), rd.Day())
				}
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("%v: %s %s の稼働実績 (-want +got):\n%s", d, name, ymKey(year, month), diff)
			}
			if isCurrent && wm.State() != workmonth.StateDraft {
				t.Fatalf("%v: 当月 %s の状態 = %s, want Draft（AC-9-7）", d, ymKey(year, month), wm.State())
			}
		}
	}
}

// TestBuildDemoData_CurrentMonthWithoutWorkdays は AC-9-8 を固定する。当月1日からリセット日の前日までに
// 稼働日が1日も無い（リセット日が当月1日、または前日までが休日だけ）とき、当月の A の Draft を
// 稼働実績0件で置く。
func TestBuildDemoData_CurrentMonthWithoutWorkdays(t *testing.T) {
	tests := []struct {
		name             string
		year, month, day int
	}{
		{"リセット日が当月1日（日曜）", 2026, 11, 1},
		{"リセット日が当月1日（木曜）", 2026, 10, 1},
		{"当月1日が日曜で、リセット日が2日（月曜）。前日までに平日が無い", 2026, 11, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contracts, workMonths := mustBuild(t, mustResetDate(t, tt.year, tt.month, tt.day))
			a := contractByName(t, contracts, displayNameA)
			found := false
			for _, wm := range workMonths {
				if wm.ContractID() == a.ID && wm.YearMonth().Year() == tt.year && wm.YearMonth().Month() == tt.month {
					found = true
					if wm.State() != workmonth.StateDraft {
						t.Errorf("当月の状態 = %s, want Draft", wm.State())
					}
					if n := len(wm.DailyRecords()); n != 0 {
						t.Errorf("当月の稼働実績 = %d件, want 0件（AC-9-8）", n)
					}
				}
			}
			if !found {
				t.Errorf("当月 %s の A の Draft が無い（4-1 の Draft の例を欠かさない＝AC-9-8）", ymKey(tt.year, tt.month))
			}
		})
	}
}

// TestBuildDemoData_AcrossAllDays は、日々のリセット日に渡って、状態・休憩・重なりの不変条件を固定する。
//   - 4-1: Draft・PendingApproval・Approved をそれぞれ1つ以上含む
//   - 2-2: 休憩の組は、ともに持つか、ともに持たない（型で表現されるが、休憩ありの休憩は勤務時間の内側）
//   - 2-4: 休憩なしの稼働実績を1件以上含む
//   - 9-10: 同じ技術者の2契約で、同じ対象日に稼働実績を置かない
func TestBuildDemoData_AcrossAllDays(t *testing.T) {
	for _, d := range eachDay(2025, 1, 1, 2028, 12, 31) {
		_, workMonths := mustBuild(t, mustResetDate(t, d[0], d[1], d[2]))

		states := map[workmonth.State]int{}
		noBreak := 0
		dateOwner := map[string]string{}
		for _, wm := range workMonths {
			states[wm.State()]++
			for _, r := range wm.DailyRecords() {
				if _, ok := r.Break(); !ok {
					noBreak++
				}
				date := fmt.Sprintf("%04d-%02d-%02d", r.Date().Year(), r.Date().Month(), r.Date().Day())
				if owner, taken := dateOwner[date]; taken && owner != wm.ContractID().String() {
					t.Fatalf("%v: 対象日 %s に2契約の稼働実績がある（AC-9-10）", d, date)
				}
				dateOwner[date] = wm.ContractID().String()
			}
		}
		for _, st := range []workmonth.State{workmonth.StateDraft, workmonth.StatePendingApproval, workmonth.StateApproved} {
			if states[st] < 1 {
				t.Fatalf("%v: 状態 %s の勤務月が無い（AC-4-1）", d, st)
			}
		}
		if noBreak < 1 {
			t.Fatalf("%v: 休憩なしの稼働実績が1件も無い（AC-2-4）", d)
		}
	}
}

// ---- V-8 の例（2026-10-27・2026-11-01） --------------------------------------

type monthExpect struct {
	Contract string // 契約表示名
	YM       string
	State    workmonth.State
	Total    [2]int  // 総稼働時間（時, 分）
	Excess   *[2]int // nil は未確定
	Shortage *[2]int
	Records  int
}

func hoursMinutes(h workmonth.WorkingHours) [2]int { return [2]int{h.Hours(), h.Minutes()} }

// TestBuildDemoData_WorkedExamples は V-8 の表を、期待値の出所としてそのまま固定する。
func TestBuildDemoData_WorkedExamples(t *testing.T) {
	zero := &[2]int{0, 0}
	tests := []struct {
		name             string
		year, month, day int
		want             []monthExpect
	}{
		{"2026-10-27（火曜。V-8）", 2026, 10, 27, []monthExpect{
			{displayNameA, "2026-05", workmonth.StateApproved, [2]int{160, 0}, zero, zero, 21},
			{displayNameA, "2026-06", workmonth.StateApproved, [2]int{203, 30}, &[2]int{23, 30}, zero, 22},
			{displayNameA, "2026-07", workmonth.StateApproved, [2]int{176, 30}, zero, zero, 23},
			{displayNameA, "2026-08", workmonth.StateApproved, [2]int{123, 15}, zero, &[2]int{16, 45}, 21},
			{displayNameA, "2026-09", workmonth.StatePendingApproval, [2]int{168, 30}, zero, zero, 22},
			{displayNameA, "2026-10", workmonth.StateDraft, [2]int{138, 30}, nil, nil, 18},
			{displayNameB, "2026-07", workmonth.StateApproved, [2]int{24, 0}, zero, zero, 4},
			{displayNameB, "2026-08", workmonth.StateApproved, [2]int{30, 0}, zero, zero, 5},
			{displayNameB, "2026-09", workmonth.StateApproved, [2]int{24, 0}, zero, zero, 4},
		}},
		{"2026-11-01（日曜。当月は0件の Draft。前月は平日22の PendingApproval）", 2026, 11, 1, []monthExpect{
			{displayNameA, "2026-10", workmonth.StatePendingApproval, [2]int{168, 30}, zero, zero, 22},
			{displayNameA, "2026-11", workmonth.StateDraft, [2]int{0, 0}, nil, nil, 0},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contracts, workMonths := mustBuild(t, mustResetDate(t, tt.year, tt.month, tt.day))
			nameOf := map[string]string{}
			for _, c := range contracts {
				nameOf[c.ID.String()] = c.DisplayName
			}
			got := map[string]monthExpect{}
			for _, wm := range workMonths {
				e := monthExpect{
					Contract: nameOf[wm.ContractID().String()],
					YM:       ymKey(wm.YearMonth().Year(), wm.YearMonth().Month()),
					State:    wm.State(),
					Total:    hoursMinutes(wm.TotalHours()),
					Records:  len(wm.DailyRecords()),
				}
				if x, ok := wm.Excess(); ok {
					v := hoursMinutes(x)
					e.Excess = &v
				}
				if s, ok := wm.Shortfall(); ok {
					v := hoursMinutes(s)
					e.Shortage = &v
				}
				got[e.Contract+" "+e.YM] = e
			}
			for _, want := range tt.want {
				if diff := cmp.Diff(want, got[want.Contract+" "+want.YM]); diff != "" {
					t.Errorf("%s %s (-want +got):\n%s", want.Contract, want.YM, diff)
				}
			}
		})
	}
}

// ---- 決定性（AC-9-4） --------------------------------------------------------

type buildView struct {
	Contracts  []contractView
	WorkMonths []monthView
}

type contractView struct {
	ID, Name, Engineer string
	Range              [4]int
	Start              string
	End                *string
}

type monthView struct {
	Contract, YM, State string
	Excess, Shortfall   *[2]int
	Records             []recordShape
}

func viewOfBuild(contracts []gateway.DemoContract, workMonths []*workmonth.WorkMonth) buildView {
	var v buildView
	for _, c := range contracts {
		start, end := periodOf(c)
		lo, up := c.SettlementRange.LowerBound(), c.SettlementRange.UpperBound()
		v.Contracts = append(v.Contracts, contractView{
			ID: c.ID.String(), Name: c.DisplayName, Engineer: c.EngineerID,
			Range: [4]int{lo.Hours(), lo.Minutes(), up.Hours(), up.Minutes()}, Start: start, End: end,
		})
	}
	for _, wm := range workMonths {
		m := monthView{
			Contract: wm.ContractID().String(), YM: ymKey(wm.YearMonth().Year(), wm.YearMonth().Month()), State: string(wm.State()),
		}
		if x, ok := wm.Excess(); ok {
			h := hoursMinutes(x)
			m.Excess = &h
		}
		if s, ok := wm.Shortfall(); ok {
			h := hoursMinutes(s)
			m.Shortfall = &h
		}
		for _, r := range wm.DailyRecords() {
			m.Records = append(m.Records, shapeOf(r))
		}
		v.WorkMonths = append(v.WorkMonths, m)
	}
	return v
}

// TestBuildDemoData_Deterministic は AC-9-4 を固定する。同じリセット日で2回組み立てると、
// 同じ契約・年月・状態・稼働実績・超過／不足の集合になる（順序も同じ）。
// 逆向きの対照として、リセット日が1か月ずれると別の集合になる。
func TestBuildDemoData_Deterministic(t *testing.T) {
	resetDate := mustResetDate(t, 2026, 10, 27)
	c1, w1 := mustBuild(t, resetDate)
	c2, w2 := mustBuild(t, resetDate)
	if diff := cmp.Diff(viewOfBuild(c1, w1), viewOfBuild(c2, w2)); diff != "" {
		t.Errorf("同じリセット日の2回の組み立てが異なる (-first +second):\n%s", diff)
	}

	c3, w3 := mustBuild(t, mustResetDate(t, 2026, 11, 27))
	if cmp.Diff(viewOfBuild(c1, w1), viewOfBuild(c3, w3)) == "" {
		t.Errorf("リセット日が違うのに同じ集合になった（組み立てがリセット日を使っていない）")
	}
}
