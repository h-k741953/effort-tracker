"use client";

// docs/specs/web-screens-implementation-design.md M-7。1日分の入力フォーム。
// 保存前の稼働時間は previewWorkingHours（computeWorkingHours の戻り値）だけから得る。

import { useId, useState } from "react";
import { Button } from "@/components/button";
import { FieldError } from "@/components/field-error";
import { SelectField } from "@/components/select-field";
import type { CalendarCell } from "@/lib/work-month-calendar";
import type { DailyRecordInput } from "@/lib/work-month-client";
import { formatHours, formatTimeRange } from "@/lib/work-month";
import {
  HOUR_OPTIONS,
  MINUTE_OPTIONS,
  NEW_TIME_ENTRY_FORM,
  TIME_ENTRY_MESSAGES,
  formFromRecord,
  previewWorkingHours,
  readTimeEntry,
  timeEntryProblems,
  type TimeEntryForm,
  type TimeField,
  type TimeSelection,
} from "@/lib/time-entry";

type DayCell = Extract<CalendarCell, { kind: "day" }>;

function breakLengthText(input: DailyRecordInput): string {
  if (input.break === null) return "";
  const diff =
    input.break.end.hour * 60 + input.break.end.minute - (input.break.start.hour * 60 + input.break.start.minute);
  return `${Math.floor(diff / 60)}:${String(diff % 60).padStart(2, "0")}`;
}

function TimeRow({
  field,
  label,
  selection,
  message,
  onChange,
}: {
  field: TimeField;
  label: string;
  selection: TimeSelection;
  message: string | undefined;
  onChange: (next: TimeSelection) => void;
}) {
  const id = useId();
  const errorId = `${id}-error`;
  const invalid = message !== undefined;
  return (
    <div className="flex flex-col gap-1" data-field={field}>
      <div className="flex items-end gap-2">
        <span className="w-20 text-sm">{label}</span>
        <SelectField
          id={`${id}-hour`}
          label={`${label}（時）`}
          hideLabel
          value={selection.hour}
          options={HOUR_OPTIONS}
          placeholder="--"
          invalid={invalid}
          errorId={invalid ? errorId : undefined}
          onChange={(hour) => onChange({ ...selection, hour })}
        />
        <SelectField
          id={`${id}-minute`}
          label={`${label}（分）`}
          hideLabel
          value={selection.minute}
          options={MINUTE_OPTIONS}
          placeholder="--"
          invalid={invalid}
          errorId={invalid ? errorId : undefined}
          onChange={(minute) => onChange({ ...selection, minute })}
        />
      </div>
      {invalid ? <FieldError id={errorId}>{message}</FieldError> : null}
    </div>
  );
}

export function DailyRecordForm({
  cell,
  busy,
  onSave,
  onDelete,
  onClose,
}: {
  cell: DayCell;
  busy: boolean;
  onSave: (input: DailyRecordInput) => void;
  onDelete: () => void;
  onClose: () => void;
}) {
  const [form, setForm] = useState<TimeEntryForm>(
    cell.record !== undefined ? formFromRecord(cell.record) : NEW_TIME_ENTRY_FORM,
  );
  const [tried, setTried] = useState(false);
  const futureId = useId();
  const dateText = `${Number(cell.date.slice(5, 7))}月${cell.day}日（${cell.holiday ? "祝" : cell.weekday}）`;

  const header = (
    <>
      <h2 className="text-base font-semibold">稼働実績の入力</h2>
      <p className="flex gap-2 text-sm">
        <span>対象日</span>
        <span>{dateText}</span>
      </p>
    </>
  );

  if (cell.future) {
    return (
      <div className="flex flex-col gap-3">
        {header}
        <FieldError id={futureId}>未来の日付には入力できません</FieldError>
        <div>
          <Button onClick={onClose}>閉じる</Button>
        </div>
      </div>
    );
  }

  const problems = timeEntryProblems(form, tried);
  const messageOf = (field: TimeField) => {
    const problem = problems.find((p) => p.field === field);
    return problem === undefined ? undefined : TIME_ENTRY_MESSAGES[problem.kind];
  };
  const entry = readTimeEntry(form);
  const preview = previewWorkingHours(form);

  const save = () => {
    if (busy) return;
    const entryNow = readTimeEntry(form);
    if (entryNow.kind === "incomplete") {
      setTried(true);
      return;
    }
    if (timeEntryProblems(form, true).length > 0) return;
    setTried(false);
    onSave(entryNow.input);
  };

  return (
    <div className="flex flex-col gap-3">
      {header}
      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">勤務時間</legend>
        <TimeRow field="start" label="開始" selection={form.start} message={messageOf("start")} onChange={(start) => setForm({ ...form, start })} />
        <TimeRow field="end" label="終了" selection={form.end} message={messageOf("end")} onChange={(end) => setForm({ ...form, end })} />
      </fieldset>
      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">休憩</legend>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={form.hasBreak}
            onChange={(event) => setForm({ ...form, hasBreak: event.target.checked })}
          />
          休憩あり
        </label>
        {form.hasBreak ? (
          <>
            <TimeRow
              field="breakStart"
              label="休憩開始"
              selection={form.breakStart}
              message={messageOf("breakStart")}
              onChange={(breakStart) => setForm({ ...form, breakStart })}
            />
            <TimeRow
              field="breakEnd"
              label="休憩終了"
              selection={form.breakEnd}
              message={messageOf("breakEnd")}
              onChange={(breakEnd) => setForm({ ...form, breakEnd })}
            />
          </>
        ) : null}
      </fieldset>
      <div aria-live="polite" className="flex flex-col text-sm">
        <span>稼働時間</span>
        {preview !== null && entry.kind === "complete" ? (
          <>
            <span className="text-xl font-semibold">
              {formatHours(preview)} <span className="text-sm font-normal">時間</span>
            </span>
            <span className="text-muted-foreground">
              {`${formatTimeRange(entry.input.startTime, entry.input.endTime)}（${
                entry.input.break !== null ? `休憩 ${breakLengthText(entry.input)}` : "休憩なし"
              }）`}
            </span>
          </>
        ) : (
          <>
            <span className="text-xl font-semibold">—</span>
            <span className="text-muted-foreground">開始・終了・休憩から自動で計算します</span>
          </>
        )}
      </div>
      <p className="text-sm text-muted-foreground">時は 0〜23、分は 15 分単位で選んでください。</p>
      <div className="flex gap-2">
        <Button variant="primary" disabled={busy} onClick={save}>
          保存
        </Button>
        {cell.record !== undefined ? (
          <Button variant="danger" disabled={busy} onClick={() => !busy && onDelete()}>
            削除
          </Button>
        ) : null}
        <Button onClick={onClose}>閉じる</Button>
      </div>
    </div>
  );
}
