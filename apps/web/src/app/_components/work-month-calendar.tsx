// docs/specs/web-screens-implementation-design.md M-6。カレンダー。フックを持たない。
// 編集できるときは日を選ぶ button、それ以外は選べない要素で同じ内容を出す。

import { dayLabel, WEEKDAY_HEADERS, type CalendarCell } from "@/lib/work-month-calendar";
import { formatHours, formatTimeRange } from "@/lib/work-month";

type DayCell = Extract<CalendarCell, { kind: "day" }>;

const MARK_CLASSES: Record<DayCell["mark"], string> = {
  hol: "bg-et-sun-bg text-et-sun",
  sat: "bg-et-sat-bg text-et-sat",
  sun: "bg-et-sun-bg text-et-sun",
  "": "",
};

const CELL_CLASSES =
  "flex min-h-20 flex-col items-start gap-1 rounded-(--et-radius-control) border border-border p-2 text-left text-sm";

function DayContent({ cell }: { cell: DayCell }) {
  return (
    <>
      <span className="flex items-center gap-1 font-medium">
        <span>{cell.day}</span>
        {cell.holiday ? (
          <span className="rounded-(--et-radius-badge) bg-et-sun-chip px-1 text-xs text-et-sun">祝</span>
        ) : null}
      </span>
      {cell.record !== undefined ? (
        <span className="text-xs">{`${formatHours(cell.record.workingHours)} 時間（${formatTimeRange(cell.record.startTime, cell.record.endTime)}）`}</span>
      ) : null}
    </>
  );
}

export function WorkMonthCalendar({
  cells,
  editable,
  selectedDate,
  onSelect,
}: {
  cells: readonly CalendarCell[];
  editable: boolean;
  selectedDate: string | null;
  onSelect: (date: string) => void;
}) {
  return (
    <div className="grid grid-cols-7 gap-1">
      {WEEKDAY_HEADERS.map((weekday) => (
        <div key={weekday} className="text-center text-sm font-medium">
          {weekday}
        </div>
      ))}
      {cells.map((cell, index) => {
        if (cell.kind === "blank") return <div key={`blank-${index}`} />;
        const dow = cell.mark === "" ? undefined : cell.mark;
        const className = `${CELL_CLASSES} ${MARK_CLASSES[cell.mark]}`;
        if (editable) {
          return (
            <button
              key={cell.date}
              type="button"
              aria-label={dayLabel(cell)}
              aria-pressed={selectedDate === cell.date}
              data-dow={dow}
              onClick={() => onSelect(cell.date)}
              className={`${className} focus-visible:ring-2 focus-visible:ring-focus-ring`}
            >
              <DayContent cell={cell} />
            </button>
          );
        }
        return (
          <div key={cell.date} role="group" aria-label={dayLabel(cell)} data-dow={dow} className={className}>
            <DayContent cell={cell} />
          </div>
        );
      })}
    </div>
  );
}
