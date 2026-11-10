/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import type { WorkTrendRow } from "@/lib/work-trends";
import { WorkTrendsTable } from "./work-trends-table";

// docs/specs/work-trends-ui.md 4-4-b・4-6・5-6、決定2（図と同じ内容の表を画面に見せる）。
// 設計 readonly-screens-implementation-design.md SP-3・SP-6・S-1（SP-7 (i)）。
// 文言の期待値は仕様の字面をここへ書く（実装の定数を輸入しない）。表は期間で絞った行を props で受ける。

afterEach(cleanup);

type Hm = { hours: number; minutes: number };
const hm = (hours: number, minutes = 0): Hm => ({ hours, minutes });
const RANGE = { lowerBound: hm(140), upperBound: hm(180) };

function tr(yearMonth: string, o: Partial<WorkTrendRow> = {}): WorkTrendRow {
  return {
    contractId: "c-1",
    yearMonth,
    state: "Approved",
    settlementRange: RANGE,
    totalHours: hm(160),
    excess: hm(0),
    shortfall: hm(0),
    ...o,
  };
}
const draft = (yearMonth: string, o: Partial<WorkTrendRow> = {}): WorkTrendRow =>
  tr(yearMonth, { state: "Draft", excess: null, shortfall: null, ...o });

// 年をまたぐ5か月。2026-02 だけ精算幅が違う（4-4-b）。
const ROWS = [
  tr("2025-12", { totalHours: hm(152, 30) }),
  tr("2026-01", { state: "PendingApproval", totalHours: hm(185, 15), excess: hm(5, 15) }),
  tr("2026-02", {
    totalHours: hm(137, 0),
    shortfall: hm(3, 0),
    settlementRange: { lowerBound: hm(150), upperBound: hm(170) },
  }),
  tr("2026-03", { totalHours: hm(160, 45) }),
  draft("2026-04", { totalHours: hm(98, 30) }),
];

const EXPECTED = [
  ["2025年12月", "152.5 時間", "精算幅に収まった", "140.0〜180.0 時間"],
  ["2026年1月", "185.25 時間", "超過 5.25 時間", "140.0〜180.0 時間"],
  ["2月", "137.0 時間", "不足 3.0 時間", "150.0〜170.0 時間"],
  ["3月", "160.75 時間", "精算幅に収まった", "140.0〜180.0 時間"],
  ["4月", "98.5 時間", "入力中", "140.0〜180.0 時間"],
];

function bodyCells(table: HTMLElement): string[][] {
  return within(table)
    .getAllByRole("row")
    .slice(1)
    .map((r) => [...r.querySelectorAll("th, td")].map((c) => c.textContent ?? ""));
}

/** 表の自身か祖先が、視覚的にも支援技術にも隠されているか（sr-only・hidden・aria-hidden="true"）。 */
function hiddenChain(table: Element): string[] {
  const found: string[] = [];
  for (let cur: Element | null = table; cur !== null; cur = cur.parentElement) {
    if (cur.classList.contains("sr-only")) found.push("sr-only");
    if (cur.hasAttribute("hidden")) found.push("hidden");
    if (cur.getAttribute("aria-hidden") === "true") found.push("aria-hidden");
  }
  return found;
}

describe("列・行（5-6・S-1）", () => {
  it("列見出しは「対象年月」「総稼働時間」「区分」「精算幅」で、その順", () => {
    render(<WorkTrendsTable rows={ROWS} />);
    const table = screen.getByRole("table");
    expect(within(table).getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "対象年月",
      "総稼働時間",
      "区分",
      "精算幅",
    ]);
  });

  it("行は受けた行と同じ月・同じ順。対象年月は図の横軸と同じ表記、総稼働時間は「162.5 時間」の形、区分と精算幅は S-1 の字面", () => {
    render(<WorkTrendsTable rows={ROWS} />);
    expect(bodyCells(screen.getByRole("table"))).toEqual(EXPECTED);
  });

  it("4-4-b: 精算幅の列は月ごとの値（精算幅の違う月は違う値）で、契約のいまの値や代表値ではない", () => {
    render(<WorkTrendsTable rows={ROWS} />);
    const ranges = bodyCells(screen.getByRole("table")).map((c) => c[3]);
    expect(new Set(ranges)).toEqual(new Set(["140.0〜180.0 時間", "150.0〜170.0 時間"]));
    expect(ranges[2]).toBe("150.0〜170.0 時間");
  });

  it("区分: 入力中の月は超過・不足の量を示さず、収まった月は札の語を持たない。精算幅の外でも保存された値が 0 なら収まった", () => {
    render(
      <WorkTrendsTable
        rows={[draft("2026-07", { totalHours: hm(200) }), tr("2026-08", { totalHours: hm(200) }), tr("2026-09", { totalHours: hm(100) })]}
      />,
    );
    expect(bodyCells(screen.getByRole("table")).map((c) => c[2])).toEqual([
      "入力中",
      "精算幅に収まった",
      "精算幅に収まった",
    ]);
  });

  it("対象年月: 年の途中から始まる期間は先頭だけに年、1件だけの期間は年を付けた形、月に先頭の 0 を付けない", () => {
    render(<WorkTrendsTable rows={[tr("2026-08"), tr("2026-09", { totalHours: hm(161) })]} />);
    expect(bodyCells(screen.getByRole("table")).map((c) => c[0])).toEqual(["2026年8月", "9月"]);
    cleanup();
    render(<WorkTrendsTable rows={[tr("2026-08")]} />);
    expect(bodyCells(screen.getByRole("table")).map((c) => c[0])).toEqual(["2026年8月"]);
  });

  it("表の文字に --danger のクラスを当てない（超過・不足の行でも）", () => {
    const { container } = render(<WorkTrendsTable rows={ROWS} />);
    expect(container.querySelectorAll("tbody tr")).toHaveLength(ROWS.length); // 表が描かれていること（空の描画で素通りさせない）
    const red = [container, ...container.querySelectorAll("*")].filter((el) =>
      [...el.classList].some((c) => /danger/.test(c)),
    );
    expect(red).toEqual([]);
  });
});

describe("画面に見える形で置く（決定2）", () => {
  it("表とその祖先に sr-only のクラス・hidden 属性・aria-hidden=\"true\" が無い", () => {
    const { container } = render(<WorkTrendsTable rows={ROWS} />);
    const table = container.querySelector("table");
    expect(table).not.toBeNull();
    expect(hiddenChain(table!)).toEqual([]);
    expect(screen.getByRole("table")).toBe(table);
  });

  it("表の中（見出し・行・セル）にも sr-only・hidden・aria-hidden が無い", () => {
    const { container } = render(<WorkTrendsTable rows={ROWS} />);
    expect(container.querySelectorAll("table td, table th").length).toBeGreaterThan(0); // 表が描かれていること
    const bad = [...container.querySelectorAll("table, table *")].filter(
      (el) =>
        el.classList.contains("sr-only") || el.hasAttribute("hidden") || el.getAttribute("aria-hidden") === "true",
    );
    expect(bad).toEqual([]);
  });

  it.each([
    ["sr-only の祖先", <div key="a" className="sr-only"><table><tbody><tr><td>x</td></tr></tbody></table></div>, "sr-only"],
    ["hidden 属性の祖先", <div key="a" hidden><table><tbody><tr><td>x</td></tr></tbody></table></div>, "hidden"],
    ["aria-hidden の祖先", <div key="a" aria-hidden="true"><table><tbody><tr><td>x</td></tr></tbody></table></div>, "aria-hidden"],
    ["表自身の sr-only", <table key="a" className="sr-only"><tbody><tr><td>x</td></tr></tbody></table>, "sr-only"],
    ["遠い祖先の sr-only", <section key="a" className="sr-only"><div><div><table><tbody><tr><td>x</td></tr></tbody></table></div></div></section>, "sr-only"],
  ])("陽性対照: %s は、同じ判定で検出される", (_name, element, kind) => {
    const { container, unmount } = render(<div>{element}</div>);
    const table = container.querySelector("table")!;
    expect(hiddenChain(table)).toContain(kind);
    unmount();
  });
});
