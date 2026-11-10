/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { WorkTrendRow } from "@/lib/work-trends";
import { WorkTrendsSummary } from "./work-trends-summary";

// docs/specs/work-trends-ui.md 4-1-b・4-1-c・4-2・4-3・4-4-a・4-4-b・4-5・4-7・8-3。
// 設計 readonly-screens-implementation-design.md SP-3・SP-6・S-1。
// 文言の期待値は仕様の字面をここへ書く（実装の定数を輸入しない）。要約は期間で絞った行を props で受ける。
// 赤（--danger）の当て先はクラスで読む（jsdom は CSS を当てない）。

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

const H_AVERAGE = "平均稼働（確定した月）";
const H_WITHIN = "精算幅に収まった月";
const H_RANGE = "精算幅";

/** 見出しを持つ Card の、見出し以外の文字（空白は畳む）。 */
function bodyOf(heading: string): string {
  const h2 = screen.getByRole("heading", { level: 2, name: heading });
  const card = h2.parentElement!;
  return (card.textContent ?? "").replace(h2.textContent ?? "", "").replace(/\s+/g, " ").trim();
}
function cardOf(heading: string): HTMLElement {
  return screen.getByRole("heading", { level: 2, name: heading }).parentElement as HTMLElement;
}

/** 要素自身かその子孫に、--danger を指すクラスが付いているか。 */
const DANGER = /danger/;
function hasDanger(root: Element): boolean {
  return [root, ...root.querySelectorAll("*")].some((el) => [...el.classList].some((c) => DANGER.test(c)));
}

// 4-1-d (a): 160時間00分（締め済）・161時間00分（承認済）→ 160.5。ほかに入力中の月と、精算幅が同じ月。
const AVERAGE_ROWS = [
  tr("2026-05", { state: "PendingApproval", totalHours: hm(160, 0) }),
  tr("2026-06", { state: "Approved", totalHours: hm(161, 0) }),
  draft("2026-07", { totalHours: hm(10, 0) }),
];

describe("Card の数と見出し（4-1-b・4-5）", () => {
  it("Card がちょうど3つで、見出しは 平均稼働（確定した月）・精算幅に収まった月・精算幅（この順）", () => {
    const { container } = render(<WorkTrendsSummary rows={AVERAGE_ROWS} />);
    const headings = [...container.querySelectorAll("h2")].map((h) => h.textContent);
    expect(headings).toEqual([H_AVERAGE, H_WITHIN, H_RANGE]);
  });
});

describe("平均（4-1-b・4-1-c・4-2）", () => {
  it("(a) の組 → 値の文字が「160.5 時間」の形（入力中の月は含めない）", () => {
    render(<WorkTrendsSummary rows={AVERAGE_ROWS} />);
    expect(bodyOf(H_AVERAGE)).toBe("160.5 時間");
  });

  it("4-1-d (f)・4-1-e: 159時間55分・160時間00分 → 「160.0 時間」（小数第1位の 0 を省かない）", () => {
    render(
      <WorkTrendsSummary
        rows={[tr("2026-05", { totalHours: hm(159, 55) }), tr("2026-06", { totalHours: hm(160, 0) })]}
      />,
    );
    expect(bodyOf(H_AVERAGE)).toBe("160.0 時間");
  });

  it("4-1-d (c): 160時間30分・160時間36分 → 「160.6 時間」", () => {
    render(
      <WorkTrendsSummary
        rows={[tr("2026-05", { totalHours: hm(160, 30) }), tr("2026-06", { totalHours: hm(160, 36) })]}
      />,
    );
    expect(bodyOf(H_AVERAGE)).toBe("160.6 時間");
  });

  it("4-2・8-3: 確定した月が0件（すべて入力中）→ 値の位置に「—」、単位「時間」なし、「確定した月はありません」を添え、0 を示さない", () => {
    render(<WorkTrendsSummary rows={[draft("2026-06", { totalHours: hm(100) }), draft("2026-07")]} />);
    const body = bodyOf(H_AVERAGE);
    expect(body).toContain("—");
    expect(body).toContain("確定した月はありません");
    expect(body).not.toContain("時間");
    expect(body).not.toMatch(/\d/);
  });

  it("陽性対照（4-2）: 値のある平均の Card は、同じ判定（「—」・添え書き・数字なし）に当てはまらない", () => {
    render(<WorkTrendsSummary rows={AVERAGE_ROWS} />);
    const body = bodyOf(H_AVERAGE);
    expect(body.includes("—")).toBe(false);
    expect(body.includes("確定した月はありません")).toBe(false);
    expect(body.includes("時間")).toBe(true);
    expect(/\d/.test(body)).toBe(true);
  });
});

describe("精算幅に収まった月（4-3・8-3）", () => {
  // 確定した月 11（収まった 9・超過 1・不足 1）と、入力中 1。
  const eleven: WorkTrendRow[] = [
    tr("2025-08"),
    tr("2025-09"),
    tr("2025-10", { state: "PendingApproval" }),
    tr("2025-11"),
    tr("2025-12", { excess: hm(0, 15) }),
    tr("2026-01"),
    tr("2026-02"),
    tr("2026-03", { shortfall: hm(2) }),
    tr("2026-04"),
    tr("2026-05"),
    tr("2026-06"),
    draft("2026-07"),
  ];

  it("「9 / 11 か月」の形（収まった月の数 / 確定した月の数。入力中は数えない）", () => {
    render(<WorkTrendsSummary rows={eleven} />);
    expect(bodyOf(H_WITHIN)).toBe("9 / 11 か月");
  });

  it("確定した月が0件 → 「0 / 0 か月」（8-3）", () => {
    render(<WorkTrendsSummary rows={[draft("2026-06"), draft("2026-07")]} />);
    expect(bodyOf(H_WITHIN)).toBe("0 / 0 か月");
  });

  it("陽性対照: 入力中を数えてしまう実装なら 12 になる。この組は 11 であって 12 ではない", () => {
    render(<WorkTrendsSummary rows={eleven} />);
    expect(bodyOf(H_WITHIN)).not.toContain("12");
    expect(eleven).toHaveLength(12);
  });
});

describe("精算幅（4-4・4-4-a・4-4-b）", () => {
  it("すべて同じ → 「140.0〜180.0 時間」", () => {
    render(<WorkTrendsSummary rows={AVERAGE_ROWS} />);
    expect(bodyOf(H_RANGE)).toBe("140.0〜180.0 時間");
  });

  it("分を持つ精算幅 → formatHours の形（140時間15分〜180時間30分 → 「140.25〜180.5 時間」）", () => {
    const range = { lowerBound: hm(140, 15), upperBound: hm(180, 30) };
    render(<WorkTrendsSummary rows={[tr("2026-06", { settlementRange: range })]} />);
    expect(bodyOf(H_RANGE)).toBe("140.25〜180.5 時間");
  });

  it("4-4-b: 月によって異なる → 「月によって異なります」（単位「時間」なし、いずれかの月の値に代表させない）", () => {
    const other = { lowerBound: hm(150), upperBound: hm(190) };
    render(<WorkTrendsSummary rows={[tr("2026-06"), tr("2026-07", { settlementRange: other })]} />);
    expect(bodyOf(H_RANGE)).toBe("月によって異なります");
  });

  it("4-4: 入力中の月だけ精算幅が違っても「月によって異なります」", () => {
    const other = { lowerBound: hm(150), upperBound: hm(190) };
    render(<WorkTrendsSummary rows={[tr("2026-06"), draft("2026-07", { settlementRange: other })]} />);
    expect(bodyOf(H_RANGE)).toBe("月によって異なります");
  });
});

describe("要約の色（4-7）", () => {
  it("超過・不足の月を含む期間でも、3つの Card のどこにも --danger のクラスが無い", () => {
    render(
      <WorkTrendsSummary
        rows={[
          tr("2026-04", { excess: hm(5, 15) }),
          tr("2026-05", { shortfall: hm(3) }),
          tr("2026-06", { state: "PendingApproval" }),
          draft("2026-07"),
        ]}
      />,
    );
    for (const h of [H_AVERAGE, H_WITHIN, H_RANGE]) {
      expect(hasDanger(cardOf(h))).toBe(false);
    }
  });

  it("確定した月が0件・精算幅が月で異なる場合も、3つの Card のどこにも --danger のクラスが無い", () => {
    const other = { lowerBound: hm(150), upperBound: hm(190) };
    render(<WorkTrendsSummary rows={[draft("2026-06"), draft("2026-07", { settlementRange: other })]} />);
    for (const h of [H_AVERAGE, H_WITHIN, H_RANGE]) {
      expect(hasDanger(cardOf(h))).toBe(false);
    }
  });

  it.each([
    ["text-danger", "text-danger"],
    ["bg-danger", "bg-danger"],
    ["border-danger", "border-danger"],
    ["子孫の text-danger", ""],
  ])("陽性対照（4-7）: %s を持つ Card は、同じ判定で検出される", (name, cls) => {
    const { container, unmount } = render(
      <div className={cls}>
        <h2>x</h2>
        {name.startsWith("子孫") ? <p className="mt-1 text-danger">1</p> : null}
      </div>,
    );
    expect(hasDanger(container.firstElementChild!)).toBe(true);
    unmount();
  });
});
