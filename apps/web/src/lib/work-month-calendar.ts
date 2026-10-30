// docs/specs/web-screens-implementation-design.md M-6・M-11。純粋な関数。
// 祝日の判定は引数 isHoliday だけを引く（lib/holidays/ を輸入しない）。

import { formatHours, formatTimeRange, type DailyRecord } from "./work-month";

export type DayMark = "sat" | "sun" | "hol" | "";
export type CalendarCell =
  | { kind: "blank" }
  | {
      kind: "day";
      date: string;
      day: number;
      weekday: string;
      mark: DayMark;
      holiday: boolean;
      future: boolean;
      record: DailyRecord | undefined;
    };
type DayCell = Extract<CalendarCell, { kind: "day" }>;

export const WEEKDAY_HEADERS: readonly string[] = ["月", "火", "水", "木", "金", "土", "日"];

export function calendarCells(params: {
  yearMonth: string;
  records: readonly DailyRecord[];
  isHoliday: (date: string) => boolean;
  today: string;
}): CalendarCell[] {
  const { yearMonth, records, isHoliday, today } = params;
  const [year, month] = yearMonth.split("-").map(Number);
  // UTC の暦で曜日を求める（0=月 … 6=日）。
  const leading = (new Date(Date.UTC(year, month - 1, 1)).getUTCDay() + 6) % 7;
  const lastDay = new Date(Date.UTC(year, month, 0)).getUTCDate();
  const cells: CalendarCell[] = Array.from({ length: leading }, () => ({ kind: "blank" }));
  for (let day = 1; day <= lastDay; day += 1) {
    const date = `${yearMonth}-${String(day).padStart(2, "0")}`;
    const weekdayIndex = (leading + day - 1) % 7;
    const holiday = isHoliday(date);
    const mark: DayMark = holiday ? "hol" : weekdayIndex === 5 ? "sat" : weekdayIndex === 6 ? "sun" : "";
    cells.push({
      kind: "day",
      date,
      day,
      weekday: WEEKDAY_HEADERS[weekdayIndex],
      mark,
      holiday,
      future: date > today,
      record: records.find((r) => r.date === date),
    });
  }
  while (cells.length % 7 !== 0) cells.push({ kind: "blank" });
  return cells;
}

export function dayLabel(cell: DayCell): string {
  const month = Number(cell.date.slice(5, 7));
  const head = `${month}月${cell.day}日（${cell.holiday ? "祝" : cell.weekday}）`;
  const body =
    cell.record === undefined
      ? "未入力"
      : `${formatHours(cell.record.workingHours)} 時間（${formatTimeRange(cell.record.startTime, cell.record.endTime)}）`;
  return `${head} ${body}${cell.future ? "（未来の日付）" : ""}`;
}

export function firstInputDay(cells: readonly CalendarCell[]): string {
  const days = cells.filter((c): c is DayCell => c.kind === "day");
  const found = days.find((c) => c.mark === "" && !c.future);
  return found !== undefined ? found.date : days[0].date;
}
