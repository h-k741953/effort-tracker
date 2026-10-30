// docs/specs/web-screens-implementation-design.md W-3・W-4・W-8。純粋な関数。

export type WorkMonthState = "Draft" | "PendingApproval" | "Approved";

export interface WorkMonthRow {
  contractId: string;
  contractDisplayName: string;
  yearMonth: string;
  state: WorkMonthState;
}

export interface ContractSummary {
  contractId: string;
  contractDisplayName: string;
}

export interface WorkMonthFilter {
  contractId: string;
  yearMonth: string;
  state: "" | WorkMonthState;
}

export interface FilterOption {
  value: string;
  label: string;
}

export const EMPTY_WORK_MONTH_FILTER: WorkMonthFilter = { contractId: "", yearMonth: "", state: "" };

/** 1ページの件数。仕様の値ではない（W-3）。 */
export const WORK_MONTH_LIST_PAGE_SIZE = 10;

const STATE_LABELS: Record<WorkMonthState, string> = {
  Draft: "下書き",
  PendingApproval: "締め済",
  Approved: "承認済",
};

export function filterWorkMonths(rows: readonly WorkMonthRow[], filter: WorkMonthFilter): WorkMonthRow[] {
  return rows.filter(
    (r) =>
      (filter.contractId === "" || r.contractId === filter.contractId) &&
      (filter.yearMonth === "" || r.yearMonth === filter.yearMonth) &&
      (filter.state === "" || r.state === filter.state),
  );
}

export function hasActiveFilter(filter: WorkMonthFilter): boolean {
  return filter.contractId !== "" || filter.yearMonth !== "" || filter.state !== "";
}

export function contractOptionsFromContracts(contracts: readonly ContractSummary[]): FilterOption[] {
  return contracts.map((c) => ({ value: c.contractId, label: c.contractDisplayName }));
}

export function contractOptionsFromRows(rows: readonly WorkMonthRow[]): FilterOption[] {
  const seen = new Set<string>();
  const options: FilterOption[] = [];
  for (const r of rows) {
    if (seen.has(r.contractId)) continue;
    seen.add(r.contractId);
    options.push({ value: r.contractId, label: r.contractDisplayName });
  }
  return options;
}

export function yearMonthOptions(rows: readonly WorkMonthRow[]): FilterOption[] {
  const months = [...new Set(rows.map((r) => r.yearMonth))].sort().reverse();
  return months.map((m) => ({ value: m, label: formatYearMonth(m) }));
}

export function stateOptions(): FilterOption[] {
  return (["Draft", "PendingApproval", "Approved"] as const).map((s) => ({
    value: s,
    label: STATE_LABELS[s],
  }));
}

export function pageCount(rowCount: number): number {
  return Math.max(1, Math.ceil(rowCount / WORK_MONTH_LIST_PAGE_SIZE));
}

export function pageRows<T>(rows: readonly T[], page: number): T[] {
  const start = (page - 1) * WORK_MONTH_LIST_PAGE_SIZE;
  return rows.slice(start, start + WORK_MONTH_LIST_PAGE_SIZE);
}

export function latestRow(rows: readonly WorkMonthRow[]): WorkMonthRow | undefined {
  return rows[0];
}

export function formatYearMonth(yearMonth: string): string {
  const [year, month] = yearMonth.split("-");
  return `${Number(year)}年${Number(month)}月`;
}
