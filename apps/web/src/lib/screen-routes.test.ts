import { describe, expect, it } from "vitest";
import { ownWorkMonthsPath, thisMonthPath, workMonthPath } from "./screen-routes";

// docs/specs/work-month-listing-ui.md AC-5-1・9-12・10-9。設計 C-2・T-2。
// ヘルプ・契約一覧・稼働の推移のパス（/help・/contracts・/work-trends）の定数名は設計が定めていないため、
// 本ファイルでは読まず、side-nav-items.test.ts が href として読む。

function parse(path: string) {
  return new URL(path, "http://localhost");
}

describe("workMonthPath（5-1）", () => {
  it("/work-months/<契約ID>/<年月>", () => {
    expect(workMonthPath("c-001", "2026-10")).toBe("/work-months/c-001/2026-10");
  });

  it.each([
    ["a/b", "2026-10", "/work-months/a%2Fb/2026-10"],
    ["a b", "2026-10", "/work-months/a%20b/2026-10"],
    ["a?b#c", "2026-10", "/work-months/a%3Fb%23c/2026-10"],
    ["契約", "2026-10", `/work-months/${encodeURIComponent("契約")}/2026-10`],
    ["c-001", "2026/10", "/work-months/c-001/2026%2F10"],
  ])("各セグメントを符号化する: %j %j", (contractId, yearMonth, expected) => {
    expect(workMonthPath(contractId, yearMonth)).toBe(expected);
  });
});

describe("ownWorkMonthsPath（9-12）", () => {
  it("契約を指定するとクエリ contract を持つ", () => {
    const url = parse(ownWorkMonthsPath({ contractId: "c-001" }));
    expect(url.pathname).toBe("/work-months");
    expect(url.searchParams.get("contract")).toBe("c-001");
  });

  it("指定が無ければクエリを持たない", () => {
    expect(ownWorkMonthsPath({})).toBe("/work-months");
  });

  it.each(["a&b=c", "a/b", "a=b", "x&contract=y"])(
    "契約ID %j をクエリ contract の値1つとして復号でき、別のパラメータを生じない",
    (contractId) => {
      const url = parse(ownWorkMonthsPath({ contractId }));
      expect(url.pathname).toBe("/work-months");
      expect([...url.searchParams.keys()]).toEqual(["contract"]);
      expect(url.searchParams.get("contract")).toBe(contractId);
    },
  );
});

describe("thisMonthPath（10-9。Q-10: 年月を含まない）", () => {
  it("/this-month?contract=<契約ID> と一致する", () => {
    expect(thisMonthPath("c-001")).toBe("/this-month?contract=c-001");
  });

  it.each(["a&b=c", "a/b", "a=b", "x&contract=y"])(
    "契約ID %j をクエリ contract の値1つとして復号でき、別のパラメータを生じない",
    (contractId) => {
      const url = parse(thisMonthPath(contractId));
      expect(url.pathname).toBe("/this-month");
      expect([...url.searchParams.keys()]).toEqual(["contract"]);
      expect(url.searchParams.get("contract")).toBe(contractId);
    },
  );
});
