/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { HOLIDAY_ROWS } from "@/lib/holidays/data";
import type { Viewer } from "@/lib/viewer";
import {
  installRoutedFetch,
  jsonResponse,
  workMonthApi,
  workMonthBody,
  type RoutedFetch,
} from "@/test-support/work-month";
import { WorkMonthHolidayView } from "./work-month-holiday-view";

// daily-record-entry-ui.md AC-9（本物のデータの配線）、設計 M-12・T-3（段2）。
// 月と日は HOLIDAY_ROWS から読み、字面で書かない。原本は読まない。fetch は手書きの Fake、next/navigation だけ vi.mock。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const ENGINEER: Viewer = { kind: "signed-in", role: "Engineer" };

let current: RoutedFetch | undefined;

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-08-15T00:00:00Z"));
  current = undefined;
});

afterEach(() => {
  if (current !== undefined) expect(current.unhandled).toEqual([]);
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("WorkMonthHolidayView（本物の祝日データの配線）", () => {
  it("HOLIDAY_ROWS の1行の日のマスに札「祝」が出て data-dow が hol", async () => {
    expect(HOLIDAY_ROWS.length).toBeGreaterThan(0);
    const row = HOLIDAY_ROWS[Math.floor(HOLIDAY_ROWS.length / 2)];
    const yearMonth = row.date.slice(0, 7);
    const month = Number(row.date.slice(5, 7));
    const day = Number(row.date.slice(8, 10));
    const sameMonth = HOLIDAY_ROWS.filter((r) => r.date.startsWith(yearMonth));

    current = installRoutedFetch({
      [`GET ${workMonthApi("c-001", yearMonth)}`]: () =>
        jsonResponse(workMonthBody({ yearMonth, dailyRecords: [], ownership: true })),
    });

    const { container } = render(
      <WorkMonthHolidayView viewer={ENGINEER} contractId="c-001" yearMonth={yearMonth} />,
    );
    await screen.findByRole("heading", { level: 1 });

    const cell = screen.getByRole("button", { name: new RegExp(`^${month}月${day}日（祝）`) });
    expect(cell.getAttribute("data-dow")).toBe("hol");
    expect(within(cell).getAllByText("祝")).toHaveLength(1);

    // 同じ月の祝日の行数だけ、札と hol が出る（データに無い日に出さない）
    expect(container.querySelectorAll('[data-dow="hol"]')).toHaveLength(sameMonth.length);
    expect(screen.getAllByText("祝")).toHaveLength(sameMonth.length);
  });
});
