import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { StatusBadge } from "@/components/status-badge";
import {
  CONTRACT_A,
  CONTRACT_B,
  CONTRACT_C,
  OWN_ROWS,
  row,
  type TestRow,
} from "@/test-support/work-month-list";
import {
  EMPTY_WORK_MONTH_FILTER,
  WORK_MONTH_LIST_PAGE_SIZE,
  contractOptionsFromContracts,
  contractOptionsFromRows,
  filterWorkMonths,
  formatYearMonth,
  hasActiveFilter,
  latestRow,
  pageCount,
  pageRows,
  stateOptions,
  yearMonthOptions,
  type WorkMonthFilter,
} from "./work-month-list";

// docs/specs/work-month-listing-ui.md AC-4-1・4-2・6-3・9-4・9-5・9-6・9-6-a・9-6-b・9-7・9-8・9-11。
// 設計 W-3・W-4・W-8・T-2。純粋関数を表で読む。ページあたり件数は定数を輸入し、数値を書かない。

function filter(partial: Partial<WorkMonthFilter>): WorkMonthFilter {
  return { ...EMPTY_WORK_MONTH_FILTER, ...partial };
}

/** StatusBadge が描く語を、部品を描いて読む（期待値を字面で書かない。T-2）。 */
function badgeLabel(state: "Draft" | "PendingApproval" | "Approved"): string {
  return renderToStaticMarkup(createElement(StatusBadge, { state })).replace(/<[^>]*>/g, "");
}

describe("EMPTY_WORK_MONTH_FILTER（9-4）", () => {
  it("3つの条件がいずれも空文字（すべて）", () => {
    expect(EMPTY_WORK_MONTH_FILTER).toEqual({ contractId: "", yearMonth: "", state: "" });
  });
});

describe("filterWorkMonths（9-4・9-5・9-7・9-8・4-1）", () => {
  it("9-4: 既定の条件は、入力と同じ行を同じ順で返す", () => {
    expect(filterWorkMonths(OWN_ROWS, EMPTY_WORK_MONTH_FILTER)).toEqual(OWN_ROWS);
  });

  it.each<[string, Partial<WorkMonthFilter>, TestRow[]]>([
    ["契約だけ", { contractId: CONTRACT_A.contractId }, [OWN_ROWS[0], OWN_ROWS[2], OWN_ROWS[3]]],
    ["対象年月だけ", { yearMonth: "2026-10" }, [OWN_ROWS[0], OWN_ROWS[1]]],
    ["状態だけ", { state: "Approved" }, [OWN_ROWS[2], OWN_ROWS[3]]],
    ["契約 × 対象年月", { contractId: CONTRACT_A.contractId, yearMonth: "2026-10" }, [OWN_ROWS[0]]],
    ["契約 × 状態", { contractId: CONTRACT_A.contractId, state: "Approved" }, [OWN_ROWS[2], OWN_ROWS[3]]],
    ["対象年月 × 状態", { yearMonth: "2026-10", state: "Draft" }, [OWN_ROWS[1]]],
    [
      "3つ全部",
      { contractId: CONTRACT_A.contractId, yearMonth: "2026-09", state: "Approved" },
      [OWN_ROWS[2]],
    ],
    ["どれも満たさない組み合わせは0件", { contractId: CONTRACT_B.contractId, state: "Approved" }, []],
    ["一覧に無い契約は0件", { contractId: "c-none" }, []],
  ])("9-5・9-8: %s", (_name, partial, expected) => {
    expect(filterWorkMonths(OWN_ROWS, filter(partial))).toEqual(expected);
  });

  it("9-7: 入力に無い行を返さない（どの条件でも入力の部分集合）", () => {
    for (const partial of [
      {},
      { contractId: CONTRACT_A.contractId },
      { state: "Draft" as const },
      { yearMonth: "2026-08" },
    ]) {
      for (const r of filterWorkMonths(OWN_ROWS, filter(partial))) {
        expect(OWN_ROWS).toContain(r);
      }
    }
  });

  it("4-1・9-8: 絞った後も入力の順（対象年月の降順）を保つ。再整列しない", () => {
    // 入力の順に意味を持たせるため、わざと昇順でない並びを与える（並べ替えないこと）。
    const input = [
      row(CONTRACT_A, "2026-08", "Approved"),
      row(CONTRACT_A, "2026-10", "Approved"),
      row(CONTRACT_A, "2026-09", "Approved"),
    ];
    expect(filterWorkMonths(input, filter({ state: "Approved" })).map((r) => r.yearMonth)).toEqual([
      "2026-08",
      "2026-10",
      "2026-09",
    ]);
  });

  it("入力の配列を書き換えず、新しい配列を返す", () => {
    const input = [...OWN_ROWS];
    const out = filterWorkMonths(input, EMPTY_WORK_MONTH_FILTER);
    expect(input).toEqual(OWN_ROWS);
    expect(out).not.toBe(input);
  });

  it("0件の入力は0件", () => {
    expect(filterWorkMonths([], filter({ state: "Draft" }))).toEqual([]);
  });
});

describe("hasActiveFilter（9-11）", () => {
  it.each<[string, WorkMonthFilter, boolean]>([
    ["3つとも空文字", EMPTY_WORK_MONTH_FILTER, false],
    ["契約だけ", filter({ contractId: "c-1" }), true],
    ["対象年月だけ", filter({ yearMonth: "2026-10" }), true],
    ["状態だけ", filter({ state: "Draft" }), true],
    ["2つ", filter({ contractId: "c-1", state: "Approved" }), true],
    ["3つ", filter({ contractId: "c-1", yearMonth: "2026-10", state: "Approved" }), true],
  ])("%s → %s", (_name, f, expected) => {
    expect(hasActiveFilter(f)).toBe(expected);
  });
});

describe("contractOptionsFromContracts（9-6。本人一覧）", () => {
  it("契約ごとに value が contractId・label が契約表示名で、入力の順", () => {
    expect(contractOptionsFromContracts([CONTRACT_B, CONTRACT_A, CONTRACT_C])).toEqual([
      { value: CONTRACT_B.contractId, label: CONTRACT_B.contractDisplayName },
      { value: CONTRACT_A.contractId, label: CONTRACT_A.contractDisplayName },
      { value: CONTRACT_C.contractId, label: CONTRACT_C.contractDisplayName },
    ]);
  });

  it("0件は空", () => {
    expect(contractOptionsFromContracts([])).toEqual([]);
  });
});

describe("contractOptionsFromRows（9-6。承認待ち一覧）", () => {
  it("行の契約を、重複なし・最初に現れた順で並べる", () => {
    const rows = [
      row(CONTRACT_B, "2026-10", "PendingApproval"),
      row(CONTRACT_A, "2026-10", "PendingApproval"),
      row(CONTRACT_B, "2026-09", "PendingApproval"),
      row(CONTRACT_A, "2026-08", "PendingApproval"),
    ];
    expect(contractOptionsFromRows(rows)).toEqual([
      { value: CONTRACT_B.contractId, label: CONTRACT_B.contractDisplayName },
      { value: CONTRACT_A.contractId, label: CONTRACT_A.contractDisplayName },
    ]);
  });

  it("0件は空", () => {
    expect(contractOptionsFromRows([])).toEqual([]);
  });
});

describe("yearMonthOptions（9-6-a）", () => {
  it("行の対象年月を、重複なし・降順で、label は formatYearMonth", () => {
    // 入力はわざと降順でない並びにする（関数が降順にすること）。
    const rows = [
      row(CONTRACT_A, "2026-08", "Approved"),
      row(CONTRACT_A, "2026-10", "Approved"),
      row(CONTRACT_B, "2026-10", "Draft"),
      row(CONTRACT_A, "2027-01", "Draft"),
      row(CONTRACT_A, "2026-09", "Approved"),
    ];
    expect(yearMonthOptions(rows)).toEqual(
      ["2027-01", "2026-10", "2026-09", "2026-08"].map((v) => ({ value: v, label: formatYearMonth(v) })),
    );
  });

  it("0件は空", () => {
    expect(yearMonthOptions([])).toEqual([]);
  });
});

describe("stateOptions（9-6-b）", () => {
  it("Draft・PendingApproval・Approved の3つで、label は StatusBadge の描く語と一致する", () => {
    const options = stateOptions();
    expect(options.map((o) => o.value)).toEqual(["Draft", "PendingApproval", "Approved"]);
    expect(options.map((o) => o.label)).toEqual([
      badgeLabel("Draft"),
      badgeLabel("PendingApproval"),
      badgeLabel("Approved"),
    ]);
    // 空の語を比べて偶然一致しないこと。
    for (const o of options) expect(o.label).not.toBe("");
  });
});

describe("pageCount（4-2・W-4）", () => {
  const SIZE = WORK_MONTH_LIST_PAGE_SIZE;
  it.each<[string, number, number]>([
    ["0件でも1ページ", 0, 1],
    ["1件", 1, 1],
    ["ちょうど1ページ分", SIZE, 1],
    ["1件超えて2ページ", SIZE + 1, 2],
    ["ちょうど2ページ分", SIZE * 2, 2],
    ["2ページ分を1件超えて3ページ", SIZE * 2 + 1, 3],
  ])("%s", (_name, rowCount, expected) => {
    expect(pageCount(rowCount)).toBe(expected);
  });
});

describe("pageRows（4-2）", () => {
  const SIZE = WORK_MONTH_LIST_PAGE_SIZE;
  const rows = Array.from({ length: SIZE * 2 + 1 }, (_, i) => i);

  it("1 始まり。page 番目の WORK_MONTH_LIST_PAGE_SIZE 件を、入力の順で返す", () => {
    expect(pageRows(rows, 1)).toEqual(rows.slice(0, SIZE));
    expect(pageRows(rows, 2)).toEqual(rows.slice(SIZE, SIZE * 2));
  });

  it("最後のページは端数の件数", () => {
    expect(pageRows(rows, 3)).toEqual([SIZE * 2]);
  });

  it("1ページに収まる件数は、そのまま全部", () => {
    expect(pageRows([1, 2, 3], 1)).toEqual([1, 2, 3]);
  });

  it("0件は空", () => {
    expect(pageRows([], 1)).toEqual([]);
  });

  it("入力の配列を書き換えない", () => {
    const input = [...rows];
    pageRows(input, 2);
    expect(input).toEqual(rows);
  });
});

describe("WORK_MONTH_LIST_PAGE_SIZE（4-2・W-3）", () => {
  it("1以上の整数である（値そのものは仕様で固定しない）", () => {
    expect(Number.isInteger(WORK_MONTH_LIST_PAGE_SIZE)).toBe(true);
    expect(WORK_MONTH_LIST_PAGE_SIZE).toBeGreaterThanOrEqual(1);
  });
});

describe("latestRow（6-3）", () => {
  it("入力の先頭の行を、同じ参照のまま返す", () => {
    expect(latestRow(OWN_ROWS)).toBe(OWN_ROWS[0]);
  });

  it("0件は undefined", () => {
    expect(latestRow([])).toBeUndefined();
  });

  it("絞る前の全体に対して呼ぶと、条件で先頭が除かれても絞る前の先頭を返す", () => {
    const f = filter({ state: "Approved" });
    const filtered = filterWorkMonths(OWN_ROWS, f);
    expect(latestRow(filtered)).not.toBe(OWN_ROWS[0]);
    expect(latestRow(OWN_ROWS)).toBe(OWN_ROWS[0]);
  });
});

describe("formatYearMonth（W-4）", () => {
  it.each([
    ["2026-07", "2026年7月"],
    ["2026-10", "2026年10月"],
    ["2027-01", "2027年1月"],
    ["2026-12", "2026年12月"],
  ])("%s → %s", (input, expected) => {
    expect(formatYearMonth(input)).toBe(expected);
  });
});
