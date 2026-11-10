// docs/specs/readonly-screens-implementation-design.md SP-4・SP-5。稼働の推移の純粋な計算。
// 入力の配列を書き換えず、新しい値を返す。超過・不足は算出し直さず、保存された値を読む
// （work-trends-ui.md 4-3-a・9-3）。

import type { WorkingHours } from "./working-hours";
import { formatYearMonth, type FilterOption, type WorkMonthState } from "./work-month-list";

/** R-11（domain-api-http-contract.md AC-10-5）の items[] の1行。 */
export interface WorkTrendRow {
  contractId: string;
  yearMonth: string;
  state: WorkMonthState;
  settlementRange: { lowerBound: WorkingHours; upperBound: WorkingHours };
  totalHours: WorkingHours;
  excess: WorkingHours | null;
  shortfall: WorkingHours | null;
}

export interface TrendPeriod {
  start: string;
  end: string;
}

export type MonthKind = "in-progress" | "excess" | "shortfall" | "within";

export type SettlementRangeSummary =
  | { kind: "same"; range: WorkTrendRow["settlementRange"] }
  | { kind: "varies" };

const MINUTES_PER_HOUR = 60;

function toMinutes(value: WorkingHours): number {
  return value.hours * MINUTES_PER_HOUR + value.minutes;
}

function isNonZero(value: WorkingHours | null): boolean {
  return value !== null && (value.hours !== 0 || value.minutes !== 0);
}

export function rowsOfContract(rows: readonly WorkTrendRow[], contractId: string): WorkTrendRow[] {
  return rows.filter((r) => r.contractId === contractId);
}

function sortedMonths(rows: readonly WorkTrendRow[]): string[] {
  return [...new Set(rows.map((r) => r.yearMonth))].sort();
}

export function defaultPeriod(rows: readonly WorkTrendRow[]): TrendPeriod | undefined {
  const months = sortedMonths(rows);
  if (months.length === 0) return undefined;
  return { start: months[0], end: months[months.length - 1] };
}

export function periodOptions(rows: readonly WorkTrendRow[]): FilterOption[] {
  return sortedMonths(rows).map((m) => ({ value: m, label: formatYearMonth(m) }));
}

export function endOptions(options: readonly FilterOption[], start: string): FilterOption[] {
  return options.filter((o) => o.value >= start);
}

export function withStart(period: TrendPeriod, start: string): TrendPeriod {
  return { start, end: period.end < start ? start : period.end };
}

export function rowsInPeriod(rows: readonly WorkTrendRow[], period: TrendPeriod): WorkTrendRow[] {
  return rows.filter((r) => r.yearMonth >= period.start && r.yearMonth <= period.end);
}

export function monthKind(row: WorkTrendRow): MonthKind {
  if (row.state === "Draft") return "in-progress";
  if (isNonZero(row.excess)) return "excess";
  if (isNonZero(row.shortfall)) return "shortfall";
  return "within";
}

function settledRows(rows: readonly WorkTrendRow[]): WorkTrendRow[] {
  return rows.filter((r) => r.state !== "Draft");
}

/** 確定した月の平均を、分の整数の演算だけで時間の小数第1位へ四捨五入する。0件は undefined。 */
export function averageLabel(rows: readonly WorkTrendRow[]): string | undefined {
  const settled = settledRows(rows);
  if (settled.length === 0) return undefined;
  const sum = settled.reduce((acc, r) => acc + toMinutes(r.totalHours), 0);
  // 平均（時間）= sum / (60 × n)。10 倍した値を四捨五入する: floor((2 × sum + 6n) / (12n))。
  const numerator = 2 * sum + 6 * settled.length;
  const denominator = 12 * settled.length;
  const tenths = (numerator - (numerator % denominator)) / denominator;
  const fraction = tenths % 10;
  return `${(tenths - fraction) / 10}.${fraction}`;
}

export function withinCount(rows: readonly WorkTrendRow[]): { within: number; settled: number } {
  const settled = settledRows(rows);
  return { within: settled.filter((r) => monthKind(r) === "within").length, settled: settled.length };
}

export function settlementRangeSummary(rows: readonly WorkTrendRow[]): SettlementRangeSummary {
  const first = rows[0].settlementRange;
  const same = rows.every(
    (r) =>
      r.settlementRange.lowerBound.hours === first.lowerBound.hours &&
      r.settlementRange.lowerBound.minutes === first.lowerBound.minutes &&
      r.settlementRange.upperBound.hours === first.upperBound.hours &&
      r.settlementRange.upperBound.minutes === first.upperBound.minutes,
  );
  return same ? { kind: "same", range: first } : { kind: "varies" };
}

/** 縦軸の範囲（時間）。入力中の月も含める（SP-7 (b)）。 */
export function axisRange(rows: readonly WorkTrendRow[]): { start: number; end: number } {
  const values = rows.flatMap((r) => [
    toMinutes(r.totalHours),
    toMinutes(r.settlementRange.lowerBound),
    toMinutes(r.settlementRange.upperBound),
  ]);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const tenHours = 10 * MINUTES_PER_HOUR;
  return {
    start: Math.max(0, Math.floor((min - 20 * MINUTES_PER_HOUR) / tenHours) * 10),
    end: Math.ceil((max + tenHours) / tenHours) * 10,
  };
}

/** 先頭の行と月が 1 の行は「2026年7月」の形、ほかは「8月」の形（S-1）。 */
export function monthAxisLabels(rows: readonly WorkTrendRow[]): string[] {
  return rows.map((r, i) => {
    const month = Number(r.yearMonth.split("-")[1]);
    return i === 0 || month === 1 ? formatYearMonth(r.yearMonth) : `${month}月`;
  });
}
