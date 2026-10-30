import { describe, expect, it } from "vitest";
import {
  decideLoginScreen,
  decideScreen,
  defaultScreenPath,
  type ScreenId,
} from "./screen-access";
import type { Viewer } from "./viewer";

// docs/specs/login-ui.md AC-6-7・6-7-a・6-8・6-8-a・6-9・6-9-a・6-9-b・6-10・6-10-a。
// 設計: docs/specs/web-screens-implementation-design.md C-2・C-4（T 節）。
//
// 開ける画面の集合は 6-7 の字面から取る。技術者: 本人の勤務月一覧・契約一覧・稼働の推移・
// ヘルプ・勤務月画面。承認者: 承認待ち一覧・ヘルプ・勤務月画面。ゲスト: デモ勤務月群・
// ヘルプ・勤務月画面。ここに無い画面は「開けない画面」で、既定の画面（6-7-a）へ移す。
// 既定の画面のパスは設計 C-2 の表（AI 起案・人間の承認前）に従う。

const ENGINEER: Viewer = { kind: "signed-in", role: "Engineer" };
const APPROVER: Viewer = { kind: "signed-in", role: "Approver" };
const GUEST: Viewer = { kind: "guest" };

const VIEWERS = [
  ["技術者", ENGINEER, "/work-months"],
  ["承認者", APPROVER, "/pending-approvals"],
  ["ゲスト", GUEST, "/demo"],
] as const;

const SCREENS: readonly ScreenId[] = [
  "own-work-months",
  "pending-approvals",
  "demo-work-months",
  "work-month",
  "help",
  "contracts",
  "work-trends",
];

// 区分ごとに開ける画面（6-7）。
const OPENABLE: Record<string, readonly ScreenId[]> = {
  技術者: ["own-work-months", "contracts", "work-trends", "help", "work-month"],
  承認者: ["pending-approvals", "help", "work-month"],
  ゲスト: ["demo-work-months", "help", "work-month"],
};

describe("6-7-a: 既定の画面", () => {
  it.each(VIEWERS)("%s の既定の画面は %s", (_label, viewer, path) => {
    expect(defaultScreenPath(viewer)).toBe(path);
  });
});

describe("6-7・6-8・6-8-a・6-9・6-9-a: ScreenId 7種 × 区分3種", () => {
  for (const [label, viewer, defaultPath] of VIEWERS) {
    for (const screen of SCREENS) {
      const opens = OPENABLE[label].includes(screen);
      it(`${label} が ${screen} を開く → ${opens ? "show" : `redirect ${defaultPath}`}`, () => {
        expect(decideScreen(screen, viewer)).toEqual(
          opens ? { kind: "show" } : { kind: "redirect", to: defaultPath },
        );
      });
    }
  }
});

describe("6-9-a: 勤務月画面は全区分で移さない", () => {
  it.each(VIEWERS)("%s", (_label, viewer) => {
    expect(decideScreen("work-month", viewer)).toEqual({ kind: "show" });
  });
});

describe("6-10・6-10-a: ログイン画面", () => {
  it("6-10: 技術者は本人の勤務月一覧へ移す", () => {
    expect(decideLoginScreen(ENGINEER)).toEqual({ kind: "redirect", to: "/work-months" });
  });
  it("6-10: 承認者は承認待ち一覧へ移す", () => {
    expect(decideLoginScreen(APPROVER)).toEqual({ kind: "redirect", to: "/pending-approvals" });
  });
  it("6-10-a: ゲストは移さない（表示する）", () => {
    expect(decideLoginScreen(GUEST)).toEqual({ kind: "show" });
  });
});

describe("6-9-b: 判定は純粋で、同じ入力に同じ結果を返す", () => {
  it("繰り返し呼んでも結果が変わらず、入力の viewer を書き換えない", () => {
    const viewer: Viewer = { kind: "signed-in", role: "Approver" };
    const before = JSON.stringify(viewer);
    const first = decideScreen("contracts", viewer);
    const second = decideScreen("contracts", viewer);
    expect(second).toEqual(first);
    expect(JSON.stringify(viewer)).toBe(before);
  });
});
