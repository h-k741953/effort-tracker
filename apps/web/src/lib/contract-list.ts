// docs/specs/readonly-screens-implementation-design.md KP-3。契約一覧の行の型と契約期間の表記。

import type { WorkingHours } from "./working-hours";
import { formatYearMonth, type ContractSummary, type WorkMonthState } from "./work-month-list";

/** R-10（domain-api-http-contract.md AC-10-4）の items[] の1行。 */
export interface ContractListItem extends ContractSummary {
  settlementRange: { lowerBound: WorkingHours; upperBound: WorkingHours };
  contractPeriod: { start: string; end: string | null };
  workMonth: { yearMonth: string; state: WorkMonthState; generated: boolean };
}

/** contracts-ui.md 2-3-a・2-3-b。終了が定まらないときは「〜」の後に何も置かない。 */
export function formatContractPeriod(period: ContractListItem["contractPeriod"]): string {
  const start = formatYearMonth(period.start);
  return period.end === null ? `${start}〜` : `${start}〜${formatYearMonth(period.end)}`;
}
