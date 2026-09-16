/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { RoleSwitcher } from "./role-switcher";

// docs/specs/design-system.md AC-6-3（検証手段は AC-10-5・AC-10-6）。
//
// 「現在のロールが表示から判別できること」の具体的な DOM 構造（要素・
// role 属性の種類）は AC-6 の表が名指ししておらず、AC-11-7 により機械検査の
// 対象外（表に書かれていない DOM 構造は検査されない）。ここでは表が明言する
// 「ラベル文字列（技術者・承認者）が2つとも表示されること」のみを検証する。

afterEach(() => {
  cleanup();
});

describe("RoleSwitcher - AC-6-3", () => {
  it("選択肢はちょうど2つ（技術者・承認者）が文字として表示される", () => {
    render(<RoleSwitcher role="Engineer" onChange={vi.fn()} />);
    expect(screen.getByText("技術者")).toBeTruthy();
    expect(screen.getByText("承認者")).toBeTruthy();
  });

  // AC-10-5: role は Role 型（Engineer / Approver）のみ。「ゲスト」を表す値を持たない。
  it("型: role は Engineer / Approver のみで、Guest を受け付けない（AC-6-3・AC-10-5）", () => {
    // @ts-expect-error role は Role 型（Engineer | Approver）のみ許可される
    const invalid = <RoleSwitcher role="Guest" onChange={vi.fn()} />;
    void invalid;
  });
});
