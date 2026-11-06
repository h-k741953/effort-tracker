/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import { CONTRACTS_PATH, CONTRACT_A, contractsBody, installFakeFetch, jsonResponse } from "@/test-support/work-month-list";
import type { ScreenDeps } from "../_lib/screen-deps";
import { helpScreen } from "./help-screen";

// docs/specs/help-ui.md AC-1（1-1〜1-5）・5-2（画面全体）。設計 H-1・H-3・H-4、T-0-1・T-0-2、C-3。
// next/navigation だけを vi.mock し、ScreenDeps は手書きの Fake を渡す。fetch は手書きの Fake。
// サイドバーの契約取得（bff-wiring R-10）は本文の取得ではない（H-3）ため、本文とサイドバーを分けて読む。

const state = vi.hoisted(() => ({ redirects: [] as string[] }));

vi.mock("next/navigation", () => ({
  redirect: (to: string) => {
    state.redirects.push(to);
    throw new Error(`NEXT_REDIRECT:${to}`);
  },
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}));

function fakeDeps(viewer: ResolveViewerResult, theme: Theme = "light"): ScreenDeps {
  return { getCurrentViewer: async () => viewer, getTheme: async () => theme };
}

const ENGINEER: ResolveViewerResult = { kind: "signed-in", role: "Engineer" };
const APPROVER: ResolveViewerResult = { kind: "signed-in", role: "Approver" };
const GUEST: ResolveViewerResult = { kind: "guest" };

function backend() {
  return installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse(contractsBody([CONTRACT_A])) });
}

/** 技術者のサイドバーの契約取得（非同期）が済むまで待つ。他の区分では何も待たない。 */
async function settle(viewer: ResolveViewerResult, fake: ReturnType<typeof backend>) {
  if (viewer.kind === "signed-in" && viewer.role === "Engineer") {
    await waitFor(() => expect(fake.count(CONTRACTS_PATH)).toBe(1));
  }
  await new Promise((resolve) => setTimeout(resolve, 0));
}

function sidebar() {
  const nav = screen.getAllByRole("navigation").find((n) => within(n).queryByRole("link", { name: "ヘルプ" }));
  expect(nav).toBeDefined();
  return nav!;
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const VIEWERS: Array<[string, ResolveViewerResult]> = [
  ["技術者", ENGINEER],
  ["承認者", APPROVER],
  ["ゲスト", GUEST],
];

describe("区分によらず開ける（1-1・1-4・1-5）", () => {
  it.each(VIEWERS)("%s: redirect せず、ヘッダー・サイドバー・本文（h1 ヘルプ）を描く", async (_name, viewer) => {
    const fake = backend();
    render(await helpScreen(fakeDeps(viewer)));
    await settle(viewer, fake);
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getAllByRole("main")).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1, name: "ヘルプ" })).toBeTruthy();
  });

  it("1-1: <main> の中身は3区分で同じ（ロールによって本文を変えない）", async () => {
    const mains: string[] = [];
    for (const [, viewer] of VIEWERS) {
      const fake = backend();
      render(await helpScreen(fakeDeps(viewer)));
      await settle(viewer, fake);
      mains.push(screen.getByRole("main").innerHTML);
      cleanup();
      vi.unstubAllGlobals();
    }
    // 陽性対照: 空の同士の一致ではない（本文が実際に描かれている）。
    expect(mains[0]).toContain("ヘルプ");
    expect(mains[0].length).toBeGreaterThan(200);
    expect(mains[1]).toBe(mains[0]);
    expect(mains[2]).toBe(mains[0]);
  });

  it("misconfigured: 例外を投げ、redirect も描画もしない（C-3）", async () => {
    backend();
    await expect(helpScreen(fakeDeps({ kind: "misconfigured" }))).rejects.toThrow();
    await expect(helpScreen(fakeDeps({ kind: "misconfigured" }))).rejects.not.toThrow(/NEXT_REDIRECT/);
    expect(state.redirects).toEqual([]);
  });
});

describe("サイドバー（1-2・work-month-listing-ui 10-5）", () => {
  it.each(VIEWERS)("%s: aria-current を持つ項目は「ヘルプ」だけ", async (_name, viewer) => {
    const fake = backend();
    render(await helpScreen(fakeDeps(viewer)));
    await settle(viewer, fake);
    const nav = sidebar();
    // 陽性対照: 走査の対象に項目が複数ある（ヘルプ以外の項目もある）うえで、選択中がヘルプだけ。
    if (viewer.kind !== "guest") {
      expect(within(nav).getAllByRole("link").length).toBeGreaterThan(1);
    }
    expect([...nav.querySelectorAll("[aria-current]")].map((el) => el.textContent)).toEqual(["ヘルプ"]);
  });
});

describe("ヘッダーのログインへのリンク（1-3・login-ui 9-6）", () => {
  it("ゲスト: 名前「ログイン」・行き先 / のリンクが出る", async () => {
    const fake = backend();
    render(await helpScreen(fakeDeps(GUEST)));
    await settle(GUEST, fake);
    const links = within(screen.getByRole("banner")).getAllByRole("link", { name: "ログイン" });
    expect(links).toHaveLength(1);
    expect(links[0].getAttribute("href")).toBe("/");
  });

  it.each([
    ["技術者", ENGINEER],
    ["承認者", APPROVER],
  ] as Array<[string, ResolveViewerResult]>)("%s: 出ない", async (_name, viewer) => {
    const fake = backend();
    render(await helpScreen(fakeDeps(viewer)));
    await settle(viewer, fake);
    expect(within(screen.getByRole("banner")).queryAllByRole("link", { name: "ログイン" })).toHaveLength(0);
  });
});

describe("データの取得（5-2・H-3）", () => {
  it("ゲスト: fetch は0回", async () => {
    const fake = backend();
    render(await helpScreen(fakeDeps(GUEST)));
    await settle(GUEST, fake);
    expect(fake.urls).toHaveLength(0);
  });

  it("承認者: fetch は0回", async () => {
    const fake = backend();
    render(await helpScreen(fakeDeps(APPROVER)));
    await settle(APPROVER, fake);
    expect(fake.urls).toHaveLength(0);
  });

  it("技術者: 契約の取得（サイドバー。R-10）だけで、それ以外の経路を呼ばない", async () => {
    const fake = backend();
    render(await helpScreen(fakeDeps(ENGINEER)));
    await settle(ENGINEER, fake);
    // 陽性対照: 契約の取得は実際に1回起きている（0件の「他に無い」ではない）。
    expect(fake.count(CONTRACTS_PATH)).toBe(1);
    expect(fake.urls.map((u) => u.pathname)).toEqual([CONTRACTS_PATH]);
  });
});
