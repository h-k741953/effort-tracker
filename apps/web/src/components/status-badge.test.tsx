/** @vitest-environment jsdom */
import { describe, expect, it } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
import { StatusBadge } from "./status-badge";

// docs/specs/design-system.md AC-6-1（検証手段は AC-10-6）。
//
// state ごとに日本語ラベルを文字として出すこと（Draft→下書き／
// PendingApproval→締め済／Approved→承認済。P-4）を検証する。
// 色・トークンの使用は AC-10-1（globals.css）と AC-10-3（forbidden-patterns）
// が別に持つため、ここでは表示文字列のみを検証する。

afterEach(() => {
  cleanup();
});

describe("StatusBadge - AC-6-1", () => {
  it.each<{ state: "Draft" | "PendingApproval" | "Approved"; label: string }>([
    { state: "Draft", label: "下書き" },
    { state: "PendingApproval", label: "締め済" },
    { state: "Approved", label: "承認済" },
  ])("state=$state のとき文字として「$label」を出す", ({ state, label }) => {
    render(<StatusBadge state={state} />);
    expect(screen.getByText(label)).toBeTruthy();
  });

  // AC-10-5: 公開 props は state の1つのみ（AC-6 表の前文「表に無い props（とくに
  // role...）を足さない」）。tsc --noEmit で禁じた props が型エラーになることを示す。
  it("型: state は Draft / PendingApproval / Approved の3値のみである（AC-10-5）", () => {
    // @ts-expect-error state は3値のリテラルユニオンのみ許可される
    const invalid = <StatusBadge state="Closed" />;
    void invalid;
  });

  it("型: state 以外の props（例: role）を持たない（AC-6 表の前文・AC-7-2）", () => {
    // @ts-expect-error StatusBadge は state 以外の props を受け取らない
    const invalid = <StatusBadge state="Draft" role="Approver" />;
    void invalid;
  });
});
