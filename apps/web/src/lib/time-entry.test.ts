import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { readSpecTimeRows, type SpecTimeRow } from "@/test-support/daily-record-spec-rows";
import { dailyRecord, hm, t } from "@/test-support/work-month";
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
  type TimeEntryProblem,
  type TimeEntryProblemKind,
  type TimeSelection,
} from "./time-entry";
import { computeWorkingHours, type TimeOfDay } from "./working-hours";

// docs/specs/daily-record-entry-ui.md 3-2-a・3-4・3-5・3-6-a・3-6-b・3-6-f・4-1・4-5〜4-9・5-1。
// 設計 M-7・M-11・T-3。選択肢の value の字面（"9" か "09" か）は仕様が固めていないため、
// 形は選択肢の定義から引き（sel）、数値として読む。

const SPEC_PATH = path.join(process.cwd(), "..", "..", "docs", "specs", "daily-record-entry.md");

/** 時・分の選択肢から、数値に対応する value を引く。 */
function optionValue(options: readonly { value: string; label: string }[], n: number): string {
  const found = options.find((o) => Number(o.value) === n);
  if (found === undefined) throw new Error(`選択肢に ${n} が無い`);
  return found.value;
}
const sel = (hour: number, minute: number): TimeSelection => ({
  hour: optionValue(HOUR_OPTIONS, hour),
  minute: optionValue(MINUTE_OPTIONS, minute),
});
const UNSELECTED: TimeSelection = { hour: "", minute: "" };

function form(overrides: Partial<TimeEntryForm> = {}): TimeEntryForm {
  return {
    start: sel(9, 0),
    end: sel(18, 0),
    hasBreak: true,
    breakStart: sel(12, 0),
    breakEnd: sel(13, 0),
    ...overrides,
  };
}

function bySort(problems: TimeEntryProblem[]): TimeEntryProblem[] {
  return [...problems].sort((a, b) => a.field.localeCompare(b.field));
}

describe("HOUR_OPTIONS・MINUTE_OPTIONS（3-2-a。D-10 の条文と過不足なく一致し、昇順）", () => {
  const spec = readFileSync(SPEC_PATH, "utf-8");
  const d10 = spec.split("\n").find((line) => line.startsWith("| D-10 |"));

  it("D-10 の行を読めている（陽性対照）", () => {
    expect(d10, "daily-record-entry.md に D-10 の行が見つからない").toBeDefined();
  });

  const hourRange = d10?.match(/時は (\d+)〜(\d+)/);
  const minuteList = d10?.match(/分は ([\d・]+) の4値/);

  it("D-10 から時の値域と分の4値を読めている（陽性対照）", () => {
    expect(hourRange).not.toBeNull();
    expect(minuteList).not.toBeNull();
  });

  it("時: 値域の過不足なく昇順。表示は数字そのまま", () => {
    const [lo, hi] = [Number(hourRange![1]), Number(hourRange![2])];
    const want = Array.from({ length: hi - lo + 1 }, (_, i) => lo + i);
    expect(HOUR_OPTIONS.map((o) => Number(o.value))).toEqual(want);
    expect(HOUR_OPTIONS.map((o) => o.label)).toEqual(want.map(String));
    expect(HOUR_OPTIONS.every((o) => o.value !== "")).toBe(true);
  });

  it("分: 4値の過不足なく昇順。表示は2桁", () => {
    const want = minuteList![1].split("・").map(Number);
    expect(want).toHaveLength(4);
    expect(MINUTE_OPTIONS.map((o) => Number(o.value))).toEqual(want);
    expect(MINUTE_OPTIONS.map((o) => o.label)).toEqual(want.map((n) => String(n).padStart(2, "0")));
    expect(MINUTE_OPTIONS.every((o) => o.value !== "")).toBe(true);
  });
});

describe("TIME_ENTRY_MESSAGES（M-7。4-9）", () => {
  it("7つの文の字面", () => {
    expect(TIME_ENTRY_MESSAGES).toEqual({
      "unselected-both": "時刻を選んでください",
      "unselected-hour": "時を選んでください",
      "unselected-minute": "分を選んでください",
      "end-not-after-start": "終了は開始より後の時刻にしてください",
      "break-end-not-after-start": "休憩の終了は休憩の開始より後にしてください",
      "break-outside": "休憩は勤務時間（開始〜終了）の中に収めてください",
      zero: "稼働時間が 0 になります。休憩の時刻を見直してください",
    });
  });

  it("7つの文は互いに異なる", () => {
    expect(new Set(Object.values(TIME_ENTRY_MESSAGES)).size).toBe(7);
  });
});

describe("NEW_TIME_ENTRY_FORM（M-7。開始・終了は未選択、休憩あり 12:00〜13:00）", () => {
  it("開始・終了が未選択で、休憩ありの 12:00〜13:00", () => {
    expect(NEW_TIME_ENTRY_FORM.start).toEqual(UNSELECTED);
    expect(NEW_TIME_ENTRY_FORM.end).toEqual(UNSELECTED);
    expect(NEW_TIME_ENTRY_FORM.hasBreak).toBe(true);
    expect(NEW_TIME_ENTRY_FORM.breakStart).toEqual(sel(12, 0));
    expect(NEW_TIME_ENTRY_FORM.breakEnd).toEqual(sel(13, 0));
  });
});

describe("timeEntryProblems — 未選択の3つの理由（4-1。tried が真のときだけ）", () => {
  const cases: Array<[string, TimeEntryForm, TimeEntryProblemKind]> = [
    ["時と分がどちらも未選択", form({ start: UNSELECTED }), "unselected-both"],
    ["時だけ未選択", form({ start: { hour: "", minute: sel(9, 0).minute } }), "unselected-hour"],
    ["分だけ未選択", form({ start: { hour: sel(9, 0).hour, minute: "" } }), "unselected-minute"],
  ];

  it.each(cases)("%s: tried 真 → 開始の行に1つ", (_name, f, kind) => {
    expect(timeEntryProblems(f, true)).toEqual([{ field: "start", kind }]);
  });

  it.each(cases)("%s: tried 偽 → 出ない", (_name, f) => {
    expect(timeEntryProblems(f, false)).toEqual([]);
  });

  it("新規の初期値（開始・終了が未選択）で tried 真 → 開始と終了の行に1つずつ（行ごとに理由を1つ）", () => {
    expect(bySort(timeEntryProblems(NEW_TIME_ENTRY_FORM, true))).toEqual([
      { field: "end", kind: "unselected-both" },
      { field: "start", kind: "unselected-both" },
    ]);
  });

  it("休憩の2行にも割り当てる（休憩あり）", () => {
    const f = form({ breakStart: UNSELECTED, breakEnd: { hour: "", minute: sel(0, 15).minute } });
    expect(bySort(timeEntryProblems(f, true))).toEqual([
      { field: "breakEnd", kind: "unselected-hour" },
      { field: "breakStart", kind: "unselected-both" },
    ]);
  });

  it("休憩なしのとき休憩の行を見ない（休憩が未選択でも理由を返さない。3-4）", () => {
    const f = form({ hasBreak: false, breakStart: UNSELECTED, breakEnd: UNSELECTED });
    expect(timeEntryProblems(f, true)).toEqual([]);
  });
});

describe("timeEntryProblems — 組み合わせの理由（4-5〜4-8。tried を問わず、選ばれた時点から）", () => {
  const cases: Array<[string, TimeEntryForm, TimeEntryProblem]> = [
    ["終了が開始と同じ", form({ start: sel(9, 0), end: sel(9, 0), hasBreak: false }), { field: "end", kind: "end-not-after-start" }],
    ["終了が開始より前", form({ start: sel(18, 0), end: sel(9, 0), hasBreak: false }), { field: "end", kind: "end-not-after-start" }],
    [
      "休憩終了が休憩開始と同じ",
      form({ breakStart: sel(12, 0), breakEnd: sel(12, 0) }),
      { field: "breakEnd", kind: "break-end-not-after-start" },
    ],
    [
      "休憩終了が休憩開始より前",
      form({ breakStart: sel(13, 0), breakEnd: sel(12, 0) }),
      { field: "breakEnd", kind: "break-end-not-after-start" },
    ],
    [
      "休憩が開始より前にはみ出す",
      form({ start: sel(9, 0), end: sel(18, 0), breakStart: sel(8, 45), breakEnd: sel(10, 0) }),
      { field: "breakEnd", kind: "break-outside" },
    ],
    [
      "休憩が終了より後にはみ出す",
      form({ start: sel(9, 0), end: sel(18, 0), breakStart: sel(17, 0), breakEnd: sel(18, 15) }),
      { field: "breakEnd", kind: "break-outside" },
    ],
    [
      "休憩が勤務時間と一致して稼働が 0",
      form({ start: sel(9, 0), end: sel(10, 0), breakStart: sel(9, 0), breakEnd: sel(10, 0) }),
      { field: "breakEnd", kind: "zero" },
    ],
  ];

  it.each(cases)("%s", (_name, f, problem) => {
    expect(timeEntryProblems(f, false)).toEqual([problem]);
    expect(timeEntryProblems(f, true)).toEqual([problem]);
  });

  it("端点を含めて収まる休憩（開始と同時・終了と同時）は理由なし", () => {
    expect(timeEntryProblems(form({ breakStart: sel(9, 0), breakEnd: sel(10, 0) }), true)).toEqual([]);
    expect(timeEntryProblems(form({ breakStart: sel(17, 0), breakEnd: sel(18, 0) }), true)).toEqual([]);
  });

  it("上の行が優先される: 終了が開始より後でないとき、休憩の理由を重ねない", () => {
    const f = form({ start: sel(18, 0), end: sel(9, 0), breakStart: sel(13, 0), breakEnd: sel(12, 0) });
    expect(timeEntryProblems(f, true)).toEqual([{ field: "end", kind: "end-not-after-start" }]);
  });

  it("上の行が優先される: 休憩の前後関係が不正なら、位置・稼働 0 の理由を重ねない", () => {
    const f = form({ start: sel(9, 0), end: sel(18, 0), breakStart: sel(19, 0), breakEnd: sel(8, 0) });
    expect(timeEntryProblems(f, true)).toEqual([{ field: "breakEnd", kind: "break-end-not-after-start" }]);
  });

  it("休憩なしのとき、休憩の値が不正でも理由を返さない", () => {
    const f = form({ hasBreak: false, breakStart: sel(13, 0), breakEnd: sel(12, 0) });
    expect(timeEntryProblems(f, true)).toEqual([]);
  });

  it("妥当な値は理由なし", () => {
    expect(timeEntryProblems(form(), true)).toEqual([]);
    expect(timeEntryProblems(form({ hasBreak: false }), true)).toEqual([]);
  });
});

describe("理由の有無と computeWorkingHours の一致（不変条件。daily-record-entry.md AC-3 の 3-B・3-C の表から読む）", () => {
  const allRows = readSpecTimeRows();
  // 選択肢（時 0〜23・分 4値）で表せない時刻（例: 24:00）を持つ行は、画面から指定できない（3-2-a）ため対象外。
  const selectable = (time: TimeOfDay) =>
    HOUR_OPTIONS.some((o) => Number(o.value) === time.hour) && MINUTE_OPTIONS.some((o) => Number(o.value) === time.minute);
  const isSelectable = (row: SpecTimeRow) =>
    selectable(row.start) && selectable(row.end) && (row.brk === null || (selectable(row.brk.start) && selectable(row.brk.end)));
  const rows = allRows.filter(isSelectable);

  const asForm = (row: SpecTimeRow): TimeEntryForm => {
    const pick = (time: TimeOfDay) => sel(time.hour, time.minute);
    return {
      start: pick(row.start),
      end: pick(row.end),
      hasBreak: row.brk !== null,
      breakStart: row.brk ? pick(row.brk.start) : UNSELECTED,
      breakEnd: row.brk ? pick(row.brk.end) : UNSELECTED,
    };
  };
  const label = (row: SpecTimeRow) =>
    `${row.id}: ${row.start.hour}:${row.start.minute}〜${row.end.hour}:${row.end.minute}` +
    (row.brk ? ` 休憩 ${row.brk.start.hour}:${row.brk.start.minute}〜${row.brk.end.hour}:${row.brk.end.minute}` : " 休憩なし");

  it("選択肢で表せないために対象外にした行は、すべて「弾く」の行である", () => {
    const excluded = allRows.filter((r) => !isSelectable(r));
    expect(excluded.every((r) => r.reject)).toBe(true);
  });

  it("表から時刻の組を持つ行を読めている（陽性対照。0件で緑にしない）", () => {
    expect(rows.length).toBeGreaterThan(0);
    expect(rows.some((r) => r.reject)).toBe(true);
    expect(rows.some((r) => !r.reject)).toBe(true);
  });

  it.each(rows.map((r) => [label(r), r] as const))("%s: 理由が無いこと ⇔ computeWorkingHours が null でない", (_name, row) => {
    const problems = timeEntryProblems(asForm(row), true);
    const computed = computeWorkingHours(row.start, row.end, row.brk);
    expect(problems.length === 0).toBe(computed !== null);
  });
});

describe("previewWorkingHours（3-6-a・3-6-b・3-6-f）", () => {
  it("未選択が残ると null（開始・終了・休憩のどれでも）", () => {
    expect(previewWorkingHours(NEW_TIME_ENTRY_FORM)).toBeNull();
    expect(previewWorkingHours(form({ end: UNSELECTED }))).toBeNull();
    expect(previewWorkingHours(form({ start: { hour: sel(9, 0).hour, minute: "" } }))).toBeNull();
    expect(previewWorkingHours(form({ breakEnd: { hour: "", minute: sel(0, 0).minute } }))).toBeNull();
  });

  it("休憩なしのとき、休憩の未選択は無関係", () => {
    expect(previewWorkingHours(form({ hasBreak: false, breakStart: UNSELECTED, breakEnd: UNSELECTED }))).toEqual({
      hours: 9,
      minutes: 0,
    });
  });

  it("そろえば computeWorkingHours の戻り値と深く等しい", () => {
    expect(previewWorkingHours(form())).toEqual(
      computeWorkingHours({ hour: 9, minute: 0 }, { hour: 18, minute: 0 }, {
        start: { hour: 12, minute: 0 },
        end: { hour: 13, minute: 0 },
      }),
    );
    expect(previewWorkingHours(form())).toEqual({ hours: 8, minutes: 0 });
  });

  it("弾く組は null（0 や負の値を返さない）", () => {
    expect(previewWorkingHours(form({ start: sel(9, 0), end: sel(9, 0), hasBreak: false }))).toBeNull();
    expect(
      previewWorkingHours(form({ start: sel(9, 0), end: sel(10, 0), breakStart: sel(9, 0), breakEnd: sel(10, 0) })),
    ).toBeNull();
  });
});

describe("readTimeEntry（M-11）", () => {
  it("未選択が残ると incomplete", () => {
    expect(readTimeEntry(NEW_TIME_ENTRY_FORM)).toEqual({ kind: "incomplete" });
    expect(readTimeEntry(form({ breakStart: UNSELECTED }))).toEqual({ kind: "incomplete" });
  });

  it("休憩あり: 数値の時刻と break の組を返す（稼働時間を含めない）", () => {
    expect(readTimeEntry(form())).toEqual({
      kind: "complete",
      input: {
        startTime: { hour: 9, minute: 0 },
        endTime: { hour: 18, minute: 0 },
        break: { start: { hour: 12, minute: 0 }, end: { hour: 13, minute: 0 } },
      },
    });
  });

  it("休憩なし: break は null（休憩の選択が残っていても送る値に含めない）", () => {
    expect(readTimeEntry(form({ hasBreak: false }))).toEqual({
      kind: "complete",
      input: { startTime: { hour: 9, minute: 0 }, endTime: { hour: 18, minute: 0 }, break: null },
    });
    expect(readTimeEntry(form({ hasBreak: false, breakStart: UNSELECTED, breakEnd: UNSELECTED })).kind).toBe(
      "complete",
    );
  });
});

describe("formFromRecord（5-1）", () => {
  it("break が null なら hasBreak は偽", () => {
    const record = dailyRecord("2026-07-02", t(9, 15), t(17, 45), null, hm(8, 30));
    expect(formFromRecord(record).hasBreak).toBe(false);
  });

  it("休憩ありなら hasBreak は真で、時刻は記録のとおり（読み取りで往復する）", () => {
    const record = dailyRecord("2026-07-01", t(9, 15), t(18, 0), { start: t(12, 30), end: t(13, 15) }, hm(8, 0));
    const f = formFromRecord(record);
    expect(f.hasBreak).toBe(true);
    expect(readTimeEntry(f)).toEqual({
      kind: "complete",
      input: { startTime: t(9, 15), endTime: t(18, 0), break: { start: t(12, 30), end: t(13, 15) } },
    });
  });

  it("休憩なしの記録も往復する（break: null）", () => {
    const record = dailyRecord("2026-07-02", t(9, 0), t(17, 45), null, hm(8, 45));
    expect(readTimeEntry(formFromRecord(record))).toEqual({
      kind: "complete",
      input: { startTime: t(9, 0), endTime: t(17, 45), break: null },
    });
  });
});
