/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { StatusBadge } from "@/components/status-badge";
import { workMonthPath } from "@/lib/screen-routes";
import {
  DEMO_PATH,
  OWN_ROWS,
  installFakeFetch,
  jsonResponse,
  pagedListHandler,
  type TestRow,
} from "@/test-support/work-month-list";
import { DemoWorkMonthsView } from "./demo-work-months-view";

// docs/specs/work-month-listing-ui.md AC-3-1・3-2・5-1・7-3・9-3。設計 W-5・T-2。
// next/navigation だけを vi.mock し、fetch は手書きの Fake（T-0-1）。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const ERROR_TEXT = "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";

beforeEach(() => {
  router.refresh.mockReset();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function backend(rows: TestRow[] = OWN_ROWS) {
  return installFakeFetch({ [DEMO_PATH]: pagedListHandler(rows, 1000) });
}

async function loaded() {
  return screen.findByRole("table", { name: "サンプル勤務月" });
}

function formatted(yearMonth: string) {
  const [y, m] = yearMonth.split("-");
  return `${y}年${Number(m)}月`;
}

function badgeText(state: TestRow["state"]) {
  const { container, unmount } = render(<StatusBadge state={state} />);
  const text = container.textContent ?? "";
  unmount();
  return text;
}

describe("表示（3-1・3-2・5-1・W-5）", () => {
  it("3-1: 3つの状態の行が載る（契約表示名・「YYYY年M月」・状態バッジ）", async () => {
    backend();
    render(<DemoWorkMonthsView />);
    const table = await loaded();
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(OWN_ROWS.length);
    OWN_ROWS.forEach((r, i) => {
      const text = rows[i].textContent ?? "";
      expect(text).toContain(r.contractDisplayName);
      expect(text).toContain(formatted(r.yearMonth));
      expect(text).toContain(badgeText(r.state));
    });
    expect(new Set(OWN_ROWS.map((r) => r.state)).size).toBe(3);
  });

  it("3-2・5-1・Q-6 = (a): 各行のリンクは「開く」で、アクセシブル名は「<契約表示名> × <対象年月>を開く」・行き先は勤務月画面", async () => {
    backend();
    render(<DemoWorkMonthsView />);
    const table = await loaded();
    const rows = within(table).getAllByRole("row").slice(1);
    OWN_ROWS.forEach((r, i) => {
      const link = within(rows[i]).getByRole("link", {
        name: `${r.contractDisplayName} × ${formatted(r.yearMonth)}を開く`,
      });
      expect(link.textContent).toBe("開く");
      expect(link.getAttribute("href")).toBe(workMonthPath(r.contractId, r.yearMonth));
    });
  });

  it("3-2・3-5: role=note に、デモ用であること・ゲストとして閲覧していること・入力・締め・承認ができないことを示す", async () => {
    backend();
    render(<DemoWorkMonthsView />);
    await loaded();
    const note = screen.getByRole("note");
    expect(note.textContent).toContain("デモ用のサンプルです（閲覧のみ・毎日リセットされます）");
    expect(note.textContent).toContain("ゲストとして閲覧しています。入力・締め・承認はできません。");
  });

  it("見出し「サンプル勤務月」を出す", async () => {
    backend();
    render(<DemoWorkMonthsView />);
    await loaded();
    expect(screen.getByRole("heading", { name: "サンプル勤務月" })).toBeTruthy();
  });

  it("9-3: 絞り込み（role=search）も Pagination も置かない", async () => {
    backend();
    render(<DemoWorkMonthsView />);
    await loaded();
    expect(screen.queryByRole("search")).toBeNull();
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByRole("navigation", { name: "ページ送り" })).toBeNull();
    expect(screen.queryByRole("button", { name: "次へ" })).toBeNull();
  });

  it("R-4 を1回だけ GET する（ページを送らない。engineerId を付けない）", async () => {
    const fake = backend();
    render(<DemoWorkMonthsView />);
    await loaded();
    expect(fake.urls).toHaveLength(1);
    expect(fake.urls[0].pathname).toBe(DEMO_PATH);
    expect(fake.urls[0].searchParams.has("engineerId")).toBe(false);
    expect(fake.methods).toEqual(["GET"]);
  });
});

describe("空状態", () => {
  it("0件: title「サンプル勤務月はありません」だけを示し、description と action を持たない", async () => {
    backend([]);
    render(<DemoWorkMonthsView />);
    const title = await screen.findByText("サンプル勤務月はありません");
    expect(title.parentElement?.children).toHaveLength(1);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("table")).toBeNull();
  });
});

describe("取得の失敗（7-3。Q-6 = (a)）", () => {
  it.each([500, 429])("%i → role=alert に本人一覧と同じ文言を出し、表を出さない", async (status) => {
    installFakeFetch({ [DEMO_PATH]: () => jsonResponse({}, status) });
    render(<DemoWorkMonthsView />);
    expect((await screen.findByRole("alert")).textContent).toContain(ERROR_TEXT);
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("fetch が例外 → role=alert", async () => {
    vi.stubGlobal("fetch", async () => {
      throw new Error("network");
    });
    render(<DemoWorkMonthsView />);
    expect((await screen.findByRole("alert")).textContent).toContain(ERROR_TEXT);
  });

  it("再試行で再び GET する", async () => {
    let failing = true;
    const fake = installFakeFetch({
      [DEMO_PATH]: (url) => (failing ? jsonResponse({}, 500) : pagedListHandler(OWN_ROWS, 1000)(url)),
    });
    render(<DemoWorkMonthsView />);
    await screen.findByRole("alert");
    expect(fake.count(DEMO_PATH)).toBe(1);
    failing = false;
    fireEvent.click(screen.getByRole("button", { name: "再試行" }));
    await loaded();
    expect(fake.count(DEMO_PATH)).toBe(2);
  });

  it.each([401, 403])("%i → router.refresh がちょうど1回", async (status) => {
    installFakeFetch({ [DEMO_PATH]: () => jsonResponse({}, status) });
    render(<DemoWorkMonthsView />);
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(router.refresh).toHaveBeenCalledTimes(1);
  });
});
