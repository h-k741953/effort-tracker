/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { CloseAction, ReviewActions } from "./work-month-operations";

// docs/specs/monthly-closing-ui.md 1-4・2-1〜2-3・5-3、approval-ui.md 2-1・10-1〜10-7・10-9。設計 M-8・M-11・T-3。
// 承認・差戻しの確認の title・description・confirmLabel の字面は approval-ui.md 10-9 の表（10-9-a・10-9-b）から写した。
// 取消の操作のラベル「取消」は ConfirmDialog が持つ。

afterEach(cleanup);

const tokens = (el: Element) => (el.getAttribute("class") ?? "").split(/\s+/);

describe("CloseAction（締め UI 1-4・2-1〜2-3・5-3）", () => {
  function renderClose(options: { busy?: boolean; yearMonth?: string } = {}) {
    const log = { confirmed: 0 };
    const view = render(
      <CloseAction
        yearMonth={options.yearMonth ?? "2026-07"}
        busy={options.busy ?? false}
        onConfirm={() => {
          log.confirmed += 1;
        }}
      />,
    );
    return { ...view, log };
  }

  it("区画: 見出し・説明文・「締める」の button。未入力の警告を出さない", () => {
    renderClose();
    expect(screen.getByRole("heading", { name: "勤務月を締める" })).toBeTruthy();
    expect(screen.getByText("入力が済んだら締めてください。締めると承認されるまで編集できません。")).toBeTruthy();
    expect(screen.getByRole("button", { name: "締める" })).toBeTruthy();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryAllByRole("alert")).toHaveLength(0);
  });

  it("「締める」を押しても onConfirm は呼ばれず、確認の dialog が title と2段落で出る（2-1）", () => {
    const { log } = renderClose({ yearMonth: "2026-07" });
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    expect(log.confirmed).toBe(0);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("heading", { name: "2026年7月の勤務月を締めますか？" })).toBeTruthy();
    expect(dialog.querySelectorAll("p")).toHaveLength(2);
    expect(within(dialog).getByText("締めると稼働実績は編集できなくなり、ご自身で取り消すことはできません。")).toBeTruthy();
    expect(within(dialog).getByText("下書きに戻す方法は、承認者による差戻しだけです。")).toBeTruthy();
  });

  it("title の年月は対象年月から組む（2026-10 → 2026年10月）", () => {
    renderClose({ yearMonth: "2026-10" });
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    expect(within(screen.getByRole("dialog")).getByRole("heading", { name: "2026年10月の勤務月を締めますか？" })).toBeTruthy();
  });

  it("確定ラベルは「締める」で、主要の種別（primary）", () => {
    renderClose();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    const confirm = within(screen.getByRole("dialog")).getByRole("button", { name: "締める" });
    expect(tokens(confirm)).toContain("bg-primary");
    expect(tokens(confirm)).not.toContain("bg-danger");
  });

  it("取消で onConfirm 0回・dialog が消える（2-3）", () => {
    const { log } = renderClose();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "取消" }));
    expect(log.confirmed).toBe(0);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("Esc で onConfirm 0回・dialog が消える（2-3）", () => {
    const { log } = renderClose();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(log.confirmed).toBe(0);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("確定で onConfirm が1回（2-2）", () => {
    const { log } = renderClose();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "締める" }));
    expect(log.confirmed).toBe(1);
  });

  it("busy 真で確定しても onConfirm は0回", () => {
    const { log } = renderClose({ busy: true });
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    const dialog = screen.queryByRole("dialog");
    if (dialog !== null) fireEvent.click(within(dialog).getByRole("button", { name: "締める" }));
    expect(log.confirmed).toBe(0);
  });

  it("取り消す・下書きへ戻す操作を置かない（締め UI 4-1）", () => {
    renderClose();
    expect(screen.queryByRole("button", { name: /取り消|下書きへ戻|下書きに戻/ })).toBeNull();
  });
});

describe("ReviewActions（承認 UI 2-1・10-1〜10-7・10-9）", () => {
  function renderReview(options: { busy?: boolean } = {}) {
    const log = { approved: 0, rejected: 0 };
    const view = render(
      <ReviewActions
        busy={options.busy ?? false}
        onApprove={() => {
          log.approved += 1;
        }}
        onReject={() => {
          log.rejected += 1;
        }}
      />,
    );
    return { ...view, log };
  }

  it("区画: 見出し「承認」・説明文・「差戻し」「承認」の2つの button（この順）。確認は閉じている", () => {
    renderReview();
    expect(screen.getByRole("heading", { name: "承認" })).toBeTruthy();
    expect(screen.getByText("内容を確認して、承認または差戻しをしてください。")).toBeTruthy();
    const buttons = screen.getAllByRole("button");
    expect(buttons.map((b) => b.textContent)).toEqual(["差戻し", "承認"]);
    expect(tokens(buttons[0])).toContain("bg-danger");
    expect(tokens(buttons[1])).toContain("bg-primary");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("「承認」を押すと onApprove を呼ばずに、10-9-a の title・description・confirmLabel の dialog が出る", () => {
    const { log } = renderReview();
    fireEvent.click(screen.getByRole("button", { name: "承認" }));
    expect(log.approved).toBe(0);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("heading", { name: "この勤務月を承認しますか？" })).toBeTruthy();
    expect(
      within(dialog).getByText("承認すると承認済になります。承認済の勤務月は、差戻しを含めて以降の操作ができません。"),
    ).toBeTruthy();
    const confirm = within(dialog).getByRole("button", { name: "承認する" });
    expect(tokens(confirm)).toContain("bg-primary"); // 10-6: 承認の確定は primary
    expect(tokens(confirm)).not.toContain("bg-danger");
  });

  it("「差戻し」を押すと onReject を呼ばずに、10-9-b の title・description・confirmLabel の dialog が出る", () => {
    const { log } = renderReview();
    fireEvent.click(screen.getByRole("button", { name: "差戻し" }));
    expect(log.rejected).toBe(0);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("heading", { name: "この勤務月を差し戻しますか？" })).toBeTruthy();
    expect(within(dialog).getByText("差し戻すと下書きに戻り、技術者が稼働実績を修正できるようになります。")).toBeTruthy();
    const confirm = within(dialog).getByRole("button", { name: "差し戻す" });
    expect(tokens(confirm)).toContain("bg-danger"); // 10-6: 差戻しの確定は danger
    expect(tokens(confirm)).not.toContain("bg-primary");
  });

  it("差戻しの dialog に理由の入力欄（textbox・combobox）が無い（10-5・2-1）", () => {
    renderReview();
    fireEvent.click(screen.getByRole("button", { name: "差戻し" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).queryAllByRole("textbox")).toHaveLength(0);
    expect(within(dialog).queryAllByRole("combobox")).toHaveLength(0);
    expect(dialog.querySelectorAll("textarea, input, select")).toHaveLength(0);
  });

  it.each([
    ["承認", "承認する", "approved"],
    ["差戻し", "差し戻す", "rejected"],
  ] as const)("%s: 取消で0回・dialog が消える（10-4）", (opener, _confirm, key) => {
    const { log } = renderReview();
    fireEvent.click(screen.getByRole("button", { name: opener }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "取消" }));
    expect(log[key]).toBe(0);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it.each([
    ["承認", "approved"],
    ["差戻し", "rejected"],
  ] as const)("%s: Esc で0回・dialog が消える（10-4）", (opener, key) => {
    const { log } = renderReview();
    fireEvent.click(screen.getByRole("button", { name: opener }));
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(log[key]).toBe(0);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it.each([
    ["承認", "承認する", "approved", "rejected"],
    ["差戻し", "差し戻す", "rejected", "approved"],
  ] as const)("%s: 確定でその操作だけが1回（10-3）", (opener, confirmLabel, key, other) => {
    const { log } = renderReview();
    fireEvent.click(screen.getByRole("button", { name: opener }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: confirmLabel }));
    expect(log[key]).toBe(1);
    expect(log[other]).toBe(0);
  });

  it.each([
    ["承認", "承認する"],
    ["差戻し", "差し戻す"],
  ])("%s: busy 真で確定しても0回", (opener, confirmLabel) => {
    const { log } = renderReview({ busy: true });
    fireEvent.click(screen.getByRole("button", { name: opener }));
    const dialog = screen.queryByRole("dialog");
    if (dialog !== null) fireEvent.click(within(dialog).getByRole("button", { name: confirmLabel }));
    expect(log.approved + log.rejected).toBe(0);
  });
});
