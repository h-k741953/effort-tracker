/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { StatusBadge } from "@/components/status-badge";
import type { ContractListItem } from "@/lib/contract-list";
import { ownWorkMonthsPath } from "@/lib/screen-routes";
import { ContractsTable } from "./contracts-table";

// docs/specs/contracts-ui.md 1-5・2-1〜2-6・3-1〜3-5。設計 KP-1・KP-3・KP-4。
// 文言の期待値は仕様の字面をここへ書く（実装の定数を輸入しない）。表は props（contracts）だけで描く。

afterEach(cleanup);

const RANGE_A = { lowerBound: { hours: 140, minutes: 0 }, upperBound: { hours: 180, minutes: 0 } };
const RANGE_C = { lowerBound: { hours: 140, minutes: 15 }, upperBound: { hours: 180, minutes: 30 } };

// 契約識別子の昇順とは限らない順で与える（並べ替えないことを読む＝1-5）。
const C3: ContractListItem = {
  contractId: "c-3",
  contractDisplayName: "C社 / 運用",
  settlementRange: RANGE_C,
  contractPeriod: { start: "2026-06", end: "2026-06" },
  workMonth: { yearMonth: "2026-11", state: "Approved", generated: true },
};
const C1: ContractListItem = {
  contractId: "c-1",
  contractDisplayName: "A社 / 保守",
  settlementRange: RANGE_A,
  contractPeriod: { start: "2025-04", end: "2027-03" },
  workMonth: { yearMonth: "2026-11", state: "PendingApproval", generated: true },
};
const C2: ContractListItem = {
  contractId: "c-2",
  contractDisplayName: "B社 / 開発",
  settlementRange: { lowerBound: { hours: 150, minutes: 0 }, upperBound: { hours: 190, minutes: 0 } },
  contractPeriod: { start: "2025-04", end: null },
  workMonth: { yearMonth: "2026-11", state: "Draft", generated: false },
};
// 今月が契約期間の外（過ぎた契約）。勤務月はある。
const PAST: ContractListItem = {
  contractId: "c-4",
  contractDisplayName: "D社 / 旧案件",
  settlementRange: RANGE_A,
  contractPeriod: { start: "2024-01", end: "2025-12" },
  workMonth: { yearMonth: "2026-11", state: "Approved", generated: true },
};
// 今月が契約期間の外（まだ始まっていない契約）。勤務月は未生成。
const FUTURE: ContractListItem = {
  contractId: "c-5",
  contractDisplayName: "E社 / 新案件",
  settlementRange: RANGE_A,
  contractPeriod: { start: "2027-01", end: null },
  workMonth: { yearMonth: "2026-11", state: "Draft", generated: false },
};
// 符号化の要る文字を含む識別子・表示名。
const ODD: ContractListItem = {
  contractId: "c 6&x/y",
  contractDisplayName: "F社 / 特殊 & 案件",
  settlementRange: RANGE_A,
  contractPeriod: { start: "2025-04", end: null },
  workMonth: { yearMonth: "2026-11", state: "Draft", generated: true },
};

const LABEL: Record<ContractListItem["workMonth"]["state"], string> = {
  Draft: "下書き",
  PendingApproval: "締め済",
  Approved: "承認済",
};

const INTERACTIVE = "a, button, [role='link'], [role='button']";

/** 要素がリンク・ボタンの中（自身を含む）にあるか。 */
function insideInteractive(el: Element): boolean {
  return el.closest(INTERACTIVE) !== null;
}

/** 状態のセルが、状態の語だけを示しているか（未生成を区別する文字を足していない）。 */
function showsOnlyLabel(cell: HTMLElement, state: ContractListItem["workMonth"]["state"]): boolean {
  return cell.textContent === LABEL[state];
}

function bodyRows() {
  return within(screen.getByRole("table")).getAllByRole("row").slice(1);
}

describe("ContractsTable — 見出しと行の順（2-6・1-5）", () => {
  it("2-6・2-4: 列見出しは 契約・精算幅・契約期間・今月の状態・操作 の順（技術者の列は無い）", () => {
    render(<ContractsTable contracts={[C1]} />);
    expect(screen.getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "契約",
      "精算幅",
      "契約期間",
      "今月の状態",
      "操作",
    ]);
  });

  it("陽性対照（2-4）: 技術者の列を持つ表は、同じ判定（見出しの完全一致）で検出される", () => {
    const { unmount } = render(
      <table>
        <thead>
          <tr>
            <th>契約</th>
            <th>技術者</th>
            <th>精算幅</th>
            <th>契約期間</th>
            <th>今月の状態</th>
            <th>操作</th>
          </tr>
        </thead>
      </table>,
    );
    const headers = screen.getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers).not.toEqual(["契約", "精算幅", "契約期間", "今月の状態", "操作"]);
    expect(headers).toContain("技術者");
    unmount();
  });

  it("1-5: 行は入力の順に並ぶ（並べ替えない）。行数は入力と同じ", () => {
    const input = [C3, C1, C2, PAST, FUTURE, ODD];
    render(<ContractsTable contracts={input} />);
    const rows = bodyRows();
    expect(rows).toHaveLength(input.length);
    input.forEach((c, i) => {
      expect(within(rows[i]).getAllByRole("cell")[0].textContent).toBe(c.contractDisplayName);
    });
  });
});

describe("ContractsTable — 各行の表示（2-1・2-2・2-2-a・2-3）", () => {
  it("契約表示名・精算幅（K-1 の形）・契約期間（2-3-a・2-3-b）を、列の順に示す", () => {
    render(<ContractsTable contracts={[C3, C1, C2]} />);
    const expected = [
      ["C社 / 運用", "140.25〜180.5 時間", "2026年6月〜2026年6月"],
      ["A社 / 保守", "140.0〜180.0 時間", "2025年4月〜2027年3月"],
      ["B社 / 開発", "150.0〜190.0 時間", "2025年4月〜"],
    ];
    const rows = bodyRows();
    expected.forEach((cols, i) => {
      const cells = within(rows[i]).getAllByRole("cell");
      expect(cells.slice(0, 3).map((c) => c.textContent)).toEqual(cols);
    });
  });
});

describe("ContractsTable — 今月の状態（2-5・2-5-b〜2-5-e）", () => {
  it("2-5-b: Draft・PendingApproval・Approved がそれぞれ 下書き・締め済・承認済（4列目）", () => {
    render(<ContractsTable contracts={[C1, C3, ODD]} />);
    const rows = bodyRows();
    const texts = rows.map((r) => within(r).getAllByRole("cell")[3].textContent);
    expect(texts).toEqual(["締め済", "承認済", "下書き"]);
  });

  it("2-5-c: generated が偽で Draft の行も 下書き だけを示し、未生成を区別する文字を足さない（生成済みの Draft 行と同じ）", () => {
    render(<ContractsTable contracts={[C2, ODD]} />);
    const [ungenerated, generated] = bodyRows().map((r) => within(r).getAllByRole("cell")[3]);
    expect(showsOnlyLabel(ungenerated, "Draft")).toBe(true);
    expect(showsOnlyLabel(generated, "Draft")).toBe(true);
    expect(ungenerated.textContent).toBe(generated.textContent);
  });

  it("陽性対照（2-5-c）: 未生成を示す文字を足したセルは、同じ判定で検出される", () => {
    const { container, unmount } = render(
      <table>
        <tbody>
          <tr>
            <td>下書き（未生成）</td>
          </tr>
          <tr>
            <td>下書き未</td>
          </tr>
        </tbody>
      </table>,
    );
    for (const cell of container.querySelectorAll("td")) {
      expect(showsOnlyLabel(cell as HTMLElement, "Draft")).toBe(false);
    }
    unmount();
  });

  it("2-5-d: 今月が契約期間の外（過ぎた・まだ始まっていない）でも状態を示す", () => {
    render(<ContractsTable contracts={[PAST, FUTURE]} />);
    const [past, future] = bodyRows().map((r) => within(r).getAllByRole("cell")[3]);
    expect(showsOnlyLabel(past, "Approved")).toBe(true);
    expect(showsOnlyLabel(future, "Draft")).toBe(true);
  });

  it("2-5-e: 状態の表示（StatusBadge）は、どの行でもリンク・ボタンの中に無い", () => {
    const input = [C1, C2, C3, PAST, FUTURE, ODD];
    render(<ContractsTable contracts={input} />);
    bodyRows().forEach((r, i) => {
      const cell = within(r).getAllByRole("cell")[3];
      const badge = within(cell).getByText(LABEL[input[i].workMonth.state]);
      expect(insideInteractive(badge)).toBe(false);
    });
  });

  it("陽性対照（2-5-e）: StatusBadge をリンクの中・ボタンの中に置いた描画は、同じ判定で検出される", () => {
    const { unmount } = render(
      <div>
        <a href="/x" data-testid="in-link">
          <StatusBadge state="Draft" />
        </a>
        <button type="button" data-testid="in-button">
          <StatusBadge state="Approved" />
        </button>
        <div data-testid="plain">
          <StatusBadge state="PendingApproval" />
        </div>
      </div>,
    );
    expect(insideInteractive(within(screen.getByTestId("in-link")).getByText("下書き"))).toBe(true);
    expect(insideInteractive(within(screen.getByTestId("in-button")).getByText("承認済"))).toBe(true);
    expect(insideInteractive(within(screen.getByTestId("plain")).getByText("締め済"))).toBe(false);
    unmount();
  });
});

describe("ContractsTable — 勤務月を見る導線（3-1・3-2・3-3・3-4・3-5）", () => {
  it("3-1・3-3・3-4: 各行にリンクがちょうど1つ。表示は 勤務月を見る、名は {契約表示名}の勤務月を見る、href は ownWorkMonthsPath", () => {
    const input = [C1, C2, C3, PAST, FUTURE, ODD];
    render(<ContractsTable contracts={input} />);
    const rows = bodyRows();
    input.forEach((c, i) => {
      const links = within(rows[i]).getAllByRole("link");
      expect(links).toHaveLength(1);
      expect(links[0].textContent).toBe("勤務月を見る");
      expect(within(rows[i]).getByRole("link", { name: `${c.contractDisplayName}の勤務月を見る` })).toBe(links[0]);
      expect(links[0].getAttribute("href")).toBe(ownWorkMonthsPath({ contractId: c.contractId }));
    });
    // 行ごとに区別できる（アクセシブル名が重ならない）。
    const names = screen.getAllByRole("link").map((l) => l.getAttribute("aria-label") ?? l.textContent);
    expect(new Set(names).size).toBe(input.length);
  });

  it("3-5: 勤務月が無い（generated が偽の）契約にもリンクがある", () => {
    render(<ContractsTable contracts={[C2, FUTURE]} />);
    for (const r of bodyRows()) {
      expect(within(r).getAllByRole("link")).toHaveLength(1);
    }
  });
});
