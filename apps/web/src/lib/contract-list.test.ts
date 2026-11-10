import { describe, expect, it } from "vitest";
import { formatContractPeriod } from "./contract-list";

// docs/specs/contracts-ui.md 2-3・2-3-a・2-3-b。設計 KP-3・KP-4。
// 字面は仕様の条文から書き、実装の定数を輸入しない。

describe("formatContractPeriod（2-3-a・2-3-b）", () => {
  it.each<[string, { start: string; end: string | null }, string]>([
    ["2-3-a 開始 2025-04・終了 2027-03", { start: "2025-04", end: "2027-03" }, "2025年4月〜2027年3月"],
    ["2-3-a 開始と終了が同じ年月", { start: "2026-06", end: "2026-06" }, "2026年6月〜2026年6月"],
    ["2-3-a 月が2桁（10月〜12月）", { start: "2025-10", end: "2026-12" }, "2025年10月〜2026年12月"],
    ["2-3-a 年をまたぐ", { start: "2025-12", end: "2026-01" }, "2025年12月〜2026年1月"],
    ["2-3-b 終了が null（〜の後に何も無い）", { start: "2025-04", end: null }, "2025年4月〜"],
    ["2-3-b 終了が null・月が2桁", { start: "2025-11", end: null }, "2025年11月〜"],
  ])("%s → %s", (_name, period, expected) => {
    expect(formatContractPeriod(period)).toBe(expected);
  });

  it("2-3-b: 終了が null のとき、〜で終わり、後に語を足さない", () => {
    const text = formatContractPeriod({ start: "2025-04", end: null });
    expect(text.endsWith("〜")).toBe(true);
    expect(text.split("〜")).toEqual(["2025年4月", ""]);
  });

  it("2-3-a: 月に先頭の 0 を付けない", () => {
    const text = formatContractPeriod({ start: "2025-04", end: "2027-03" });
    expect(/[年〜]0\d月/.test(text)).toBe(false);
    // 陽性対照: 先頭の 0 を付けた字面を、この判定は検出する。
    expect(/[年〜]0\d月/.test("2025年04月〜2027年03月")).toBe(true);
  });

  it("2-3: 日を示さない", () => {
    const text = formatContractPeriod({ start: "2025-04", end: "2027-03" });
    expect(text.includes("日")).toBe(false);
    // 陽性対照: 日を含む字面を、この判定は検出する。
    expect("2025年4月1日〜".includes("日")).toBe(true);
  });
});
