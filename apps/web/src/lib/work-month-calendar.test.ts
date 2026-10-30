import { describe, expect, it } from "vitest";
import { dailyRecord, hm, t } from "@/test-support/work-month";
import {
  WEEKDAY_HEADERS,
  calendarCells,
  dayLabel,
  firstInputDay,
  type CalendarCell,
} from "./work-month-calendar";

// docs/specs/daily-record-entry-ui.md 1-4・1-5・4-2・9-1〜9-7・9-9。設計 M-6・M-11・T-3。
// 祝日は Fake の isHoliday（日付の集合）で与える。lib/holidays/ を輸入しない（M-1）。

type DayCell = Extract<CalendarCell, { kind: "day" }>;

const NO_HOLIDAY = () => false;
const holidaysOf =
  (...dates: string[]) =>
  (date: string) =>
    dates.includes(date);

function cellsOf(params: {
  yearMonth: string;
  records?: Parameters<typeof calendarCells>[0]["records"];
  isHoliday?: (date: string) => boolean;
  today?: string;
}) {
  return calendarCells({
    yearMonth: params.yearMonth,
    records: params.records ?? [],
    isHoliday: params.isHoliday ?? NO_HOLIDAY,
    today: params.today ?? "2099-01-01",
  });
}

function dayCells(cells: readonly CalendarCell[]): DayCell[] {
  return cells.filter((c): c is DayCell => c.kind === "day");
}

function dayOf(cells: readonly CalendarCell[], date: string): DayCell {
  const found = dayCells(cells).find((c) => c.date === date);
  if (found === undefined) throw new Error(`日 ${date} のマスが無い`);
  return found;
}

describe("マスの数と先頭の空き（M-6。週の始まりは月曜）", () => {
  it.each([
    ["2026-06", 0, 30, 35, "1日が月曜"],
    ["2026-03", 6, 31, 42, "1日が日曜"],
    ["2026-02", 6, 28, 35, "1日が日曜・平年の2月"],
    ["2028-02", 1, 29, 35, "うるう年の2月"],
  ])("%s → 先頭の空き %i・日 %i・計 %i（%s）", (yearMonth, blanks, days, total) => {
    const cells = cellsOf({ yearMonth });
    expect(cells).toHaveLength(total);
    expect(dayCells(cells)).toHaveLength(days);
    expect(cells.slice(0, blanks).every((c) => c.kind === "blank")).toBe(true);
    expect(cells[blanks]).toMatchObject({ kind: "day", day: 1 });
    // 日は1日から末日まで続き、その後ろは空きマスだけ
    expect(dayCells(cells).map((c) => c.day)).toEqual(Array.from({ length: days }, (_, i) => i + 1));
    expect(cells.slice(blanks + days).every((c) => c.kind === "blank")).toBe(true);
    expect(cells.length % 7).toBe(0);
  });

  it("date は YYYY-MM-DD", () => {
    const cells = cellsOf({ yearMonth: "2026-10" });
    expect(dayCells(cells)[0].date).toBe("2026-10-01");
    expect(dayCells(cells)[30].date).toBe("2026-10-31");
  });
});

describe("weekday と WEEKDAY_HEADERS", () => {
  it("WEEKDAY_HEADERS は月〜日の7つ", () => {
    expect(WEEKDAY_HEADERS).toEqual(["月", "火", "水", "木", "金", "土", "日"]);
  });

  it("weekday は「月」〜「日」（2026-10-01 は木）。祝日でも曜日の字を持つ", () => {
    const cells = cellsOf({ yearMonth: "2026-10", isHoliday: holidaysOf("2026-10-12") });
    expect(dayOf(cells, "2026-10-01").weekday).toBe("木");
    expect(dayOf(cells, "2026-10-02").weekday).toBe("金");
    expect(dayOf(cells, "2026-10-03").weekday).toBe("土");
    expect(dayOf(cells, "2026-10-04").weekday).toBe("日");
    expect(dayOf(cells, "2026-10-12").weekday).toBe("月");
  });
});

describe("mark と holiday（9-1〜9-6）", () => {
  const cells = cellsOf({
    yearMonth: "2026-10",
    isHoliday: holidaysOf("2026-10-02", "2026-10-03", "2026-10-11"),
  });

  it.each([
    ["9-1: 祝日の平日（金）", "2026-10-02", "hol", true],
    ["9-3: 祝日の土曜（土曜より祝日を優先）", "2026-10-03", "hol", true],
    ["9-2: 祝日の日曜", "2026-10-11", "hol", true],
    ["9-4: 祝日でない日曜", "2026-10-04", "sun", false],
    ["9-5: 祝日でない土曜", "2026-10-10", "sat", false],
    ["9-6: 祝日でない平日", "2026-10-05", "", false],
  ])("%s → mark=%s・holiday=%s", (_name, date, mark, holiday) => {
    const cell = dayOf(cells, date);
    expect(cell.mark).toBe(mark);
    expect(cell.holiday).toBe(holiday);
  });

  it("isHoliday が呼ばれる引数は YYYY-MM-DD", () => {
    const seen: string[] = [];
    cellsOf({
      yearMonth: "2026-10",
      isHoliday: (date) => {
        seen.push(date);
        return false;
      },
    });
    expect(seen.length).toBeGreaterThan(0);
    for (const date of seen) expect(date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(new Set(seen).has("2026-10-01")).toBe(true);
  });
});

describe("future（4-2。JST 当日より後）", () => {
  const cells = cellsOf({ yearMonth: "2026-10", today: "2026-10-02" });

  it("当日は偽・翌日は真", () => {
    expect(dayOf(cells, "2026-10-02").future).toBe(false);
    expect(dayOf(cells, "2026-10-03").future).toBe(true);
    expect(dayOf(cells, "2026-10-01").future).toBe(false);
  });
});

describe("record（日付で結び付く）", () => {
  it("該当する日にだけ record が付く", () => {
    const record = dailyRecord("2026-10-05", t(9, 0), t(18, 0), null, hm(9, 0));
    const cells = cellsOf({ yearMonth: "2026-10", records: [record] });
    expect(dayOf(cells, "2026-10-05").record).toEqual(record);
    expect(dayCells(cells).filter((c) => c.record !== undefined)).toHaveLength(1);
  });
});

describe("dayLabel（M-6）", () => {
  it("実績のある日: 「10月2日（金） 9.75 時間（9:00〜19:45）」", () => {
    const record = dailyRecord("2026-10-02", t(9, 0), t(19, 45), { start: t(12, 0), end: t(13, 0) }, hm(9, 45));
    const cells = cellsOf({ yearMonth: "2026-10", records: [record], today: "2026-10-30" });
    expect(dayLabel(dayOf(cells, "2026-10-02"))).toBe("10月2日（金） 9.75 時間（9:00〜19:45）");
  });

  it("祝日は曜日の代わりに「祝」: 「10月12日（祝） 未入力」", () => {
    const cells = cellsOf({ yearMonth: "2026-10", isHoliday: holidaysOf("2026-10-12"), today: "2026-10-30" });
    expect(dayLabel(dayOf(cells, "2026-10-12"))).toBe("10月12日（祝） 未入力");
  });

  it("実績の無い日は「 未入力」", () => {
    const cells = cellsOf({ yearMonth: "2026-10", today: "2026-10-30" });
    expect(dayLabel(dayOf(cells, "2026-10-05"))).toBe("10月5日（月） 未入力");
  });

  it("未来の日は末尾に「（未来の日付）」を続ける", () => {
    const cells = cellsOf({ yearMonth: "2026-10", today: "2026-10-01" });
    expect(dayLabel(dayOf(cells, "2026-10-02"))).toBe("10月2日（金） 未入力（未来の日付）");
  });
});

describe("firstInputDay（M-6。祝日・土曜・日曜でなく未来でもない最初の日）", () => {
  it("祝日・土日を飛ばす: 10-01・10-02 が祝日なら 10-05（月）", () => {
    const cells = cellsOf({ yearMonth: "2026-10", isHoliday: holidaysOf("2026-10-01", "2026-10-02"), today: "2026-10-30" });
    expect(firstInputDay(cells)).toBe("2026-10-05");
  });

  it("休日でなければ1日", () => {
    const cells = cellsOf({ yearMonth: "2026-10", today: "2026-10-30" });
    expect(firstInputDay(cells)).toBe("2026-10-01");
  });

  it("未来の日を飛ばす: 該当が無ければ（未来の10-05 を選ばずに）1日を返す", () => {
    const cells = cellsOf({
      yearMonth: "2026-10",
      isHoliday: holidaysOf("2026-10-01", "2026-10-02"),
      today: "2026-10-03",
    });
    expect(firstInputDay(cells)).toBe("2026-10-01");
  });

  it("該当が無ければその月の1日（月全体が未来）", () => {
    const cells = cellsOf({ yearMonth: "2026-10", today: "2026-09-30" });
    expect(firstInputDay(cells)).toBe("2026-10-01");
  });
});
