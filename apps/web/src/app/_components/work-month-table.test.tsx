/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { StatusBadge, type StatusBadgeState } from "@/components/status-badge";
import { workMonthPath } from "@/lib/screen-routes";
import { OWN_ROWS, row, CONTRACT_A } from "@/test-support/work-month-list";
import { WorkMonthTable } from "./work-month-table";

// docs/specs/work-month-listing-ui.md AC-1-2・2-3・5-1。設計 W-4・W-5・W-8・T-2。
// 表は props（label・rows）だけで描く。契約表示名・「YYYY年M月」・状態バッジ・リンク「開く」を読む。

afterEach(cleanup);

/** StatusBadge を描いて、その語を読む（期待値を字面で書かない）。 */
function badgeText(state: StatusBadgeState) {
  const { container, unmount } = render(<StatusBadge state={state} />);
  const text = container.textContent ?? "";
  unmount();
  return text;
}

function bodyRows() {
  return within(screen.getByRole("table")).getAllByRole("row").slice(1); // 先頭は見出し行
}

describe("WorkMonthTable", () => {
  it("label が <table> のアクセシブル名になる", () => {
    render(<WorkMonthTable label="承認待ちの勤務月" rows={OWN_ROWS} />);
    expect(screen.getByRole("table", { name: "承認待ちの勤務月" })).toBeTruthy();
  });

  it("列見出しは「契約」「対象年月」「状態」と、視覚的に隠した「操作」", () => {
    render(<WorkMonthTable label="すべての勤務月" rows={OWN_ROWS} />);
    expect(screen.getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "契約",
      "対象年月",
      "状態",
      "操作",
    ]);
  });

  it("1-2・2-3: 各行に契約表示名・「YYYY年M月」・状態バッジを、与えた行の順で示す（3つの状態）", () => {
    render(<WorkMonthTable label="すべての勤務月" rows={OWN_ROWS} />);
    const rows = bodyRows();
    expect(rows).toHaveLength(OWN_ROWS.length);
    const expectedYearMonth = ["2026年10月", "2026年10月", "2026年9月", "2026年8月"];
    OWN_ROWS.forEach((r, i) => {
      const cells = within(rows[i]).getAllByRole("cell");
      expect(cells[0].textContent).toBe(r.contractDisplayName);
      expect(cells[1].textContent).toBe(expectedYearMonth[i]);
      expect(cells[2].textContent).toBe(badgeText(r.state));
    });
    // 3つの状態の語が互いに異なる（バッジの取り違えを検出する）。
    expect(new Set(OWN_ROWS.map((r) => badgeText(r.state))).size).toBe(3);
  });

  it("5-1: 各行のリンクの文言は「開く」、アクセシブル名は「<契約表示名> × <対象年月>を開く」、行き先は勤務月画面", () => {
    render(<WorkMonthTable label="すべての勤務月" rows={OWN_ROWS} />);
    const rows = bodyRows();
    const expectedYearMonth = ["2026年10月", "2026年10月", "2026年9月", "2026年8月"];
    OWN_ROWS.forEach((r, i) => {
      const link = within(rows[i]).getByRole("link", {
        name: `${r.contractDisplayName} × ${expectedYearMonth[i]}を開く`,
      });
      expect(link.textContent).toBe("開く");
      expect(link.getAttribute("href")).toBe(workMonthPath(r.contractId, r.yearMonth));
    });
  });

  it("5-1: 契約 ID・年月に符号化の要る文字があっても、行き先は workMonthPath と一致する", () => {
    const odd = { ...row(CONTRACT_A, "2026-10", "Draft"), contractId: "a/b c" };
    render(<WorkMonthTable label="すべての勤務月" rows={[odd]} />);
    const link = within(bodyRows()[0]).getByRole("link");
    expect(link.getAttribute("href")).toBe(workMonthPath("a/b c", "2026-10"));
  });

  it("行が0件のとき、見出し行だけの表になる（本体の行を作らない）", () => {
    render(<WorkMonthTable label="すべての勤務月" rows={[]} />);
    expect(bodyRows()).toHaveLength(0);
  });
});
