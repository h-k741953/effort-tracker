/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { SideNav } from "./side-nav";

// docs/specs/design-system.md AC-6-13（6-13-i〜6-13-iii。検証手段は AC-10-6・10-6-q）。
//
// items を2つ以上、footerItems を1つ以上与えた描画で 6-13-ii の数・順・名・href を読み、current を
// 1つだけ真にした描画と、すべて偽にした描画で 6-13-iii を読む。名の文字列は期待値に持たない
// （6-13-i）。項目の文言と href は、テストが与えた値から取る（どの項目を出すかは部品の外。7-2）。

afterEach(() => {
  cleanup();
});

const ITEMS = [
  { label: "勤務月", href: "/work-months", current: false },
  { label: "契約", href: "/contracts", current: true },
  { label: "稼働の推移", href: "/trends", current: false },
];
const FOOTER_ITEMS = [{ label: "ヘルプ", href: "/help", current: false }];

describe("SideNav - AC-6-13", () => {
  it("6-13-i: navigation のロールと空でないアクセシブル名で問い合わせられる（名の文字列は期待値に持たない）", () => {
    render(<SideNav items={ITEMS} footerItems={FOOTER_ITEMS} />);
    const navs = screen.getAllByRole("navigation");
    expect(navs).toHaveLength(1);
    expect(screen.getByRole("navigation", { name: /\S/ })).toBe(navs[0]);
    expect(navs[0].tagName).toBe("NAV");
  });

  it("6-13-ii: link の要素が items と footerItems の数の和だけあり、文書の順で items、footerItems の順に並ぶ", () => {
    render(<SideNav items={ITEMS} footerItems={FOOTER_ITEMS} />);
    const links = screen.getAllByRole("link");
    expect(links).toHaveLength(ITEMS.length + FOOTER_ITEMS.length);
    expect(links.map((l) => [l.textContent, l.getAttribute("href")])).toEqual(
      [...ITEMS, ...FOOTER_ITEMS].map((i) => [i.label, i.href]),
    );
  });

  it("6-13-ii: 各項目のアクセシブル名が label である", () => {
    render(<SideNav items={ITEMS} footerItems={FOOTER_ITEMS} />);
    for (const item of [...ITEMS, ...FOOTER_ITEMS]) {
      expect(screen.getByRole("link", { name: item.label }).getAttribute("href")).toBe(item.href);
    }
  });

  it("6-13-ii: footerItems を省略すると items の数だけになる（既定の項目を内蔵しない）", () => {
    render(<SideNav items={ITEMS} />);
    expect(screen.getAllByRole("link")).toHaveLength(ITEMS.length);
  });

  it("6-13-ii: items が空でも footerItems だけが出る（項目を内蔵しない）", () => {
    render(<SideNav items={[]} footerItems={FOOTER_ITEMS} />);
    expect(screen.getAllByRole("link").map((l) => l.textContent)).toEqual(FOOTER_ITEMS.map((i) => i.label));
  });

  it("6-13-iii: current が真の項目にだけ aria-current=\"page\" がある", () => {
    render(<SideNav items={ITEMS} footerItems={FOOTER_ITEMS} />);
    const links = screen.getAllByRole("link");
    const current = links.filter((l) => l.getAttribute("aria-current") === "page");
    expect(current.map((l) => l.textContent)).toEqual(["契約"]);
    // 偽の項目には aria-current が付かない（"false" の値も付けない）。
    for (const link of links.filter((l) => l.textContent !== "契約")) {
      expect(link.hasAttribute("aria-current")).toBe(false);
    }
  });

  it("6-13-iii: footerItems の項目が current のときも aria-current=\"page\" が付く", () => {
    render(<SideNav items={ITEMS.map((i) => ({ ...i, current: false }))} footerItems={[{ ...FOOTER_ITEMS[0], current: true }]} />);
    expect(screen.getByRole("link", { name: "ヘルプ" }).getAttribute("aria-current")).toBe("page");
  });

  it("6-13-iii: すべて偽で与えたとき、aria-current を持つ項目が無い", () => {
    const { container } = render(
      <SideNav items={ITEMS.map((i) => ({ ...i, current: false }))} footerItems={FOOTER_ITEMS} />,
    );
    expect(container.querySelectorAll("[aria-current]")).toHaveLength(0);
  });

  // AC-7-2: ロール・認証状態・勤務月の状態を受け取らない（どの項目を出すかを判定しない）。
  it("型: 表に無い props を受け取らない（AC-6 の網羅）", () => {
    // @ts-expect-error role は公開 props に無い（AC-7-2）
    const withRole = <SideNav items={[]} role="Engineer" />;
    // @ts-expect-error 項目は label / href / current の3つで構成される（current を省略できない）
    const missingCurrent = <SideNav items={[{ label: "a", href: "/a" }]} />;
    void withRole;
    void missingCurrent;
  });
});
