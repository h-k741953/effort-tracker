// docs/specs/web-screens-implementation-design.md W-4・W-5・W-8。フックを持たない（値とハンドラを props で受ける）。

import { Button } from "@/components/button";
import { SelectField } from "@/components/select-field";
import { hasActiveFilter, type FilterOption, type WorkMonthFilter } from "@/lib/work-month-list";

const ALL = "すべて";

export function WorkMonthFilters({
  filter,
  contractOptions,
  yearMonthOptions,
  stateOptions,
  onChange,
  onClear,
}: {
  filter: WorkMonthFilter;
  contractOptions: readonly FilterOption[];
  yearMonthOptions: readonly FilterOption[];
  stateOptions?: readonly FilterOption[];
  onChange: (filter: WorkMonthFilter) => void;
  onClear: () => void;
}) {
  return (
    <div role="search" aria-label="勤務月の絞り込み" className="flex flex-wrap items-end gap-3">
      <SelectField
        id="work-month-filter-contract"
        label="契約"
        value={filter.contractId}
        options={contractOptions}
        placeholder={ALL}
        onChange={(contractId) => onChange({ ...filter, contractId })}
      />
      <SelectField
        id="work-month-filter-year-month"
        label="対象年月"
        value={filter.yearMonth}
        options={yearMonthOptions}
        placeholder={ALL}
        onChange={(yearMonth) => onChange({ ...filter, yearMonth })}
      />
      {stateOptions !== undefined ? (
        <SelectField
          id="work-month-filter-state"
          label="状態"
          value={filter.state}
          options={stateOptions}
          placeholder={ALL}
          onChange={(state) => onChange({ ...filter, state: state as WorkMonthFilter["state"] })}
        />
      ) : null}
      {hasActiveFilter(filter) ? <Button onClick={onClear}>条件をクリア</Button> : null}
    </div>
  );
}
