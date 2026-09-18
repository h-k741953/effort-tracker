/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, cleanup } from "@testing-library/react";
import { Button } from "./button";
import { RoleSwitcher } from "./role-switcher";
import { ConfirmDialog } from "./confirm-dialog";
import { NumberField } from "./number-field";
import { Pagination } from "./pagination";
import { ErrorBanner } from "./error-banner";

// docs/specs/design-system.md AC-4-5（検証手段は AC-10-3-a）。
//
// AC-5-1 の共通コンポーネントのうち、対話要素（button / input / リンク）を
// 出力するものすべてを、対話要素が出る条件で描画し、出力に含まれる対話
// 要素の1つ1つについて、(i) フォーカス可視時にリングを与えるユーティリティ
// （focus-visible:ring- で始まるもの）と (ii) リング色として --focus-ring を
// 指すユーティリティ（ring-focus-ring。AC-1-3 のマッピング）の両方が
// class に現れることを読む。1要素でも欠けたら落ちる形にする。

afterEach(() => {
  cleanup();
});

const INTERACTIVE_SELECTOR = "button, input, a";

// AC-5-1 の共通コンポーネントのうち、対話要素を出力するものの一覧。
// 対話要素が出る条件（ConfirmDialog は open を真に、ErrorBanner は
// onRetry を与える）で描画する。
const CASES: Array<{ name: string; render: () => { container: HTMLElement } }> = [
  { name: "Button", render: () => render(<Button>送信</Button>) },
  {
    name: "RoleSwitcher",
    render: () => render(<RoleSwitcher role="Engineer" onChange={vi.fn()} />),
  },
  {
    name: "ConfirmDialog",
    render: () =>
      render(
        <ConfirmDialog
          open
          title="締めますか"
          description="この操作は取り消せません"
          confirmLabel="締める"
          onConfirm={vi.fn()}
          onCancel={vi.fn()}
        />,
      ),
  },
  {
    name: "NumberField",
    render: () =>
      render(<NumberField id="hours" label="時間" value={5} onChange={vi.fn()} />),
  },
  {
    name: "Pagination",
    render: () => render(<Pagination page={2} pageCount={5} onPageChange={vi.fn()} />),
  },
  {
    name: "ErrorBanner",
    render: () => render(<ErrorBanner onRetry={vi.fn()}>失敗しました</ErrorBanner>),
  },
];

describe("フォーカスリング - AC-4-5（AC-10-3-a）", () => {
  // 列挙が0件のときは失敗する（読み取りが空振りしたまま緑にならないように
  // する。10-3-a の要求）。
  it("対話要素を出すコンポーネントの列挙は0件ではない", () => {
    expect(CASES.length).toBeGreaterThan(0);
  });

  it.each(CASES)(
    "$name: 出力される対話要素のすべてが focus-visible:ring- と ring-focus-ring の両方を持つ",
    ({ render: renderCase }) => {
      const { container } = renderCase();
      const elements = Array.from(
        container.querySelectorAll<HTMLElement>(INTERACTIVE_SELECTOR),
      );
      // 列挙（描画された対話要素）が0件のときも失敗させる。
      expect(elements.length).toBeGreaterThan(0);

      for (const element of elements) {
        const classes = element.className.split(/\s+/).filter(Boolean);
        // (i) フォーカス可視時にリングを与えるユーティリティ
        // （focus-visible:ring- で始まるもの）。
        expect(classes.some((c) => c.startsWith("focus-visible:ring-"))).toBe(true);
        // (ii) リング色として --focus-ring を指すユーティリティ
        // （ring-focus-ring。AC-1-3 のマッピング）。Tailwind の状態
        // バリアント（focus-visible: 接頭辞）付きで現れる形を許すため、
        // 末尾一致で読む。
        expect(classes.some((c) => c.endsWith("ring-focus-ring"))).toBe(true);
      }
    },
  );
});
