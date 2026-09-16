/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, within, fireEvent } from "@testing-library/react";
import { useState } from "react";
import { RoleSwitcher } from "./role-switcher";
import type { Role } from "@/lib/role-cookie";

// docs/specs/design-system.md AC-6-3（検証手段は AC-10-5・AC-10-6）。
//
// 「現在のロールが表示から判別できること」の具体的な DOM 構造（要素・
// role 属性の種類）は AC-6 の表が名指ししておらず、AC-11-7 により機械検査の
// 対象外（表に書かれていない DOM 構造は検査されない）。表が明言する
// 「ラベル文字列（技術者・承認者）が2つとも表示されること」に加え、
// 「AC-6 の DOM 契約」6-3-i（グループが単一の radiogroup として空でない
// アクセシブル名を持つこと）・6-3-ii（radio がちょうど2つで、role props に
// 対応する側だけが選択済みであること）・6-3-iii（同一文書に複数配置しても
// 各インスタンスが独立したグループとして振る舞うこと）を検証する
// （AC-10-6-a）。

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

  it("6-3-iii: 同一文書に2つ描画しても、各インスタンスの radiogroup が独立している", () => {
    render(
      <>
        <RoleSwitcher role="Engineer" onChange={vi.fn()} />
        <RoleSwitcher role="Approver" onChange={vi.fn()} />
      </>,
    );
    const groups = screen.getAllByRole("radiogroup");
    expect(groups).toHaveLength(2);

    const [engineerGroup, approverGroup] = groups;
    // 一方のインスタンスの選択が、他方の選択を解除しないことを読む。
    expect(
      within(engineerGroup).getByRole("radio", { name: "技術者", checked: true }),
    ).toBeTruthy();
    expect(
      within(engineerGroup).getByRole("radio", { name: "承認者", checked: false }),
    ).toBeTruthy();
    expect(
      within(approverGroup).getByRole("radio", { name: "承認者", checked: true }),
    ).toBeTruthy();
    expect(
      within(approverGroup).getByRole("radio", { name: "技術者", checked: false }),
    ).toBeTruthy();
  });

  it("6-3-iii: 一方のインスタンスで選択を変更しても、他方のインスタンスの選択を解除しない", () => {
    // 「一方のインスタンスの選択が、他方の選択を解除しない」を、初期描画
    // だけでなく実際の選択操作（クリック）を挟んで検証する。role props を
    // 呼び出し側の state として管理する構成にすることで、onChange から
    // 実際の選択の反映までを再現する。
    function Wrapper() {
      const [role1, setRole1] = useState<Role>("Engineer");
      const [role2, setRole2] = useState<Role>("Approver");
      return (
        <>
          <RoleSwitcher role={role1} onChange={setRole1} />
          <RoleSwitcher role={role2} onChange={setRole2} />
        </>
      );
    }

    render(<Wrapper />);
    const [group1, group2] = screen.getAllByRole("radiogroup");

    // group2 は「承認者」が選択済みのまま変えない。group1 の選択を
    // 「承認者」へ変更する操作が、group2 の選択を解除しないことを読む。
    fireEvent.click(within(group1).getByRole("radio", { name: "承認者" }));

    expect(
      within(group2).getByRole("radio", { name: "承認者", checked: true }),
    ).toBeTruthy();
  });

  it("6-3-iii: 2つのインスタンスの radiogroup は、内側の radio が持つ name を互いに共有しない", () => {
    // AC-10-6-a が課す「name を直接読む検査」。上2件の独立性の観測は、
    // name が全インスタンスで固定であっても落ちない（jsdom 上では React の
    // 制御された input が選択状態を復元するため。AC-11-15）。name の一意性を
    // 判別できるのはこの検査だけである。
    //
    // 6-3-iii は name の文字列を本仕様で固定しない（一意であることだけを
    // 求める）ため、期待値に具体的な name 文字列を持たせず、「一方の
    // radiogroup の radio の name が、他方の radiogroup のいずれの radio の
    // name とも一致しない」ことだけを読む。
    // なお、この検査はネイティブの <input type="radio"> を前提とする
    // （AC-11-15）。
    render(
      <>
        <RoleSwitcher role="Engineer" onChange={vi.fn()} />
        <RoleSwitcher role="Approver" onChange={vi.fn()} />
      </>,
    );
    const groups = screen.getAllByRole("radiogroup");
    expect(groups).toHaveLength(2);

    const namesOf = (group: HTMLElement) =>
      within(group)
        .getAllByRole("radio")
        .map((radio) => radio.getAttribute("name"));

    const [namesInFirstGroup, namesInSecondGroup] = groups.map(namesOf);
    const sharedNames = namesInFirstGroup.filter((name) =>
      namesInSecondGroup.includes(name),
    );
    expect(sharedNames).toEqual([]);
  });

  // AC-10-5: role は Role 型（Engineer / Approver）のみ。「ゲスト」を表す値を持たない。
  it("型: role は Engineer / Approver のみで、Guest を受け付けない（AC-6-3・AC-10-5）", () => {
    // @ts-expect-error role は Role 型（Engineer | Approver）のみ許可される
    const invalid = <RoleSwitcher role="Guest" onChange={vi.fn()} />;
    void invalid;
  });
});
