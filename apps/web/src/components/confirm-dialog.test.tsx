/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, fireEvent, within } from "@testing-library/react";
import { ConfirmDialog } from "./confirm-dialog";

// docs/specs/design-system.md AC-6-9（検証手段は AC-10-5・AC-10-6）。
//
// 「取消操作」自体のラベル文字列は props 表に cancelLabel 等の対応が無く
// 仕様から一意に定まらない（AC-11-11）。したがって取消操作は「アクセシブル
// 名が confirmLabel と一致しない、残る1つのボタン」として識別する
// （6-9-i）。表が明言する Esc キーの挙動（6-9-i の一部）に加え、クリックに
// よる確定・取消（6-9-i・6-9-ii）、open が真になったときの初期フォーカス
// （6-9-iii）を検証する（AC-10-6-b・AC-10-6-e）。

afterEach(() => {
  cleanup();
});

describe("ConfirmDialog - AC-6-9", () => {
  it("open が偽のとき何も出力しない", () => {
    render(
      <ConfirmDialog
        open={false}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("open が真のとき role='dialog' と aria-modal='true' を持ち、title と aria-labelledby で結ぶ", () => {
    render(
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    const dialog = screen.getByRole("dialog");
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    const labelledBy = dialog.getAttribute("aria-labelledby");
    expect(labelledBy).toBeTruthy();
    const titleEl = document.getElementById(labelledBy as string);
    expect(titleEl?.textContent).toBe("締めますか");
  });

  it("open が真のとき confirmLabel と description を表示する", () => {
    render(
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "締める" })).toBeTruthy();
    expect(screen.getByText("この操作は取り消せません")).toBeTruthy();
  });

  it("Esc キーは onCancel を呼び、onConfirm を呼ばない（AC-6-9・AC-10-6）", () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("6-9-i: コンポーネント自身が出すボタンはちょうど2つで、確定でない方を押すと onCancel のみが呼ばれる", () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    const dialog = screen.getByRole("dialog");
    const buttons = within(dialog).getAllByRole("button");
    expect(buttons).toHaveLength(2);

    // 確定の操作はアクセシブル名が confirmLabel と一致するボタンとして識別する。
    const confirmButton = within(dialog).getByRole("button", { name: "締める" });
    // 取消の操作はラベル文字列を固定せず、「残る1つ」として識別する（AC-11-11）。
    const cancelButton = buttons.find((button) => button !== confirmButton);
    expect(cancelButton).toBeTruthy();

    fireEvent.click(cancelButton as HTMLElement);
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("6-9-ii: 確定の操作（アクセシブル名が confirmLabel と一致するボタン）を押すと onConfirm のみが呼ばれる", () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
  });

  it("6-9-iii: open が真で描画された時点でフォーカスがダイアログの内側にある（初期描画）", () => {
    render(
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    const dialog = screen.getByRole("dialog");
    expect(
      dialog === document.activeElement || dialog.contains(document.activeElement),
    ).toBe(true);
  });

  it("6-9-iii: open を偽で描画したのち真へ更新した場合もフォーカスがダイアログの内側にある（再描画）", () => {
    const { rerender } = render(
      <ConfirmDialog
        open={false}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    rerender(
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    const dialog = screen.getByRole("dialog");
    expect(
      dialog === document.activeElement || dialog.contains(document.activeElement),
    ).toBe(true);
  });

  // AC-10-5: confirmVariant は primary / danger の2値のみ（secondary を含まない）。
  it("型: confirmVariant は primary / danger のみである（AC-10-5）", () => {
    const invalid = (
      <ConfirmDialog
        open={true}
        title="締めますか"
        description="この操作は取り消せません"
        confirmLabel="締める"
        // @ts-expect-error confirmVariant は primary | danger のみ許可される
        confirmVariant="secondary"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />
    );
    void invalid;
  });
});
