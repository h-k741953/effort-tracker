/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import type { ResolveViewerResult, Viewer } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import { CONTRACT_A, CONTRACTS_PATH, contractsBody, installFakeFetch, jsonResponse } from "@/test-support/work-month-list";
import type { ScreenDeps } from "../_lib/screen-deps";
import { workMonthScreen } from "./work-month-screen";

// docs/specs/login-ui.md 6-7・6-9、work-month-screen-ui.md 1-8・1-9。設計 M-1・M-10・M-11・T-0-2・T-3。
// 本体 workMonthScreen へ手書きの Fake の ScreenDeps・params・手書きの Fake の view を渡し、
// next/navigation だけを vi.mock する。自前のモジュールは差し替えない（T-0-1）。

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

/** 受けた props を data-* 属性で描くだけの Fake の view。 */
function FakeView(props: { viewer: Viewer; contractId: string; yearMonth: string }) {
  return (
    <div
      data-testid="fake-view"
      data-viewer-kind={props.viewer.kind}
      data-viewer-role={props.viewer.kind === "signed-in" ? props.viewer.role : ""}
      data-contract-id={props.contractId}
      data-year-month={props.yearMonth}
    />
  );
}

const PARAMS = { contractId: "c-001", yearMonth: "2026-07" };
const VIEWERS: Array<[string, ResolveViewerResult]> = [
  ["技術者", { kind: "signed-in", role: "Engineer" }],
  ["承認者", { kind: "signed-in", role: "Approver" }],
  ["ゲスト", { kind: "guest" }],
];

function backend() {
  return installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse(contractsBody([CONTRACT_A])) });
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("区分による出し分け（login-ui 6-7・6-9。区分3種はいずれも開ける）", () => {
  it.each(VIEWERS)("%s: redirect せず、Fake の view をちょうど1回、props 付きで AppShell の子に描く", async (_name, viewer) => {
    backend();
    render(await workMonthScreen(fakeDeps(viewer), PARAMS, FakeView));
    expect(state.redirects).toEqual([]);
    const views = screen.getAllByTestId("fake-view");
    expect(views).toHaveLength(1);
    expect(views[0].getAttribute("data-contract-id")).toBe("c-001");
    expect(views[0].getAttribute("data-year-month")).toBe("2026-07");
    expect(views[0].getAttribute("data-viewer-kind")).toBe(viewer.kind);
    if (viewer.kind === "signed-in") {
      expect(views[0].getAttribute("data-viewer-role")).toBe(viewer.role);
    }
    // AppShell の子（<main>）として描かれ、<header> と <nav> が出る（1-8・1-9）
    expect(within(screen.getByRole("main")).getByTestId("fake-view")).toBe(views[0]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getAllByRole("navigation").length).toBeGreaterThan(0);
  });

  it("misconfigured: 例外を投げ、redirect も描画もしない（redirect の打ち切りではない）", async () => {
    backend();
    await expect(workMonthScreen(fakeDeps({ kind: "misconfigured" }), PARAMS, FakeView)).rejects.toThrow();
    await expect(workMonthScreen(fakeDeps({ kind: "misconfigured" }), PARAMS, FakeView)).rejects.not.toThrow(
      /NEXT_REDIRECT/,
    );
    expect(state.redirects).toEqual([]);
  });
});

describe("fetch を呼ばない（M-1）", () => {
  it.each(VIEWERS)("%s: 本体の呼び出しが戻るまで globalThis.fetch が呼ばれない", async (_name, viewer) => {
    const fake = backend();
    await workMonthScreen(fakeDeps(viewer), PARAMS, FakeView);
    expect(fake.urls).toHaveLength(0);
  });

  it("misconfigured でも呼ばれない", async () => {
    const fake = backend();
    await expect(workMonthScreen(fakeDeps({ kind: "misconfigured" }), PARAMS, FakeView)).rejects.toThrow();
    expect(fake.urls).toHaveLength(0);
  });
});

describe("サイドバーの current（M-1。AppShell へ勤務月の位置を渡す）", () => {
  function markedLabels() {
    return [...document.querySelectorAll("nav [aria-current]")].map((el) => (el.textContent ?? "").trim());
  }

  it("ゲスト: 「サンプル勤務月」が選択中", async () => {
    backend();
    render(await workMonthScreen(fakeDeps({ kind: "guest" }), PARAMS, FakeView));
    expect(markedLabels()).toEqual(["サンプル勤務月"]);
  });

  it("承認者: 「承認待ち」が選択中", async () => {
    backend();
    render(await workMonthScreen(fakeDeps({ kind: "signed-in", role: "Approver" }), PARAMS, FakeView));
    expect(markedLabels()).toEqual(["承認待ち"]);
  });
});
