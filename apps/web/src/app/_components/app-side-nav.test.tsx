/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { thisMonthPath } from "@/lib/screen-routes";
import type { Viewer } from "@/lib/viewer";
import {
  CONTRACTS_PATH,
  CONTRACT_A,
  CONTRACT_B,
  CONTRACT_C,
  contractsBody,
  installFakeFetch,
  jsonResponse,
  type TestContract,
} from "@/test-support/work-month-list";
import { AppSideNav } from "./app-side-nav";

// docs/specs/work-month-listing-ui.md AC-10-1・10-7・10-9-a〜10-9-d・10-9-g。設計 W-6・W-8・T-2。
// 自分の契約は BFF（R-10）から fetch で取る。fetch は手書きの Fake を globalThis へ置く（T-0-1）。
// next/navigation は Next のランタイムのモジュールなので vi.mock してよい（router.refresh を呼ぶ形の差し替え）。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const ENGINEER: Viewer = { kind: "signed-in", role: "Engineer" };
const APPROVER: Viewer = { kind: "signed-in", role: "Approver" };
const GUEST: Viewer = { kind: "guest" };

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function backend(contracts: TestContract[]) {
  return installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse(contractsBody(contracts)) });
}

function nav() {
  return screen.getByRole("navigation");
}

/** 「今月の入力」の取得が済んで、項目が出る（link か button）のを待つ。 */
async function entry() {
  return waitFor(() => {
    const found = within(nav()).queryByRole("link", { name: "今月の入力" }) ??
      within(nav()).queryByRole("button", { name: "今月の入力" });
    expect(found).not.toBeNull();
    return found as HTMLElement;
  });
}

/** 取得の結果が描画へ反映されるまで待つ（0件・失敗のように「出ない」ことを読む前に使う）。 */
async function settled(fake: { urls: URL[] }) {
  await waitFor(() => expect(fake.urls.length).toBeGreaterThan(0));
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

describe("技術者（10-1・10-9-a〜10-9-d）", () => {
  it("10-1: メインメニューのナビゲーションに、技術者の項目（自分の勤務月・契約一覧・稼働の推移・ヘルプ）を出す", async () => {
    backend([CONTRACT_A]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    await entry();
    const names = within(nav()).getAllByRole("link").map((l) => l.textContent);
    expect(names).toEqual(["今月の入力", "自分の勤務月", "契約一覧", "稼働の推移", "ヘルプ"]);
    expect(
      [...nav().querySelectorAll("[aria-current]")].map((el) => el.textContent),
    ).toEqual(["自分の勤務月"]);
  });

  it("10-9-a: 契約が1件 → 「今月の入力」はリンクで、行き先はその契約の thisMonthPath", async () => {
    backend([CONTRACT_A]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    const link = await entry();
    expect(link.tagName).toBe("A");
    expect(link.getAttribute("href")).toBe(thisMonthPath(CONTRACT_A.contractId));
  });

  it("10-9-b: 契約が2件以上 → 「今月の入力」は button（aria-expanded=false）。押す前に選択肢のリンクも勤務月画面へのリンクも無い", async () => {
    backend([CONTRACT_A, CONTRACT_B, CONTRACT_C]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    const button = await entry();
    expect(button.tagName).toBe("BUTTON");
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    for (const c of [CONTRACT_A, CONTRACT_B, CONTRACT_C]) {
      expect(within(nav()).queryByRole("link", { name: c.contractDisplayName })).toBeNull();
    }
    // 勤務月画面（/work-months/<契約ID>/<年月>）へのリンクが無い。一覧（/work-months）自体は項目として在る。
    const hrefs = [...document.querySelectorAll("a")].map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.length).toBeGreaterThan(0);
    expect(hrefs.some((h) => h.startsWith("/work-months/"))).toBe(false);
    expect(hrefs.some((h) => h.startsWith("/this-month"))).toBe(false);
  });

  it("10-9-c: 押すと契約表示名の link が契約の数だけ R-10 の順で現れ、それぞれがその契約の thisMonthPath を指す", async () => {
    const contracts = [CONTRACT_C, CONTRACT_A, CONTRACT_B]; // 応答の順（並べ替えない）
    backend(contracts);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    const button = await entry();
    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    const ordered = [...nav().querySelectorAll("a, button")].map((el) => (el.textContent ?? "").trim());
    const start = ordered.indexOf("今月の入力");
    expect(ordered.slice(start + 1, start + 1 + contracts.length)).toEqual(
      contracts.map((c) => c.contractDisplayName),
    );
    for (const c of contracts) {
      expect(within(nav()).getByRole("link", { name: c.contractDisplayName }).getAttribute("href")).toBe(
        thisMonthPath(c.contractId),
      );
    }
  });

  it("10-9-d: 契約が0件 → 「今月の入力」を出さない（項目は自分の勤務月・契約一覧・稼働の推移・ヘルプ）", async () => {
    const fake = backend([]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    await settled(fake);
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    expect(screen.queryByRole("button", { name: "今月の入力" })).toBeNull();
    expect(within(nav()).getAllByRole("link").map((l) => l.textContent)).toEqual([
      "自分の勤務月",
      "契約一覧",
      "稼働の推移",
      "ヘルプ",
    ]);
  });

  it.each([500, 429])("10-9-d: 契約の取得が %i で失敗 → 「今月の入力」を出さない", async (status) => {
    const fake = installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse({}, status) });
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    await settled(fake);
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    expect(screen.queryByRole("button", { name: "今月の入力" })).toBeNull();
    expect(screen.getByRole("link", { name: "自分の勤務月" })).toBeTruthy();
  });

  it("10-9-d: fetch が例外 → 「今月の入力」を出さない", async () => {
    const calls: unknown[] = [];
    vi.stubGlobal("fetch", async (...args: unknown[]) => {
      calls.push(args);
      throw new TypeError("network");
    });
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    await waitFor(() => expect(calls.length).toBeGreaterThan(0));
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    expect(screen.queryByRole("button", { name: "今月の入力" })).toBeNull();
  });

  it("取得中は「今月の入力」を出さない（W-6。取得の結果が届いてから出る）", async () => {
    let release: (r: Response) => void = () => {};
    installFakeFetch({
      [CONTRACTS_PATH]: () => new Promise<Response>((resolve) => (release = resolve)),
    });
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    expect(screen.queryByRole("link", { name: "今月の入力" })).toBeNull();
    expect(screen.queryByRole("button", { name: "今月の入力" })).toBeNull();
    await act(async () => {
      release(jsonResponse(contractsBody([CONTRACT_A])));
    });
    expect(await entry()).toBeTruthy();
  });

  it("R-10 に JST の当月の yearMonth を付けて GET する（2026-10-31T15:00:00Z は 2026-11）", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-31T15:00:00Z"));
    const fake = backend([CONTRACT_A]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    await entry();
    const requests = fake.urls.filter((u) => u.pathname === CONTRACTS_PATH);
    expect(requests).toHaveLength(1);
    expect(requests[0].searchParams.get("yearMonth")).toBe("2026-11");
  });

  it("current=this-month のとき「今月の入力」だけが選択中", async () => {
    backend([CONTRACT_A]);
    render(<AppSideNav viewer={ENGINEER} current="this-month" />);
    await entry();
    expect([...nav().querySelectorAll("[aria-current]")].map((el) => el.textContent)).toEqual(["今月の入力"]);
  });
});

describe("承認者・ゲスト（10-3・10-4）", () => {
  it.each<[string, Viewer, "pending-approvals" | "demo-work-months", string[]]>([
    ["承認者", APPROVER, "pending-approvals", ["承認待ち", "ヘルプ"]],
    ["ゲスト", GUEST, "demo-work-months", ["サンプル勤務月", "ヘルプ"]],
  ])("%s: 契約を取得せず（fetch が呼ばれない）、「今月の入力」を出さない", async (_n, viewer, current, labels) => {
    const fake = backend([CONTRACT_A]);
    render(<AppSideNav viewer={viewer} current={current} />);
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(fake.urls).toHaveLength(0);
    expect(within(nav()).getAllByRole("link").map((l) => l.textContent)).toEqual(labels);
    expect(screen.queryByRole("button", { name: "今月の入力" })).toBeNull();
  });
});

describe("状態を変えない（10-7・10-9-g。Q-10）", () => {
  it("開閉と描画で、GET 以外を送らない", async () => {
    const fake = backend([CONTRACT_A, CONTRACT_B]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    const button = await entry();
    fireEvent.click(button);
    fireEvent.click(button);
    fireEvent.click(button);
    expect(fake.methods.length).toBeGreaterThan(0);
    expect(new Set(fake.methods)).toEqual(new Set(["GET"]));
    expect(fake.urls.every((u) => u.pathname === CONTRACTS_PATH)).toBe(true);
  });

  it("開閉で router を動かさない（遷移はリンクで行う）", async () => {
    backend([CONTRACT_A, CONTRACT_B]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    const button = await entry();
    fireEvent.click(button);
    fireEvent.click(button);
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("描画したまま時計を JST の月の境目（2026-10-31T15:00:00Z）の前後へ進めても、行き先の文字列が変わらない（年月を含まない）", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-31T14:59:59Z"));
    backend([CONTRACT_A, CONTRACT_B]);
    render(<AppSideNav viewer={ENGINEER} current="own-work-months" />);
    const button = await entry();
    fireEvent.click(button);
    const hrefsOf = () =>
      [CONTRACT_A, CONTRACT_B].map((c) =>
        within(nav()).getByRole("link", { name: c.contractDisplayName }).getAttribute("href"),
      );
    const before = hrefsOf();
    vi.setSystemTime(new Date("2026-10-31T15:00:01Z"));
    fireEvent.click(button);
    fireEvent.click(button);
    expect(hrefsOf()).toEqual(before);
    expect(before).toEqual([CONTRACT_A, CONTRACT_B].map((c) => thisMonthPath(c.contractId)));
    for (const href of before) {
      expect(href).not.toMatch(/20\d\d-\d\d/);
    }
  });
});

// docs/specs/work-month-listing-ui.md AC-10-5（勤務月画面）。設計 M-10・M-11・T-3。
// current に WorkMonthLocation を渡す。技術者は自分で取った R-10 の結果と描画の時点の new Date() で
// 「今月の入力」か「自分の勤務月」かが決まる。時計は Date だけを固める。
describe("勤務月画面の選択中（10-5）", () => {
  const OWN_CONTRACT: TestContract = { contractId: "c-001", contractDisplayName: "A社 / 保守" };

  function location(contractId: string, yearMonth: string) {
    return { kind: "work-month", contractId, yearMonth } as const;
  }

  function currentLabels() {
    return [...nav().querySelectorAll("[aria-current]")].map((el) => (el.textContent ?? "").trim());
  }

  it("技術者・c-001 × 2026-10（当月）: 「今月の入力」に aria-current=page が付き、「自分の勤務月」に付かない。ちょうど1つ", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-15T00:00:00Z"));
    backend([OWN_CONTRACT]);
    render(<AppSideNav viewer={ENGINEER} current={location("c-001", "2026-10")} />);
    const link = await entry();
    await waitFor(() => expect(link.getAttribute("aria-current")).toBe("page"));
    expect(screen.getByRole("link", { name: "自分の勤務月" }).getAttribute("aria-current")).toBeNull();
    expect(currentLabels()).toEqual(["今月の入力"]);
  });

  it("技術者・c-other × 2026-10（他人の契約）: 「自分の勤務月」に付き、「今月の入力」に付かない。ちょうど1つ", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-15T00:00:00Z"));
    backend([OWN_CONTRACT]);
    render(<AppSideNav viewer={ENGINEER} current={location("c-other", "2026-10")} />);
    const link = await entry();
    expect(link.getAttribute("aria-current")).toBeNull();
    expect(screen.getByRole("link", { name: "自分の勤務月" }).getAttribute("aria-current")).toBe("page");
    expect(currentLabels()).toEqual(["自分の勤務月"]);
  });

  it("技術者・自分の契約でも当月でない（2026-09）: 「自分の勤務月」に付く", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-15T00:00:00Z"));
    backend([OWN_CONTRACT]);
    render(<AppSideNav viewer={ENGINEER} current={location("c-001", "2026-09")} />);
    await entry();
    expect(currentLabels()).toEqual(["自分の勤務月"]);
  });

  it("承認者: 「承認待ち」に付く。fetch を呼ばない。ちょうど1つ", async () => {
    const fake = backend([OWN_CONTRACT]);
    render(<AppSideNav viewer={APPROVER} current={location("c-001", "2026-10")} />);
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(currentLabels()).toEqual(["承認待ち"]);
    expect(fake.urls).toHaveLength(0);
  });

  it("ゲスト: 「サンプル勤務月」に付く。fetch を呼ばない。ちょうど1つ", async () => {
    const fake = backend([OWN_CONTRACT]);
    render(<AppSideNav viewer={GUEST} current={location("c-001", "2026-10")} />);
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(currentLabels()).toEqual(["サンプル勤務月"]);
    expect(fake.urls).toHaveLength(0);
  });
});
