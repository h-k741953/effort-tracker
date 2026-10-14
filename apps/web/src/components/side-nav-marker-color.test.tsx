/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { render, cleanup } from "@testing-library/react";
import { SideNav } from "./side-nav";

// docs/specs/design-system.md AC-6-13-iv（検証手段は AC-10-3-p。限界は 11-52）。
//
// SideNav の項目の丸印（link の内側の aria-hidden="true" の要素。ちょうど1つ）の class に、
// current が真なら bg-primary、偽なら bg-et-sidebar-muted が、バリアントを伴わない1語の全体として
// 現れ、取り違えた側の語は現れないことを読む。ホバー時の色・大きさ・枠線の有無は読まない（11-52）。
// 項目の真偽は描画に与えた値から取り、aria-current から逆算しない（10-6-q と独立に落とす）。
// DOM へ描画して class を読むため、10-9 に従いドックブロックを置く。

afterEach(() => {
  cleanup();
});

/** 丸印の class を空白で語に分け、1語の全体が token と一致する語の数を返す（hover: 等を前置した語は数えない）。 */
function countWholeWord(classAttr: string, token: string): number {
  return classAttr.split(/\s+/).filter((w) => w === token).length;
}

/** 10-3-p (ii) の述語。current が真なら bg-primary が在り bg-et-sidebar-muted が無い／偽ならその逆。 */
function markerColorConforms(classAttr: string, current: boolean): boolean {
  const primary = countWholeWord(classAttr, "bg-primary");
  const muted = countWholeWord(classAttr, "bg-et-sidebar-muted");
  return current ? primary > 0 && muted === 0 : muted > 0 && primary === 0;
}

describe("SideNav の丸印の色 - AC-6-13-iv（AC-10-3-p）", () => {
  describe("述語の両側（10-3-g と同じ形）", () => {
    it.each([
      ["size-2 rounded-full bg-primary", true, true],
      ["size-2 rounded-full border border-current", true, false],
      ["bg-et-sidebar-muted", true, false],
      ["bg-primary bg-et-sidebar-muted", true, false],
      ["hover:bg-primary", true, false],
      ["bg-current", true, false],
      ["bg-et-sidebar-muted", false, true],
      ["bg-primary", false, false],
    ])("class=%j current=%s -> 適合=%s", (classAttr, current, expected) => {
      expect(markerColorConforms(classAttr, current)).toBe(expected);
    });
  });

  const ITEMS = [
    { label: "勤務月", href: "/work-months" },
    { label: "契約", href: "/contracts" },
    { label: "稼働の推移", href: "/trends" },
  ];
  const FOOTER = [{ label: "ヘルプ", href: "/help" }];

  // 真にする items の添字（null はすべて偽）。
  it.each([
    ["items の1つだけ真", 1],
    ["すべて偽", null],
  ])("実描画: %s", (_name, currentIndex) => {
    const items = ITEMS.map((i, idx) => ({ ...i, current: idx === currentIndex }));
    const footerItems = FOOTER.map((i) => ({ ...i, current: false }));
    const expected = [...items, ...footerItems];
    const { container } = render(<SideNav items={items} footerItems={footerItems} />);
    const links = [...container.querySelectorAll("a")];

    // (iii) 読んだ link の数が和と一致（0件は失敗）。
    expect(links.length).toBeGreaterThan(0);
    expect(links).toHaveLength(items.length + footerItems.length);

    links.forEach((link, idx) => {
      const current = expected[idx].current; // 与えた値から取る
      // (i) aria-hidden="true" の子孫がちょうど1つ。
      const markers = link.querySelectorAll('[aria-hidden="true"]');
      expect(markers, expected[idx].label).toHaveLength(1);
      const classAttr = markers[0].getAttribute("class") ?? "";
      // (ii)
      expect(markerColorConforms(classAttr, current), `${expected[idx].label}: ${classAttr}`).toBe(true);
    });
  });

  it("すべて偽で与えたとき、bg-primary を持つ丸印が無い", () => {
    const { container } = render(
      <SideNav items={ITEMS.map((i) => ({ ...i, current: false }))} footerItems={FOOTER.map((i) => ({ ...i, current: false }))} />,
    );
    const markers = [...container.querySelectorAll('a [aria-hidden="true"]')];
    expect(markers.length).toBeGreaterThan(0);
    expect(markers.filter((m) => countWholeWord(m.getAttribute("class") ?? "", "bg-primary") > 0)).toHaveLength(0);
  });
});
