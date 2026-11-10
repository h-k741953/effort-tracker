// docs/specs/readonly-screens-implementation-design.md SP-3。フックを持たない（値とハンドラを props で受ける）。

import { SelectField } from "@/components/select-field";
import type { FilterOption } from "@/lib/work-month-list";
import { endOptions, withStart, type TrendPeriod } from "@/lib/work-trends";

export function WorkTrendsFilters({
  contractId,
  contractOptions,
  period,
  periodOptions,
  onContractChange,
  onPeriodChange,
}: {
  contractId: string;
  contractOptions: readonly FilterOption[];
  /** 省くと期間のグループを出さない（8-2）。 */
  period?: TrendPeriod;
  periodOptions: readonly FilterOption[];
  onContractChange: (contractId: string) => void;
  onPeriodChange: (period: TrendPeriod) => void;
}) {
  return (
    <div role="search" aria-label="稼働の推移の絞り込み" className="flex flex-wrap items-end gap-3">
      <SelectField
        id="work-trends-contract"
        label="契約"
        value={contractId}
        options={contractOptions}
        onChange={onContractChange}
      />
      {period !== undefined ? (
        <div role="group" aria-label="期間" className="flex flex-wrap items-end gap-3">
          <SelectField
            id="work-trends-period-start"
            label="期間（開始）"
            value={period.start}
            options={periodOptions}
            onChange={(start) => onPeriodChange(withStart(period, start))}
          />
          <SelectField
            id="work-trends-period-end"
            label="期間（終了）"
            value={period.end}
            options={endOptions(periodOptions, period.start)}
            onChange={(end) => onPeriodChange({ start: period.start, end })}
          />
        </div>
      ) : null}
    </div>
  );
}
