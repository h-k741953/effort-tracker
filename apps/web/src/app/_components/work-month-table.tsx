// docs/specs/web-screens-implementation-design.md W-4・W-5・W-8。フックを持たない（どちらからも使える）。

import Link from "next/link";
import { StatusBadge } from "@/components/status-badge";
import { workMonthPath } from "@/lib/screen-routes";
import { formatYearMonth, type WorkMonthRow } from "@/lib/work-month-list";

export function WorkMonthTable({ label, rows }: { label: string; rows: readonly WorkMonthRow[] }) {
  return (
    <table aria-label={label} className="w-full text-left text-sm">
      <thead>
        <tr>
          <th scope="col" className="px-2 py-1 font-medium">契約</th>
          <th scope="col" className="px-2 py-1 font-medium">対象年月</th>
          <th scope="col" className="px-2 py-1 font-medium">状態</th>
          <th scope="col" className="px-2 py-1">
            <span className="sr-only">操作</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => {
          const month = formatYearMonth(r.yearMonth);
          return (
            <tr key={`${r.contractId}/${r.yearMonth}`} className="border-t border-border">
              <td className="px-2 py-2">{r.contractDisplayName}</td>
              <td className="px-2 py-2">{month}</td>
              <td className="px-2 py-2">
                <StatusBadge state={r.state} />
              </td>
              <td className="px-2 py-2">
                <Link
                  href={workMonthPath(r.contractId, r.yearMonth)}
                  aria-label={`${r.contractDisplayName} × ${month}を開く`}
                  className="text-primary underline focus-visible:ring-2 focus-visible:ring-focus-ring"
                >
                  開く
                </Link>
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
