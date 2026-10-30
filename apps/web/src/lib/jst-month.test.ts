import { describe, expect, it } from "vitest";
import { currentJstYearMonth } from "./jst-month";

// docs/specs/work-month-listing-ui.md AC-10-9。設計 W-7・T-2。
// 与えた時刻の JST（UTC+9。夏時間なし）の年月 YYYY-MM を返す。時刻は引数で注入する。

describe("currentJstYearMonth の境目（10-9）", () => {
  it.each([
    ["2026-10-31T14:59:59Z", "2026-10"],
    ["2026-10-31T15:00:00Z", "2026-11"],
    ["2026-12-31T14:59:59Z", "2026-12"],
    ["2026-12-31T15:00:00Z", "2027-01"],
  ])("%s → %s", (iso, expected) => {
    expect(currentJstYearMonth(new Date(iso))).toBe(expected);
  });
});
