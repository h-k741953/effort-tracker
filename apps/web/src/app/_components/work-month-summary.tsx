// docs/specs/web-screens-implementation-design.md M-5。総稼働時間・精算幅・超過／不足。フックを持たない。

import { Card } from "@/components/card";
import { formatHours, type WorkMonthDetail } from "@/lib/work-month";
import type { WorkingHours } from "@/lib/working-hours";

const ZERO: WorkingHours = { hours: 0, minutes: 0 };

function isZero(value: WorkingHours): boolean {
  return value.hours === 0 && value.minutes === 0;
}

function Value({ children, danger }: { children: string; danger?: boolean }) {
  return (
    <p className={danger ? "text-2xl font-semibold text-danger" : "text-2xl font-semibold"}>
      {children} <span className="text-sm font-normal">時間</span>
    </p>
  );
}

function Deviation({ label, value }: { label: string; value: WorkingHours }) {
  const danger = !isZero(value);
  return (
    <Card title={danger ? <span className="text-danger">{label}</span> : label}>
      <Value danger={danger}>{formatHours(value)}</Value>
    </Card>
  );
}

export function WorkMonthSummary({ workMonth }: { workMonth: WorkMonthDetail }) {
  const { settlementRange, totalHours, state, excess, shortfall } = workMonth;
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Card title="総稼働時間">
        <Value>{formatHours(totalHours)}</Value>
      </Card>
      <Card title="精算幅">
        <Value>{`${formatHours(settlementRange.lowerBound)}〜${formatHours(settlementRange.upperBound)}`}</Value>
      </Card>
      {state === "Draft" ? (
        <Card title="超過／不足">
          <p className="text-sm">締め時に確定します</p>
        </Card>
      ) : (
        <>
          <Deviation label="超過" value={excess ?? ZERO} />
          <Deviation label="不足" value={shortfall ?? ZERO} />
        </>
      )}
    </div>
  );
}
