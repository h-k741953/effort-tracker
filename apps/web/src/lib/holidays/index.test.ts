import { describe, expect, it } from "vitest";
import { HOLIDAY_ROWS } from "./data";
import { isHoliday } from "./index";

// docs/specs/daily-record-entry-ui.md 10-2・10-5、AC-9、設計 M-12・T-3（段2）。
// 期待値は HOLIDAY_ROWS から読む（日付を字面で書かない）。原本は読まない。

const DATES = new Set(HOLIDAY_ROWS.map((r) => r.date));
const FIRST_YEAR = Number(HOLIDAY_ROWS[0]?.date.slice(0, 4));

describe("isHoliday（10-2・10-5）", () => {
  it("HOLIDAY_ROWS が0件でない（以下を空の列で恒真にしない）", () => {
    expect(HOLIDAY_ROWS.length).toBeGreaterThan(0);
  });

  it("HOLIDAY_ROWS のすべての date で真（名称を問わない）", () => {
    const notHoliday = HOLIDAY_ROWS.filter((r) => !isHoliday(r.date)).map((r) => r.date);
    expect(notHoliday).toEqual([]);
  });

  it("データの最初の年の範囲で、行に無い日は偽", () => {
    const candidates: string[] = [];
    for (let d = new Date(Date.UTC(FIRST_YEAR, 0, 1)); d.getUTCFullYear() === FIRST_YEAR; d.setUTCDate(d.getUTCDate() + 1)) {
      const date = d.toISOString().slice(0, 10);
      if (!DATES.has(date)) candidates.push(date);
    }
    expect(candidates.length).toBeGreaterThan(0);
    const wrong = candidates.filter((date) => isHoliday(date));
    expect(wrong).toEqual([]);
  });

  it("データの最初の年より前の日（前年の 01-01）は偽（計算で補わない）", () => {
    expect(Number.isInteger(FIRST_YEAR)).toBe(true);
    expect(isHoliday(`${String(FIRST_YEAR - 1).padStart(4, "0")}-01-01`)).toBe(false);
  });
});
