import { describe, expect, it } from "vitest";
import { sideNavItems } from "./side-nav-items";
import { thisMonthPath } from "./screen-routes";
import type { Viewer } from "./viewer";

// docs/specs/work-month-listing-ui.md AC-10-2・10-3・10-4・10-4-a・10-5・10-6・10-9-a〜10-9-d。設計 W-6・T-2。
// 項目の文言・並び・href は 10-2〜10-6 と C-2 の表。href は C-2 の6つ（ヘルプ・契約一覧・稼働の推移を含む）。

const ENGINEER: Viewer = { kind: "signed-in", role: "Engineer" };
const APPROVER: Viewer = { kind: "signed-in", role: "Approver" };
const GUEST: Viewer = { kind: "guest" };

type CurrentKey =
  | "this-month"
  | "own-work-months"
  | "contracts"
  | "work-trends"
  | "pending-approvals"
  | "demo-work-months"
  | "help";

const HIDDEN = { kind: "hidden" } as const;
const LINK = { kind: "link", href: thisMonthPath("c-001") } as const;
const CHOICE = {
  kind: "choice",
  options: [
    { label: "A社 / 保守", href: thisMonthPath("c-001") },
    { label: "B社 / 開発", href: thisMonthPath("c-002") },
  ],
} as const;

const HELP = { label: "ヘルプ", href: "/help" };

function labelsAndHrefs(items: ReadonlyArray<{ label: string; href?: string }>) {
  return items.map((i) => ({ label: i.label, href: i.href }));
}

describe("項目の集合・文言・並び（10-2〜10-4・10-6）", () => {
  it("10-2: 技術者（今月の入力が link）→ 今月の入力・自分の勤務月・契約一覧・稼働の推移、最下部にヘルプ", () => {
    const { items, footerItems } = sideNavItems({ viewer: ENGINEER, current: "own-work-months", thisMonth: LINK });
    expect(labelsAndHrefs(items)).toEqual([
      { label: "今月の入力", href: LINK.href },
      { label: "自分の勤務月", href: "/work-months" },
      { label: "契約一覧", href: "/contracts" },
      { label: "稼働の推移", href: "/work-trends" },
    ]);
    expect(labelsAndHrefs(footerItems)).toEqual([HELP]);
  });

  it("10-3: 承認者 → 承認待ちだけ、最下部にヘルプ", () => {
    const { items, footerItems } = sideNavItems({ viewer: APPROVER, current: "pending-approvals", thisMonth: HIDDEN });
    expect(labelsAndHrefs(items)).toEqual([{ label: "承認待ち", href: "/pending-approvals" }]);
    expect(labelsAndHrefs(footerItems)).toEqual([HELP]);
  });

  it("10-4・10-4-a: ゲスト → サンプル勤務月だけ（承認担当者一覧の項目を出さない）、最下部にヘルプ", () => {
    const { items, footerItems } = sideNavItems({ viewer: GUEST, current: "demo-work-months", thisMonth: HIDDEN });
    expect(labelsAndHrefs(items)).toEqual([{ label: "サンプル勤務月", href: "/demo" }]);
    expect(labelsAndHrefs(footerItems)).toEqual([HELP]);
  });

  it.each([
    ["承認者", APPROVER, "pending-approvals"],
    ["ゲスト", GUEST, "demo-work-months"],
  ] as const)("10-3・10-4: %s には thisMonth が link・choice でも「今月の入力」を出さない", (_n, viewer, current) => {
    for (const thisMonth of [LINK, CHOICE]) {
      const { items, footerItems } = sideNavItems({ viewer, current, thisMonth });
      expect([...items, ...footerItems].map((i) => i.label)).not.toContain("今月の入力");
    }
  });
});

describe("今月の入力の形（10-9-a〜10-9-d）", () => {
  it("10-9-d: thisMonth が hidden → 技術者でも「今月の入力」が無い", () => {
    const { items } = sideNavItems({ viewer: ENGINEER, current: "own-work-months", thisMonth: HIDDEN });
    expect(items.map((i) => i.label)).toEqual(["自分の勤務月", "契約一覧", "稼働の推移"]);
  });

  it("10-9-a: link → href を持つ項目（options を持たない）", () => {
    const { items } = sideNavItems({ viewer: ENGINEER, current: "own-work-months", thisMonth: LINK });
    const entry = items.find((i) => i.label === "今月の入力");
    expect(entry).toBeDefined();
    expect(entry).toHaveProperty("href", LINK.href);
    expect(entry).not.toHaveProperty("options");
  });

  it("10-9-b・10-9-c: choice → options を持つ項目（href を持たない。options は入力のまま）", () => {
    const { items } = sideNavItems({ viewer: ENGINEER, current: "own-work-months", thisMonth: CHOICE });
    const entry = items.find((i) => i.label === "今月の入力");
    expect(entry).toBeDefined();
    expect(entry).not.toHaveProperty("href");
    expect(entry).toHaveProperty("options", CHOICE.options);
    // 並びは link のときと同じ（先頭）。
    expect(items[0].label).toBe("今月の入力");
  });
});

describe("選択中（10-5。一覧の3画面）— current がちょうど1つ真", () => {
  it.each<[string, Viewer, CurrentKey, string]>([
    ["技術者 × own-work-months", ENGINEER, "own-work-months", "自分の勤務月"],
    ["技術者 × this-month", ENGINEER, "this-month", "今月の入力"],
    ["技術者 × contracts", ENGINEER, "contracts", "契約一覧"],
    ["技術者 × work-trends", ENGINEER, "work-trends", "稼働の推移"],
    ["技術者 × help", ENGINEER, "help", "ヘルプ"],
    ["承認者 × pending-approvals", APPROVER, "pending-approvals", "承認待ち"],
    ["承認者 × help", APPROVER, "help", "ヘルプ"],
    ["ゲスト × demo-work-months", GUEST, "demo-work-months", "サンプル勤務月"],
    ["ゲスト × help", GUEST, "help", "ヘルプ"],
  ])("%s", (_name, viewer, current, expectedLabel) => {
    for (const thisMonth of viewer.kind === "signed-in" && viewer.role === "Engineer" ? [LINK, CHOICE] : [HIDDEN]) {
      const { items, footerItems } = sideNavItems({ viewer, current, thisMonth });
      const selected = [...items, ...footerItems].filter((i) => i.current);
      expect(selected.map((i) => i.label)).toEqual([expectedLabel]);
    }
  });
});
