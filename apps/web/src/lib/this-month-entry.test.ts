import { describe, expect, it } from "vitest";
import { thisMonthPath } from "./screen-routes";
import { thisMonthEntry } from "./this-month-entry";

// docs/specs/work-month-listing-ui.md AC-6-2・7-1・10-9-a〜10-9-d。設計 W-6・T-2。
// 契約の形は E-8 の行（contractId・contractDisplayName。domain-api-http-contract.md AC-10-4）。

const A = { contractId: "c-001", contractDisplayName: "A社 / 保守" };
const B = { contractId: "c-002", contractDisplayName: "B社 / 開発" };
const C = { contractId: "c-003", contractDisplayName: "C社 / 運用" };

describe("thisMonthEntry", () => {
  it("10-9-d: 契約が0件 → hidden", () => {
    expect(thisMonthEntry([])).toEqual({ kind: "hidden" });
  });

  it("10-9-a: 契約が1件 → link（その契約の thisMonthPath）", () => {
    expect(thisMonthEntry([A])).toEqual({ kind: "link", href: thisMonthPath("c-001") });
  });

  it.each([
    ["2件", [A, B]],
    ["3件", [C, A, B]],
  ])("10-9-b・10-9-c: 契約が%s → choice（入力の順・契約表示名・各契約の thisMonthPath）", (_name, contracts) => {
    const entry = thisMonthEntry(contracts);
    expect(entry).toEqual({
      kind: "choice",
      options: contracts.map((c) => ({
        label: c.contractDisplayName,
        href: thisMonthPath(c.contractId),
      })),
    });
  });

  it("10-9-c: 契約期間で選択肢を絞らない（期間が過去に終わった契約も除かない）", () => {
    const ended = { ...A, contractPeriod: { start: "2020-04", end: "2020-05" } };
    const open = { ...B, contractPeriod: { start: "2099-01", end: null } };
    const contracts = [ended, open];
    const entry = thisMonthEntry(contracts);
    expect(entry.kind).toBe("choice");
    if (entry.kind === "choice") {
      expect(entry.options.map((o) => o.label)).toEqual([A.contractDisplayName, B.contractDisplayName]);
    }
  });

  it("Q-10: 年月を引数に取らない（引数は契約の配列の1つ）", () => {
    expect(thisMonthEntry.length).toBe(1);
  });
});
