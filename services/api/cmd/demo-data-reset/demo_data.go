package main

import (
	"fmt"
	"time"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// デモの契約の識別子は、ここ1箇所に持つ（docs/specs/demo-data-reset.md AC-6-4）。
const (
	demoContractIDA = "00000000-0000-4000-8000-0000000000a1"
	demoContractIDB = "00000000-0000-4000-8000-0000000000b1"
)

// 日の型（V-5）。
type dayKind int

const (
	kindStandard dayKind = iota
	kindOver
	kindShort
	kindB
)

// plannedMonth は V-3 の勤務月1件の配置。monthsAgo はリセット日の年月から数えた N か月前。
type plannedMonth struct {
	monthsAgo int
	approved  bool
	pending   bool
	kind      dayKind
}

var (
	// 契約 A（平日）。当月（monthsAgo 0）は Draft。
	demoPlanA = []plannedMonth{
		{5, true, false, kindStandard},
		{4, true, false, kindOver},
		{3, true, false, kindStandard},
		{2, true, false, kindShort},
		{1, false, true, kindStandard},
		{0, false, false, kindStandard},
	}
	// 契約 B（土曜）。
	demoPlanB = []plannedMonth{
		{3, true, false, kindB},
		{2, true, false, kindB},
		{1, true, false, kindB},
	}
)

// buildDemoData は V-1〜V-6 に従い、デモの契約と勤務月を組み立てる（AC-9）。
// 入力はリセット日とデモの技術者識別子だけで、時計・乱数・環境変数を読まない（AC-9-5）。
func buildDemoData(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error) {
	idA, err := workmonth.NewContractID(demoContractIDA)
	if err != nil {
		return nil, nil, fmt.Errorf("demo data: contract id: %w", err)
	}
	idB, err := workmonth.NewContractID(demoContractIDB)
	if err != nil {
		return nil, nil, fmt.Errorf("demo data: contract id: %w", err)
	}
	rangeA, err := newRange(140, 180)
	if err != nil {
		return nil, nil, err
	}
	rangeB, err := newRange(20, 40)
	if err != nil {
		return nil, nil, err
	}

	startA, err := yearMonthBefore(resetDate, 6)
	if err != nil {
		return nil, nil, err
	}
	startB, err := yearMonthBefore(resetDate, 3)
	if err != nil {
		return nil, nil, err
	}
	endB, err := yearMonthBefore(resetDate, 1)
	if err != nil {
		return nil, nil, err
	}

	contracts := []gateway.DemoContract{
		{ID: idA, DisplayName: "基幹システム更改", EngineerID: engineerID, SettlementRange: rangeA, PeriodStart: startA},
		{ID: idB, DisplayName: "運用保守", EngineerID: engineerID, SettlementRange: rangeB, PeriodStart: startB, PeriodEnd: &endB},
	}

	var workMonths []*workmonth.WorkMonth
	for _, plan := range []struct {
		id    workmonth.ContractID
		rng   workmonth.SettlementRange
		slots []plannedMonth
	}{
		{idA, rangeA, demoPlanA},
		{idB, rangeB, demoPlanB},
	} {
		for _, s := range plan.slots {
			wm, err := buildWorkMonth(resetDate, plan.id, plan.rng, s)
			if err != nil {
				return nil, nil, err
			}
			workMonths = append(workMonths, wm)
		}
	}
	return contracts, workMonths, nil
}

func newRange(lowerHours, upperHours int) (workmonth.SettlementRange, error) {
	lower, err := workmonth.NewWorkingHours(lowerHours, 0)
	if err != nil {
		return workmonth.SettlementRange{}, fmt.Errorf("demo data: settlement range: %w", err)
	}
	upper, err := workmonth.NewWorkingHours(upperHours, 0)
	if err != nil {
		return workmonth.SettlementRange{}, fmt.Errorf("demo data: settlement range: %w", err)
	}
	r, err := workmonth.NewSettlementRange(lower, upper)
	if err != nil {
		return workmonth.SettlementRange{}, fmt.Errorf("demo data: settlement range: %w", err)
	}
	return r, nil
}

// yearMonthBefore はリセット日の年月から n か月さかのぼった年月を返す（AC-9-9。年をまたぐ）。
func yearMonthBefore(resetDate workmonth.Date, n int) (workmonth.YearMonth, error) {
	index := resetDate.Year()*12 + (resetDate.Month() - 1) - n
	ym, err := workmonth.NewYearMonth(index/12, index%12+1)
	if err != nil {
		return workmonth.YearMonth{}, fmt.Errorf("demo data: year month: %w", err)
	}
	return ym, nil
}

func buildWorkMonth(resetDate workmonth.Date, contractID workmonth.ContractID, rng workmonth.SettlementRange, s plannedMonth) (*workmonth.WorkMonth, error) {
	ym, err := yearMonthBefore(resetDate, s.monthsAgo)
	if err != nil {
		return nil, err
	}
	wm, err := workmonth.New(contractID, ym, rng)
	if err != nil {
		return nil, fmt.Errorf("demo data: new work month: %w", err)
	}

	// 当月は、リセット日の前日まで（AC-9-7）。過去の月は月末まで。
	// monthEnd は月末日。lastDay は記録を置く打ち切り日（当月はリセット日の前日）。
	monthEnd := lastDayOf(ym.Year(), ym.Month())
	lastDay := monthEnd
	if s.monthsAgo == 0 {
		lastDay = resetDate.Day() - 1
	}
	wantWeekday := s.kind != kindB // B は土曜、A は平日。日曜は置かない。
	k := 0
	for day := 1; day <= monthEnd; day++ {
		w := time.Date(ym.Year(), time.Month(ym.Month()), day, 0, 0, 0, 0, time.UTC).Weekday()
		isWorkday := w == time.Saturday
		if wantWeekday {
			isWorkday = w >= time.Monday && w <= time.Friday
		}
		if !isWorkday {
			continue
		}
		k++
		if day > lastDay {
			continue
		}
		date, err := workmonth.NewDate(ym.Year(), ym.Month(), day)
		if err != nil {
			return nil, fmt.Errorf("demo data: date: %w", err)
		}
		record, err := buildDailyRecord(date, s.kind, k)
		if err != nil {
			return nil, err
		}
		if err := wm.EnterDailyRecord(record, resetDate); err != nil {
			return nil, fmt.Errorf("demo data: enter daily record: %w", err)
		}
	}

	if s.pending || s.approved {
		if err := wm.Close(); err != nil {
			return nil, fmt.Errorf("demo data: close: %w", err)
		}
	}
	if s.approved {
		if err := wm.Approve(); err != nil {
			return nil, fmt.Errorf("demo data: approve: %w", err)
		}
	}
	return wm, nil
}

func lastDayOf(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// workday は V-5 の1日分。hasBreak が false なら休憩なし。
type workday struct {
	startH, startM, endH, endM int
	hasBreak                   bool
}

// 休憩は、休憩ありの型すべてで 12:00〜13:00。
func lunchWorkday(startH, startM, endH, endM int) workday {
	return workday{startH, startM, endH, endM, true}
}

// workdayFor は V-5 の表を引く。k は月の中で何番目の稼働日か（1 から）。
func workdayFor(kind dayKind, k int) workday {
	switch kind {
	case kindStandard:
		switch k % 5 {
		case 1:
			return lunchWorkday(9, 0, 18, 0)
		case 2:
			return lunchWorkday(9, 0, 18, 30)
		case 3:
			return lunchWorkday(8, 45, 17, 45)
		case 4:
			return lunchWorkday(9, 0, 17, 30)
		default:
			return workday{9, 0, 15, 0, false}
		}
	case kindOver:
		if k%2 == 1 {
			return lunchWorkday(9, 0, 19, 0)
		}
		return lunchWorkday(9, 0, 19, 30)
	case kindShort:
		if k%2 == 1 {
			return lunchWorkday(9, 0, 15, 45)
		}
		return lunchWorkday(9, 0, 16, 0)
	default: // kindB
		return lunchWorkday(10, 0, 17, 0)
	}
}

func buildDailyRecord(date workmonth.Date, kind dayKind, k int) (workmonth.DailyRecord, error) {
	p := workdayFor(kind, k)
	start, err := workmonth.NewTimeOfDay(p.startH, p.startM)
	if err != nil {
		return workmonth.DailyRecord{}, fmt.Errorf("demo data: start time: %w", err)
	}
	end, err := workmonth.NewTimeOfDay(p.endH, p.endM)
	if err != nil {
		return workmonth.DailyRecord{}, fmt.Errorf("demo data: end time: %w", err)
	}
	var brk *workmonth.Break
	if p.hasBreak {
		bs, err := workmonth.NewTimeOfDay(12, 0)
		if err != nil {
			return workmonth.DailyRecord{}, fmt.Errorf("demo data: break start: %w", err)
		}
		be, err := workmonth.NewTimeOfDay(13, 0)
		if err != nil {
			return workmonth.DailyRecord{}, fmt.Errorf("demo data: break end: %w", err)
		}
		b, err := workmonth.NewBreak(bs, be)
		if err != nil {
			return workmonth.DailyRecord{}, fmt.Errorf("demo data: break: %w", err)
		}
		brk = &b
	}
	record, err := workmonth.NewDailyRecord(date, start, end, brk)
	if err != nil {
		return workmonth.DailyRecord{}, fmt.Errorf("demo data: daily record: %w", err)
	}
	return record, nil
}
