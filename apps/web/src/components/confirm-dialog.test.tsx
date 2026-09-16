/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { ConfirmDialog } from "./confirm-dialog";

// docs/specs/design-system.md AC-6-9（検証手段は AC-10-5・AC-10-6）。
//
// 「取消操作」自体のラベル文字列は props 表に cancelLabel 等の対応が無く
// 仕様から一意に定まらないため、ここでは表が明言する Esc キーの挙動のみを
// 検証する（AC-10-6「ConfirmDialog の Esc は keydown イベントを発火させ、
// onCancel が呼ばれ onConfirm が呼ばれないことを検証する」）。

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
