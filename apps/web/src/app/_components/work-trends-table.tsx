// docs/specs/readonly-screens-implementation-design.md SP-3。図と同じ内容の表。フックを持たない。
// 画面に見える形で置き、視覚的にも支援技術にも隠さない（決定2）。表の文字に --danger を当てない。

import { formatHours, formatSettlementRange } from "@/lib/work-month";
import { monthAxisLabels, monthKind, type WorkTrendRow } from "@/lib/work-trends";

function kindLabel(row: WorkTrendRow): string {
  switch (monthKind(row)) {
    case "in-progress":
      return "入力中";
    case "excess":
      return `超過 ${formatHours(row.excess!)} 時間`;
    case "shortfall":
      return `不足 ${formatHours(row.shortfall!)} 時間`;
    case "within":
      return "精算幅に収まった";
  }
}

export function WorkTrendsTable({ rows }: { rows: readonly WorkTrendRow[] }) {
  const labels = monthAxisLabels(rows);
  return (
    <table className="w-full text-left text-sm">
      <thead>
        <tr>
          <th scope="col" className="px-2 py-1 font-medium">対象年月</th>
          <th scope="col" className="px-2 py-1 font-medium">総稼働時間</th>
          <th scope="col" className="px-2 py-1 font-medium">区分</th>
          <th scope="col" className="px-2 py-1 font-medium">精算幅</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r, i) => (
          <tr key={r.yearMonth} className="border-t border-border">
            <th scope="row" className="px-2 py-2 font-normal">{labels[i]}</th>
            <td className="px-2 py-2">{`${formatHours(r.totalHours)} 時間`}</td>
            <td className="px-2 py-2">{kindLabel(r)}</td>
            <td className="px-2 py-2">{`${formatSettlementRange(r.settlementRange)} 時間`}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
