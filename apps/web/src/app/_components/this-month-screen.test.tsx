import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { defaultScreenPath } from "@/lib/screen-access";
import { ownWorkMonthsPath, workMonthPath } from "@/lib/screen-routes";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { ThisMonthDeps } from "../_lib/screen-deps";
import { thisMonthScreen, type ThisMonthSearchParams } from "./this-month-screen";

// docs/specs/work-month-listing-ui.md AC-10-9・10-9-a・10-9-c・10-9-g、login-ui.md AC-6-7-a。
// 設計 W-7・T-0-2・T-2。本体 thisMonthScreen へ手書きの Fake の ThisMonthDeps（getCurrentViewer と now）を渡し、
// next/navigation の redirect だけを vi.mock する（送り先を記録して例外を投げる。本物と同じく打ち切る形）。
// 自前のモジュールは差し替えない（T-0-1）。転送の状態コードは読まず、redirect に渡した送り先だけを読む（W-7）。

const state = vi.hoisted(() => ({ redirects: [] as string[] }));

vi.mock("next/navigation", () => ({
  redirect: (to: string) => {
    state.redirects.push(to);
    throw new Error(`NEXT_REDIRECT:${to}`);
  },
}));

const ENGINEER: ResolveViewerResult = { kind: "signed-in", role: "Engineer" };
const APPROVER: ResolveViewerResult = { kind: "signed-in", role: "Approver" };
const GUEST: ResolveViewerResult = { kind: "guest" };
const MISCONFIGURED: ResolveViewerResult = { kind: "misconfigured" };

/** deps.now の呼び出し回数を数える手書きの Fake。 */
function fakeDeps(viewer: ResolveViewerResult, nowIso = "2026-10-15T00:00:00Z") {
  const counter = { now: 0 };
  const deps: ThisMonthDeps = {
    getCurrentViewer: async () => viewer,
    now: () => {
      counter.now += 1;
      return new Date(nowIso);
    },
  };
  return { deps, counter };
}

/** 呼ばれたら記録する fetch を大域へ置く（10-9-g。本体は通信しない）。 */
function installFetchRecorder() {
  const calls: unknown[] = [];
  vi.stubGlobal("fetch", async (...args: unknown[]) => {
    calls.push(args);
    throw new Error("fetch must not be called");
  });
  return calls;
}

let fetchCalls: unknown[];

beforeEach(() => {
  state.redirects = [];
  fetchCalls = installFetchRecorder();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("技術者・契約 ID あり: JST の当月の勤務月画面へ送る（10-9・10-9-a・10-9-c）", () => {
  it.each([
    ["2026-10-31T14:59:59Z", "2026-10"],
    ["2026-10-31T15:00:00Z", "2026-11"],
    ["2026-12-31T14:59:59Z", "2026-12"],
    ["2026-12-31T15:00:00Z", "2027-01"],
  ])("contract=c-001・now=%s → workMonthPath(c-001, %s)", async (nowIso, yearMonth) => {
    const { deps, counter } = fakeDeps(ENGINEER, nowIso);
    const expected = workMonthPath("c-001", yearMonth);
    await expect(thisMonthScreen(deps, { contract: "c-001" })).rejects.toThrow(`NEXT_REDIRECT:${expected}`);
    expect(state.redirects).toEqual([expected]);
    expect(counter.now).toBe(1);
    expect(fetchCalls).toHaveLength(0);
  });

  it("契約 ID に符号化の要る文字があっても、行き先は workMonthPath と一致する", async () => {
    const { deps } = fakeDeps(ENGINEER, "2026-10-15T00:00:00Z");
    await expect(thisMonthScreen(deps, { contract: "a/b c" })).rejects.toThrow();
    expect(state.redirects).toEqual([workMonthPath("a/b c", "2026-10")]);
  });

  it("他人の契約の ID を区別しない: contract=c-other でも workMonthPath(c-other, 当月) へ送る（確かめるのは画面3 とドメイン API。W-7）", async () => {
    const { deps } = fakeDeps(ENGINEER, "2026-10-15T00:00:00Z");
    await expect(thisMonthScreen(deps, { contract: "c-other" })).rejects.toThrow();
    expect(state.redirects).toEqual([workMonthPath("c-other", "2026-10")]);
    expect(fetchCalls).toHaveLength(0);
  });
});

describe("承認者・ゲスト: 勤務月画面へ送らず、区分の既定の画面へ送る（login-ui 6-7-a）", () => {
  it.each<[string, ResolveViewerResult]>([
    ["承認者", APPROVER],
    ["ゲスト", GUEST],
  ])("%s（contract=c-001 を付けても）→ defaultScreenPath", async (_name, viewer) => {
    const { deps } = fakeDeps(viewer, "2026-10-15T00:00:00Z");
    const expected = defaultScreenPath(viewer as Parameters<typeof defaultScreenPath>[0]);
    await expect(thisMonthScreen(deps, { contract: "c-001" })).rejects.toThrow(`NEXT_REDIRECT:${expected}`);
    expect(state.redirects).toEqual([expected]);
    expect(state.redirects[0].startsWith("/work-months/")).toBe(false);
    expect(fetchCalls).toHaveLength(0);
  });

  it("承認者の送り先は承認待ち一覧、ゲストの送り先はデモ勤務月群（区分で異なる）", async () => {
    await expect(thisMonthScreen(fakeDeps(APPROVER).deps, { contract: "c-001" })).rejects.toThrow();
    await expect(thisMonthScreen(fakeDeps(GUEST).deps, { contract: "c-001" })).rejects.toThrow();
    expect(state.redirects).toEqual(["/pending-approvals", "/demo"]);
  });
});

describe("技術者で contract が無い・空文字・配列: 本人の勤務月一覧へ送る（W-7）", () => {
  it.each<[string, ThisMonthSearchParams]>([
    ["無い", {}],
    ["undefined", { contract: undefined }],
    ["空文字", { contract: "" }],
    ["配列（2件）", { contract: ["c-001", "c-002"] }],
    ["配列（1件）", { contract: ["c-001"] }],
    ["別名のクエリだけ", { other: "c-001" }],
  ])("contract が%s → ownWorkMonthsPath({})", async (_name, searchParams) => {
    const { deps } = fakeDeps(ENGINEER, "2026-10-15T00:00:00Z");
    const expected = ownWorkMonthsPath({});
    await expect(thisMonthScreen(deps, searchParams)).rejects.toThrow(`NEXT_REDIRECT:${expected}`);
    expect(state.redirects).toEqual([expected]);
    expect(fetchCalls).toHaveLength(0);
  });
});

describe("misconfigured: 例外を投げ、redirect を呼ばない（C-3。Q-4 (ii) = (a)）", () => {
  it.each<[string, ThisMonthSearchParams]>([
    ["contract あり", { contract: "c-001" }],
    ["contract なし", {}],
  ])("%s", async (_name, searchParams) => {
    const { deps } = fakeDeps(MISCONFIGURED);
    await expect(thisMonthScreen(deps, searchParams)).rejects.toThrow();
    // redirect の打ち切りの例外ではない。
    await expect(thisMonthScreen(fakeDeps(MISCONFIGURED).deps, searchParams)).rejects.not.toThrow(
      /NEXT_REDIRECT/,
    );
    expect(state.redirects).toEqual([]);
    expect(fetchCalls).toHaveLength(0);
  });
});

describe("どの行でも: redirect はちょうど1回で、JSX を返さず、fetch を呼ばない（10-9-g）", () => {
  it.each<[string, ResolveViewerResult, ThisMonthSearchParams]>([
    ["技術者・契約あり", ENGINEER, { contract: "c-001" }],
    ["技術者・契約なし", ENGINEER, {}],
    ["承認者", APPROVER, { contract: "c-001" }],
    ["ゲスト", GUEST, { contract: "c-001" }],
  ])("%s", async (_name, viewer, searchParams) => {
    const { deps } = fakeDeps(viewer);
    // 戻り値が得られた（=resolve した）なら、JSX を返したことになり失敗する。
    await expect(thisMonthScreen(deps, searchParams)).rejects.toThrow(/^NEXT_REDIRECT:/);
    expect(state.redirects).toHaveLength(1);
    expect(fetchCalls).toHaveLength(0);
  });
});
