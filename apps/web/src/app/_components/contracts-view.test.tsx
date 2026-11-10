/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { EmptyState } from "@/components/empty-state";
import type { ContractListItem } from "@/lib/contract-list";
import { CONTRACTS_PATH, installFakeFetch, jsonResponse } from "@/test-support/work-month-list";
import { ContractsView } from "./contracts-view";

// docs/specs/contracts-ui.md 1-1・1-3・1-4・2-7・2-8・4-1〜4-3・6-1〜6-3。設計 KP-2・KP-4・K-1。
// next/navigation だけを vi.mock し、fetch は手書きの Fake を大域へ置く。文言は仕様の字面をここへ書く。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const ERROR_TEXT = "契約を読み込めませんでした。通信状態を確認して、もう一度お試しください。";
const DESCRIPTION = "担当している契約の精算幅と契約期間です。";
const RANGE = { lowerBound: { hours: 140, minutes: 0 }, upperBound: { hours: 180, minutes: 0 } };

const PAST: ContractListItem = {
  contractId: "c-1",
  contractDisplayName: "A社 / 保守",
  settlementRange: RANGE,
  contractPeriod: { start: "2024-01", end: "2025-12" },
  workMonth: { yearMonth: "2026-11", state: "Approved", generated: true },
};
const FUTURE: ContractListItem = {
  contractId: "c-2",
  contractDisplayName: "B社 / 開発",
  settlementRange: RANGE,
  contractPeriod: { start: "2027-01", end: null },
  workMonth: { yearMonth: "2026-11", state: "Draft", generated: false },
};
const OPEN_ENDED: ContractListItem = {
  contractId: "c-3",
  contractDisplayName: "C社 / 運用",
  settlementRange: { lowerBound: { hours: 140, minutes: 15 }, upperBound: { hours: 180, minutes: 30 } },
  contractPeriod: { start: "2025-04", end: null },
  workMonth: { yearMonth: "2026-11", state: "PendingApproval", generated: true },
};
const ITEMS = [OPEN_ENDED, PAST, FUTURE];

beforeEach(() => {
  router.refresh.mockReset();
  router.push.mockReset();
  router.replace.mockReset();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function backend(items: ContractListItem[] = ITEMS) {
  return installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse({ items }) });
}

const INTERACTIVE = "a, button, [role='link'], [role='button']";
const CONTROLS =
  "button, input, select, textarea, form, [role='button'], [role='menuitem'], [role='menu'], [role='checkbox'], [role='radio'], [role='combobox'], [role='listbox'], [role='switch'], [role='textbox']";

/** 描画に、表または状態の語（StatusBadge）が出ているか。 */
function showsTableOrBadge(container: HTMLElement): boolean {
  if (container.querySelector("table") !== null) return true;
  const text = container.textContent ?? "";
  return ["下書き", "締め済", "承認済"].some((w) => text.includes(w));
}

async function loaded() {
  return screen.findByRole("table");
}

describe("通常の表示（1-1・1-3・1-4・2-7・2-8）", () => {
  it("2-7・2-8: h1 は 契約一覧、説明文は K-1 の字面", async () => {
    backend();
    render(<ContractsView />);
    await loaded();
    expect(screen.getByRole("heading", { level: 1, name: "契約一覧" })).toBeTruthy();
    expect(screen.getByText(DESCRIPTION)).toBeTruthy();
  });

  it("1-3・1-4・1-5: 応答の行がすべて、応答の順のまま載る（過ぎた・始まっていない・終了 null・今月未生成を除かない）", async () => {
    backend();
    render(<ContractsView />);
    const table = await loaded();
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(ITEMS.length);
    expect(rows.map((r) => within(r).getAllByRole("cell")[0].textContent)).toEqual([
      "C社 / 運用",
      "A社 / 保守",
      "B社 / 開発",
    ]);
    const periods = rows.map((r) => within(r).getAllByRole("cell")[2].textContent);
    expect(periods).toEqual(["2025年4月〜", "2024年1月〜2025年12月", "2027年1月〜"]);
    const states = rows.map((r) => within(r).getAllByRole("cell")[3].textContent);
    expect(states).toEqual(["締め済", "承認済", "下書き"]);
  });
});

describe("取得の要求（KP-2・2-5-a）", () => {
  it("R-10 へ GET を1回だけ送る（描画の後。engineerId なし。R-1 など勤務月の取得を呼ばない）", async () => {
    const fake = backend();
    render(<ContractsView />);
    await loaded();
    expect(fake.urls).toHaveLength(1);
    expect(fake.urls[0].pathname).toBe(CONTRACTS_PATH);
    expect(fake.urls[0].searchParams.has("engineerId")).toBe(false);
    expect(fake.methods).toEqual(["GET"]);
  });

  it.each([
    ["2026-10-31T14:59:59Z", "2026-10"],
    ["2026-10-31T15:00:00Z", "2026-11"],
  ])("now=%s → yearMonth=%s（JST の当月）を付ける", async (iso, yearMonth) => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(iso));
    const fake = backend();
    render(<ContractsView />);
    await loaded();
    expect(fake.urls[0].searchParams.get("yearMonth")).toBe(yearMonth);
  });
});

describe("取得中（KP-2）", () => {
  it("領域に aria-busy=\"true\" があり、文言を出さず、表・エラー・空状態も出さない。取得後は付けない", async () => {
    vi.stubGlobal("fetch", () => new Promise<Response>(() => {}));
    const { container } = render(<ContractsView />);
    const busy = container.querySelector('[aria-busy="true"]');
    expect(busy).not.toBeNull();
    expect(busy!.textContent).toBe("");
    expect(container.querySelector("table")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText("担当している契約はありません")).toBeNull();
    cleanup();

    backend();
    const second = render(<ContractsView />);
    await loaded();
    expect(second.container.querySelector('[aria-busy="true"]')).toBeNull();
  });

  it("陽性対照: 文言を持つ aria-busy の領域は、同じ判定（textContent が空）で検出される", () => {
    const { container, unmount } = render(<div aria-busy="true">読み込み中</div>);
    expect(container.querySelector('[aria-busy="true"]')!.textContent).not.toBe("");
    unmount();
  });
});

describe("0件（6-1・6-2）", () => {
  it("EmptyState の title が 担当している契約はありません。表を出さない", async () => {
    backend([]);
    const { container } = render(<ContractsView />);
    await screen.findByText("担当している契約はありません");
    expect(container.querySelector("table")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("6-2: 0件のとき、契約を作る導線（リンク・ボタン）を画面のどこにも出さない", async () => {
    backend([]);
    const { container } = render(<ContractsView />);
    const title = await screen.findByText("担当している契約はありません");
    expect(title.closest("div")!.querySelector(INTERACTIVE)).toBeNull();
    expect(container.querySelector(INTERACTIVE)).toBeNull();
  });

  it("陽性対照（6-2）: action を渡した EmptyState は、同じ判定でリンク・ボタンを検出される", () => {
    const { container, unmount } = render(
      <div>
        <EmptyState title="t" action={<a href="/new">作る</a>} />
        <EmptyState title="u" action={<button type="button">作る</button>} />
      </div>,
    );
    expect(container.querySelectorAll(INTERACTIVE)).toHaveLength(2);
    unmount();
  });
});

describe("取得エラー（6-3）", () => {
  it.each<[string, () => Response]>([
    ["500", () => jsonResponse({}, 500)],
    ["429", () => jsonResponse({}, 429)],
    ["JSON でない本文", () => new Response("x", { status: 200 })],
  ])("%s → role=alert に K-1 の文を出し、表も StatusBadge も出さない", async (_name, respond) => {
    installFakeFetch({ [CONTRACTS_PATH]: respond });
    const { container } = render(<ContractsView />);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(ERROR_TEXT);
    expect(showsTableOrBadge(container)).toBe(false);
  });

  it("fetch が例外 → role=alert に K-1 の文を出し、表も StatusBadge も出さない", async () => {
    vi.stubGlobal("fetch", async () => {
      throw new Error("network");
    });
    const { container } = render(<ContractsView />);
    expect((await screen.findByRole("alert")).textContent).toContain(ERROR_TEXT);
    expect(showsTableOrBadge(container)).toBe(false);
  });

  it("陽性対照（6-3）: 成功して表を描いた画面は、同じ判定（表・StatusBadge の有無）で検出される", async () => {
    backend();
    const { container } = render(<ContractsView />);
    await loaded();
    expect(showsTableOrBadge(container)).toBe(true);
  });

  it("再試行で再び GET し、成功すればエラーが消えて表が出る", async () => {
    let failing = true;
    const fake = installFakeFetch({
      [CONTRACTS_PATH]: () => (failing ? jsonResponse({}, 500) : jsonResponse({ items: ITEMS })),
    });
    render(<ContractsView />);
    await screen.findByRole("alert");
    expect(fake.count(CONTRACTS_PATH)).toBe(1);

    failing = false;
    fireEvent.click(screen.getByRole("button", { name: "再試行" }));
    await loaded();
    expect(fake.count(CONTRACTS_PATH)).toBe(2);
    expect(fake.methods).toEqual(["GET", "GET"]);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it.each([401, 403])("%i → router.refresh がちょうど1回（区分の判定をサーバーに任せる）", async (status) => {
    installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse({}, status) });
    const { container } = render(<ContractsView />);
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(router.refresh).toHaveBeenCalledTimes(1);
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
    expect(showsTableOrBadge(container)).toBe(false);
  });
});

describe("閲覧のみ（4-1〜4-3）", () => {
  it("読み込んだ状態で、ボタン・入力欄・選択欄・チェックボックス・ラジオ・フォーム・メニュー項目が無い（リンクだけ）", async () => {
    backend();
    const { container } = render(<ContractsView />);
    await loaded();
    expect(container.querySelectorAll(CONTROLS)).toHaveLength(0);
    expect(screen.getAllByRole("link")).toHaveLength(ITEMS.length);
  });

  it.each([
    ["button", <button key="a" type="button">x</button>],
    ["input", <input key="a" />],
    ["checkbox", <input key="a" type="checkbox" />],
    ["radio", <input key="a" type="radio" />],
    ["select", <select key="a" />],
    ["textarea", <textarea key="a" />],
    ["form", <form key="a" />],
    ["menuitem", <div key="a" role="menuitem">x</div>],
  ])("陽性対照（4-1・4-2）: %s を含む描画は、同じ判定で検出される", (_name, el) => {
    const { container, unmount } = render(<div>{el}</div>);
    expect(container.querySelectorAll(CONTROLS).length).toBeGreaterThan(0);
    unmount();
  });

  it("4-3: 描画しても再試行しても GET 以外を送らず、push/replace を呼ばない", async () => {
    let failing = true;
    const fake = installFakeFetch({
      [CONTRACTS_PATH]: () => (failing ? jsonResponse({}, 500) : jsonResponse({ items: ITEMS })),
    });
    render(<ContractsView />);
    await screen.findByRole("alert");
    failing = false;
    fireEvent.click(screen.getByRole("button", { name: "再試行" }));
    await loaded();
    expect(fake.methods.length).toBeGreaterThan(0);
    expect(new Set(fake.methods)).toEqual(new Set(["GET"]));
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("陽性対照（4-3）: Fake は GET 以外の送信を実際に記録する", async () => {
    const fake = installFakeFetch({ [CONTRACTS_PATH]: () => jsonResponse({ items: [] }) });
    await fetch(CONTRACTS_PATH, { method: "POST", body: "{}" });
    expect(new Set(fake.methods)).toEqual(new Set(["POST"]));
  });
});
