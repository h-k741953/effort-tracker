/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import type { ContractListItem } from "@/lib/contract-list";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import { CONTRACTS_PATH, installFakeFetch, jsonResponse } from "@/test-support/work-month-list";
import type { ScreenDeps } from "../_lib/screen-deps";
import { contractsScreen } from "./contracts-screen";

// docs/specs/contracts-ui.md 5-1・5-3・5-4・5-5。設計 KP-1・KP-4・T-0-2・C-3。
// 本体へ手書きの Fake の ScreenDeps を渡し、next/navigation だけを vi.mock する。
// redirect は本物と同じく「投げて打ち切る」形。サイドバーも R-10 を呼ぶ（KP-5 (c)）。

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

const ITEM: ContractListItem = {
  contractId: "c-1",
  contractDisplayName: "A社 / 保守",
  settlementRange: { lowerBound: { hours: 140, minutes: 0 }, upperBound: { hours: 180, minutes: 0 } },
  contractPeriod: { start: "2025-04", end: null },
  workMonth: { yearMonth: "2026-11", state: "Draft", generated: false },
};

function backend() {
  return installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse({ items: [ITEM] }) });
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("区分による出し分け（5-1・5-3・5-4・5-5）", () => {
  it("技術者: redirect せず、ヘッダー・サイドバー・契約一覧の表を描く", async () => {
    backend();
    render(await contractsScreen(fakeDeps(ENGINEER)));
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1, name: "契約一覧" })).toBeTruthy();
    const table = await screen.findByRole("table");
    expect(within(table).getByText("A社 / 保守")).toBeTruthy();
  });

  it("5-1: サイドバーで aria-current を持つ項目は 契約一覧 だけ", async () => {
    backend();
    render(await contractsScreen(fakeDeps(ENGINEER)));
    await screen.findByRole("table");
    const nav = screen.getAllByRole("navigation").find((n) => within(n).queryByRole("link", { name: "契約一覧" }));
    expect(nav).toBeDefined();
    const current = [...nav!.querySelectorAll("[aria-current]")].map((el) => el.textContent);
    expect(current).toEqual(["契約一覧"]);
  });

  it("5-3・5-4: 承認者は承認待ち一覧へ redirect し、表を描かず、R-10 も呼ばない", async () => {
    const fake = backend();
    await expect(contractsScreen(fakeDeps(APPROVER))).rejects.toThrow("NEXT_REDIRECT:/pending-approvals");
    expect(state.redirects).toEqual(["/pending-approvals"]);
    expect(screen.queryByRole("table")).toBeNull();
    expect(fake.urls).toHaveLength(0);
  });

  it("5-3: ゲストはデモ勤務月群へ redirect し、表を描かず、R-10 も呼ばない", async () => {
    const fake = backend();
    await expect(contractsScreen(fakeDeps(GUEST))).rejects.toThrow("NEXT_REDIRECT:/demo");
    expect(state.redirects).toEqual(["/demo"]);
    expect(screen.queryByRole("table")).toBeNull();
    expect(fake.urls).toHaveLength(0);
  });

  it("C-3: misconfigured は例外を投げ、redirect も描画もしない（redirect の打ち切りではない）", async () => {
    const fake = backend();
    await expect(contractsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.toThrow();
    await expect(contractsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.not.toThrow(/NEXT_REDIRECT/);
    expect(state.redirects).toEqual([]);
    expect(screen.queryByRole("table")).toBeNull();
    expect(fake.urls).toHaveLength(0);
  });
});
