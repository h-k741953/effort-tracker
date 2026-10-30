// docs/specs/web-screens-implementation-design.md M-12。祝日の判定。計算で補わず、HOLIDAY_ROWS の date の集合だけを引く。

import { HOLIDAY_ROWS } from "./data";

const HOLIDAY_DATES: ReadonlySet<string> = new Set(HOLIDAY_ROWS.map((row) => row.date));

export function isHoliday(date: string): boolean {
  return HOLIDAY_DATES.has(date);
}
