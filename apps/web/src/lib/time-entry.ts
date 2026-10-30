// docs/specs/web-screens-implementation-design.md M-7・M-11。純粋な関数。
// 稼働時間は computeWorkingHours の戻り値だけから得る（daily-record-entry-ui.md 3-6-f）。

import type { DailyRecord } from "./work-month";
import type { DailyRecordInput } from "./work-month-client";
import { computeWorkingHours, type Break, type TimeOfDay, type WorkingHours } from "./working-hours";

export interface TimeSelection {
  hour: string;
  minute: string;
}
export interface TimeEntryForm {
  start: TimeSelection;
  end: TimeSelection;
  hasBreak: boolean;
  breakStart: TimeSelection;
  breakEnd: TimeSelection;
}
export type TimeField = "start" | "end" | "breakStart" | "breakEnd";
export type TimeEntryProblemKind =
  | "unselected-both"
  | "unselected-hour"
  | "unselected-minute"
  | "end-not-after-start"
  | "break-end-not-after-start"
  | "break-outside"
  | "zero";
export interface TimeEntryProblem {
  field: TimeField;
  kind: TimeEntryProblemKind;
}

// daily-record-entry.md D-10: 時は 0〜23、分は 0・15・30・45 の4値。
const HOURS = Array.from({ length: 24 }, (_, i) => i);
const MINUTES = [0, 15, 30, 45];

export const HOUR_OPTIONS: readonly { value: string; label: string }[] = HOURS.map((h) => ({
  value: String(h),
  label: String(h),
}));
export const MINUTE_OPTIONS: readonly { value: string; label: string }[] = MINUTES.map((m) => ({
  value: String(m),
  label: String(m).padStart(2, "0"),
}));

const UNSELECTED: TimeSelection = { hour: "", minute: "" };

export const NEW_TIME_ENTRY_FORM: TimeEntryForm = {
  start: UNSELECTED,
  end: UNSELECTED,
  hasBreak: true,
  breakStart: { hour: "12", minute: "0" },
  breakEnd: { hour: "13", minute: "0" },
};

export const TIME_ENTRY_MESSAGES: Readonly<Record<TimeEntryProblemKind, string>> = {
  "unselected-both": "時刻を選んでください",
  "unselected-hour": "時を選んでください",
  "unselected-minute": "分を選んでください",
  "end-not-after-start": "終了は開始より後の時刻にしてください",
  "break-end-not-after-start": "休憩の終了は休憩の開始より後にしてください",
  "break-outside": "休憩は勤務時間（開始〜終了）の中に収めてください",
  zero: "稼働時間が 0 になります。休憩の時刻を見直してください",
};

function selectionOf(time: TimeOfDay): TimeSelection {
  return { hour: String(time.hour), minute: String(time.minute) };
}

export function formFromRecord(record: DailyRecord): TimeEntryForm {
  return {
    start: selectionOf(record.startTime),
    end: selectionOf(record.endTime),
    hasBreak: record.break !== null,
    breakStart: record.break !== null ? selectionOf(record.break.start) : NEW_TIME_ENTRY_FORM.breakStart,
    breakEnd: record.break !== null ? selectionOf(record.break.end) : NEW_TIME_ENTRY_FORM.breakEnd,
  };
}

function readTime(selection: TimeSelection): TimeOfDay | undefined {
  if (selection.hour === "" || selection.minute === "") return undefined;
  return { hour: Number(selection.hour), minute: Number(selection.minute) };
}

function minutesOf(time: TimeOfDay): number {
  return time.hour * 60 + time.minute;
}

interface ReadTimes {
  start: TimeOfDay | undefined;
  end: TimeOfDay | undefined;
  breakStart: TimeOfDay | undefined;
  breakEnd: TimeOfDay | undefined;
}

function readTimes(form: TimeEntryForm): ReadTimes {
  return {
    start: readTime(form.start),
    end: readTime(form.end),
    breakStart: form.hasBreak ? readTime(form.breakStart) : undefined,
    breakEnd: form.hasBreak ? readTime(form.breakEnd) : undefined,
  };
}

export function readTimeEntry(
  form: TimeEntryForm,
): { kind: "incomplete" } | { kind: "complete"; input: DailyRecordInput } {
  const { start, end, breakStart, breakEnd } = readTimes(form);
  if (start === undefined || end === undefined) return { kind: "incomplete" };
  if (!form.hasBreak) return { kind: "complete", input: { startTime: start, endTime: end, break: null } };
  if (breakStart === undefined || breakEnd === undefined) return { kind: "incomplete" };
  return {
    kind: "complete",
    input: { startTime: start, endTime: end, break: { start: breakStart, end: breakEnd } },
  };
}

function unselectedKind(selection: TimeSelection): TimeEntryProblemKind | undefined {
  if (selection.hour === "" && selection.minute === "") return "unselected-both";
  if (selection.hour === "") return "unselected-hour";
  if (selection.minute === "") return "unselected-minute";
  return undefined;
}

export function timeEntryProblems(form: TimeEntryForm, tried: boolean): TimeEntryProblem[] {
  const problems: TimeEntryProblem[] = [];
  if (tried) {
    const rows: Array<[TimeField, TimeSelection]> = [
      ["start", form.start],
      ["end", form.end],
      ...(form.hasBreak
        ? ([
            ["breakStart", form.breakStart],
            ["breakEnd", form.breakEnd],
          ] as Array<[TimeField, TimeSelection]>)
        : []),
    ];
    for (const [field, selection] of rows) {
      const kind = unselectedKind(selection);
      if (kind !== undefined) problems.push({ field, kind });
    }
  }

  const { start, end, breakStart, breakEnd } = readTimes(form);
  if (start !== undefined && end !== undefined && minutesOf(end) <= minutesOf(start)) {
    problems.push({ field: "end", kind: "end-not-after-start" });
    return problems;
  }
  if (breakStart !== undefined && breakEnd !== undefined && minutesOf(breakEnd) <= minutesOf(breakStart)) {
    problems.push({ field: "breakEnd", kind: "break-end-not-after-start" });
    return problems;
  }
  if (start !== undefined && end !== undefined && breakStart !== undefined && breakEnd !== undefined) {
    if (minutesOf(breakStart) < minutesOf(start) || minutesOf(breakEnd) > minutesOf(end)) {
      problems.push({ field: "breakEnd", kind: "break-outside" });
    } else if (computeWorkingHours(start, end, { start: breakStart, end: breakEnd }) === null) {
      problems.push({ field: "breakEnd", kind: "zero" });
    }
  }
  return problems;
}

export function previewWorkingHours(form: TimeEntryForm): WorkingHours | null {
  const entry = readTimeEntry(form);
  if (entry.kind === "incomplete") return null;
  const brk: Break | null = entry.input.break;
  return computeWorkingHours(entry.input.startTime, entry.input.endTime, brk);
}
