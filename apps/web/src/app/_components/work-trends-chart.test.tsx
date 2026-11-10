/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { WorkTrendRow } from "@/lib/work-trends";
import { WorkTrendsChart } from "./work-trends-chart";

// docs/specs/work-trends-ui.md 2-2・2-4・3-5・4-6・5-1〜5-5・5-4-a・5-6（図のアクセシブル名）・5-7・5-9。
// 設計 readonly-screens-implementation-design.md SP-3・SP-6・S-1（SP-7 (b)・(i)）。
// 文言は仕様の字面をここへ書く（実装の定数を輸入しない）。図は期間で絞った行を props で受ける。
// 赤（--danger）の当て先は、jsdom が CSS を当てないためクラスで読む。棒・帯の割合は style の % で読む。
// 図の中の要素は、文字（葉の要素の textContent）と、文字を持たない要素（棒・見本）の位置で見つける。

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

// 年をまたぐ5か月。収まった・超過・不足・収まった・入力中。値はすべて異なる。
// 縦軸: 最小 98.5（入力中）→ floor(7.85)*10 = 70、最大 185.25 → ceil(19.525)*10 = 200。
const MIXED = [
  tr("2025-12", { totalHours: hm(152, 30) }),
  tr("2026-01", { state: "PendingApproval", totalHours: hm(185, 15), excess: hm(5, 15) }),
  tr("2026-02", { totalHours: hm(137, 0), shortfall: hm(3, 0) }),
  tr("2026-03", { totalHours: hm(160, 45) }),
  draft("2026-04", { totalHours: hm(98, 30) }),
];
const MIXED_LABELS = ["2025年12月", "2026年1月", "2月", "3月", "4月"];
const MIXED_VALUES = ["152.5", "185.25", "137.0", "160.75", "98.5"];
const LEGEND = ["稼働時間", "超過した分", "不足している分", "入力中（下書き）", "精算幅"];

// ---- DOM の読み取り補助 ----

const DANGER = /danger/;
const hasDangerClass = (el: Element) => [...el.classList].some((c) => DANGER.test(c));
const MONTH_LABEL = /^(\d{4}年)?\d{1,2}月$/;
const NUMBER = /^\d+(\.\d+)?$/;
const TAG = /^(超過|不足) \S+$/;

/** 文字を持つ要素のうち、文字を持つ子の要素を持たないもの（文字を持たない見本などの子は持ってよい）。 */
function leaves(root: Element): HTMLElement[] {
  return [...root.querySelectorAll<HTMLElement>("*")].filter(
    (el) =>
      (el.textContent ?? "").trim() !== "" &&
      [...el.children].every((c) => (c.textContent ?? "").trim() === ""),
  );
}
const text = (el: Element) => (el.textContent ?? "").trim();
const leavesWhere = (root: Element, pred: (t: string) => boolean) => leaves(root).filter((l) => pred(text(l)));
const leafEq = (root: Element, t: string) => leavesWhere(root, (x) => x === t);

/** el から上へ、root の手前まで（root を含まない）の要素と自身。 */
function chainUp(el: Element, stop: Element | null = null): Element[] {
  const out: Element[] = [];
  for (let cur: Element | null = el; cur !== null && cur !== stop; cur = cur.parentElement) out.push(cur);
  return out;
}
/** 自身かその祖先（文書の根まで）に --danger のクラスがあるか。 */
const redByChain = (el: Element) => chainUp(el).some(hasDangerClass);
/** 自身かその子孫に --danger のクラスがあるか。 */
const redInside = (root: Element) => [root, ...root.querySelectorAll("*")].some(hasDangerClass);

function figure(): HTMLElement {
  return screen.getByRole("img", { name: "月ごとの総稼働時間" });
}
function cardRoot(): HTMLElement {
  return screen.getByRole("heading", { level: 2, name: "月ごとの総稼働時間" }).parentElement as HTMLElement;
}

/** 図の中で、月の表記の葉を含み、その月の値の葉も含む最も内側の要素＝月の列。 */
function columnOf(fig: Element, label: string, value: string): HTMLElement {
  const labelEl = leafEq(fig, label)[0];
  expect(labelEl, `横軸の表記 ${label}`).toBeDefined();
  for (let cur: Element | null = labelEl; cur !== null && fig.contains(cur); cur = cur.parentElement) {
    if (leafEq(cur, value).length > 0) return cur as HTMLElement;
  }
  throw new Error(`月の列が見つからない: ${label} / ${value}`);
}

/** 要素の style 属性にある % の数値（そのまま）。 */
function percentsIn(root: Element): number[] {
  const out: number[] = [];
  for (const el of [root, ...root.querySelectorAll("*")]) {
    const style = el.getAttribute("style") ?? "";
    for (const m of style.matchAll(/(-?\d+(?:\.\d+)?)%/g)) out.push(Number(m[1]));
  }
  return out;
}
const styleSignature = (root: Element) =>
  [root, ...root.querySelectorAll("*")]
    .map((el) => el.getAttribute("style") ?? "")
    .filter((s) => /%/.test(s))
    .sort()
    .join("|");

/** 図の中の --danger の当て先のうち、5-4 の範囲外のもの（文字を持つ要素は 札 の文字だけが許される）。 */
function illegalDanger(fig: Element): Element[] {
  const outside = chainUp(fig.parentElement as Element).filter(hasDangerClass); // 図の外の祖先
  const inside = [fig, ...fig.querySelectorAll("*")].filter(hasDangerClass);
  return [...outside, ...inside.filter((el) => text(el) !== "" && !TAG.test(text(el)))];
}

/** Card の中で、表を除く文字の葉（見出しも除く）。 */
function cardLeaves(card: Element): string[] {
  const table = card.querySelector("table");
  return leaves(card)
    .filter((l) => (table === null || !table.contains(l)) && l.tagName !== "H2")
    .map(text);
}
const ANNOTATION = /^縦軸は \d+時間から始まります。$/;
/** 凡例・月の表記・値・札・入力中・縦軸の注記のどれでもない文字（目盛りの文字など）。 */
function strayTexts(card: Element, labels: string[], values: string[], tags: string[]): string[] {
  const allowed = new Set([...LEGEND, ...labels, ...values, ...tags, "入力中"]);
  return cardLeaves(card).filter((t) => !allowed.has(t) && !ANNOTATION.test(t));
}

describe("見出し・凡例・図の名前・表の位置（5-9・5-6・S-1）", () => {
  it("5-9: Card の見出しは h2 で「月ごとの総稼働時間」", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    expect(screen.getByRole("heading", { level: 2, name: "月ごとの総稼働時間" })).toBeTruthy();
  });

  it("凡例の5語が S-1 の字面で、それぞれちょうど1つ（表を除く）", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const card = cardRoot();
    const table = card.querySelector("table");
    for (const word of LEGEND) {
      const hits = leafEq(card, word).filter((l) => table === null || !table.contains(l));
      expect(hits, word).toHaveLength(1);
    }
  });

  it("5-6: 図の要素は role=img で、アクセシブル名は「月ごとの総稼働時間」", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    expect(figure()).toBeTruthy();
  });

  it("図の後に、同じ Card の中に表がある（図の中にはない）", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const table = screen.getByRole("table");
    expect(cardRoot().contains(table)).toBe(true);
    expect(figure().contains(table)).toBe(false);
    expect(figure().compareDocumentPosition(table) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});

describe("月の列・横軸の表記・各月の値（3-5・4-6・5-1・5-6・S-1。SP-7 (i)）", () => {
  it("月の列が期間の月と同じ数で、横軸の表記が S-1 の形（先頭の月と1月にだけ年。ほかは「2月」の形）・行の順", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const labels = leavesWhere(figure(), (t) => MONTH_LABEL.test(t)).map(text);
    expect(labels).toEqual(MIXED_LABELS);
    expect(labels).toHaveLength(MIXED.length);
  });

  it("年の途中から始まる期間: 先頭だけに年が付く（「2026年7月」「8月」）。1件だけの期間: 年を付けた形", () => {
    render(<WorkTrendsChart rows={[tr("2026-07"), tr("2026-08", { totalHours: hm(161) })]} />);
    expect(leavesWhere(figure(), (t) => MONTH_LABEL.test(t)).map(text)).toEqual(["2026年7月", "8月"]);
    cleanup();
    render(<WorkTrendsChart rows={[tr("2026-08")]} />);
    expect(leavesWhere(figure(), (t) => MONTH_LABEL.test(t)).map(text)).toEqual(["2026年8月"]);
  });

  it("月に先頭の 0 を付けない（「9月」であって「09月」ではない）", () => {
    render(<WorkTrendsChart rows={[tr("2026-08"), tr("2026-09", { totalHours: hm(161) })]} />);
    const labels = leavesWhere(figure(), (t) => MONTH_LABEL.test(t)).map(text);
    expect(labels).toEqual(["2026年8月", "9月"]);
    expect(labels.some((l) => /年?0\d月/.test(l))).toBe(false);
  });

  it("各月の値は formatHours の形で、単位「時間」を添えない。値の葉の並びは行の順", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const fig = figure();
    expect(leavesWhere(fig, (t) => NUMBER.test(t)).map(text)).toEqual(MIXED_VALUES);
    const withUnit = leavesWhere(fig, (t) => t.includes("時間") && !ANNOTATION.test(t));
    expect(withUnit).toEqual([]);
  });

  it("陽性対照: 単位を添えた値・目盛りの数は、同じ判定（数だけの葉の並び）で検出される", () => {
    const { container, unmount } = render(
      <div role="img" aria-label="x">
        <span>152.5 時間</span>
        <span>120</span>
        <span>152.5</span>
      </div>,
    );
    const fig = container.firstElementChild as HTMLElement;
    expect(leavesWhere(fig, (t) => NUMBER.test(t)).map(text)).toEqual(["120", "152.5"]);
    expect(leavesWhere(fig, (t) => t.includes("時間")).map(text)).toEqual(["152.5 時間"]);
    unmount();
  });
});

describe("札・入力中（2-2・2-4・5-3・5-5）", () => {
  it("超過の月に「超過 5.25」、不足の月に「不足 3.0」の札が1つずつあり、それぞれの月の列の中にある（単位を添えない）", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const fig = figure();
    expect(leavesWhere(fig, (t) => TAG.test(t)).map(text)).toEqual(["超過 5.25", "不足 3.0"]);
    const excessCol = columnOf(fig, "2026年1月", "185.25");
    const shortfallCol = columnOf(fig, "2月", "137.0");
    expect(leafEq(excessCol, "超過 5.25")).toHaveLength(1);
    expect(leafEq(shortfallCol, "不足 3.0")).toHaveLength(1);
  });

  it("収まった月（2025年12月・3月）と入力中の月（4月）には札が無い", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const fig = figure();
    for (const [label, value] of [
      ["2025年12月", "152.5"],
      ["3月", "160.75"],
      ["4月", "98.5"],
    ]) {
      expect(leavesWhere(columnOf(fig, label, value), (t) => /^(超過|不足)/.test(t)), label).toEqual([]);
    }
  });

  it("入力中の月の列には「入力中」の語が値と別の要素であり、ほかの月の列には無い", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const fig = figure();
    const col = columnOf(fig, "4月", "98.5");
    const word = leafEq(col, "入力中");
    expect(word).toHaveLength(1);
    expect(leafEq(col, "98.5")[0]).not.toBe(word[0]);
    expect(leafEq(col, "98.5")[0].contains(word[0])).toBe(false);
    for (const [label, value] of [
      ["2025年12月", "152.5"],
      ["2026年1月", "185.25"],
      ["2月", "137.0"],
      ["3月", "160.75"],
    ]) {
      expect(leafEq(columnOf(fig, label, value), "入力中"), label).toHaveLength(0);
    }
    expect(leafEq(fig, "入力中")).toHaveLength(1);
  });

  it("2-4: 入力中の月は超過・不足の量も、収まったかどうかの語も示さない（括弧などの字も足さない）", () => {
    const { container } = render(
      <WorkTrendsChart rows={[draft("2026-07", { totalHours: hm(98, 30) })]} />,
    );
    const col = columnOf(figure(), "2026年7月", "98.5");
    expect(leaves(col).map(text).sort()).toEqual(["2026年7月", "98.5", "入力中"].sort());
    expect(container.querySelector("table")).not.toBeNull();
  });
});

describe("赤（--danger）の当て先（5-4・5-4-a・5-5・P-12）", () => {
  it("超過・不足の札の文字は赤。値・入力中・横軸の文字は、自身も祖先も赤でない", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const fig = figure();
    for (const tag of ["超過 5.25", "不足 3.0"]) {
      expect(redByChain(leafEq(fig, tag)[0]), tag).toBe(true);
    }
    for (const t of [...MIXED_VALUES, ...MIXED_LABELS, "入力中"]) {
      for (const el of leafEq(fig, t)) expect(redByChain(el), t).toBe(false);
    }
  });

  it("超過の月・不足の月の列には、文字を持たない赤い要素（棒）がある", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const fig = figure();
    for (const [label, value] of [
      ["2026年1月", "185.25"],
      ["2月", "137.0"],
    ]) {
      const col = columnOf(fig, label, value);
      const bars = [col, ...col.querySelectorAll("*")].filter((el) => hasDangerClass(el) && text(el) === "");
      expect(bars.length, label).toBeGreaterThan(0);
    }
  });

  it("収まった月・入力中の月の列には、自身にも子孫にも赤い要素が無い（棒・語・値・帯・横軸）", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const fig = figure();
    for (const [label, value] of [
      ["2025年12月", "152.5"],
      ["3月", "160.75"],
      ["4月", "98.5"],
    ]) {
      expect(redInside(columnOf(fig, label, value)), label).toBe(false);
    }
  });

  it("赤い要素は、文字を持たない要素（棒・見本）か 札 の文字だけで、図の外の祖先にも赤が無い", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    expect(illegalDanger(figure())).toEqual([]);
  });

  it("凡例: 超過した分・不足している分の見本だけが赤。稼働時間・入力中（下書き）・精算幅の見本は赤でない", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const card = cardRoot();
    const swatchesOf = (word: string) => {
      const label = leafEq(card, word).filter((l) => card.querySelector("table")?.contains(l) !== true)[0];
      for (let cur: Element | null = label; cur !== null && cur !== card; cur = cur.parentElement) {
        const swatches = [...cur.querySelectorAll("*")].filter((el) => text(el) === "");
        if (swatches.length > 0) return { label, swatches };
      }
      throw new Error(`見本が見つからない: ${word}`);
    };
    for (const word of ["超過した分", "不足している分"]) {
      const { swatches } = swatchesOf(word);
      expect(swatches.some(hasDangerClass), word).toBe(true);
    }
    for (const word of ["稼働時間", "入力中（下書き）", "精算幅"]) {
      const { label, swatches } = swatchesOf(word);
      expect(swatches.some(hasDangerClass), word).toBe(false);
      expect(redByChain(label), word).toBe(false);
    }
    for (const word of LEGEND) expect(redByChain(swatchesOf(word).label), word).toBe(false);
  });

  it("陽性対照: 値の文字・入力中の語・横軸の文字に赤を当てた図、文字を持つ赤い要素を含む図は、同じ判定で検出される", () => {
    const { container, unmount } = render(
      <div>
        <div role="img" aria-label="x">
          <div className="bg-danger">
            <span>152.5</span>
          </div>
        </div>
      </div>,
    );
    const fig = container.querySelector("[role='img']") as HTMLElement;
    expect(illegalDanger(fig).length).toBeGreaterThan(0);
    expect(redByChain(leafEq(fig, "152.5")[0])).toBe(true);
    unmount();

    const second = render(
      <div className="text-danger">
        <div role="img" aria-label="x">
          <span>超過 5.25</span>
        </div>
      </div>,
    );
    expect(illegalDanger(second.container.querySelector("[role='img']") as HTMLElement).length).toBeGreaterThan(0);
    second.unmount();

    const third = render(
      <div role="img" aria-label="x">
        <div>
          <span className="border-danger">入力中</span>
          <span className="text-danger">超過 5.25</span>
        </div>
      </div>,
    );
    const thirdFig = third.container.querySelector("[role='img']") as HTMLElement;
    expect(illegalDanger(thirdFig)).toHaveLength(1);
    expect(redInside(thirdFig)).toBe(true);
    third.unmount();
  });
});

describe("帯と棒（5-1・5-2。割合を与える値）", () => {
  // 総稼働時間は同じ 150、精算幅だけ違う2つの月と、1つ目と同じ月。縦軸: 最小 140 → 120、最大 190 → 200。
  const A = tr("2026-05", { totalHours: hm(150), settlementRange: { lowerBound: hm(140), upperBound: hm(180) } });
  const B = tr("2026-06", { totalHours: hm(150), settlementRange: { lowerBound: hm(145), upperBound: hm(190) } });
  const A2 = tr("2026-07", { totalHours: hm(150), settlementRange: { lowerBound: hm(140), upperBound: hm(180) } });

  it("精算幅の違う2つの月で、棒の値が同じでも、列に与える割合が違う（帯が月ごとの settlementRange で描かれる）", () => {
    render(<WorkTrendsChart rows={[A, B, A2]} />);
    const fig = figure();
    const colA = columnOf(fig, "2026年5月", "150.0");
    const colB = columnOf(fig, "6月", "150.0");
    expect(styleSignature(colA)).not.toBe("");
    expect(styleSignature(colA)).not.toBe(styleSignature(colB));
  });

  it("陽性対照: 精算幅が同じ2つの月（5月と7月）は同じ割合の組になる。月の違い・表記では変わらない", () => {
    render(<WorkTrendsChart rows={[A, B, A2]} />);
    const fig = figure();
    const colA = columnOf(fig, "2026年5月", "150.0");
    const colA2 = columnOf(fig, "7月", "150.0");
    expect(colA).not.toBe(colA2);
    expect(styleSignature(colA)).toBe(styleSignature(colA2));
  });

  it("収まった月の棒の割合は縦軸の範囲に対する値、帯の割合は下限・上限の位置（縦軸 120〜190。棒 30/70、帯の下限 20/70・高さ 40/70）", () => {
    render(<WorkTrendsChart rows={[tr("2026-05", { totalHours: hm(150) }), tr("2026-06", { totalHours: hm(155) })]} />);
    // 縦軸: 最小 140 → 120、最大 180 → 190。幅 70。
    const col = columnOf(figure(), "2026年5月", "150.0");
    const percents = percentsIn(col);
    const has = (x: number) => percents.some((p) => Math.abs(p - x) < 0.6);
    expect(has((30 / 70) * 100), `棒 ${percents.join(",")}`).toBe(true);
    expect(has((40 / 70) * 100), `帯の高さ ${percents.join(",")}`).toBe(true);
    expect(has((20 / 70) * 100) || has((10 / 70) * 100), `帯の位置 ${percents.join(",")}`).toBe(true);
  });

  it("陽性対照: 割合の読み取りは、style の % を実際に拾い、別の値を拾わない", () => {
    const { container, unmount } = render(
      <div>
        <span style={{ height: "42.857%" }} />
        <span style={{ bottom: "28.5%", width: "10px" }} />
      </div>,
    );
    expect(percentsIn(container)).toEqual([42.857, 28.5]);
    expect(styleSignature(container)).toContain("height: 42.857%");
    unmount();
  });
});

describe("縦軸の注記と目盛り（5-7。SP-7 (b)）", () => {
  it("縦軸の始まりが 0 より大きい組（70）→「縦軸は 70時間から始まります。」を整数で出す", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    expect(cardRoot().textContent).toContain("縦軸は 70時間から始まります。");
  });

  it("別の組（精算幅 140・総稼働時間 150 → 始まり 120）→「縦軸は 120時間から始まります。」", () => {
    render(<WorkTrendsChart rows={[tr("2026-07", { totalHours: hm(150) })]} />);
    expect(cardRoot().textContent).toContain("縦軸は 120時間から始まります。");
  });

  it("縦軸の始まりが 0 の組（入力中の月が 20.5 時間 → 0）→ 注記を出さない", () => {
    render(<WorkTrendsChart rows={[tr("2026-07", { totalHours: hm(150) }), draft("2026-08", { totalHours: hm(20, 30) })]} />);
    expect(cardRoot().textContent).not.toMatch(/縦軸は/);
    expect(cardRoot().textContent).not.toContain("から始まります");
  });

  it("縦軸の目盛りの文字が無い（凡例・月の表記・値・札・入力中・注記のほかの文字を図に置かない）", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    expect(strayTexts(cardRoot(), MIXED_LABELS, MIXED_VALUES, ["超過 5.25", "不足 3.0"])).toEqual([]);
  });

  it("陽性対照: 目盛りの文字（始まり・終わりの数）を足した図は、同じ判定で検出される", () => {
    render(<WorkTrendsChart rows={MIXED} />);
    const card = cardRoot();
    const tick = document.createElement("span");
    tick.textContent = "200";
    figure().appendChild(tick);
    const tick2 = document.createElement("span");
    tick2.textContent = "70時間";
    card.appendChild(tick2);
    expect(strayTexts(card, MIXED_LABELS, MIXED_VALUES, ["超過 5.25", "不足 3.0"]).sort()).toEqual(["200", "70時間"]);
  });
});
