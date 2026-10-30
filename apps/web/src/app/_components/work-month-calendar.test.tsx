/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { WEEKDAY_HEADERS, dayLabel, type CalendarCell } from "@/lib/work-month-calendar";
import { dailyRecord, hm, t } from "@/test-support/work-month";
import { WorkMonthCalendar } from "./work-month-calendar";

// docs/specs/work-month-screen-ui.md 1-6、daily-record-entry-ui.md 1-1・1-4〜1-6・9-1〜9-11。
// 設計 M-6・M-11・T-3。isHoliday を経ずに cells を props で渡す（手で組んだ小さなマス列）。
// 注: マス列は 2026-10 の一部の日を抜き出したもので、日付の連続を要求しない（描画の検査のため）。

type DayCell = Extract<CalendarCell, { kind: "day" }>;

const RECORD = dailyRecord("2026-10-05", t(9, 0), t(18, 0), { start: t(12, 0), end: t(13, 0) }, hm(8, 0));

function day(
  date: string,
  weekday: string,
  mark: DayCell["mark"],
  extra: Partial<DayCell> = {},
): DayCell {
  return {
    kind: "day",
    date,
    day: Number(date.slice(8)),
    weekday,
    mark,
    holiday: mark === "hol",
    future: false,
    record: undefined,
    ...extra,
  };
}

const DAYS: DayCell[] = [
  day("2026-10-01", "木", ""),
  day("2026-10-02", "金", "hol"), // 平日の祝日
  day("2026-10-03", "土", "sat"),
  day("2026-10-04", "日", "hol"), // 日曜の祝日
  day("2026-10-05", "月", "", { record: RECORD }),
  day("2026-10-10", "土", "hol"), // 土曜の祝日
  day("2026-10-11", "日", "sun"),
  day("2026-10-30", "金", "", { future: true }),
];
const CELLS: CalendarCell[] = [{ kind: "blank" }, { kind: "blank" }, ...DAYS, { kind: "blank" }];

afterEach(cleanup);

function renderCalendar(props: { editable: boolean; selectedDate?: string | null; onSelect?: (date: string) => void }) {
  const selected: string[] = [];
  const onSelect = props.onSelect ?? ((date: string) => selected.push(date));
  const view = render(
    <WorkMonthCalendar
      cells={CELLS}
      editable={props.editable}
      selectedDate={props.selectedDate ?? null}
      onSelect={onSelect}
    />,
  );
  return { ...view, selected };
}

const dowOf = (container: HTMLElement) =>
  [...container.querySelectorAll("[data-dow]")].map((el) => el.getAttribute("data-dow"));

describe("編集できるとき（M-6）", () => {
  it("日のマスが button（数は日の数）で、アクセシブル名は dayLabel", () => {
    renderCalendar({ editable: true });
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(DAYS.length);
    for (const cell of DAYS) {
      expect(screen.getByRole("button", { name: dayLabel(cell) })).toBeTruthy();
    }
  });

  it("選んだ日だけ aria-pressed=true", () => {
    renderCalendar({ editable: true, selectedDate: "2026-10-05" });
    for (const cell of DAYS) {
      const button = screen.getByRole("button", { name: dayLabel(cell) });
      expect(button.getAttribute("aria-pressed") === "true", cell.date).toBe(cell.date === "2026-10-05");
    }
  });

  it("押すと onSelect がその日付で1回", () => {
    const { selected } = renderCalendar({ editable: true });
    fireEvent.click(screen.getByRole("button", { name: dayLabel(DAYS[1]) }));
    expect(selected).toEqual(["2026-10-02"]);
  });

  it("未来の日も押せる（選ぶとフォームが理由を出す）", () => {
    const { selected } = renderCalendar({ editable: true });
    fireEvent.click(screen.getByRole("button", { name: dayLabel(DAYS[7]) }));
    expect(selected).toEqual(["2026-10-30"]);
  });
});

describe("編集できないとき（M-6）", () => {
  it("button が0個で、どの要素を押しても onSelect が呼ばれない", () => {
    const { container, selected } = renderCalendar({ editable: false });
    expect(screen.queryAllByRole("button")).toHaveLength(0);
    for (const el of container.querySelectorAll("*")) fireEvent.click(el);
    expect(selected).toEqual([]);
  });
});

describe("実績のある日（入力 UI 1-1・1-6）", () => {
  it.each([true, false])("editable=%s: 稼働時間（formatHours）と時刻の範囲が出て、休憩の時刻は出ない", (editable) => {
    const { container } = renderCalendar({ editable });
    const text = container.textContent ?? "";
    expect(text).toContain("8.0");
    expect(text).toContain("9:00〜18:00");
    expect(text).not.toContain("12:00");
    expect(text).not.toContain("13:00");
  });
});

describe("祝日の札と data-dow（9-1〜9-6・9-8・9-9・9-11）", () => {
  it.each([true, false])("editable=%s: 祝日のマス3つに札「祝」が1つずつ、data-dow は hol・sat・sun、無印の日に data-dow が無い", (editable) => {
    const { container } = renderCalendar({ editable });
    expect(screen.getAllByText("祝")).toHaveLength(3);
    const marked = [...container.querySelectorAll("[data-dow]")];
    expect(dowOf(container).sort()).toEqual(["hol", "hol", "hol", "sat", "sun"]);
    for (const el of marked.filter((m) => m.getAttribute("data-dow") === "hol")) {
      expect(within(el as HTMLElement).getAllByText("祝")).toHaveLength(1);
    }
    for (const el of marked.filter((m) => m.getAttribute("data-dow") !== "hol")) {
      expect(within(el as HTMLElement).queryByText("祝")).toBeNull();
    }
  });

  it("9-11: editable の真偽で札と data-dow が変わらない", () => {
    const a = renderCalendar({ editable: true });
    const editableDow = dowOf(a.container);
    const editableBadges = screen.getAllByText("祝").length;
    cleanup();
    const b = renderCalendar({ editable: false });
    expect(dowOf(b.container)).toEqual(editableDow);
    expect(screen.getAllByText("祝")).toHaveLength(editableBadges);
  });
});

describe("曜日の見出し（入力 UI 1-5）", () => {
  it("月〜日の7つが、この順で出る", () => {
    renderCalendar({ editable: true });
    const headers = screen.getAllByText(/^[月火水木金土日]$/).map((el) => el.textContent);
    expect(headers).toEqual([...WEEKDAY_HEADERS]);
  });
});
