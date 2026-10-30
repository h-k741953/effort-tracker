import { describe, expect, it } from "vitest";
import { HOLIDAY_ROWS, type HolidayRow } from "./data";

// docs/specs/daily-record-entry-ui.md 10-3・10-4・10-4-a・10-5、設計 M-12・T-3（段2）。
// 期待値はデータ自身から読む（年・件数を字面で書かない）。原本（syukujitsu.csv）は読まない。

/** YYYY-MM-DD で実在する日付か（2026-02-30 のような値を弾く）。 */
function isRealDate(value: string): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (m === null) return false;
  const [year, month, day] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const d = new Date(Date.UTC(year, month - 1, day));
  return d.getUTCFullYear() === year && d.getUTCMonth() === month - 1 && d.getUTCDate() === day;
}

/** 載る年のうち、最初の年から最後の年までの間で行が1つも無い年。 */
function missingYears(rows: readonly HolidayRow[]): number[] {
  const years = new Set(rows.map((r) => Number(r.date.slice(0, 4))));
  const all = [...years];
  const first = Math.min(...all);
  const last = Math.max(...all);
  const missing: number[] = [];
  for (let y = first; y <= last; y++) {
    if (!years.has(y)) missing.push(y);
  }
  return missing;
}

describe("述語の判別力（陽性対照）", () => {
  it.each(["2026-02-30", "2026-13-01", "2026-00-10", "2026-1-5", "2026/01/05", "26-01-05", ""])(
    "実在しない・形の違う日付を弾く: %s",
    (value) => {
      expect(isRealDate(value)).toBe(false);
    },
  );

  it.each(["2026-01-01", "2028-02-29", "2026-12-31"])("実在する日付を通す: %s", (value) => {
    expect(isRealDate(value)).toBe(true);
  });

  it("年の欠けの述語は、1年を抜いた列で落ちる", () => {
    const years = [...new Set(HOLIDAY_ROWS.map((r) => Number(r.date.slice(0, 4))))].sort((a, b) => a - b);
    expect(years.length).toBeGreaterThanOrEqual(3);
    const middle = years[Math.floor(years.length / 2)];
    const withoutMiddle = HOLIDAY_ROWS.filter((r) => Number(r.date.slice(0, 4)) !== middle);
    expect(missingYears(withoutMiddle)).toEqual([middle]);
  });
});

describe("HOLIDAY_ROWS（10-3・10-4・10-5）", () => {
  it("0件でない", () => {
    expect(HOLIDAY_ROWS.length).toBeGreaterThan(0);
  });

  it("各行の date は YYYY-MM-DD で実在する日付", () => {
    const bad = HOLIDAY_ROWS.filter((r) => !isRealDate(r.date)).map((r) => r.date);
    expect(bad).toEqual([]);
  });

  it("各行の name は空文字でない", () => {
    const bad = HOLIDAY_ROWS.filter((r) => typeof r.name !== "string" || r.name === "").map((r) => r.date);
    expect(bad).toEqual([]);
  });

  it("date が重複しない", () => {
    const dates = HOLIDAY_ROWS.map((r) => r.date);
    expect(new Set(dates).size).toBe(dates.length);
  });

  it("date が昇順", () => {
    const dates = HOLIDAY_ROWS.map((r) => r.date);
    expect(dates).toEqual([...dates].sort());
  });

  it("載る年が最初の年から最後の年まで欠けなく続く（10-4-a）", () => {
    expect(HOLIDAY_ROWS.length).toBeGreaterThan(0); // 空の列で恒真にしない
    expect(missingYears(HOLIDAY_ROWS)).toEqual([]);
  });
});
