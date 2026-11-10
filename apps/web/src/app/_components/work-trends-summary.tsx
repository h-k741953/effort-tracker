// docs/specs/readonly-screens-implementation-design.md SP-3。平均・精算幅に収まった月・精算幅。フックを持たない。
// どの Card にも --danger を当てない（work-trends-ui.md 4-7）。

import { Card } from "@/components/card";
import { formatSettlementRange } from "@/lib/work-month";
import { averageLabel, settlementRangeSummary, withinCount, type WorkTrendRow } from "@/lib/work-trends";

function Value({ children }: { children: string }) {
  return (
    <p className="text-2xl font-semibold">
      {children} <span className="text-sm font-normal">時間</span>
    </p>
  );
}

export function WorkTrendsSummary({ rows }: { rows: readonly WorkTrendRow[] }) {
  const average = averageLabel(rows);
  const { within, settled } = withinCount(rows);
  const range = settlementRangeSummary(rows);
  return (
    <div className="grid gap-4 sm:grid-cols-3">
      <Card title="平均稼働（確定した月）">
        {average === undefined ? (
          <>
            <p className="text-2xl font-semibold">—</p>
            <p className="text-sm text-muted-foreground">確定した月はありません</p>
          </>
        ) : (
          <Value>{average}</Value>
        )}
      </Card>
      <Card title="精算幅に収まった月">
        <p className="text-2xl font-semibold">{`${within} / ${settled} か月`}</p>
      </Card>
      <Card title="精算幅">
        {range.kind === "same" ? (
          <Value>{formatSettlementRange(range.range)}</Value>
        ) : (
          <p className="text-2xl font-semibold">月によって異なります</p>
        )}
      </Card>
    </div>
  );
}
