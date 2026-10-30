/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, fireEvent, within } from "@testing-library/react";
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

// docs/specs/design-system.md AC-6-13-v（検証手段は AC-10-6-q。2026-10-29 改定）。
//
// 選択肢を開く項目（options を持ち href を持たない）。href を持つ項目と options（2つ以上）を持つ項目を
// 混ぜ、footerItems を1つ以上与えた描画で、閉じた並び・button の名・aria-expanded・押した後の選択肢の
// link の数・順・名・href・現れる位置・再び押して閉じることを読む。名と href は与えた値から取る。

const OPTIONS = [
  { label: "A社 / 保守", href: "/this-month?contract=c-1" },
  { label: "B社 / 開発", href: "/this-month?contract=c-2" },
];

function mixedItems(optionsCurrent = false) {
  return [
    { label: "今月の入力", options: OPTIONS, current: optionsCurrent },
    { label: "勤務月", href: "/work-months", current: false },
    { label: "契約", href: "/contracts", current: false },
  ];
}

/** nav の中の link と button を文書の順に並べ、アクセシブル名の列にする。 */
function orderedControls(nav: HTMLElement): string[] {
  return [...nav.querySelectorAll("a, button")].map((el) => (el.textContent ?? "").trim());
}

describe("SideNav - AC-6-13-v（選択肢を開く項目）", () => {
  it("閉じた状態の並び: href を持つ項目の link と options を持つ項目の button が items、footerItems の順に与えた順で並ぶ", () => {
    render(<SideNav items={mixedItems()} footerItems={FOOTER_ITEMS} />);
    const nav = screen.getByRole("navigation");
    expect(orderedControls(nav)).toEqual(["今月の入力", "勤務月", "契約", "ヘルプ"]);
    expect(screen.getAllByRole("link").map((l) => l.textContent)).toEqual(["勤務月", "契約", "ヘルプ"]);
  });

  it("options を持つ項目は button のロールで、名が label と一致し、link のロールで問い合わせられない", () => {
    render(<SideNav items={mixedItems()} footerItems={FOOTER_ITEMS} />);
    expect(screen.getByRole("button", { name: "今月の入力" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
  });

  it("描画した直後は閉じている: aria-expanded が \"false\" で、options のリンクが文書に無い", () => {
    render(<SideNav items={mixedItems()} footerItems={FOOTER_ITEMS} />);
    expect(screen.getByRole("button", { name: "今月の入力" }).getAttribute("aria-expanded")).toBe("false");
    for (const option of OPTIONS) {
      expect(screen.queryByRole("link", { name: option.label })).toBeNull();
    }
  });

  it("押すと開く: aria-expanded が \"true\" になり、options と同じ数・順・名・href の link が、その button より後・次の項目より前に現れる", () => {
    render(<SideNav items={mixedItems()} footerItems={FOOTER_ITEMS} />);
    const button = screen.getByRole("button", { name: "今月の入力" });
    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");

    const nav = screen.getByRole("navigation");
    expect(orderedControls(nav)).toEqual([
      "今月の入力",
      ...OPTIONS.map((o) => o.label),
      "勤務月",
      "契約",
      "ヘルプ",
    ]);
    for (const option of OPTIONS) {
      expect(within(nav).getByRole("link", { name: option.label }).getAttribute("href")).toBe(option.href);
    }
  });

  it("開いた状態で再び押すと閉じる: aria-expanded が \"false\" に戻り、選択肢のリンクが文書から消える", () => {
    render(<SideNav items={mixedItems()} footerItems={FOOTER_ITEMS} />);
    const button = screen.getByRole("button", { name: "今月の入力" });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("false");
    for (const option of OPTIONS) {
      expect(screen.queryByRole("link", { name: option.label })).toBeNull();
    }
  });

  it("6-13-iii: options を持つ項目の current が真のとき、その button に aria-current=\"page\" がある", () => {
    render(<SideNav items={mixedItems(true)} footerItems={FOOTER_ITEMS} />);
    expect(screen.getByRole("button", { name: "今月の入力" }).getAttribute("aria-current")).toBe("page");
    for (const link of screen.getAllByRole("link")) {
      expect(link.hasAttribute("aria-current")).toBe(false);
    }
  });

  it("6-13-iii: current が偽のとき、その button に aria-current が付かない", () => {
    render(<SideNav items={mixedItems(false)} footerItems={FOOTER_ITEMS} />);
    expect(screen.getByRole("button", { name: "今月の入力" }).hasAttribute("aria-current")).toBe(false);
  });

  it("開閉の操作で、部品は通信しない（fetch が呼ばれない）", () => {
    const calls: unknown[] = [];
    vi.stubGlobal("fetch", (...args: unknown[]) => {
      calls.push(args);
      return Promise.reject(new Error("fetch must not be called"));
    });
    try {
      render(<SideNav items={mixedItems()} footerItems={FOOTER_ITEMS} />);
      const button = screen.getByRole("button", { name: "今月の入力" });
      fireEvent.click(button);
      fireEvent.click(button);
      expect(calls).toHaveLength(0);
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
