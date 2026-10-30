/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { StatusBadge } from "@/components/status-badge";
import { workMonthPath } from "@/lib/screen-routes";
import { WORK_MONTH_LIST_PAGE_SIZE } from "@/lib/work-month-list";
import {
  CONTRACT_A,
  CONTRACT_B,
  PENDING_PATH,
  installFakeFetch,
  jsonResponse,
  pagedListHandler,
  row,
  type TestRow,
} from "@/test-support/work-month-list";
import { PendingApprovalsView } from "./pending-approvals-view";

// docs/specs/work-month-listing-ui.md AC-2-1・2-3・2-5・4-1・4-2・5-1・7-2・7-3・9-2・9-6・9-6-a・9-9〜9-11。
// 設計 W-3・W-5・T-2。next/navigation だけを vi.mock し、fetch は手書きの Fake（T-0-1）。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const ERROR_TEXT = "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";

/** 承認待ち（PendingApproval）の行。A は2か月分（契約の選択肢が重複しないことを読む）。 */
const PENDING_ROWS: TestRow[] = [
  row(CONTRACT_A, "2026-10", "PendingApproval"),
  row(CONTRACT_B, "2026-10", "PendingApproval"),
  row(CONTRACT_A, "2026-09", "PendingApproval"),
];

beforeEach(() => {
  router.refresh.mockReset();
  router.push.mockReset();
  router.replace.mockReset();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function backend(rows: TestRow[] = PENDING_ROWS, chunk = 1000) {
  return installFakeFetch({ [PENDING_PATH]: pagedListHandler(rows, chunk) });
}

async function loaded() {
  return screen.findByRole("table", { name: "承認待ちの勤務月" });
}

function tableRows() {
  return within(screen.getByRole("table", { name: "承認待ちの勤務月" })).getAllByRole("row").slice(1);
}

function badgeText() {
  const { container, unmount } = render(<StatusBadge state="PendingApproval" />);
  const text = container.textContent ?? "";
  unmount();
  return text;
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

function manyRows(n: number): TestRow[] {
  const rows: TestRow[] = [];
  for (let i = n - 1; i >= 0; i--) {
    const year = 2000 + Math.floor(i / 12);
    const month = String((i % 12) + 1).padStart(2, "0");
    rows.push(row(CONTRACT_A, `${year}-${month}`, "PendingApproval"));
  }
  return rows;
}

describe("表示（2-1・2-3・2-5・5-1・W-5）", () => {
  it("見出しと説明を出す", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    expect(screen.getByRole("heading", { level: 1, name: "承認待ちの勤務月" })).toBeTruthy();
    expect(
      screen.getByText("勤務月を開いて内容を確認し、承認または差し戻しをしてください。"),
    ).toBeTruthy();
  });

  it("2-5・2-3: 返った行をすべて、契約表示名・「YYYY年M月」・状態バッジつきで応答の順に載せる（4-1）", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    const rows = tableRows();
    expect(rows).toHaveLength(PENDING_ROWS.length);
    PENDING_ROWS.forEach((r, i) => {
      const text = rows[i].textContent ?? "";
      expect(text).toContain(r.contractDisplayName);
      expect(text).toContain(formatted(r.yearMonth));
      expect(text).toContain(badgeText());
    });
  });

  it("5-1: 各行のリンク「開く」のアクセシブル名は「<契約表示名> × <対象年月>を開く」・行き先は勤務月画面", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    const rows = tableRows();
    PENDING_ROWS.forEach((r, i) => {
      const link = within(rows[i]).getByRole("link", {
        name: `${r.contractDisplayName} × ${formatted(r.yearMonth)}を開く`,
      });
      expect(link.textContent).toBe("開く");
      expect(link.getAttribute("href")).toBe(workMonthPath(r.contractId, r.yearMonth));
    });
  });

  it("直近のカードを持たない（本人一覧と違い、直近の導線を置かない）", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    expect(screen.queryByRole("link", { name: "直近の勤務月を開く" })).toBeNull();
  });
});

describe("取得の要求（W-3）", () => {
  it("engineerId・limit・state を付けず offset=0 から GET し、全件を取得する", async () => {
    const fake = backend(PENDING_ROWS, 2);
    render(<PendingApprovalsView />);
    await loaded();
    expect(fake.offsets(PENDING_PATH)).toEqual(["0", "2"]);
    for (const url of fake.urls) {
      for (const key of ["engineerId", "limit", "state"]) {
        expect(url.searchParams.has(key)).toBe(false);
      }
    }
    expect(tableRows()).toHaveLength(PENDING_ROWS.length);
  });
});

describe("絞り込み（9-2・9-6・9-6-a・9-9・9-10・9-11）", () => {
  it("9-2: 条件は「契約」「対象年月」の2つで、状態の条件を出さない", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    const search = screen.getByRole("search", { name: "勤務月の絞り込み" });
    expect(within(search).getAllByRole("combobox").map((c) => c.getAttribute("id"))).toHaveLength(2);
    expect(within(search).getByRole("combobox", { name: "契約" })).toBeTruthy();
    expect(within(search).getByRole("combobox", { name: "対象年月" })).toBeTruthy();
    expect(within(search).queryByRole("combobox", { name: "状態" })).toBeNull();
  });

  it("9-6: 契約の選択肢は載る行の契約の集合（重複なし）。表示は契約表示名", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    const labels = within(filterSelect("契約"))
      .getAllByRole("option")
      .map((o) => o.textContent)
      .filter((t) => t !== "すべて");
    expect([...labels].sort()).toEqual(
      [CONTRACT_A.contractDisplayName, CONTRACT_B.contractDisplayName].sort(),
    );
  });

  it("9-6-a: 対象年月の選択肢は載る行の対象年月（重複なし・降順）", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    const labels = within(filterSelect("対象年月"))
      .getAllByRole("option")
      .map((o) => o.textContent)
      .filter((t) => t !== "すべて");
    expect(labels).toEqual(["2026年10月", "2026年9月"]);
  });

  it("9-5・9-6: 契約で絞ると、その契約の行だけが入力の順のまま残る", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    choose(filterSelect("契約"), CONTRACT_A.contractDisplayName);
    expect(tableRows().map((r) => within(r).getByRole("link").getAttribute("href"))).toEqual([
      workMonthPath("c-1", "2026-10"),
      workMonthPath("c-1", "2026-09"),
    ]);
  });

  it("9-11: 条件が無いとき「条件をクリア」は出ず、あると出て、押すと既定へ戻る", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    const search = () => screen.getByRole("search", { name: "勤務月の絞り込み" });
    expect(within(search()).queryByRole("button", { name: "条件をクリア" })).toBeNull();
    choose(filterSelect("対象年月"), "2026年9月");
    expect(tableRows()).toHaveLength(1);
    fireEvent.click(within(search()).getByRole("button", { name: "条件をクリア" }));
    expect(filterSelect("対象年月").value).toBe("");
    expect(tableRows()).toHaveLength(PENDING_ROWS.length);
  });

  it("9-10: 条件に合う行が0件のとき、その旨と解除の導線を示す", async () => {
    backend();
    render(<PendingApprovalsView />);
    await loaded();
    choose(filterSelect("契約"), CONTRACT_B.contractDisplayName);
    choose(filterSelect("対象年月"), "2026年9月"); // B の 2026-09 は無い
    expect(screen.getByText("条件に合う勤務月はありません")).toBeTruthy();
    expect(screen.getByText("絞り込みの条件を変えるか、クリアしてください。")).toBeTruthy();
    expect(screen.queryByText("承認待ちの勤務月はありません")).toBeNull();
    expect(screen.getAllByRole("button", { name: "条件をクリア" }).length).toBeGreaterThanOrEqual(1);
  });

  it("4-2・9-9: 件数が定数を1件超えると2ページになり、条件を変えると1ページ目へ戻る", async () => {
    backend(manyRows(WORK_MONTH_LIST_PAGE_SIZE + 1));
    render(<PendingApprovalsView />);
    await loaded();
    expect(tableRows()).toHaveLength(WORK_MONTH_LIST_PAGE_SIZE);
    fireEvent.click(screen.getByRole("button", { name: "次へ" }));
    expect(tableRows()).toHaveLength(1);
    choose(filterSelect("契約"), CONTRACT_A.contractDisplayName); // 全行が A。件数は変わらない
    expect(tableRows()).toHaveLength(WORK_MONTH_LIST_PAGE_SIZE);
  });
});

describe("空状態（7-2）", () => {
  it("0件: 「承認待ちの勤務月はありません」と説明だけを示し、action（リンク・ボタン）を持たない", async () => {
    backend([]);
    render(<PendingApprovalsView />);
    const title = await screen.findByText("承認待ちの勤務月はありません");
    expect(screen.getByText("技術者が勤務月を締めると、ここに表示されます。")).toBeTruthy();
    expect(title.parentElement?.children).toHaveLength(2);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("table")).toBeNull();
  });
});

describe("取得の失敗（7-3。Q-6 = (a) 本人一覧の文言を流用）", () => {
  it.each([500, 429])("%i → role=alert に文言を出し、表を出さない", async (status) => {
    installFakeFetch({ [PENDING_PATH]: () => jsonResponse({}, status) });
    render(<PendingApprovalsView />);
    expect((await screen.findByRole("alert")).textContent).toContain(ERROR_TEXT);
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("再試行で再び GET する", async () => {
    let failing = true;
    const fake = installFakeFetch({
      [PENDING_PATH]: (url) => (failing ? jsonResponse({}, 500) : pagedListHandler(PENDING_ROWS, 1000)(url)),
    });
    render(<PendingApprovalsView />);
    await screen.findByRole("alert");
    const before = fake.count(PENDING_PATH);
    failing = false;
    fireEvent.click(screen.getByRole("button", { name: "再試行" }));
    await loaded();
    expect(fake.count(PENDING_PATH)).toBeGreaterThan(before);
  });

  it.each([401, 403])("%i → router.refresh がちょうど1回", async (status) => {
    installFakeFetch({ [PENDING_PATH]: () => jsonResponse({}, status) });
    render(<PendingApprovalsView />);
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(router.refresh).toHaveBeenCalledTimes(1);
  });
});

describe("状態を変えない（9-13 に準ずる）", () => {
  it("どの操作でも GET 以外を送らない", async () => {
    const fake = backend(manyRows(WORK_MONTH_LIST_PAGE_SIZE + 1));
    render(<PendingApprovalsView />);
    await loaded();
    fireEvent.click(screen.getByRole("button", { name: "次へ" }));
    choose(filterSelect("契約"), CONTRACT_A.contractDisplayName);
    expect(new Set(fake.methods)).toEqual(new Set(["GET"]));
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });
});
