// docs/specs/web-screens-implementation-design.md M-3・M-5・M-11。純粋な関数と型。
// 勤務月の表現（domain-api-http-contract.md AC-10-1）の型、本人かの真偽の読み取り、時間・時刻の表記。

import type { Break, TimeOfDay, WorkingHours } from "./working-hours";
import type { WorkMonthState } from "./work-month-list";

export type Ownership = "own" | "other" | "unknown";

// 本人かの真偽のキー名。仮の名前であり、#155 の決定に従う。字面を持つのはここだけ（M-3）。
export const OPERATOR_OWNERSHIP_KEY = "operatorIsOwner";

export function readOwnership(body: unknown): Ownership {
  if (typeof body !== "object" || body === null || Array.isArray(body)) return "unknown";
  const value = (body as Record<string, unknown>)[OPERATOR_OWNERSHIP_KEY];
  if (value === true) return "own";
  if (value === false) return "other";
  return "unknown";
}

export interface DailyRecord {
  date: string;
  startTime: TimeOfDay;
  endTime: TimeOfDay;
  break: Break | null;
  workingHours: WorkingHours;
  roundedWorkingHours: WorkingHours;
}

export interface WorkMonthDetail {
  contractId: string;
  contractDisplayName: string;
  yearMonth: string;
  state: WorkMonthState;
  generated: boolean;
  settlementRange: { lowerBound: WorkingHours; upperBound: WorkingHours };
  totalHours: WorkingHours;
  excess: WorkingHours | null;
  shortfall: WorkingHours | null;
  dailyRecords: DailyRecord[];
}

/** 小数第2位までに丸め、末尾が 0 なら1字だけ落とす（M-5）。 */
export function formatHours(value: WorkingHours): string {
  const rounded = Math.round((value.hours + value.minutes / 60) * 100) / 100;
  const text = rounded.toFixed(2);
  return text.endsWith("0") ? text.slice(0, -1) : text;
}

export function formatTimeOfDay(time: TimeOfDay): string {
  return `${time.hour}:${String(time.minute).padStart(2, "0")}`;
}

export function formatTimeRange(start: TimeOfDay, end: TimeOfDay): string {
  return `${formatTimeOfDay(start)}〜${formatTimeOfDay(end)}`;
}
