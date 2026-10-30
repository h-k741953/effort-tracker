/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { StatusBadge, type StatusBadgeState } from "@/components/status-badge";
import { thisMonthPath, workMonthPath } from "@/lib/screen-routes";
import { WORK_MONTH_LIST_PAGE_SIZE } from "@/lib/work-month-list";
import {
  CONTRACTS_PATH,
  CONTRACT_A,
  CONTRACT_B,
  CONTRACT_C,
  OWN_PATH,
  OWN_ROWS,
  contractsBody,
  installFakeFetch,
  jsonResponse,
  pagedListHandler,
  row,
  type TestRow,
} from "@/test-support/work-month-list";
import { OwnWorkMonthsView } from "./own-work-months-view";

// docs/specs/work-month-listing-ui.md AC-1-2・1-3・4-1・4-2・5-1・6-1・6-2・7-1・7-3・9-1・9-4・9-6・9-6-a・9-6-b・9-8〜9-14。
// 設計 W-2・W-3・W-4・T-0・T-2。
// 差し替えは next/navigation（Next のランタイム）だけを vi.mock し、fetch は手書きの Fake を大域へ置く。
// 自前のモジュールは差し替えない（T-0-1）。ページあたり件数は定数を輸入する（4-2）。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const CONTRACTS = [CONTRACT_A, CONTRACT_B, CONTRACT_C];
const ERROR_TEXT = "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";

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

function backend(rows: TestRow[] = OWN_ROWS, contracts = CONTRACTS, chunk = 1000) {
  return installFakeFetch({
    [OWN_PATH]: pagedListHandler(rows, chunk),
    [CONTRACTS_PATH]: () => jsonResponse(contractsBody(contracts)),
  });
}

async function loaded() {
  return screen.findByRole("table", { name: "すべての勤務月" });
}

function tableRows() {
  const table = screen.getByRole("table", { name: "すべての勤務月" });
  return within(table).getAllByRole("row").slice(1); // 先頭は見出し行
}

function badgeText(state: StatusBadgeState) {
  const { container, unmount } = render(<StatusBadge state={state} />);
  const text = container.textContent ?? "";
  unmount();
  return text;
}

function openLinkName(r: TestRow) {
  return `${r.contractDisplayName} × ${formatted(r.yearMonth)}を開く`;
}

function formatted(yearMonth: string) {
  const [y, m] = yearMonth.split("-");
  return `${y}年${Number(m)}月`;
}

function filterSelect(name: string) {
  const search = screen.getByRole("search", { name: "勤務月の絞り込み" });
  return within(search).getByRole("combobox", { name }) as HTMLSelectElement;
}

function choose(select: HTMLSelectElement, optionName: string) {
  const option = within(select).getByRole("option", { name: optionName }) as HTMLOptionElement;
  fireEvent.change(select, { target: { value: option.value } });
}

/** 対象年月がすべて異なる、同一契約・同一状態の n 行（降順）。 */
function manyRows(n: number): TestRow[] {
  const rows: TestRow[] = [];
  for (let i = n - 1; i >= 0; i--) {
    const year = 2000 + Math.floor(i / 12);
    const month = String((i % 12) + 1).padStart(2, "0");
    rows.push(row(CONTRACT_A, `${year}-${month}`, "Draft"));
  }
  return rows;
}

describe("通常の表示（1-2・1-3・4-1・5-1・W-4）", () => {
  it("見出しと説明を出す", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    expect(screen.getByRole("heading", { level: 1, name: "自分の勤務月" })).toBeTruthy();
    expect(screen.getByText("担当しているすべての契約の勤務月です。")).toBeTruthy();
  });

  it("1-3: 3つの状態の行がすべて載り、1-2: 契約表示名・「YYYY年M月」・状態バッジを示す（応答の並びのまま。4-1）", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const rows = tableRows();
    expect(rows).toHaveLength(OWN_ROWS.length);
    OWN_ROWS.forEach((r, i) => {
      const text = rows[i].textContent ?? "";
      expect(text).toContain(r.contractDisplayName);
      expect(text).toContain(formatted(r.yearMonth));
      expect(text).toContain(badgeText(r.state));
    });
    const states = new Set(OWN_ROWS.map((r) => r.state));
    expect(states.size).toBe(3);
  });

  it("表の列見出しは「契約」「対象年月」「状態」と視覚的に隠した「操作」", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    const table = await loaded();
    expect(within(table).getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "契約",
      "対象年月",
      "状態",
      "操作",
    ]);
  });

  it("5-1: 各行にリンク「開く」があり、アクセシブル名は「<契約表示名> × <対象年月>を開く」・行き先は勤務月画面", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const rows = tableRows();
    OWN_ROWS.forEach((r, i) => {
      const link = within(rows[i]).getByRole("link", { name: openLinkName(r) });
      expect(link.textContent).toBe("開く");
      expect(link.getAttribute("href")).toBe(workMonthPath(r.contractId, r.yearMonth));
    });
  });

  it("取得中は一覧の領域に aria-busy=\"true\" を付け、取得後は付けない", async () => {
    vi.stubGlobal("fetch", () => new Promise<Response>(() => {}));
    const { container } = render(<OwnWorkMonthsView />);
    expect(container.querySelector('[aria-busy="true"]')).not.toBeNull();
    cleanup();

    backend();
    const second = render(<OwnWorkMonthsView />);
    await loaded();
    expect(second.container.querySelector('[aria-busy="true"]')).toBeNull();
  });
});

describe("取得の要求（1-1・W-3）", () => {
  it("R-2 に engineerId・limit・state を付けず、offset=0 から GET する。R-10 に JST の当月の yearMonth を付ける", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-31T14:59:59Z"));
    const fake = backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const own = fake.urls.filter((u) => u.pathname === OWN_PATH);
    expect(own).toHaveLength(1);
    expect(own[0].searchParams.get("offset")).toBe("0");
    for (const key of ["engineerId", "limit", "state"]) {
      expect(own[0].searchParams.has(key)).toBe(false);
    }
    const contracts = fake.urls.filter((u) => u.pathname === CONTRACTS_PATH);
    expect(contracts).toHaveLength(1);
    expect(contracts[0].searchParams.get("yearMonth")).toBe("2026-10");
  });

  it("月の境目: 2026-10-31T15:00:00Z（JST では 11 月）は yearMonth=2026-11", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-31T15:00:00Z"));
    const fake = backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const contracts = fake.urls.filter((u) => u.pathname === CONTRACTS_PATH);
    expect(contracts[0].searchParams.get("yearMonth")).toBe("2026-11");
  });

  it("offset を受け取った件数ずつ進め、全件を取得する（2ページ以上）", async () => {
    const rows = [...OWN_ROWS, row(CONTRACT_A, "2026-07", "Draft")]; // 5件
    const fake = backend(rows, CONTRACTS, 2);
    render(<OwnWorkMonthsView />);
    await loaded();
    expect(fake.offsets(OWN_PATH)).toEqual(["0", "2", "4"]);
    expect(tableRows()).toHaveLength(Math.min(rows.length, WORK_MONTH_LIST_PAGE_SIZE));
  });

  it("items が空の応答で止まる（total に届かなくても無限に要求しない）", async () => {
    const fake = installFakeFetch({
      [OWN_PATH]: (url) => {
        const offset = Number(url.searchParams.get("offset") ?? "0");
        const items = offset === 0 ? OWN_ROWS.slice(0, 2) : [];
        return jsonResponse({ items, total: 10, limit: 2, offset });
      },
      [CONTRACTS_PATH]: () => jsonResponse(contractsBody(CONTRACTS)),
    });
    render(<OwnWorkMonthsView />);
    await loaded();
    expect(fake.offsets(OWN_PATH)).toEqual(["0", "2"]);
    expect(tableRows()).toHaveLength(2);
  });

  it("(契約, 対象年月) が重複した行は、先に来たものを残す", async () => {
    const dup = row(CONTRACT_A, "2026-10", "Approved"); // 先頭と同じ契約・年月で状態が違う
    const rows = [OWN_ROWS[0], OWN_ROWS[1], dup, OWN_ROWS[2]];
    backend(rows, CONTRACTS, 2);
    render(<OwnWorkMonthsView />);
    await loaded();
    const links = within(screen.getByRole("table", { name: "すべての勤務月" })).getAllByRole("link", {
      name: openLinkName(OWN_ROWS[0]),
    });
    expect(links).toHaveLength(1);
    const target = tableRows().find((r) => within(r).queryByRole("link", { name: openLinkName(OWN_ROWS[0]) }));
    expect(target?.textContent).toContain(badgeText("PendingApproval"));
    expect(target?.textContent).not.toContain(badgeText("Approved"));
  });
});

describe("直近（6-1・6-3）", () => {
  it("リンク「直近の勤務月を開く」が絞る前の先頭の勤務月を指す。絞り込んでも変わらない", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const expected = workMonthPath(OWN_ROWS[0].contractId, OWN_ROWS[0].yearMonth);
    expect(screen.getByRole("link", { name: "直近の勤務月を開く" }).getAttribute("href")).toBe(expected);

    choose(filterSelect("状態"), badgeText("Approved"));
    expect(tableRows()).toHaveLength(2);
    expect(screen.getByRole("link", { name: "直近の勤務月を開く" }).getAttribute("href")).toBe(expected);
  });
});

describe("絞り込み（9-1・9-4〜9-6-b・9-8・9-9・9-11・9-14）", () => {
  it("9-1・9-14・9-4: role=search の領域に「契約」「対象年月」「状態」の選択があり、既定は「すべて」（値は空文字）", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const search = screen.getByRole("search", { name: "勤務月の絞り込み" });
    expect(within(search).getAllByRole("combobox")).toHaveLength(3);
    for (const name of ["契約", "対象年月", "状態"]) {
      const select = filterSelect(name);
      expect(select.value).toBe("");
      expect(within(select).getByRole("option", { name: "すべて" })).toBeTruthy();
    }
  });

  it("9-4: 既定のとき入力と同じ行を同じ順で載せる", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const links = tableRows().map((r) => within(r).getByRole("link").getAttribute("href"));
    expect(links).toEqual(OWN_ROWS.map((r) => workMonthPath(r.contractId, r.yearMonth)));
  });

  it("9-6: 契約の選択肢は R-10 の契約（勤務月の無い契約を含む）。表示は契約表示名で応答の順", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const labels = within(filterSelect("契約"))
      .getAllByRole("option")
      .map((o) => o.textContent)
      .filter((t) => t !== "すべて");
    expect(labels).toEqual(CONTRACTS.map((c) => c.contractDisplayName));
  });

  it("9-6-a: 対象年月の選択肢は載る行の対象年月（重複なし・降順）。表示は「YYYY年M月」", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const labels = within(filterSelect("対象年月"))
      .getAllByRole("option")
      .map((o) => o.textContent)
      .filter((t) => t !== "すべて");
    expect(labels).toEqual(["2026年10月", "2026年9月", "2026年8月"]);
  });

  it("9-6-b: 状態の選択肢は3つで、表示は StatusBadge が描く語と一致する", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const labels = within(filterSelect("状態"))
      .getAllByRole("option")
      .map((o) => o.textContent)
      .filter((t) => t !== "すべて");
    const expected = (["Draft", "PendingApproval", "Approved"] as const).map(badgeText);
    expect([...labels].sort()).toEqual([...expected].sort());
  });

  it("9-5・9-8: 契約で絞ると、その契約の行だけが入力の順のまま残る", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    choose(filterSelect("契約"), CONTRACT_A.contractDisplayName);
    const expected = OWN_ROWS.filter((r) => r.contractId === CONTRACT_A.contractId);
    expect(tableRows().map((r) => within(r).getByRole("link").getAttribute("href"))).toEqual(
      expected.map((r) => workMonthPath(r.contractId, r.yearMonth)),
    );
  });

  it("9-5: 複数の条件はすべて満たす行だけを載せる（契約 × 対象年月 × 状態）", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    choose(filterSelect("契約"), CONTRACT_A.contractDisplayName);
    choose(filterSelect("対象年月"), "2026年9月");
    choose(filterSelect("状態"), badgeText("Approved"));
    const rows = tableRows();
    expect(rows).toHaveLength(1);
    expect(within(rows[0]).getByRole("link").getAttribute("href")).toBe(workMonthPath("c-1", "2026-09"));

    choose(filterSelect("状態"), badgeText("Draft"));
    expect(screen.queryByRole("table", { name: "すべての勤務月" })).toBeNull();
  });

  it("9-11: 条件が無いとき「条件をクリア」は出ず、1つ以上あるとき出て、押すとすべて既定へ戻り再び消える", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    const search = () => screen.getByRole("search", { name: "勤務月の絞り込み" });
    expect(within(search()).queryByRole("button", { name: "条件をクリア" })).toBeNull();

    choose(filterSelect("状態"), badgeText("Approved"));
    choose(filterSelect("契約"), CONTRACT_A.contractDisplayName);
    fireEvent.click(within(search()).getByRole("button", { name: "条件をクリア" }));

    for (const name of ["契約", "対象年月", "状態"]) {
      expect(filterSelect(name).value).toBe("");
    }
    expect(within(search()).queryByRole("button", { name: "条件をクリア" })).toBeNull();
    expect(tableRows()).toHaveLength(OWN_ROWS.length);
  });

  it("9-10: 絞り込みの結果が0件のとき、条件に合う勤務月が無いことと解除の導線を示し、初回入力の導線を出さない", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    choose(filterSelect("契約"), CONTRACT_C.contractDisplayName); // 勤務月の無い契約
    expect(screen.getByText("条件に合う勤務月はありません")).toBeTruthy();
    expect(screen.getByText("絞り込みの条件を変えるか、クリアしてください。")).toBeTruthy();
    expect(screen.queryByRole("table", { name: "すべての勤務月" })).toBeNull();
    expect(screen.queryByText("勤務月がまだありません")).toBeNull();
    expect(screen.queryByRole("link", { name: "稼働実績を入力する" })).toBeNull();
    expect(screen.queryByRole("button", { name: "稼働実績を入力する" })).toBeNull();

    const clear = screen.getAllByRole("button", { name: "条件をクリア" });
    expect(clear.length).toBeGreaterThanOrEqual(1);
    fireEvent.click(clear[clear.length - 1]);
    expect(tableRows()).toHaveLength(OWN_ROWS.length);
  });
});

describe("ページ（4-2・9-9）", () => {
  it("ちょうど1ページ分（件数 = 定数）のとき、ページは1つで「次へ」が押せない", async () => {
    backend(manyRows(WORK_MONTH_LIST_PAGE_SIZE));
    render(<OwnWorkMonthsView />);
    await loaded();
    expect(tableRows()).toHaveLength(WORK_MONTH_LIST_PAGE_SIZE);
    const nav = screen.getByRole("navigation", { name: "ページ送り" });
    expect((within(nav).getByRole("button", { name: "次へ" }) as HTMLButtonElement).disabled).toBe(true);
    expect((within(nav).getByRole("button", { name: "前へ" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("1件超えると2ページになり、2ページ目に1件が出る", async () => {
    backend(manyRows(WORK_MONTH_LIST_PAGE_SIZE + 1));
    render(<OwnWorkMonthsView />);
    await loaded();
    expect(tableRows()).toHaveLength(WORK_MONTH_LIST_PAGE_SIZE);
    fireEvent.click(screen.getByRole("button", { name: "次へ" }));
    expect(tableRows()).toHaveLength(1);
  });

  it.each([
    ["状態", () => badgeText("Draft")], // 全行が Draft。絞っても件数は変わらない
    ["契約", () => CONTRACT_A.contractDisplayName], // 全行が A。絞っても件数は変わらない
  ])("9-9: 条件（%s）を変えると1ページ目へ戻る", async (name, optionName) => {
    backend(manyRows(WORK_MONTH_LIST_PAGE_SIZE + 1), CONTRACTS);
    render(<OwnWorkMonthsView />);
    await loaded();
    fireEvent.click(screen.getByRole("button", { name: "次へ" }));
    expect(tableRows()).toHaveLength(1);

    choose(filterSelect(name), optionName());
    expect(tableRows()).toHaveLength(WORK_MONTH_LIST_PAGE_SIZE);
  });
});

describe("空状態（6-2・7-1。Q-8 付随）", () => {
  it("勤務月0件・契約0件: 空状態の文言を示し、初回入力の導線（リンク・ボタン）を出さない", async () => {
    backend([], []);
    const { container } = render(<OwnWorkMonthsView />);
    const title = await screen.findByText("勤務月がまだありません");
    expect(screen.getByText("稼働実績を入力すると、その月の勤務月がここに表示されます。")).toBeTruthy();
    expect(title.parentElement?.children).toHaveLength(2); // title と description だけ（action 無し）
    expect(screen.queryByRole("link", { name: "稼働実績を入力する" })).toBeNull();
    expect(screen.queryByRole("button", { name: "稼働実績を入力する" })).toBeNull();
    expect(container.querySelector("table")).toBeNull();
  });

  it("勤務月0件・契約1件: リンク「稼働実績を入力する」がその契約の thisMonthPath を指す", async () => {
    backend([], [CONTRACT_A]);
    render(<OwnWorkMonthsView />);
    await screen.findByText("勤務月がまだありません");
    const link = await screen.findByRole("link", { name: "稼働実績を入力する" });
    expect(link.getAttribute("href")).toBe(thisMonthPath(CONTRACT_A.contractId));
  });

  it("勤務月0件・契約2件以上: button が閉じて出て、押すと契約の選択肢が入力の順・文書の順で button より後に開き、再び押すと閉じる", async () => {
    const fake = backend([], CONTRACTS);
    render(<OwnWorkMonthsView />);
    await screen.findByText("勤務月がまだありません");
    const button = await screen.findByRole("button", { name: "稼働実績を入力する" });
    expect(button.getAttribute("aria-expanded")).toBe("false");
    for (const c of CONTRACTS) {
      expect(screen.queryByRole("link", { name: c.contractDisplayName })).toBeNull();
    }
    const before = fake.urls.length;

    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    const links = CONTRACTS.map((c) => screen.getByRole("link", { name: c.contractDisplayName }));
    links.forEach((link, i) => {
      expect(link.getAttribute("href")).toBe(thisMonthPath(CONTRACTS[i].contractId));
      expect(button.compareDocumentPosition(link) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      if (i > 0) {
        expect(links[i - 1].compareDocumentPosition(link) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      }
    });

    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("false");
    for (const c of CONTRACTS) {
      expect(screen.queryByRole("link", { name: c.contractDisplayName })).toBeNull();
    }
    expect(fake.urls.length).toBe(before); // 開閉で通信しない
  });
});

describe("契約を指定して開く（9-12。W-4: 値が選択肢にあれば初期値、無ければ無視）", () => {
  it("選択肢にある契約: 契約の条件の初期値になり、その契約の行だけが載る", async () => {
    backend();
    render(<OwnWorkMonthsView initialContractId={CONTRACT_B.contractId} />);
    await loaded();
    const select = filterSelect("契約");
    expect(select.selectedOptions[0].textContent).toBe(CONTRACT_B.contractDisplayName);
    expect(tableRows()).toHaveLength(1);
    expect(within(tableRows()[0]).getByRole("link").getAttribute("href")).toBe(workMonthPath("c-2", "2026-10"));
  });

  it("選択肢に無い契約: 無視して「すべて」とし、全行が載る", async () => {
    backend();
    render(<OwnWorkMonthsView initialContractId="c-unknown" />);
    await loaded();
    expect(filterSelect("契約").value).toBe("");
    expect(tableRows()).toHaveLength(OWN_ROWS.length);
  });

  it("指定が無い: 「すべて」", async () => {
    backend();
    render(<OwnWorkMonthsView />);
    await loaded();
    expect(filterSelect("契約").value).toBe("");
  });
});

describe("取得の失敗（7-3）", () => {
  it.each([
    ["R-2 が 500", { own: 500, contracts: 200 }],
    ["R-2 が 429", { own: 429, contracts: 200 }],
    ["R-10 が 500（行だけを出さない）", { own: 200, contracts: 500 }],
  ])("%s → role=alert に文言を出し、表を出さない", async (_name, status) => {
    installFakeFetch({
      [OWN_PATH]: status.own === 200 ? pagedListHandler(OWN_ROWS, 1000) : () => jsonResponse({}, status.own),
      [CONTRACTS_PATH]: () =>
        status.contracts === 200 ? jsonResponse(contractsBody(CONTRACTS)) : jsonResponse({}, status.contracts),
    });
    render(<OwnWorkMonthsView />);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(ERROR_TEXT);
    expect(screen.queryByRole("table", { name: "すべての勤務月" })).toBeNull();
  });

  it("fetch が例外 → role=alert に文言を出す", async () => {
    vi.stubGlobal("fetch", async () => {
      throw new Error("network");
    });
    render(<OwnWorkMonthsView />);
    expect((await screen.findByRole("alert")).textContent).toContain(ERROR_TEXT);
  });

  it("再試行で再び GET し、成功すればエラーが消えて表が出る", async () => {
    let failing = true;
    const fake = installFakeFetch({
      [OWN_PATH]: (url) =>
        failing ? jsonResponse({}, 500) : pagedListHandler(OWN_ROWS, 1000)(url),
      [CONTRACTS_PATH]: () => jsonResponse(contractsBody(CONTRACTS)),
    });
    render(<OwnWorkMonthsView />);
    await screen.findByRole("alert");
    const before = fake.count(OWN_PATH);

    failing = false;
    fireEvent.click(screen.getByRole("button", { name: "再試行" }));
    await loaded();
    expect(fake.count(OWN_PATH)).toBeGreaterThan(before);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it.each([401, 403])("%i → router.refresh がちょうど1回（区分の判定をサーバーに任せる）", async (status) => {
    installFakeFetch({
      [OWN_PATH]: () => jsonResponse({}, status),
      [CONTRACTS_PATH]: () => jsonResponse(contractsBody(CONTRACTS)),
    });
    render(<OwnWorkMonthsView />);
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(router.refresh).toHaveBeenCalledTimes(1);
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });
});

describe("状態を変えない（9-13）・URL に書き戻さない（W-2）", () => {
  it("取得・絞り込み・ページ送り・解除のどの操作でも GET 以外を送らず、push/replace を呼ばない", async () => {
    const fake = backend(manyRows(WORK_MONTH_LIST_PAGE_SIZE + 1), CONTRACTS);
    render(<OwnWorkMonthsView />);
    await loaded();
    fireEvent.click(screen.getByRole("button", { name: "次へ" }));
    choose(filterSelect("状態"), badgeText("Draft"));
    fireEvent.click(
      within(screen.getByRole("search", { name: "勤務月の絞り込み" })).getByRole("button", {
        name: "条件をクリア",
      }),
    );
    expect(fake.methods.length).toBeGreaterThan(0);
    expect(new Set(fake.methods)).toEqual(new Set(["GET"]));
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });
});
