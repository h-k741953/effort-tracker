/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import type { ContractListItem } from "@/lib/contract-list";
import type { WorkTrendRow } from "@/lib/work-trends";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import { CONTRACTS_PATH, installFakeFetch, jsonResponse } from "@/test-support/work-month-list";
import type { ScreenDeps } from "../_lib/screen-deps";
import { workTrendsScreen } from "./work-trends-screen";

// docs/specs/work-trends-ui.md 7-1・7-3・7-4・7-4-a・7-5。設計 SP-1・SP-6・T-0-2・C-3。
// 本体へ手書きの Fake の ScreenDeps を渡し、next/navigation だけを vi.mock する。
// redirect は本物と同じく「投げて打ち切る」形。サイドバーも R-10 を呼ぶ。本文は R-10 と R-11 を呼ぶ。

const state = vi.hoisted(() => ({ redirects: [] as string[] }));

vi.mock("next/navigation", () => ({
  redirect: (to: string) => {
    state.redirects.push(to);
    throw new Error(`NEXT_REDIRECT:${to}`);
  },
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}));

const WORK_TRENDS_PATH = "/api/work-trends";

function fakeDeps(viewer: ResolveViewerResult, theme: Theme = "light"): ScreenDeps {
  return { getCurrentViewer: async () => viewer, getTheme: async () => theme };
}

const ENGINEER: ResolveViewerResult = { kind: "signed-in", role: "Engineer" };
const APPROVER: ResolveViewerResult = { kind: "signed-in", role: "Approver" };
const GUEST: ResolveViewerResult = { kind: "guest" };

const RANGE = { lowerBound: { hours: 140, minutes: 0 }, upperBound: { hours: 180, minutes: 0 } };
const ITEM: ContractListItem = {
  contractId: "c-1",
  contractDisplayName: "A社 / 保守",
  settlementRange: RANGE,
  contractPeriod: { start: "2025-04", end: null },
  workMonth: { yearMonth: "2026-11", state: "Draft", generated: false },
};
const TREND: WorkTrendRow = {
  contractId: "c-1",
  yearMonth: "2026-10",
  state: "Approved",
  settlementRange: RANGE,
  totalHours: { hours: 160, minutes: 0 },
  excess: { hours: 0, minutes: 0 },
  shortfall: { hours: 0, minutes: 0 },
};

function backend() {
  return installFakeFetch({
    [CONTRACTS_PATH]: () => jsonResponse({ items: [ITEM] }),
    [WORK_TRENDS_PATH]: () => jsonResponse({ items: [TREND] }),
  });
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("区分による出し分け（7-1・7-3・7-4・7-4-a・7-5）", () => {
  it("技術者: redirect せず、ヘッダー・サイドバー・稼働の推移の見出しと図を描く", async () => {
    backend();
    render(await workTrendsScreen(fakeDeps(ENGINEER)));
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1, name: "稼働の推移" })).toBeTruthy();
    expect(await screen.findByRole("img", { name: "月ごとの総稼働時間" })).toBeTruthy();
  });

  it("7-1: サイドバーで aria-current を持つ項目は 稼働の推移 だけ", async () => {
    backend();
    render(await workTrendsScreen(fakeDeps(ENGINEER)));
    await screen.findByRole("img", { name: "月ごとの総稼働時間" });
    const nav = screen
      .getAllByRole("navigation")
      .find((n) => within(n).queryByRole("link", { name: "稼働の推移" }));
    expect(nav).toBeDefined();
    const current = [...nav!.querySelectorAll("[aria-current]")].map((el) => el.textContent);
    expect(current).toEqual(["稼働の推移"]);
  });

  it("7-3・7-4-a: 承認者は承認待ち一覧へ redirect し、推移を描かず、R-10・R-11 も呼ばない", async () => {
    const fake = backend();
    await expect(workTrendsScreen(fakeDeps(APPROVER))).rejects.toThrow("NEXT_REDIRECT:/pending-approvals");
    expect(state.redirects).toEqual(["/pending-approvals"]);
    expect(screen.queryByRole("img", { name: "月ごとの総稼働時間" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "稼働の推移" })).toBeNull();
    expect(fake.urls).toHaveLength(0);
  });

  it("7-3・7-4-a: ゲストはデモ勤務月群へ redirect し、推移を描かず、R-10・R-11 も呼ばない", async () => {
    const fake = backend();
    await expect(workTrendsScreen(fakeDeps(GUEST))).rejects.toThrow("NEXT_REDIRECT:/demo");
    expect(state.redirects).toEqual(["/demo"]);
    expect(screen.queryByRole("img", { name: "月ごとの総稼働時間" })).toBeNull();
    expect(fake.urls).toHaveLength(0);
  });

  it("陽性対照: 技術者の描画は、同じ判定（図の有無・取得の数）で、描いたこと・取得したことが検出される", async () => {
    const fake = backend();
    render(await workTrendsScreen(fakeDeps(ENGINEER)));
    await screen.findByRole("img", { name: "月ごとの総稼働時間" });
    expect(fake.urls.length).toBeGreaterThan(0);
    expect(fake.count(WORK_TRENDS_PATH)).toBe(1);
  });

  it("C-3: misconfigured は例外を投げ、redirect も描画もしない（redirect の打ち切りではない）", async () => {
    const fake = backend();
    await expect(workTrendsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.toThrow();
    await expect(workTrendsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.not.toThrow(/NEXT_REDIRECT/);
    expect(state.redirects).toEqual([]);
    expect(screen.queryByRole("img", { name: "月ごとの総稼働時間" })).toBeNull();
    expect(fake.urls).toHaveLength(0);
  });
});
