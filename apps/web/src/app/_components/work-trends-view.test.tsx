/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { EmptyState } from "@/components/empty-state";
import type { ContractListItem } from "@/lib/contract-list";
import type { WorkTrendRow } from "@/lib/work-trends";
import { CONTRACTS_PATH, installFakeFetch, jsonResponse } from "@/test-support/work-month-list";
import { WorkTrendsView } from "./work-trends-view";

// docs/specs/work-trends-ui.md 1-3・1-4・3-1〜3-4・6-1・6-2・7-6・8-1・8-2・8-4。設計 SP-2・SP-3・SP-6・S-1。
// next/navigation だけを vi.mock し、fetch は手書きの Fake を大域へ置く。文言は仕様の字面をここへ書く。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const WORK_TRENDS_PATH = "/api/work-trends";
const ERROR_TEXT = "稼働の推移を読み込めませんでした。通信状態を確認して、もう一度お試しください。";
const CONTRACT_ERROR_TEXT = "契約を読み込めませんでした。通信状態を確認して、もう一度お試しください。";
const DESCRIPTION = "月ごとの総稼働時間と、契約の精算幅との関係です。";

type Hm = { hours: number; minutes: number };
const hm = (hours: number, minutes = 0): Hm => ({ hours, minutes });
const RANGE = { lowerBound: hm(140), upperBound: hm(180) };

function contract(id: string, name: string): ContractListItem {
  return {
    contractId: id,
    contractDisplayName: name,
    settlementRange: RANGE,
    contractPeriod: { start: "2025-04", end: null },
    workMonth: { yearMonth: "2026-11", state: "Draft", generated: false },
  };
}
function tr(contractId: string, yearMonth: string, o: Partial<WorkTrendRow> = {}): WorkTrendRow {
  return {
    contractId,
    yearMonth,
    state: "Approved",
    settlementRange: RANGE,
    totalHours: hm(160),
    excess: hm(0),
    shortfall: hm(0),
    ...o,
  };
}

// R-10 の順は c-2・c-1・c-3（並べ替えないことを読む＝1-3）。c-3 は勤務月が無い。
const B = contract("c-2", "B社 / 開発");
const A = contract("c-1", "A社 / 保守");
const C = contract("c-3", "C社 / 運用");
const CONTRACTS = [B, A, C];

// R-11 は契約識別子の昇順・同じ契約の中は対象年月の昇順（AC-10-5）。
const ROWS: WorkTrendRow[] = [
  tr("c-1", "2026-05", { state: "PendingApproval", totalHours: hm(160) }),
  tr("c-1", "2026-06", { totalHours: hm(161) }),
  tr("c-1", "2026-07", { totalHours: hm(150) }),
  tr("c-1", "2026-08", { state: "Draft", totalHours: hm(40), excess: null, shortfall: null }),
  tr("c-2", "2026-03", { totalHours: hm(170) }),
  tr("c-2", "2026-04", { totalHours: hm(172) }),
];

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

type Responder = () => Response;
const okContracts = (items: ContractListItem[] = CONTRACTS): Responder => () => jsonResponse({ items });
const okTrends = (items: WorkTrendRow[] = ROWS): Responder => () => jsonResponse({ items });
const status = (code: number): Responder => () => jsonResponse({}, code);

function backend(contracts: Responder = okContracts(), trends: Responder = okTrends()) {
  return installFakeFetch({ [CONTRACTS_PATH]: contracts, [WORK_TRENDS_PATH]: trends });
}

// ---- 読み取り補助 ----

async function loaded() {
  return screen.findByRole("search", { name: "稼働の推移の絞り込み" });
}
const search = () => screen.getByRole("search", { name: "稼働の推移の絞り込み" });
const contractSelect = () => within(search()).getByRole("combobox", { name: "契約" }) as HTMLSelectElement;
const periodGroup = () => within(search()).getByRole("group", { name: "期間" });
const startSelect = () => within(periodGroup()).getByRole("combobox", { name: "期間（開始）" }) as HTMLSelectElement;
const endSelect = () => within(periodGroup()).getByRole("combobox", { name: "期間（終了）" }) as HTMLSelectElement;
const optionTexts = (select: HTMLSelectElement) => within(select).getAllByRole("option").map((o) => o.textContent);
const change = (select: HTMLSelectElement, value: string) => fireEvent.change(select, { target: { value } });

function figure(): HTMLElement {
  return screen.getByRole("img", { name: "月ごとの総稼働時間" });
}
const MONTH_LABEL = /^(\d{4}年)?\d{1,2}月$/;
function axisLabels(): string[] {
  return [...figure().querySelectorAll<HTMLElement>("*")]
    .filter((el) => el.children.length === 0 && MONTH_LABEL.test((el.textContent ?? "").trim()))
    .map((el) => (el.textContent ?? "").trim());
}
function summaryBody(heading: string): string {
  const h2 = screen.getByRole("heading", { level: 2, name: heading });
  return ((h2.parentElement!.textContent ?? "").replace(h2.textContent ?? "", "")).replace(/\s+/g, " ").trim();
}

/** 絞り込み・要約・図・表のいずれかが出ているか。 */
function shows(container: HTMLElement) {
  return {
    search: container.querySelector("[role='search']") !== null,
    headings: container.querySelectorAll("h2").length,
    figure: container.querySelector("[role='img']") !== null,
    table: container.querySelector("table") !== null,
    selects: container.querySelectorAll("select").length,
  };
}
const NOTHING = { search: false, headings: 0, figure: false, table: false, selects: 0 };

const INTERACTIVE = "a, button, [role='link'], [role='button']";
const CONTROLS_EXCEPT_SELECT =
  "button, input, textarea, form, [role='button'], [role='menuitem'], [role='menu'], [role='checkbox'], [role='radio'], [role='combobox'], [role='listbox'], [role='switch'], [role='textbox']";

describe("通常の表示（7-6・1-3・1-4）", () => {
  it("h1 は 稼働の推移、説明文は S-1 の字面", async () => {
    backend();
    render(<WorkTrendsView />);
    await loaded();
    expect(screen.getByRole("heading", { level: 1, name: "稼働の推移" })).toBeTruthy();
    expect(screen.getByText(DESCRIPTION)).toBeTruthy();
  });

  it("絞り込みの領域は「稼働の推移の絞り込み」、契約の選択のラベルは「契約」、期間は「期間」のグループに「期間（開始）」「期間（終了）」", async () => {
    backend();
    render(<WorkTrendsView />);
    await loaded();
    expect(contractSelect()).toBeTruthy();
    expect(startSelect()).toBeTruthy();
    expect(endSelect()).toBeTruthy();
    expect(within(search()).getAllByRole("combobox")).toHaveLength(3);
  });

  it("1-3・1-4: 契約の選択肢は R-10 の順・契約表示名で「すべて」を置かず、既定は先頭の契約", async () => {
    backend();
    render(<WorkTrendsView />);
    await loaded();
    expect(optionTexts(contractSelect())).toEqual(["B社 / 開発", "A社 / 保守", "C社 / 運用"]);
    expect(contractSelect().value).toBe("c-2");
  });

  it("1-2: 選んだ契約の月だけを示す（既定の c-2 の図は 3月・4月だけ。c-1 の月を混ぜない）", async () => {
    backend();
    render(<WorkTrendsView />);
    await loaded();
    expect(axisLabels()).toEqual(["2026年3月", "4月"]);
    expect(summaryBody("平均稼働（確定した月）")).toBe("171.0 時間");
    expect(summaryBody("精算幅に収まった月")).toBe("2 / 2 か月");
  });

  it("3-1: 期間の既定は選んだ契約の全月（開始は最も古い月、終了は最も新しい月）。選択肢は昇順で「2026年3月」の形", async () => {
    backend();
    render(<WorkTrendsView />);
    await loaded();
    expect(startSelect().value).toBe("2026-03");
    expect(endSelect().value).toBe("2026-04");
    expect(optionTexts(startSelect())).toEqual(["2026年3月", "2026年4月"]);
    expect(optionTexts(endSelect())).toEqual(["2026年3月", "2026年4月"]); // 終了の選択肢は開始と同じかそれより後
  });
});

describe("期間の選択（3-1〜3-4）", () => {
  async function onContractA() {
    backend();
    render(<WorkTrendsView />);
    await loaded();
    change(contractSelect(), "c-1");
  }

  it("契約を c-1 に変えると、期間の既定が c-1 の全月（2026年5月〜8月）になり、図と要約も c-1 の月になる", async () => {
    await onContractA();
    expect(startSelect().value).toBe("2026-05");
    expect(endSelect().value).toBe("2026-08");
    expect(optionTexts(startSelect())).toEqual(["2026年5月", "2026年6月", "2026年7月", "2026年8月"]);
    expect(axisLabels()).toEqual(["2026年5月", "6月", "7月", "8月"]);
    // 確定した月は 160・161・150（入力中の 8月は含めない）→ 157.0。収まった 3 / 3。
    expect(summaryBody("平均稼働（確定した月）")).toBe("157.0 時間");
    expect(summaryBody("精算幅に収まった月")).toBe("3 / 3 か月");
  });

  it("3-3: 終了の選択肢は開始と同じかそれより後だけ。開始を終了より後へ変えると、終了が開始にそろう", async () => {
    await onContractA();
    change(startSelect(), "2026-06");
    expect(endSelect().value).toBe("2026-08"); // 終了より前へ変えたときはそろえない
    expect(optionTexts(endSelect())).toEqual(["2026年6月", "2026年7月", "2026年8月"]);

    change(endSelect(), "2026-07");
    expect(endSelect().value).toBe("2026-07");
    expect(optionTexts(startSelect())).toEqual(["2026年5月", "2026年6月", "2026年7月", "2026年8月"]);

    change(startSelect(), "2026-08"); // 終了 2026-07 より後
    expect(startSelect().value).toBe("2026-08");
    expect(endSelect().value).toBe("2026-08");
    expect(optionTexts(endSelect())).toEqual(["2026年8月"]);
  });

  it("3-6・3-6-a: 期間を変えると、平均・精算幅に収まった月・図が選んだ期間の月だけになる", async () => {
    await onContractA();
    change(endSelect(), "2026-06");
    expect(axisLabels()).toEqual(["2026年5月", "6月"]);
    expect(summaryBody("平均稼働（確定した月）")).toBe("160.5 時間");
    expect(summaryBody("精算幅に収まった月")).toBe("2 / 2 か月");
    // 入力中の 8月だけの期間 → 確定した月が0件
    change(startSelect(), "2026-08");
    expect(axisLabels()).toEqual(["2026年8月"]);
    expect(summaryBody("平均稼働（確定した月）")).toContain("確定した月はありません");
    expect(summaryBody("精算幅に収まった月")).toBe("0 / 0 か月");
  });

  it("3-4: 契約を変えると期間を既定へ戻す（c-1 で絞り込み → c-2 → c-1 に戻ると 5月〜8月）", async () => {
    await onContractA();
    change(startSelect(), "2026-06");
    change(endSelect(), "2026-07");
    expect(axisLabels()).toEqual(["2026年6月", "7月"]);

    change(contractSelect(), "c-2");
    expect(startSelect().value).toBe("2026-03");
    expect(endSelect().value).toBe("2026-04");

    change(contractSelect(), "c-1");
    expect(startSelect().value).toBe("2026-05");
    expect(endSelect().value).toBe("2026-08");
    expect(axisLabels()).toEqual(["2026年5月", "6月", "7月", "8月"]);
  });

  it("6-2・SP-2: 契約・期間の選択を変えても fetch を呼ばない（R-10・R-11 は1回ずつ、GET だけ）", async () => {
    const fake = backend();
    render(<WorkTrendsView />);
    await loaded();
    expect(fake.urls).toHaveLength(2);
    change(contractSelect(), "c-1");
    change(startSelect(), "2026-06");
    change(endSelect(), "2026-07");
    change(startSelect(), "2026-07");
    change(contractSelect(), "c-2");
    expect(fake.urls).toHaveLength(2);
    expect(fake.count(CONTRACTS_PATH)).toBe(1);
    expect(fake.count(WORK_TRENDS_PATH)).toBe(1);
    expect(fake.methods).toEqual(["GET", "GET"]);
  });

  it("陽性対照（6-2）: 選択の変更のたびに取得し直す実装なら、同じ判定（要求の数）で検出される", async () => {
    const fake = installFakeFetch({ [CONTRACTS_PATH]: okContracts(), [WORK_TRENDS_PATH]: okTrends() });
    await fetch(WORK_TRENDS_PATH);
    await fetch(WORK_TRENDS_PATH);
    expect(fake.count(WORK_TRENDS_PATH)).toBe(2);
    expect(fake.urls).not.toHaveLength(1);
  });
});

describe("取得の要求（SP-2）", () => {
  it("R-10（yearMonth だけ）と R-11（クエリなし）へ GET を1回ずつ。engineerId を付けない", async () => {
    const fake = backend();
    render(<WorkTrendsView />);
    await loaded();
    expect(fake.count(CONTRACTS_PATH)).toBe(1);
    expect(fake.count(WORK_TRENDS_PATH)).toBe(1);
    const r10 = fake.urls.find((u) => u.pathname === CONTRACTS_PATH)!;
    const r11 = fake.urls.find((u) => u.pathname === WORK_TRENDS_PATH)!;
    expect([...r10.searchParams.keys()]).toEqual(["yearMonth"]);
    expect(r11.search).toBe("");
    expect(fake.urls.some((u) => u.searchParams.has("engineerId"))).toBe(false);
    expect(fake.methods).toEqual(["GET", "GET"]);
  });
});

describe("取得中", () => {
  it("領域に aria-busy=\"true\" があり、文言を出さず、絞り込み・要約・図・表・エラー・空状態も出さない。取得後は付けない", async () => {
    vi.stubGlobal("fetch", () => new Promise<Response>(() => {}));
    const { container } = render(<WorkTrendsView />);
    const busy = container.querySelector('[aria-busy="true"]');
    expect(busy).not.toBeNull();
    expect(busy!.textContent).toBe("");
    expect(shows(container)).toEqual(NOTHING);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText("担当している契約はありません")).toBeNull();
    cleanup();

    backend();
    const second = render(<WorkTrendsView />);
    await loaded();
    expect(second.container.querySelector('[aria-busy="true"]')).toBeNull();
  });

  it("陽性対照: 文言を持つ aria-busy の領域は、同じ判定（textContent が空）で検出される", () => {
    const { container, unmount } = render(<div aria-busy="true">読み込み中</div>);
    expect(container.querySelector('[aria-busy="true"]')!.textContent).not.toBe("");
    unmount();
  });
});

describe("0件（8-1・8-2）", () => {
  it.each<[string, WorkTrendRow[]]>([
    ["R-11 も空", []],
    ["R-11 に行がある（R-10 に契約が無ければ契約は選べない）", ROWS],
  ])("8-1: 契約が0件（%s）→ EmptyState「担当している契約はありません」だけが出て、絞り込み・要約・図・表が無い", async (_name, rows) => {
    backend(okContracts([]), okTrends(rows));
    const { container } = render(<WorkTrendsView />);
    await screen.findByText("担当している契約はありません");
    expect(shows(container)).toEqual(NOTHING);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(container.querySelector(INTERACTIVE)).toBeNull();
    expect(screen.getByRole("heading", { level: 1, name: "稼働の推移" })).toBeTruthy();
  });

  it("8-2: 選んだ契約の勤務月が0件 → 契約の選択は残り、期間のグループは出ず、EmptyState「この契約の勤務月はまだありません」。要約・図・表が無い", async () => {
    backend(okContracts([A, B]), okTrends(ROWS.filter((r) => r.contractId === "c-2")));
    // R-10 の先頭は A（c-1）で、R-11 に c-1 の行が無い。
    const { container } = render(<WorkTrendsView />);
    await screen.findByText("この契約の勤務月はまだありません");
    expect(search()).toBeTruthy();
    expect(contractSelect().value).toBe("c-1");
    expect(within(search()).getAllByRole("combobox")).toHaveLength(1);
    expect(within(search()).queryByRole("group", { name: "期間" })).toBeNull();
    expect(container.querySelector("[role='img']")).toBeNull();
    expect(container.querySelector("table")).toBeNull();
    expect(container.querySelectorAll("h2")).toHaveLength(0);
    expect(container.querySelector(INTERACTIVE)).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("8-2 → 勤務月のある契約に切り替えると、空状態が消えて期間のグループ・要約・図が出る", async () => {
    backend(okContracts([A, B]), okTrends(ROWS.filter((r) => r.contractId === "c-2")));
    render(<WorkTrendsView />);
    await screen.findByText("この契約の勤務月はまだありません");
    change(contractSelect(), "c-2");
    expect(screen.queryByText("この契約の勤務月はまだありません")).toBeNull();
    expect(periodGroup()).toBeTruthy();
    expect(figure()).toBeTruthy();
    expect(axisLabels()).toEqual(["2026年3月", "4月"]);
  });

  it("陽性対照（8-1・8-2）: action を渡した EmptyState は、同じ判定でリンク・ボタンを検出される", () => {
    const { container, unmount } = render(
      <div>
        <EmptyState title="t" action={<a href="/new">作る</a>} />
        <EmptyState title="u" action={<button type="button">作る</button>} />
      </div>,
    );
    expect(container.querySelectorAll(INTERACTIVE)).toHaveLength(2);
    unmount();
  });

  it("陽性対照（8-1・8-2）: 勤務月のある契約の画面は、同じ判定（絞り込み・見出し・図・表・選択欄）で内容が検出される", async () => {
    backend();
    const { container } = render(<WorkTrendsView />);
    await loaded();
    expect(shows(container)).toEqual({ search: true, headings: 4, figure: true, table: true, selects: 3 });
  });
});

describe("取得エラー（8-4・決定1）", () => {
  it.each<[string, Responder, Responder]>([
    ["R-10 だけ失敗（500）", status(500), okTrends()],
    ["R-11 だけ失敗（500）", okContracts(), status(500)],
    ["両方失敗（500）", status(500), status(500)],
    ["R-10 が 429", status(429), okTrends()],
    ["R-11 の本文が JSON でない", okContracts(), () => new Response("x", { status: 200 })],
    ["R-10 の items が配列でない", () => jsonResponse({ items: {} }), okTrends()],
  ])("%s → role=alert に S-1 の文を出し、絞り込み・要約・図・表を出さず、契約一覧の文も出さない", async (_name, r10, r11) => {
    backend(r10, r11);
    const { container } = render(<WorkTrendsView />);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(ERROR_TEXT);
    expect(container.textContent).not.toContain(CONTRACT_ERROR_TEXT);
    expect(shows(container)).toEqual(NOTHING);
    expect(screen.queryByText("担当している契約はありません")).toBeNull();
    expect(screen.queryByText("この契約の勤務月はまだありません")).toBeNull();
  });

  it("fetch が例外 → role=alert に S-1 の文を出し、何も出さない", async () => {
    vi.stubGlobal("fetch", async () => {
      throw new Error("network");
    });
    const { container } = render(<WorkTrendsView />);
    expect((await screen.findByRole("alert")).textContent).toContain(ERROR_TEXT);
    expect(shows(container)).toEqual(NOTHING);
  });

  it("陽性対照（8-4）: 成功した画面は、同じ判定で内容が検出され、エラーの文は出ない", async () => {
    backend();
    const { container } = render(<WorkTrendsView />);
    await loaded();
    expect(shows(container)).not.toEqual(NOTHING);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(container.textContent).not.toContain(ERROR_TEXT);
  });

  it.each<[string, boolean, boolean]>([
    ["R-10 だけ失敗", true, false],
    ["R-11 だけ失敗", false, true],
    ["両方失敗", true, true],
  ])("再試行（%s）で R-10 と R-11 を再び GET し、成功すればエラーが消えて内容が出る", async (_name, fail10, fail11) => {
    let failing = true;
    const fake = backend(
      () => (fail10 && failing ? jsonResponse({}, 500) : jsonResponse({ items: CONTRACTS })),
      () => (fail11 && failing ? jsonResponse({}, 500) : jsonResponse({ items: ROWS })),
    );
    render(<WorkTrendsView />);
    await screen.findByRole("alert");
    expect(fake.count(CONTRACTS_PATH)).toBe(1);
    expect(fake.count(WORK_TRENDS_PATH)).toBe(1);

    failing = false;
    fireEvent.click(screen.getByRole("button", { name: "再試行" }));
    await loaded();
    expect(fake.count(CONTRACTS_PATH)).toBe(2);
    expect(fake.count(WORK_TRENDS_PATH)).toBe(2);
    expect(new Set(fake.methods)).toEqual(new Set(["GET"]));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it.each<[string, Responder, Responder]>([
    ["R-10 が 401", status(401), okTrends()],
    ["R-10 が 403", status(403), okTrends()],
    ["R-11 が 401", okContracts(), status(401)],
    ["R-11 が 403", okContracts(), status(403)],
    ["R-10 が 401 で R-11 が 500", status(401), status(500)],
    ["R-10 が 500 で R-11 が 403", status(500), status(403)],
  ])("%s → router.refresh がちょうど1回で、ほかに何も出さない（エラーも出さない）", async (_name, r10, r11) => {
    backend(r10, r11);
    const { container } = render(<WorkTrendsView />);
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(router.refresh).toHaveBeenCalledTimes(1);
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
    expect(shows(container)).toEqual(NOTHING);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText("担当している契約はありません")).toBeNull();
  });
});

describe("閲覧のみ（6-1・6-2・6-3）", () => {
  it("読み込んだ状態で、選択欄（契約・期間の開始・終了の3つ）のほかに、ボタン・入力欄・フォーム・リンクなどの操作要素が無い", async () => {
    backend();
    const { container } = render(<WorkTrendsView />);
    await loaded();
    expect(container.querySelectorAll("select")).toHaveLength(3);
    const others = [...container.querySelectorAll(CONTROLS_EXCEPT_SELECT)].filter((el) => el.tagName !== "SELECT");
    expect(others).toEqual([]);
    expect(container.querySelector(INTERACTIVE)).toBeNull();
  });

  it.each([
    ["button", <button key="a" type="button">x</button>],
    ["input", <input key="a" />],
    ["checkbox", <input key="a" type="checkbox" />],
    ["radio", <input key="a" type="radio" />],
    ["textarea", <textarea key="a" />],
    ["form", <form key="a" />],
    ["menuitem", <div key="a" role="menuitem">x</div>],
    ["link", <a key="a" href="/x">x</a>],
  ])("陽性対照（6-1・6-3）: %s を含む描画は、同じ判定で検出される", (_name, el) => {
    const { container, unmount } = render(<div>{el}</div>);
    const others = [...container.querySelectorAll(`${CONTROLS_EXCEPT_SELECT}, ${INTERACTIVE}`)].filter(
      (e) => e.tagName !== "SELECT",
    );
    expect(others.length).toBeGreaterThan(0);
    unmount();
  });

  it("6-1・6-2: 描画・選択の変更・再試行のどれでも GET 以外を送らず、push/replace を呼ばない", async () => {
    let failing = true;
    const fake = backend(
      () => (failing ? jsonResponse({}, 500) : jsonResponse({ items: CONTRACTS })),
      okTrends(),
    );
    render(<WorkTrendsView />);
    await screen.findByRole("alert");
    failing = false;
    fireEvent.click(screen.getByRole("button", { name: "再試行" }));
    await loaded();
    change(contractSelect(), "c-1");
    change(startSelect(), "2026-07");
    change(endSelect(), "2026-08");
    expect(fake.methods.length).toBeGreaterThan(0);
    expect(new Set(fake.methods)).toEqual(new Set(["GET"]));
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("陽性対照（6-1・6-2）: Fake は GET 以外の送信を実際に記録する", async () => {
    const fake = installFakeFetch({ [WORK_TRENDS_PATH]: () => jsonResponse({ items: [] }) });
    await fetch(WORK_TRENDS_PATH, { method: "POST", body: "{}" });
    expect(new Set(fake.methods)).toEqual(new Set(["POST"]));
  });
});

/** 表の自身か祖先（document の根まで）が、視覚的にも支援技術にも隠されているか（sr-only・hidden・aria-hidden="true"）。 */
function hiddenChain(el: Element): string[] {
  const found: string[] = [];
  for (let cur: Element | null = el; cur !== null; cur = cur.parentElement) {
    if (cur.classList.contains("sr-only")) found.push("sr-only");
    if (cur.hasAttribute("hidden")) found.push("hidden");
    if (cur.getAttribute("aria-hidden") === "true") found.push("aria-hidden");
  }
  return found;
}

describe("表を画面に見える形で置く（決定2・SP-3。本体の側が表を隠さない）", () => {
  it("取得後の本体に描かれた表の自身と祖先（document の根まで）に、sr-only・hidden・aria-hidden=\"true\" が無い", async () => {
    backend();
    const { container } = render(<WorkTrendsView />);
    await loaded();
    const table = container.querySelector("table");
    expect(table).not.toBeNull();
    expect(container.querySelectorAll("table tbody tr").length).toBeGreaterThan(0); // 表が描かれていること
    expect(hiddenChain(table!)).toEqual([]);
    expect(screen.getByRole("table")).toBe(table);
  });
});
