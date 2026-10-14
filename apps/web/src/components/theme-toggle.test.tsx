/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, within, fireEvent } from "@testing-library/react";
import { useState } from "react";
import { ThemeToggle } from "./theme-toggle";
import type { Theme } from "@/lib/theme-cookie";

// docs/specs/design-system.md AC-6-14（6-14-i〜6-14-v。検証手段は AC-10-6・10-6-r）。
//
// 6-14-i〜6-14-iv を RoleSwitcher の 10-6-a・10-6-g と同じ読み方で読む（name を直接読む検査を
// 含む）。6-14-v は、document.documentElement の data-theme 属性・localStorage の全体・
// document.cookie を発火の前と後で読み、変わらないことを読む。前の値は、それぞれ空でない値を
// 検査の側で先に置いてから読む（空どうしの一致にしない）。

beforeEach(() => {
  document.documentElement.removeAttribute("data-theme");
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute("data-theme");
  localStorage.clear();
  for (const part of document.cookie.split(";")) {
    const name = part.split("=")[0].trim();
    if (name) document.cookie = `${name}=; Max-Age=0; Path=/`;
  }
});

describe("ThemeToggle - AC-6-14", () => {
  const namesOf = (group: HTMLElement) =>
    within(group)
      .getAllByRole("radio")
      .map((radio) => radio.getAttribute("name"));

  it("6-14-i: 2つの選択肢を包む単一の要素が radiogroup のロールと空でないアクセシブル名で問い合わせられる", () => {
    render(<ThemeToggle theme="light" onChange={vi.fn()} />);
    const groups = screen.getAllByRole("radiogroup");
    expect(groups).toHaveLength(1);
    expect(screen.getByRole("radiogroup", { name: /\S/ })).toBe(groups[0]);
  });

  it.each([
    { theme: "light" as const, checkedLabel: "ライト", uncheckedLabel: "ダーク" },
    { theme: "dark" as const, checkedLabel: "ダーク", uncheckedLabel: "ライト" },
  ])(
    "6-14-ii: theme=$theme のとき radio はちょうど2つで、$checkedLabel のみが選択済みになる",
    ({ theme, checkedLabel, uncheckedLabel }) => {
      render(<ThemeToggle theme={theme} onChange={vi.fn()} />);
      expect(screen.getAllByRole("radio")).toHaveLength(2);
      expect(screen.getByRole("radio", { name: checkedLabel, checked: true })).toBeTruthy();
      expect(screen.getByRole("radio", { name: uncheckedLabel, checked: false })).toBeTruthy();
    },
  );

  it("6-14-iii: 同一文書に2つ描画しても、各 radiogroup の内側で対応する側だけが選択済みである", () => {
    render(
      <>
        <ThemeToggle theme="light" onChange={vi.fn()} />
        <ThemeToggle theme="dark" onChange={vi.fn()} />
      </>,
    );
    const groups = screen.getAllByRole("radiogroup");
    expect(groups).toHaveLength(2);
    const [lightGroup, darkGroup] = groups;
    expect(within(lightGroup).getByRole("radio", { name: "ライト", checked: true })).toBeTruthy();
    expect(within(lightGroup).getByRole("radio", { name: "ダーク", checked: false })).toBeTruthy();
    expect(within(darkGroup).getByRole("radio", { name: "ダーク", checked: true })).toBeTruthy();
    expect(within(darkGroup).getByRole("radio", { name: "ライト", checked: false })).toBeTruthy();
  });

  it("6-14-iii: 一方のインスタンスで選択を変更しても、他方のインスタンスの選択を解除しない", () => {
    function Wrapper() {
      const [theme1, setTheme1] = useState<Theme>("light");
      const [theme2, setTheme2] = useState<Theme>("dark");
      return (
        <>
          <ThemeToggle theme={theme1} onChange={setTheme1} />
          <ThemeToggle theme={theme2} onChange={setTheme2} />
        </>
      );
    }
    render(<Wrapper />);
    const [group1, group2] = screen.getAllByRole("radiogroup");
    fireEvent.click(within(group1).getByRole("radio", { name: "ダーク" }));
    expect(within(group2).getByRole("radio", { name: "ダーク", checked: true })).toBeTruthy();
  });

  // jsdom 上では React の制御された input が選択状態を復元するため、独立性の観測は name が
  // 全インスタンスで固定でも落ちない（11-15）。name を直接読む検査を併せて課す（10-6-a と同じ）。
  it("6-14-iii: 2つのインスタンスの radiogroup は、内側の radio が持つ name を互いに共有しない", () => {
    render(
      <>
        <ThemeToggle theme="light" onChange={vi.fn()} />
        <ThemeToggle theme="dark" onChange={vi.fn()} />
      </>,
    );
    const groups = screen.getAllByRole("radiogroup");
    expect(groups).toHaveLength(2);
    const [first, second] = groups.map(namesOf);
    expect(first.filter((name) => second.includes(name))).toEqual([]);
  });

  it("6-14-iii: 各 radiogroup の内側では、2つの radio が同じ name を共有する（単一である）", () => {
    render(
      <>
        <ThemeToggle theme="light" onChange={vi.fn()} />
        <ThemeToggle theme="dark" onChange={vi.fn()} />
      </>,
    );
    const groups = screen.getAllByRole("radiogroup");
    expect(groups).toHaveLength(2);
    for (const names of groups.map(namesOf)) {
      expect(names).toHaveLength(2);
      expect(new Set(names).size).toBe(1);
      expect(names[0]).toBeTruthy();
    }
  });

  it("6-14-iv: 選択済みでない側を発火させると onChange がちょうど1回、その選択肢の値を引数に呼ばれる（両向き）", () => {
    const fromLight = vi.fn();
    render(<ThemeToggle theme="light" onChange={fromLight} />);
    fireEvent.click(screen.getByRole("radio", { name: "ダーク" }));
    expect(fromLight).toHaveBeenCalledTimes(1);
    expect(fromLight.mock.calls[0]).toEqual(["dark"]);

    cleanup();

    const fromDark = vi.fn();
    render(<ThemeToggle theme="dark" onChange={fromDark} />);
    fireEvent.click(screen.getByRole("radio", { name: "ライト" }));
    expect(fromDark).toHaveBeenCalledTimes(1);
    expect(fromDark.mock.calls[0]).toEqual(["light"]);
  });

  describe("6-14-v: 副作用の不在（保存・属性の付け替え・OS の設定の読み取りをしない）", () => {
    function snapshot() {
      return {
        dataTheme: document.documentElement.getAttribute("data-theme"),
        localStorage: JSON.stringify(Object.entries(localStorage).sort()),
        cookie: document.cookie,
      };
    }

    it.each([
      { from: "light" as const, to: "ダーク" },
      { from: "dark" as const, to: "ライト" },
    ])("theme=$from から $to を選んでも、data-theme・localStorage・document.cookie が変わらない", ({ from, to }) => {
      // 前の値は、検査の側で空でない値を先に置く（空どうしの一致にしない）。
      document.documentElement.setAttribute("data-theme", from);
      localStorage.setItem("probe", "before");
      document.cookie = "probe=before; Path=/";
      const before = snapshot();
      expect(before.dataTheme).toBe(from);
      expect(before.localStorage).not.toBe("[]");
      expect(before.cookie).not.toBe("");

      render(<ThemeToggle theme={from} onChange={vi.fn()} />);
      fireEvent.click(screen.getByRole("radio", { name: to }));

      expect(snapshot()).toEqual(before);
    });

    it("描画しただけでも、data-theme・localStorage・document.cookie が変わらない", () => {
      document.documentElement.setAttribute("data-theme", "light");
      localStorage.setItem("probe", "before");
      document.cookie = "probe=before; Path=/";
      const before = snapshot();
      render(<ThemeToggle theme="dark" onChange={vi.fn()} />);
      expect(snapshot()).toEqual(before);
    });
  });

  // 6-14: 太陽・月の図案は aria-hidden="true" とする（図案を置く場合）。
  it("太陽・月の図案（svg）は aria-hidden=\"true\" で、テキストを持たない", () => {
    const { container } = render(<ThemeToggle theme="light" onChange={vi.fn()} />);
    for (const svg of Array.from(container.querySelectorAll("svg"))) {
      expect(svg.getAttribute("aria-hidden")).toBe("true");
      expect(svg.querySelector("title")).toBeNull();
    }
  });

  it("型: theme は light / dark のみで、保存・属性の付け替えのための props を持たない", () => {
    // @ts-expect-error theme は Theme 型（light | dark）のみ許可される（OS の設定に従う値を持たない）
    const system = <ThemeToggle theme="system" onChange={vi.fn()} />;
    // @ts-expect-error 保存先などの props は公開 props に無い（AC-6 の網羅）
    const withStorage = <ThemeToggle theme="light" onChange={vi.fn()} storage="cookie" />;
    void system;
    void withStorage;
  });
});
