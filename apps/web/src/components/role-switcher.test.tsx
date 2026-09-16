/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { RoleSwitcher } from "./role-switcher";

// docs/specs/design-system.md AC-6-3（検証手段は AC-10-5・AC-10-6）。
//
// 「現在のロールが表示から判別できること」の具体的な DOM 構造（要素・
// role 属性の種類）は AC-6 の表が名指ししておらず、AC-11-7 により機械検査の
// 対象外（表に書かれていない DOM 構造は検査されない）。表が明言する
// 「ラベル文字列（技術者・承認者）が2つとも表示されること」に加え、
// 「AC-6 の DOM 契約」6-3-i（グループが単一の radiogroup として空でない
// アクセシブル名を持つこと）・6-3-ii（radio がちょうど2つで、role props に
// 対応する側だけが選択済みであること）を検証する（AC-10-6-a）。

afterEach(() => {
  cleanup();
});

describe("RoleSwitcher - AC-6-3", () => {
  it("選択肢はちょうど2つ（技術者・承認者）が文字として表示される", () => {
    render(<RoleSwitcher role="Engineer" onChange={vi.fn()} />);
    expect(screen.getByText("技術者")).toBeTruthy();
    expect(screen.getByText("承認者")).toBeTruthy();
  });

  it("6-3-i: 2つの選択肢を包む単一の要素が radiogroup ロールで、空でないアクセシブル名を持つ", () => {
    render(<RoleSwitcher role="Engineer" onChange={vi.fn()} />);
    const groups = screen.getAllByRole("radiogroup");
    expect(groups).toHaveLength(1);
    // ByRole の name オプションで空でないアクセシブル名を持つことを確認する
    // （dom-accessibility-api を直接 import しない。D-1-1）。
    expect(screen.getByRole("radiogroup", { name: /\S/ })).toBe(groups[0]);
  });

  it.each([
    { role: "Engineer" as const, checkedLabel: "技術者", uncheckedLabel: "承認者" },
    { role: "Approver" as const, checkedLabel: "承認者", uncheckedLabel: "技術者" },
  ])(
    "6-3-ii: role=$role のとき radio はちょうど2つで、$checkedLabel のみが選択済みになる",
    ({ role, checkedLabel, uncheckedLabel }) => {
      render(<RoleSwitcher role={role} onChange={vi.fn()} />);
      expect(screen.getAllByRole("radio")).toHaveLength(2);
      expect(
        screen.getByRole("radio", { name: checkedLabel, checked: true }),
      ).toBeTruthy();
      expect(
        screen.getByRole("radio", { name: uncheckedLabel, checked: false }),
      ).toBeTruthy();
    },
  );

  // AC-10-5: role は Role 型（Engineer / Approver）のみ。「ゲスト」を表す値を持たない。
  it("型: role は Engineer / Approver のみで、Guest を受け付けない（AC-6-3・AC-10-5）", () => {
    // @ts-expect-error role は Role 型（Engineer | Approver）のみ許可される
    const invalid = <RoleSwitcher role="Guest" onChange={vi.fn()} />;
    void invalid;
  });
});
