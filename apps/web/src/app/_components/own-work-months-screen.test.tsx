/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { thisMonthPath } from "@/lib/screen-routes";
import type { ResolveViewerResult } from "@/lib/viewer";
import type { Theme } from "@/lib/theme-cookie";
import {
  CONTRACTS_PATH,
  CONTRACT_A,
  CONTRACT_B,
  OWN_PATH,
  OWN_ROWS,
  contractsBody,
  installFakeFetch,
  jsonResponse,
  pagedListHandler,
  type TestContract,
} from "@/test-support/work-month-list";
import type { ScreenDeps } from "../_lib/screen-deps";
import { ownWorkMonthsScreen } from "./own-work-months-screen";

// docs/specs/work-month-listing-ui.md AC-2-4・9-12・10-1・10-5・10-7・10-9-a〜10-9-d、login-ui.md AC-6-9。
// 設計 W-1・W-4・W-6・T-0-2・T-2。本体へ手書きの Fake の ScreenDeps を渡し、next/navigation だけを vi.mock する。
// 自前のモジュールは差し替えない（T-0-1）。redirect は本物と同じく「投げて打ち切る」形。

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

function backend(contracts: TestContract[] = [CONTRACT_A, CONTRACT_B]) {
  return installFakeFetch({
    [OWN_PATH]: pagedListHandler(OWN_ROWS, 1000),
    [CONTRACTS_PATH]: () => jsonResponse(contractsBody(contracts)),
  });
}

beforeEach(() => {
  state.redirects = [];
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("区分による出し分け（2-4・login-ui 6-9）", () => {
  it("技術者: redirect せず、勤務月の一覧を描く（ヘッダー・サイドバー・本人一覧）", async () => {
    backend();
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), {}));
    expect(state.redirects).toEqual([]);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1, name: "自分の勤務月" })).toBeTruthy();
    expect(await screen.findByRole("table", { name: "すべての勤務月" })).toBeTruthy();
    expect(
      screen.getAllByRole("navigation").some((n) => within(n).queryByRole("link", { name: "契約一覧" }) !== null),
    ).toBe(true);
  });

  it("承認者: 承認待ち一覧へ redirect し、描画しない", async () => {
    const fake = backend();
    await expect(
      ownWorkMonthsScreen(fakeDeps({ kind: "signed-in", role: "Approver" }), {}),
    ).rejects.toThrow("NEXT_REDIRECT:/pending-approvals");
    expect(state.redirects).toEqual(["/pending-approvals"]);
    expect(fake.urls).toHaveLength(0);
  });

  it("ゲスト: デモ勤務月群へ redirect する", async () => {
    backend();
    await expect(ownWorkMonthsScreen(fakeDeps({ kind: "guest" }), {})).rejects.toThrow("NEXT_REDIRECT:/demo");
    expect(state.redirects).toEqual(["/demo"]);
  });

  it("misconfigured: 例外を投げ、redirect も描画もしない（redirect の打ち切りではない）", async () => {
    backend();
    const call = ownWorkMonthsScreen(fakeDeps({ kind: "misconfigured" }), {});
    await expect(call).rejects.toThrow();
    await expect(ownWorkMonthsScreen(fakeDeps({ kind: "misconfigured" }), {})).rejects.not.toThrow(
      /NEXT_REDIRECT/,
    );
    expect(state.redirects).toEqual([]);
  });
});

describe("searchParams の contract（9-12・W-1）", () => {
  async function selectedContractLabel(searchParams: Record<string, string | string[] | undefined>) {
    backend();
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), searchParams));
    await screen.findByRole("table", { name: "すべての勤務月" });
    const search = screen.getByRole("search", { name: "勤務月の絞り込み" });
    const select = within(search).getByRole("combobox", { name: "契約" }) as HTMLSelectElement;
    return select.selectedOptions[0].textContent;
  }

  it("文字列1つ: 契約の条件の初期値になる", async () => {
    expect(await selectedContractLabel({ contract: CONTRACT_B.contractId })).toBe(CONTRACT_B.contractDisplayName);
  });

  it("無いとき: 初期値を渡さない（「すべて」）", async () => {
    expect(await selectedContractLabel({})).toBe("すべて");
  });

  it("配列のとき: 初期値を渡さない（「すべて」）", async () => {
    expect(await selectedContractLabel({ contract: [CONTRACT_A.contractId, CONTRACT_B.contractId] })).toBe("すべて");
  });
});

describe("サイドバー（10-1・10-5・10-9）", () => {
  it("10-5: 「自分の勤務月」だけが選択中", async () => {
    backend();
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), {}));
    await screen.findByRole("table", { name: "すべての勤務月" });
    const nav = screen.getAllByRole("navigation").find((n) => within(n).queryByRole("link", { name: "契約一覧" }));
    expect(nav).toBeDefined();
    const current = [...nav!.querySelectorAll("[aria-current]")].map((el) => el.textContent);
    expect(current).toEqual(["自分の勤務月"]);
  });

  it("10-9-a: 契約1件 → 「今月の入力」がリンクで、行き先はその契約の thisMonthPath", async () => {
    backend([CONTRACT_A]);
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), {}));
    const link = await screen.findByRole("link", { name: "今月の入力" });
    expect(link.getAttribute("href")).toBe(thisMonthPath(CONTRACT_A.contractId));
  });

  it("10-9-b・10-9-c: 契約2件 → 「今月の入力」は閉じた button。押す前に選択肢のリンクは無く、押すと契約ごとの thisMonthPath が開く", async () => {
    backend([CONTRACT_A, CONTRACT_B]);
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), {}));
    const button = await screen.findByRole("button", { name: "今月の入力" });
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    const nav = button.closest("nav") as HTMLElement;
    expect(within(nav).queryByRole("link", { name: CONTRACT_A.contractDisplayName })).toBeNull();

    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    for (const c of [CONTRACT_A, CONTRACT_B]) {
      expect(within(nav).getByRole("link", { name: c.contractDisplayName }).getAttribute("href")).toBe(
        thisMonthPath(c.contractId),
      );
    }
  });

  it("10-9-d: 契約0件 → 「今月の入力」を出さない", async () => {
    backend([]);
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), {}));
    await screen.findByRole("table", { name: "すべての勤務月" });
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    expect(screen.queryByRole("button", { name: "今月の入力" })).toBeNull();
  });

  it("10-9-d: 契約の取得に失敗 → 「今月の入力」を出さない", async () => {
    installFakeFetch({
      [OWN_PATH]: pagedListHandler(OWN_ROWS, 1000),
      [CONTRACTS_PATH]: () => jsonResponse({}, 500),
    });
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), {}));
    await screen.findByRole("alert");
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    expect(screen.queryByRole("button", { name: "今月の入力" })).toBeNull();
  });

  it("Q-10・10-9-g: JST の月の境目をまたいで時計を進めても「今月の入力」の行き先は変わず、通信は GET だけ", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-31T14:59:59Z"));
    const fake = backend([CONTRACT_A]);
    render(await ownWorkMonthsScreen(fakeDeps(ENGINEER), {}));
    const link = await screen.findByRole("link", { name: "今月の入力" });
    const before = link.getAttribute("href");
    vi.setSystemTime(new Date("2026-10-31T15:00:01Z"));
    fireEvent.mouseEnter(link);
    expect(screen.getByRole("link", { name: "今月の入力" }).getAttribute("href")).toBe(before);
    expect(new Set(fake.methods)).toEqual(new Set(["GET"]));
  });
});
