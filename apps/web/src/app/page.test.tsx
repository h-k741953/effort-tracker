/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import Page from "./page";

// docs/specs/login-ui.md AC-6-10・6-10-a・9-1。設計 C-3・C-4・L-1。
// getCurrentViewer と next/navigation の redirect・next/headers の cookies を差し替える。
// redirect は本物と同じく「投げて処理を打ち切る」形で差し替える。

const state = vi.hoisted(() => ({
  viewer: { kind: "guest" } as
    | { kind: "guest" }
    | { kind: "signed-in"; role: "Engineer" | "Approver" }
    | { kind: "misconfigured" },
  redirects: [] as string[],
}));

vi.mock("./_lib/current-viewer", () => ({
  getCurrentViewer: async () => state.viewer,
}));

vi.mock("next/headers", () => ({
  cookies: async () => ({ get: () => undefined }),
}));

vi.mock("next/navigation", () => ({
  redirect: (to: string) => {
    state.redirects.push(to);
    throw new Error(`NEXT_REDIRECT:${to}`);
  },
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}));

// ページは async のサーバー部品（設計 L-1）。同期の足場でも Promise として扱う。
async function callPage() {
  return Page();
}

beforeEach(() => {
  state.redirects = [];
  state.viewer = { kind: "guest" };
});

afterEach(() => {
  cleanup();
});

describe("ログイン画面（page.tsx）", () => {
  it("6-10: 技術者は本人の勤務月一覧へ redirect し、描画しない", async () => {
    state.viewer = { kind: "signed-in", role: "Engineer" };
    await expect(callPage()).rejects.toThrow("NEXT_REDIRECT:/work-months");
    expect(state.redirects).toEqual(["/work-months"]);
  });

  it("6-10: 承認者は承認待ち一覧へ redirect する", async () => {
    state.viewer = { kind: "signed-in", role: "Approver" };
    await expect(callPage()).rejects.toThrow("NEXT_REDIRECT:/pending-approvals");
    expect(state.redirects).toEqual(["/pending-approvals"]);
  });

  it("6-10-a・9-1: ゲストは redirect されず、ヘッダーと3つの入口が描画される", async () => {
    state.viewer = { kind: "guest" };
    render(await callPage());
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("link", { name: "ゲストとして続行（閲覧のみ）" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "メールアドレスでログイン" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Google でログイン" })).toBeTruthy();
  });

  it("9-2・9-3: ゲストのログイン画面にテーマ切替はあり、ロール切替・ログインへのリンクは無い", async () => {
    state.viewer = { kind: "guest" };
    render(await callPage());
    expect(screen.getByRole("radiogroup", { name: "テーマ切替" })).toBeTruthy();
    expect(screen.queryByRole("radiogroup", { name: "ロール切替" })).toBeNull();
    expect(screen.queryByRole("link", { name: "ログイン" })).toBeNull();
  });

  it("C-3（Q-4 (ii) = (a)）: misconfigured は例外を投げ、redirect も描画もしない", async () => {
    state.viewer = { kind: "misconfigured" };
    await expect(callPage()).rejects.toThrow();
    expect(state.redirects).toEqual([]);
  });

  it("C-3: misconfigured の例外は redirect の打ち切りではない", async () => {
    state.viewer = { kind: "misconfigured" };
    await expect(callPage()).rejects.not.toThrow(/NEXT_REDIRECT/);
  });
});
