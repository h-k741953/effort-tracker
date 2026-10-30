// docs/specs/web-screens-implementation-design.md W-6・W-8。
// 自分の契約の件数で、当月の入力の導線を hidden / link / choice に分ける。年月は受けない（Q-10）。

import { thisMonthPath } from "./screen-routes";
import type { ContractSummary } from "./work-month-list";

export type ThisMonthEntry =
  | { kind: "hidden" }
  | { kind: "link"; href: string }
  | { kind: "choice"; options: readonly { label: string; href: string }[] };

export function thisMonthEntry(contracts: readonly ContractSummary[]): ThisMonthEntry {
  if (contracts.length === 0) return { kind: "hidden" };
  if (contracts.length === 1) return { kind: "link", href: thisMonthPath(contracts[0].contractId) };
  return {
    kind: "choice",
    options: contracts.map((c) => ({
      label: c.contractDisplayName,
      href: thisMonthPath(c.contractId),
    })),
  };
}
