/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import { CONTRACTS_PATH, DEMO_PATH, OWN_ROWS, installFakeFetch, pagedListHandler } from "@/test-support/work-month-list";
import type { ScreenDeps } from "../_lib/screen-deps";
import { demoWorkMonthsScreen } from "./demo-work-months-screen";

// docs/specs/work-month-listing-ui.md AC-3-1・3-3・10-1・10-4・10-5、login-ui.md AC-6-9。
// 設計 W-1・W-6・T-0-2・T-2。next/navigation だけを vi.mock し、ScreenDeps は手書きの Fake（T-0-1）。

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

function backend() {
  return installFakeFetch({ [DEMO_PATH]: pagedListHandler(OWN_ROWS, 1000) });
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("区分による出し分け（3-3・login-ui 6-9）", () => {
  it("ゲスト: redirect せず、デモ勤務月群を描く（ヘッダー・サイドバー・一覧）", async () => {
    backend();
    render(await demoWorkMonthsScreen(fakeDeps({ kind: "guest" })));
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(await screen.findByRole("table", { name: "サンプル勤務月" })).toBeTruthy();
  });

  it("技術者: 本人の勤務月一覧へ redirect する（デモ勤務月群はゲストの導線）", async () => {
    const fake = backend();
    await expect(
      demoWorkMonthsScreen(fakeDeps({ kind: "signed-in", role: "Engineer" })),
    ).rejects.toThrow("NEXT_REDIRECT:/work-months");
    expect(state.redirects).toEqual(["/work-months"]);
    expect(fake.urls).toHaveLength(0);
  });

  it("承認者: 承認待ち一覧へ redirect する", async () => {
    backend();
    await expect(
      demoWorkMonthsScreen(fakeDeps({ kind: "signed-in", role: "Approver" })),
    ).rejects.toThrow("NEXT_REDIRECT:/pending-approvals");
    expect(state.redirects).toEqual(["/pending-approvals"]);
  });

  it("misconfigured: 例外を投げ、redirect も描画もしない", async () => {
    backend();
    await expect(demoWorkMonthsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.toThrow();
    await expect(demoWorkMonthsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.not.toThrow(
      /NEXT_REDIRECT/,
    );
    expect(state.redirects).toEqual([]);
  });
});

describe("サイドバー（10-1・10-4・10-5）", () => {
  it("ゲスト: 項目は「サンプル勤務月」とヘルプだけで、「サンプル勤務月」が選択中。契約を取得しない", async () => {
    const fake = backend();
    render(await demoWorkMonthsScreen(fakeDeps({ kind: "guest" })));
    await screen.findByRole("table", { name: "サンプル勤務月" });
    const nav = screen.getAllByRole("navigation").find((n) => within(n).queryByRole("link", { name: "ヘルプ" }));
    expect(nav).toBeDefined();
    expect(within(nav!).getAllByRole("link").map((l) => l.textContent)).toEqual(["サンプル勤務月", "ヘルプ"]);
    expect([...nav!.querySelectorAll("[aria-current]")].map((el) => el.textContent)).toEqual(["サンプル勤務月"]);
    expect(fake.count(CONTRACTS_PATH)).toBe(0);
  });
});
