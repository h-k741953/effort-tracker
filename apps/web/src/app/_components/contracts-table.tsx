// docs/specs/readonly-screens-implementation-design.md KP-1・KP-3。フックを持たない。

import Link from "next/link";
import { StatusBadge } from "@/components/status-badge";
import { formatContractPeriod, type ContractListItem } from "@/lib/contract-list";
import { ownWorkMonthsPath } from "@/lib/screen-routes";
import { formatSettlementRange } from "@/lib/work-month";

export function ContractsTable({ contracts }: { contracts: readonly ContractListItem[] }) {
  return (
    <table className="w-full text-left text-sm">
      <thead>
        <tr>
          <th scope="col" className="px-2 py-1 font-medium">契約</th>
          <th scope="col" className="px-2 py-1 font-medium">精算幅</th>
          <th scope="col" className="px-2 py-1 font-medium">契約期間</th>
          <th scope="col" className="px-2 py-1 font-medium">今月の状態</th>
          <th scope="col" className="px-2 py-1">
            <span className="sr-only">操作</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {contracts.map((c) => (
          <tr key={c.contractId} className="border-t border-border">
            <td className="px-2 py-2">{c.contractDisplayName}</td>
            <td className="px-2 py-2">{`${formatSettlementRange(c.settlementRange)} 時間`}</td>
            <td className="px-2 py-2">{formatContractPeriod(c.contractPeriod)}</td>
            <td className="px-2 py-2">
              <StatusBadge state={c.workMonth.state} />
            </td>
            <td className="px-2 py-2">
              <Link
                href={ownWorkMonthsPath({ contractId: c.contractId })}
                aria-label={`${c.contractDisplayName}の勤務月を見る`}
                className="text-primary underline focus-visible:ring-2 focus-visible:ring-focus-ring"
              >
                勤務月を見る
              </Link>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
