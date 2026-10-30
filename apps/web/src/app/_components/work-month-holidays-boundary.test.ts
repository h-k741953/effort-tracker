import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// docs/specs/web-screens-implementation-design.md M-1・M-12・T-3（段の境界）。
// lib/work-month-calendar.ts・WorkMonthView・workMonthScreen と、段1 の表のテストは lib/holidays/ を輸入しない。
// 輸入するのは work-month-holiday-view.tsx だけ（M-12 の段で置く）。ファイル走査で字面を読む。

const SRC_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const SELF = "app/_components/work-month-holidays-boundary.test.ts";

/** 内容に holidays の輸入（from "…holidays…" の形）があるか。 */
function importsHolidays(content: string): boolean {
  return /from\s+["'][^"']*holidays[^"']*["']/.test(content);
}

const IMPLEMENTATIONS = [
  "lib/work-month-calendar.ts",
  "app/_components/work-month-view.tsx",
  "app/_components/work-month-screen.tsx",
];

// 段1 の表（T-3）のテストファイル。本テスト自身を除く。
const STAGE1_TESTS = [
  "lib/work-month-actions.test.ts",
  "lib/work-month.test.ts",
  "lib/work-month-client.test.ts",
  "lib/work-month-feedback.test.ts",
  "lib/work-month-calendar.test.ts",
  "lib/jst-month.test.ts",
  "lib/time-entry.test.ts",
  "lib/side-nav-items.test.ts",
  "app/_components/app-side-nav.test.tsx",
  "app/_components/work-month-screen.test.tsx",
  "app/_components/work-month-summary.test.tsx",
  "app/_components/work-month-calendar.test.tsx",
  "app/_components/daily-record-form.test.tsx",
  "app/_components/work-month-operations.test.tsx",
  "app/_components/work-month-view.test.tsx",
];

const read = (relative: string) => readFileSync(path.join(SRC_DIR, relative), "utf-8");

describe("述語の判別力（陽性対照）", () => {
  it.each([
    'import { isHoliday } from "@/lib/holidays";',
    "import { isHoliday } from '../../lib/holidays/index';",
    'import { HOLIDAY_ROWS } from "@/lib/holidays/data";',
  ])("輸入の字面を含む内容に当たる: %s", (content) => {
    expect(importsHolidays(content)).toBe(true);
  });

  it.each([
    "// holidays は work-month-holiday-view だけが輸入する",
    'const label = "holidays";',
    'import { calendarCells } from "@/lib/work-month-calendar";',
  ])("輸入でない内容には当たらない: %s", (content) => {
    expect(importsHolidays(content)).toBe(false);
  });
});

describe("走査の母集団", () => {
  it("走査するファイルは0件でない（実装3件と段1 のテスト15件。本テスト自身を含まない）", () => {
    const targets = [...IMPLEMENTATIONS, ...STAGE1_TESTS];
    expect(targets).toHaveLength(18);
    expect(targets).not.toContain(SELF);
  });
});

describe("段の境界（M-1・M-12）", () => {
  it.each(IMPLEMENTATIONS)("%s は holidays を輸入しない（ファイルが存在する）", (relative) => {
    const content = read(relative);
    expect(content.length, `${relative} が空`).toBeGreaterThan(0);
    expect(importsHolidays(content)).toBe(false);
  });

  it.each(STAGE1_TESTS)("段1 のテスト %s は holidays を輸入しない", (relative) => {
    const content = read(relative);
    expect(content.length, `${relative} が空`).toBeGreaterThan(0);
    expect(importsHolidays(content)).toBe(false);
  });
});
