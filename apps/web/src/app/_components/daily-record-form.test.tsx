/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { CalendarCell } from "@/lib/work-month-calendar";
import type { DailyRecordInput } from "@/lib/work-month-client";
import { dailyRecord, hm, t } from "@/test-support/work-month";
import { DailyRecordForm } from "./daily-record-form";

// docs/specs/daily-record-entry-ui.md 3-1・3-1-a・3-2・3-2-a・3-2-b・3-2-c・3-4・3-6・3-6-a・3-6-b・4-1・4-2・4-4・
// 4-5〜4-9・5-1・6-1。設計 M-7・M-11・T-3。props の Fake（onSave・onDelete・onClose）は手書き。
// 選択肢の value の字面は仕様が固めていないため、選択は表示（時は数字そのまま、分は2桁。M-7）で行う。

type DayCell = Extract<CalendarCell, { kind: "day" }>;

const EIGHT_NAMES = [
  "開始（時）",
  "開始（分）",
  "終了（時）",
  "終了（分）",
  "休憩開始（時）",
  "休憩開始（分）",
  "休憩終了（時）",
  "休憩終了（分）",
];

const NEW_CELL: DayCell = {
  kind: "day",
  date: "2026-07-03",
  day: 3,
  weekday: "金",
  mark: "",
  holiday: false,
  future: false,
  record: undefined,
};

afterEach(cleanup);

function renderForm(
  cell: DayCell = NEW_CELL,
  options: { busy?: boolean } = {},
) {
  const saved: DailyRecordInput[] = [];
  const log = { deleted: 0, closed: 0 };
  const view = render(
    <DailyRecordForm
      key={cell.date}
      cell={cell}
      busy={options.busy ?? false}
      onSave={(input) => saved.push(input)}
      onDelete={() => {
        log.deleted += 1;
      }}
      onClose={() => {
        log.closed += 1;
      }}
    />,
  );
  return { ...view, saved, log };
}

function select(name: string): HTMLSelectElement {
  return screen.getByRole("combobox", { name }) as HTMLSelectElement;
}

/** 表示（時は数字そのまま・分は2桁）で選ぶ。 */
function choose(name: string, label: string) {
  const el = select(name);
  const option = within(el).getByRole("option", { name: label }) as HTMLOptionElement;
  fireEvent.change(el, { target: { value: option.value } });
}

function setTime(prefix: "開始" | "終了" | "休憩開始" | "休憩終了", hour: number, minute: number) {
  choose(`${prefix}（時）`, String(hour));
  choose(`${prefix}（分）`, String(minute).padStart(2, "0"));
}

function selectedLabel(name: string): string {
  return select(name).selectedOptions[0]?.textContent ?? "";
}

/** 名前の select が aria-describedby で指す要素の文言。 */
function describedBy(name: string): string[] {
  const ids = (select(name).getAttribute("aria-describedby") ?? "").split(/\s+/).filter(Boolean);
  return ids.map((id) => document.getElementById(id)?.textContent ?? "");
}

/** 行（時・分の2つ）が指す理由の文言。 */
function rowMessages(prefix: "開始" | "終了" | "休憩開始" | "休憩終了"): string[] {
  return [...new Set([...describedBy(`${prefix}（時）`), ...describedBy(`${prefix}（分）`)])];
}

function previewRegion(container: HTMLElement): string {
  const region = container.querySelector('[aria-live="polite"]');
  expect(region, "aria-live=polite の領域が無い").not.toBeNull();
  return region!.textContent ?? "";
}

describe("構成（3-1・3-1-a・3-2・3-2-c・3-4）", () => {
  it("見出しと対象日（祝日は曜日の代わりに「祝」）", () => {
    renderForm();
    expect(screen.getByRole("heading", { name: "稼働実績の入力" })).toBeTruthy();
    expect(screen.getByText("対象日")).toBeTruthy();
    expect(screen.getByText("7月3日（金）")).toBeTruthy();
    cleanup();
    renderForm({ ...NEW_CELL, mark: "hol", holiday: true });
    expect(screen.getByText("7月3日（祝）")).toBeTruthy();
  });

  it("休憩ありで combobox が8つ。アクセシブル名が互いに異なり M-7 の8つと一致する", () => {
    renderForm();
    expect(screen.getAllByRole("combobox")).toHaveLength(8);
    for (const name of EIGHT_NAMES) expect(select(name)).toBeTruthy();
    expect(new Set(EIGHT_NAMES).size).toBe(8);
  });

  it("「休憩あり」を外すと combobox が4つになり、休憩の選択を描かない", () => {
    renderForm();
    fireEvent.click(screen.getByRole("checkbox", { name: "休憩あり" }));
    expect(screen.getAllByRole("combobox")).toHaveLength(4);
    expect(screen.queryByRole("combobox", { name: "休憩開始（時）" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "休憩終了（分）" })).toBeNull();
  });

  it("自由に打ち込む欄（text・number）が無い", () => {
    const { container } = renderForm();
    expect(screen.queryAllByRole("textbox")).toHaveLength(0);
    expect(screen.queryAllByRole("spinbutton")).toHaveLength(0);
    expect(container.querySelectorAll('input[type="text"], input[type="number"], input:not([type])')).toHaveLength(0);
  });

  it("休憩の組は1つだけで、休憩を足す操作が無い（button は「保存」「閉じる」だけ）", () => {
    renderForm();
    expect(screen.getAllByRole("button").map((b) => b.textContent).sort()).toEqual(["保存", "閉じる"]);
  });

  it("時刻の選択の初期値: 開始・終了は未選択（「--」）、休憩は 12:00〜13:00", () => {
    renderForm();
    expect(selectedLabel("開始（時）")).toBe("--");
    expect(selectedLabel("終了（分）")).toBe("--");
    expect(selectedLabel("休憩開始（時）")).toBe("12");
    expect(selectedLabel("休憩開始（分）")).toBe("00");
    expect(selectedLabel("休憩終了（時）")).toBe("13");
  });

  it("案内の文", () => {
    renderForm();
    expect(screen.getByText("時は 0〜23、分は 15 分単位で選んでください。")).toBeTruthy();
  });
});

describe("未選択のまま保存（4-1・4-4）", () => {
  it("onSave を呼ばず、開始と終了の行に「時刻を選んでください」を出し、選んだ休憩の値が残る", () => {
    const { saved } = renderForm();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(saved).toEqual([]);
    const alerts = screen.getAllByRole("alert");
    expect(alerts).toHaveLength(2);
    for (const a of alerts) expect(a.textContent).toBe("時刻を選んでください");
    expect(rowMessages("開始")).toEqual(["時刻を選んでください"]);
    expect(rowMessages("終了")).toEqual(["時刻を選んでください"]);
    expect(rowMessages("休憩開始")).toEqual([]);
    expect(selectedLabel("休憩開始（時）")).toBe("12");
    expect(selectedLabel("休憩終了（時）")).toBe("13");
  });

  it("時だけ未選択 → 「時を選んでください」、分だけ未選択 → 「分を選んでください」（その行）", () => {
    renderForm();
    choose("開始（分）", "00"); // 開始は時だけ未選択
    choose("終了（時）", "18"); // 終了は分だけ未選択
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(rowMessages("開始")).toEqual(["時を選んでください"]);
    expect(rowMessages("終了")).toEqual(["分を選んでください"]);
  });

  it("理由の出る行の select は invalid（aria-invalid=true）", () => {
    renderForm();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(select("開始（時）").getAttribute("aria-invalid")).toBe("true");
    expect(select("休憩開始（時）").getAttribute("aria-invalid")).not.toBe("true");
  });
});

describe("組み合わせの理由（4-5〜4-9。それぞれ1つの行に、互いに異なる文）", () => {
  it("終了が開始より後でない → 終了の行に1つ", () => {
    renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 9, 0);
    fireEvent.click(screen.getByRole("checkbox", { name: "休憩あり" }));
    expect(rowMessages("終了")).toEqual(["終了は開始より後の時刻にしてください"]);
    expect(screen.getAllByRole("alert")).toHaveLength(1);
  });

  it("休憩終了が休憩開始より後でない → 休憩終了の行に1つ", () => {
    renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 18, 0);
    setTime("休憩開始", 13, 0);
    setTime("休憩終了", 12, 0);
    expect(rowMessages("休憩終了")).toEqual(["休憩の終了は休憩の開始より後にしてください"]);
    expect(screen.getAllByRole("alert")).toHaveLength(1);
  });

  it("休憩が開始〜終了に収まらない → 休憩終了の行に1つ", () => {
    renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 18, 0);
    setTime("休憩開始", 8, 0);
    setTime("休憩終了", 10, 0);
    expect(rowMessages("休憩終了")).toEqual(["休憩は勤務時間（開始〜終了）の中に収めてください"]);
    expect(screen.getAllByRole("alert")).toHaveLength(1);
  });

  it("稼働時間が 0 になる → 休憩終了の行に1つ", () => {
    renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 10, 0);
    setTime("休憩開始", 9, 0);
    setTime("休憩終了", 10, 0);
    expect(rowMessages("休憩終了")).toEqual(["稼働時間が 0 になります。休憩の時刻を見直してください"]);
    expect(screen.getAllByRole("alert")).toHaveLength(1);
  });

  it("理由があるとき保存しても onSave は呼ばれず、選んだ値が残る", () => {
    const { saved } = renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 9, 0);
    fireEvent.click(screen.getByRole("checkbox", { name: "休憩あり" }));
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(saved).toEqual([]);
    expect(selectedLabel("開始（時）")).toBe("9");
    expect(selectedLabel("終了（時）")).toBe("9");
  });
});

describe("保存（3-3・3-6-c）", () => {
  it("妥当な値で onSave が readTimeEntry の input（休憩あり）で1回", () => {
    const { saved } = renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 18, 0);
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(saved).toEqual([
      {
        startTime: { hour: 9, minute: 0 },
        endTime: { hour: 18, minute: 0 },
        break: { start: { hour: 12, minute: 0 }, end: { hour: 13, minute: 0 } },
      },
    ]);
    expect(screen.queryAllByRole("alert")).toHaveLength(0);
  });

  it("休憩なしで break: null（稼働時間を含めない）", () => {
    const { saved } = renderForm();
    setTime("開始", 9, 15);
    setTime("終了", 17, 45);
    fireEvent.click(screen.getByRole("checkbox", { name: "休憩あり" }));
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(saved).toEqual([{ startTime: { hour: 9, minute: 15 }, endTime: { hour: 17, minute: 45 }, break: null }]);
    expect(Object.keys(saved[0]).sort()).toEqual(["break", "endTime", "startTime"]);
  });

  it("busy 真で保存を押しても onSave は0回", () => {
    const { saved } = renderForm(NEW_CELL, { busy: true });
    setTime("開始", 9, 0);
    setTime("終了", 18, 0);
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(saved).toEqual([]);
  });
});

describe("保存前の稼働時間（3-6・3-6-a・3-6-b）", () => {
  it("未選択が残ると「—」と、自動で計算する旨の補足", () => {
    const { container } = renderForm();
    const text = previewRegion(container);
    expect(text).toContain("稼働時間");
    expect(text).toContain("—");
    expect(text).toContain("開始・終了・休憩から自動で計算します");
  });

  it("そろうと computeWorkingHours から得た値（9:00〜18:00・休憩 12:00〜13:00 → 8.0）と補足", () => {
    const { container } = renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 18, 0);
    const text = previewRegion(container);
    expect(text).toContain("8.0");
    expect(text).toContain("時間");
    expect(text).toContain("9:00〜18:00（休憩 1:00）");
    expect(text).not.toContain("—");
  });

  it("選択を変えるたびに値が改まる（終了 19:00 → 9.0）", () => {
    const { container } = renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 18, 0);
    setTime("終了", 19, 0);
    expect(previewRegion(container)).toContain("9.0");
  });

  it("休憩なしの補足は「（休憩なし）」", () => {
    const { container } = renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 18, 0);
    fireEvent.click(screen.getByRole("checkbox", { name: "休憩あり" }));
    const text = previewRegion(container);
    expect(text).toContain("9.0");
    expect(text).toContain("9:00〜18:00（休憩なし）");
  });

  it("弾く組では「—」（0 や負の値を出さない）", () => {
    const { container } = renderForm();
    setTime("開始", 9, 0);
    setTime("終了", 9, 0);
    fireEvent.click(screen.getByRole("checkbox", { name: "休憩あり" }));
    const text = previewRegion(container);
    expect(text).toContain("—");
    expect(text).not.toContain("0.0");
    expect(text).not.toContain("-");
  });
});

describe("未来の日（4-2）", () => {
  const FUTURE: DayCell = { ...NEW_CELL, future: true };

  it("「未来の日付には入力できません」が出て、combobox と「保存」が無い。「閉じる」だけが出る", () => {
    renderForm(FUTURE);
    expect(screen.getByText("未来の日付には入力できません")).toBeTruthy();
    expect(screen.queryAllByRole("combobox")).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "保存" })).toBeNull();
    expect(screen.queryByRole("button", { name: "削除" })).toBeNull();
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual(["閉じる"]);
  });

  it("「閉じる」で onClose が1回", () => {
    const { log } = renderForm(FUTURE);
    fireEvent.click(screen.getByRole("button", { name: "閉じる" }));
    expect(log.closed).toBe(1);
  });
});

describe("編集（5-1）・削除（6-1）", () => {
  const WITH_BREAK = dailyRecord("2026-07-01", t(9, 15), t(18, 0), { start: t(12, 30), end: t(13, 15) }, hm(8, 0));
  const NO_BREAK = dailyRecord("2026-07-02", t(9, 0), t(17, 45), null, hm(8, 45));

  it("実績のある日（休憩あり）: 初期値がその時刻で、「休憩あり」が入っている。「削除」が出る", () => {
    renderForm({ ...NEW_CELL, date: "2026-07-01", day: 1, weekday: "水", record: WITH_BREAK });
    expect(selectedLabel("開始（時）")).toBe("9");
    expect(selectedLabel("開始（分）")).toBe("15");
    expect(selectedLabel("終了（時）")).toBe("18");
    expect(selectedLabel("休憩開始（分）")).toBe("30");
    expect(selectedLabel("休憩終了（分）")).toBe("15");
    expect((screen.getByRole("checkbox", { name: "休憩あり" }) as HTMLInputElement).checked).toBe(true);
    expect(screen.getByRole("button", { name: "削除" })).toBeTruthy();
  });

  it("実績のある日（休憩なし）: 「休憩あり」が外れ、combobox は4つ", () => {
    renderForm({ ...NEW_CELL, date: "2026-07-02", day: 2, weekday: "木", record: NO_BREAK });
    expect((screen.getByRole("checkbox", { name: "休憩あり" }) as HTMLInputElement).checked).toBe(false);
    expect(screen.getAllByRole("combobox")).toHaveLength(4);
    expect(selectedLabel("終了（分）")).toBe("45");
  });

  it("「削除」を押すと onDelete が1回（確認ステップを置かない）", () => {
    const { log } = renderForm({ ...NEW_CELL, date: "2026-07-01", day: 1, weekday: "水", record: WITH_BREAK });
    fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(log.deleted).toBe(1);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("実績の無い日には「削除」が出ない", () => {
    renderForm();
    expect(screen.queryByRole("button", { name: "削除" })).toBeNull();
  });

  it("busy 真で削除を押しても onDelete は0回", () => {
    const { log } = renderForm({ ...NEW_CELL, date: "2026-07-01", day: 1, weekday: "水", record: WITH_BREAK }, { busy: true });
    fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(log.deleted).toBe(0);
  });

  it("「閉じる」で onClose が1回", () => {
    const { log } = renderForm();
    fireEvent.click(screen.getByRole("button", { name: "閉じる" }));
    expect(log.closed).toBe(1);
  });
});
