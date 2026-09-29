/** @vitest-environment jsdom */
import { readdirSync } from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, cleanup } from "@testing-library/react";
import { AppHeader } from "./app-header";
import { Button } from "./button";
import { Card } from "./card";
import { ConfirmDialog } from "./confirm-dialog";
import { EmptyState } from "./empty-state";
import { ErrorBanner } from "./error-banner";
import { FieldError } from "./field-error";
import { NumberField } from "./number-field";
import { Pagination } from "./pagination";
import { RoleSwitcher } from "./role-switcher";
import { StatusBadge } from "./status-badge";

// docs/specs/design-system.md AC-4-5（検証手段は AC-10-3-a）。
//
// AC-5-1 の共通コンポーネントのうち、対話要素（button / input / リンク）を
// 出力するものすべてを、対話要素が出る条件で描画し、出力に含まれる対話
// 要素の1つ1つについて、(i) フォーカス可視時にリングを与えるユーティリティ
// （focus-visible:ring- で始まるもの）と (ii) リング色として --focus-ring を
// 指すユーティリティ（ring-focus-ring。AC-1-3 のマッピング）の両方が
// class に現れることを読む。1要素でも欠けたら落ちる形にする。
//
// AC-10-3-a の母集団: 「対話要素を出力するもの」を手書きで列挙し、その列挙
// だけを描画すると、列挙に無いコンポーネントが対話要素を出し始めても誰も
// 読まない（10-3-h が 10-3 について是正したのと同型の穴）。したがって
// **11本すべてを描画**し、出力に現れた対話要素をすべて読む。あわせて、
// 母集団が components/ 直下の実装 *.tsx の集合と一致することと、対話要素を
// 出力するコンポーネントの集合が期待と一致することを読む。

afterEach(() => {
  cleanup();
});

// jsdom 環境では import.meta.url が file: スキームに解けないため、vitest の
// ルート（apps/web）からの相対でディレクトリを解く。解き先が正しいことは、
// この検査ファイル自身がそこに在ることで読む（陽性側の足場）。
const componentsDir = path.join(process.cwd(), "src", "components");

const INTERACTIVE_SELECTOR = "button, input, a";

// AC-5-1 の共通コンポーネント11本。対話要素が出る条件（ConfirmDialog は open を
// 真に、ErrorBanner は onRetry を与える）で描画する。props は AC-6 の表の範囲。
const ALL_CASES: Array<{
  name: string;
  file: string;
  render: () => { container: HTMLElement };
}> = [
  { name: "AppHeader", file: "app-header.tsx", render: () => render(<AppHeader />) },
  {
    name: "RoleSwitcher",
    file: "role-switcher.tsx",
    render: () => render(<RoleSwitcher role="Engineer" onChange={vi.fn()} />),
  },
  {
    name: "StatusBadge",
    file: "status-badge.tsx",
    render: () => render(<StatusBadge state="Draft" />),
  },
  { name: "Button", file: "button.tsx", render: () => render(<Button>送信</Button>) },
  { name: "Card", file: "card.tsx", render: () => render(<Card title="工数">本文</Card>) },
  {
    name: "NumberField",
    file: "number-field.tsx",
    render: () =>
      render(<NumberField id="hours" label="時間" value={5} onChange={vi.fn()} />),
  },
  {
    name: "FieldError",
    file: "field-error.tsx",
    render: () => render(<FieldError id="hours-error">必須です</FieldError>),
  },
  {
    name: "ErrorBanner",
    file: "error-banner.tsx",
    render: () => render(<ErrorBanner onRetry={vi.fn()}>失敗しました</ErrorBanner>),
  },
  {
    name: "EmptyState",
    file: "empty-state.tsx",
    render: () => render(<EmptyState title="実績がありません" />),
  },
  {
    name: "ConfirmDialog",
    file: "confirm-dialog.tsx",
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
    name: "Pagination",
    file: "pagination.tsx",
    render: () => render(<Pagination page={2} pageCount={5} onPageChange={vi.fn()} />),
  },
];

// 上記の props で描画したときに対話要素（button / input / リンク）が現れる
// コンポーネント。集合として読むため、出さなくなった側・出し始めた側の
// どちらの向きの変化も落ちる。
const INTERACTIVE_COMPONENTS = [
  "Button",
  "ConfirmDialog",
  "ErrorBanner",
  "NumberField",
  "Pagination",
  "RoleSwitcher",
];

describe("フォーカスリング - AC-4-5（AC-10-3-a）", () => {
  // 列挙が0件のときは失敗する（読み取りが空振りしたまま緑にならないように
  // する。10-3-a の要求）。
  it("描画する母集団の列挙は0件ではない", () => {
    expect(ALL_CASES.length).toBeGreaterThan(0);
  });

  // AC-10-3-a の母集団: 列挙が components/ 直下の実装 *.tsx の集合と過不足なく
  // 一致することを読む。落ちたファイルはリングの検査を丸ごと免れる。
  it("描画する母集団が components/ 直下の実装 *.tsx の集合と一致する", () => {
    const entries = readdirSync(componentsDir, { withFileTypes: true });
    // 解いたディレクトリが本当に components/ であることを読む（空振り防止）。
    expect(entries.map((entry) => entry.name)).toContain("focus-visible-ring.test.tsx");
    const actual = entries
      .filter((entry) => entry.isFile())
      .map((entry) => entry.name)
      .filter((name) => name.endsWith(".tsx") && !name.endsWith(".test.tsx"))
      .sort();
    expect(actual.length).toBeGreaterThan(0);
    expect(actual).toEqual(ALL_CASES.map((c) => c.file).sort());
  });

  it.each(ALL_CASES)(
    "$name: 出力される対話要素のすべてが focus-visible:ring- と ring-focus-ring の両方を持つ",
    ({ render: renderCase }) => {
      const { container } = renderCase();
      const elements = Array.from(
        container.querySelectorAll<HTMLElement>(INTERACTIVE_SELECTOR),
      );

      for (const element of elements) {
        const classes = element.className.split(/\s+/).filter(Boolean);
        // (i) フォーカス可視時にリングを与えるユーティリティ
        // （focus-visible:ring- で始まるもの）。
        expect(
          classes.some((c) => c.startsWith("focus-visible:ring-")),
          `対話要素 <${element.tagName.toLowerCase()}> が focus-visible:ring- を持たない（AC-4-5）。class: ${element.className}`,
        ).toBe(true);
        // (ii) リング色として --focus-ring を指すユーティリティ
        // （ring-focus-ring。AC-1-3 のマッピング）。Tailwind の状態
        // バリアント（focus-visible: 接頭辞）付きで現れる形を許すため、
        // 末尾一致で読む。
        expect(
          classes.some((c) => c.endsWith("ring-focus-ring")),
          `対話要素 <${element.tagName.toLowerCase()}> が ring-focus-ring を持たない（AC-4-5）。class: ${element.className}`,
        ).toBe(true);
      }
    },
  );

  // AC-10-3-a: 対話要素が1つも出ないまま上の it.each が空回りして緑になる形を
  // 落とす。集合で読むため、出力をやめた側も、新たに出し始めた側も落ちる。
  it("対話要素を出力するコンポーネントの集合が期待と一致する", () => {
    const emitting = ALL_CASES.filter((c) => {
      const { container } = c.render();
      const count = container.querySelectorAll(INTERACTIVE_SELECTOR).length;
      cleanup();
      return count > 0;
    })
      .map((c) => c.name)
      .sort();
    expect(emitting.length).toBeGreaterThan(0);
    expect(emitting).toEqual([...INTERACTIVE_COMPONENTS].sort());
  });
});
