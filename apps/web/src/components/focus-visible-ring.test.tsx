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

// AC-10-3-a の母集団（要素の側）: 要素名だけを母集団にすると、`div` へ ARIA の
// ウィジェットロールを与えて対話要素にした実装がまるごと読まれないまま緑になる
// （列挙の母集団を実ディレクトリへ突き合わせたのと同型の穴が、要素の側に残って
// いた）。そこで (i) ネイティブの対話要素、(ii) フォーカスを受けて操作される
// ARIA のウィジェットロール、(iii) 負でない tabindex の3つを母集団とする。
// 負の tabindex はキーボードで到達できないため除く（`ConfirmDialog` が初期
// フォーカスのために置く `tabIndex={-1}` は対話要素ではない）。
const NATIVE_INTERACTIVE_SELECTOR = "button, input, select, textarea, a";
const INTERACTIVE_ARIA_ROLES = [
  "button",
  "link",
  "checkbox",
  "radio",
  "switch",
  "tab",
  "menuitem",
  "menuitemcheckbox",
  "menuitemradio",
  "option",
  "textbox",
  "searchbox",
  "combobox",
  "slider",
  "spinbutton",
];
const TABBABLE_SELECTOR = '[tabindex]:not([tabindex^="-"])';
const INTERACTIVE_SELECTOR = [
  NATIVE_INTERACTIVE_SELECTOR,
  ...INTERACTIVE_ARIA_ROLES.map((role) => `[role="${role}"]`),
  TABBABLE_SELECTOR,
].join(", ");

// AC-10-3-j: AC-4-6（`outline-none` を書くなら代替のリングを伴う）の判定を
// **要素単位**で読む。実装ファイルのテキストを1ファイル単位で読む形（10-3 が
// 持つ検査）では、`outline-none` を或る要素へ、リング指定を別の要素へ置いた
// 実装が素通りする —— ファイルのどこかにリング指定が在れば判定が満たされる
// ためである。描画結果は合成後の class を持つため、要素単位の判定はここで行う。
function violatesOutlineNoneRuleOnElement(element: Element): boolean {
  const classes = (element.getAttribute("class") ?? "").split(/\s+/).filter(Boolean);
  if (!classes.includes("outline-none")) return false;
  return !classes.some((className) => className.startsWith("focus-visible:ring-"));
}

/** 描画結果のうち、outline-none を代替のリングなしで持つ要素を返す。 */
function outlineNoneViolations(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>("*")).filter((element) =>
    violatesOutlineNoneRuleOnElement(element),
  );
}

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

  // AC-10-3-a: 母集団の取り方そのものの判別力。セレクタを要素名だけへ戻す変更、
  // あるいはロールの綴りを崩す変更が落ちるように、選ばれる側と選ばれない側の
  // 両方を、実ケースと同じセレクタで読む。ロールでない `role="alert"` と、負の
  // tabindex（キーボードで到達できない）は選ばれない側に置く。
  it("対話要素の母集団は、ネイティブ要素・ARIA ロール・負でない tabindex のいずれでも取れる", () => {
    const { container } = render(
      <div>
        <button id="native-button">押す</button>
        <div id="aria-button" role="button" />
        <div id="aria-link" role="link" />
        <div id="tabbable" tabIndex={0} />
        <div id="plain" />
        <div id="untabbable" tabIndex={-1} />
        <div id="non-widget-role" role="alert" />
      </div>,
    );
    const matched = Array.from(container.querySelectorAll<HTMLElement>(INTERACTIVE_SELECTOR))
      .map((element) => element.id)
      .sort();
    expect(
      matched,
      "対話要素の母集団が期待と一致しない。要素名だけへ戻すとロールで対話要素にした" +
        "実装が読まれず、逆に広げすぎると条文適合の実装が落ちる（AC-10-3-a）。",
    ).toEqual(["aria-button", "aria-link", "native-button", "tabbable"]);
  });

  // AC-10-3-j: outline-none を持つ要素は、同じ要素でリングを与えること。
  it.each(ALL_CASES)(
    "$name: outline-none を持つ要素は同じ要素で focus-visible:ring- を与える（AC-4-6）",
    ({ render: renderCase }) => {
      const { container } = renderCase();
      expect(
        outlineNoneViolations(container).map(
          (element) => `<${element.tagName.toLowerCase()} class="${element.getAttribute("class")}">`,
        ),
        "outline-none を持つ要素が、同じ要素でリング指定を伴っていない（AC-4-6）。" +
          "ファイル単位の判定（10-3）はリング指定が同じファイルのどこかに在れば通るため、" +
          "要素単位ではここで読む（AC-10-3-j）。",
      ).toEqual([]);
    },
  );

  // AC-10-3-j の判別力: 判定を恒偽へ潰す変更・要素の境界を越えて読む変更が落ちる
  // ように、実ケースと同じ関数へ描画結果を与えて両側を読む（10-3-g と同じ形）。
  const OUTLINE_NONE_CASES: Array<{
    name: string;
    render: () => { container: HTMLElement };
    expected: number;
  }> = [
    {
      name: "同じ要素が outline-none とリング指定を持つ",
      render: () => render(<div className="outline-none focus-visible:ring-2 ring-focus-ring" />),
      expected: 0,
    },
    {
      name: "outline-none だけを持つ",
      render: () => render(<div className="outline-none p-2" />),
      expected: 1,
    },
    {
      name: "outline-none とリング指定が別の要素に分かれている",
      render: () =>
        render(
          <div className="outline-none">
            <span className="focus-visible:ring-2 ring-focus-ring" />
          </div>,
        ),
      expected: 1,
    },
    {
      name: "outline-none を持たない",
      render: () => render(<div className="focus-visible:ring-2 ring-focus-ring" />),
      expected: 0,
    },
  ];

  it.each(OUTLINE_NONE_CASES)(
    "4-6 の要素単位の判定: $name",
    ({ render: renderCase, expected }) => {
      const { container } = renderCase();
      expect(
        outlineNoneViolations(container).length,
        "4-6 の要素単位の判定が期待と異なる（AC-10-3-j）。",
      ).toBe(expected);
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
