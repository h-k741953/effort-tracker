/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import {
  CONTRACT_A,
  CONTRACTS_PATH,
  PENDING_PATH,
  installFakeFetch,
  pagedListHandler,
  row,
} from "@/test-support/work-month-list";
import type { ScreenDeps } from "../_lib/screen-deps";
import { pendingApprovalsScreen } from "./pending-approvals-screen";

// docs/specs/work-month-listing-ui.md AC-2-4・3-3・10-1・10-3・10-5、login-ui.md AC-6-9。
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

const APPROVER: ResolveViewerResult = { kind: "signed-in", role: "Approver" };

function backend() {
  return installFakeFetch({
    [PENDING_PATH]: pagedListHandler([row(CONTRACT_A, "2026-10", "PendingApproval")], 1000),
  });
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("区分による出し分け（2-4・login-ui 6-9）", () => {
  it("承認者: redirect せず、承認待ち一覧を描く（ヘッダー・サイドバー・一覧）", async () => {
    backend();
    render(await pendingApprovalsScreen(fakeDeps(APPROVER)));
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1, name: "承認待ちの勤務月" })).toBeTruthy();
    expect(await screen.findByRole("table", { name: "承認待ちの勤務月" })).toBeTruthy();
  });

  it("技術者: 本人の勤務月一覧へ redirect する（承認待ち一覧への導線を提供しない）", async () => {
    const fake = backend();
    await expect(
      pendingApprovalsScreen(fakeDeps({ kind: "signed-in", role: "Engineer" })),
    ).rejects.toThrow("NEXT_REDIRECT:/work-months");
    expect(state.redirects).toEqual(["/work-months"]);
    expect(fake.urls).toHaveLength(0);
  });

  it("ゲスト: デモ勤務月群へ redirect する（3-3）", async () => {
    backend();
    await expect(pendingApprovalsScreen(fakeDeps({ kind: "guest" }))).rejects.toThrow("NEXT_REDIRECT:/demo");
    expect(state.redirects).toEqual(["/demo"]);
  });

  it("misconfigured: 例外を投げ、redirect も描画もしない", async () => {
    backend();
    await expect(pendingApprovalsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.toThrow();
    await expect(pendingApprovalsScreen(fakeDeps({ kind: "misconfigured" }))).rejects.not.toThrow(
      /NEXT_REDIRECT/,
    );
    expect(state.redirects).toEqual([]);
  });
});

describe("サイドバー（10-1・10-3・10-5）", () => {
  it("承認者: 項目は「承認待ち」とヘルプだけで、「承認待ち」が選択中。契約を取得しない", async () => {
    const fake = backend();
    render(await pendingApprovalsScreen(fakeDeps(APPROVER)));
    await screen.findByRole("table", { name: "承認待ちの勤務月" });
    const nav = screen.getAllByRole("navigation").find((n) => within(n).queryByRole("link", { name: "ヘルプ" }));
    expect(nav).toBeDefined();
    expect(within(nav!).getAllByRole("link").map((l) => l.textContent)).toEqual(["承認待ち", "ヘルプ"]);
    expect([...nav!.querySelectorAll("[aria-current]")].map((el) => el.textContent)).toEqual(["承認待ち"]);
    expect(fake.count(CONTRACTS_PATH)).toBe(0);
  });
});
