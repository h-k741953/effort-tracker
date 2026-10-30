/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import type { ScreenDeps } from "../_lib/screen-deps";
import { loginScreen } from "./login-screen";

// docs/specs/login-ui.md AC-6-10・6-10-a・9-1。設計 L-1・C-3・C-4・T-0-2・T-0-3。
// 本体 loginScreen へ手書きの Fake の ScreenDeps を注入する（自前モジュールは vi.mock しない。T-0-1）。
// vi.mock するのは Next のランタイムの next/navigation だけ。
// redirect は本物と同じく「投げて処理を打ち切る」形で差し替える。

const state = vi.hoisted(() => ({ redirects: [] as string[] }));

vi.mock("next/navigation", () => ({
  redirect: (to: string) => {
    state.redirects.push(to);
    throw new Error(`NEXT_REDIRECT:${to}`);
  },
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}));

function fakeDeps(viewer: ResolveViewerResult, theme: Theme = "light"): ScreenDeps {
  return {
    getCurrentViewer: async () => viewer,
    getTheme: async () => theme,
  };
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
});

describe("ログイン画面の本体（loginScreen）", () => {
  it("6-10: 技術者は本人の勤務月一覧へ redirect し、描画しない", async () => {
    await expect(
      loginScreen(fakeDeps({ kind: "signed-in", role: "Engineer" })),
    ).rejects.toThrow("NEXT_REDIRECT:/work-months");
    expect(state.redirects).toEqual(["/work-months"]);
  });

  it("6-10: 承認者は承認待ち一覧へ redirect する", async () => {
    await expect(
      loginScreen(fakeDeps({ kind: "signed-in", role: "Approver" })),
    ).rejects.toThrow("NEXT_REDIRECT:/pending-approvals");
    expect(state.redirects).toEqual(["/pending-approvals"]);
  });

  it("6-10-a・9-1: ゲストは redirect されず、ヘッダーと3つの入口が描画される", async () => {
    render(await loginScreen(fakeDeps({ kind: "guest" })));
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("link", { name: "ゲストとして続行（閲覧のみ）" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "メールアドレスでログイン" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Google でログイン" })).toBeTruthy();
  });

  it("9-2・9-3: ゲストのログイン画面にテーマ切替はあり、ロール切替・ログインへのリンクは無い", async () => {
    render(await loginScreen(fakeDeps({ kind: "guest" })));
    expect(screen.getByRole("radiogroup", { name: "テーマ切替" })).toBeTruthy();
    expect(screen.queryByRole("radiogroup", { name: "ロール切替" })).toBeNull();
    expect(screen.queryByRole("link", { name: "ログイン" })).toBeNull();
  });

  it("C-3（Q-4 (ii) = (a)）: misconfigured は例外を投げ、redirect も描画もしない", async () => {
    await expect(loginScreen(fakeDeps({ kind: "misconfigured" }))).rejects.toThrow();
    expect(state.redirects).toEqual([]);
  });

  it("C-3: misconfigured の例外は redirect の打ち切りではない", async () => {
    await expect(loginScreen(fakeDeps({ kind: "misconfigured" }))).rejects.not.toThrow(
      /NEXT_REDIRECT/,
    );
  });
});
