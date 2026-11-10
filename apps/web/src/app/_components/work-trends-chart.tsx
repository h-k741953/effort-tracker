// docs/specs/readonly-screens-implementation-design.md SP-3。月ごとの総稼働時間の図。フックを持たない。
// HTML と CSS で描く（依存・SVG を使わない。SP-7 (a)）。色はクラスで当て、style は高さ・位置の割合だけに使う。
// 赤（--danger）は work-trends-ui.md 5-4 の要素だけ: 超過の棒の上限を超えた部分、不足の月の棒の全体と
// 棒の上端から下限までの部分、札の文字、凡例の超過・不足の見本。

import type { CSSProperties } from "react";
import { Card } from "@/components/card";
import { formatHours } from "@/lib/work-month";
import type { WorkingHours } from "@/lib/working-hours";
import { axisRange, monthAxisLabels, monthKind, type WorkTrendRow } from "@/lib/work-trends";
import { WorkTrendsTable } from "./work-trends-table";

const LEGEND_SWATCH = "inline-block size-3 rounded-sm";
const BAR_CLASS = "bg-primary";
const DRAFT_CLASS = "bg-muted-foreground/40";
const BAND_CLASS = "bg-primary/15";
const DANGER_CLASS = "bg-danger";
const SHORT_CLASS = "bg-danger/40";

function hoursOf(value: WorkingHours): number {
  return (value.hours * 60 + value.minutes) / 60;
}

function percent(value: number): string {
  return `${Math.round(value * 1000) / 1000}%`;
}

function Segment({ className, bottom, height }: { className: string; bottom: number; height: number }) {
  const style: CSSProperties = { bottom: percent(bottom), height: percent(Math.max(0, height)) };
  return <div className={`absolute inset-x-0 mx-auto w-6 ${className}`} style={style} />;
}

function Column({ row, label, axis }: { row: WorkTrendRow; label: string; axis: { start: number; end: number } }) {
  const width = axis.end - axis.start;
  const at = (hours: number) => ((hours - axis.start) / width) * 100;
  const total = hoursOf(row.totalHours);
  const lower = hoursOf(row.settlementRange.lowerBound);
  const upper = hoursOf(row.settlementRange.upperBound);
  const kind = monthKind(row);
  const band: CSSProperties = { bottom: percent(at(lower)), height: percent(at(upper) - at(lower)) };

  let tag: string | undefined;
  if (kind === "excess" && row.excess !== null) tag = `超過 ${formatHours(row.excess)}`;
  if (kind === "shortfall" && row.shortfall !== null) tag = `不足 ${formatHours(row.shortfall)}`;

  return (
    <div className="flex min-w-8 flex-1 flex-col items-center gap-1 text-xs">
      <div className="relative h-48 w-full">
        <div className={`absolute inset-x-0 ${BAND_CLASS}`} style={band} />
        {kind === "excess" ? (
          <>
            <Segment className={BAR_CLASS} bottom={0} height={at(Math.min(total, upper))} />
            <Segment className={DANGER_CLASS} bottom={at(upper)} height={at(total) - at(upper)} />
          </>
        ) : null}
        {kind === "shortfall" ? (
          <>
            <Segment className={DANGER_CLASS} bottom={0} height={at(total)} />
            <Segment className={SHORT_CLASS} bottom={at(total)} height={at(lower) - at(total)} />
          </>
        ) : null}
        {kind === "within" ? <Segment className={BAR_CLASS} bottom={0} height={at(total)} /> : null}
        {kind === "in-progress" ? <Segment className={DRAFT_CLASS} bottom={0} height={at(total)} /> : null}
      </div>
      {tag !== undefined ? <span className="font-medium text-danger">{tag}</span> : null}
      <span className="tabular-nums">{formatHours(row.totalHours)}</span>
      {kind === "in-progress" ? <span className="text-muted-foreground">入力中</span> : null}
      <span className="tabular-nums text-muted-foreground">{label}</span>
    </div>
  );
}

function LegendItem({ swatch, children }: { swatch: string; children: string }) {
  return (
    <span className="flex items-center gap-2">
      <span className={`${LEGEND_SWATCH} ${swatch}`} />
      <span>{children}</span>
    </span>
  );
}

export function WorkTrendsChart({ rows }: { rows: readonly WorkTrendRow[] }) {
  const axis = axisRange(rows);
  const labels = monthAxisLabels(rows);
  return (
    <Card title="月ごとの総稼働時間">
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-4 text-sm">
          <LegendItem swatch={BAR_CLASS}>稼働時間</LegendItem>
          <LegendItem swatch={DANGER_CLASS}>超過した分</LegendItem>
          <LegendItem swatch={SHORT_CLASS}>不足している分</LegendItem>
          <LegendItem swatch={DRAFT_CLASS}>入力中（下書き）</LegendItem>
          <LegendItem swatch={BAND_CLASS}>精算幅</LegendItem>
        </div>
        <div className="overflow-x-auto">
          <div role="img" aria-label="月ごとの総稼働時間" className="flex gap-1">
            {rows.map((r, i) => (
              <Column key={r.yearMonth} row={r} label={labels[i]} axis={axis} />
            ))}
          </div>
        </div>
        {axis.start > 0 ? (
          <p className="text-xs text-muted-foreground">{`縦軸は ${axis.start}時間から始まります。`}</p>
        ) : null}
        <WorkTrendsTable rows={rows} />
      </div>
    </Card>
  );
}
