/** @vitest-environment jsdom */
import { readFileSync } from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup, within } from "@testing-library/react";
import { Button, type ButtonVariant } from "./button";
import { StatusBadge, type StatusBadgeState } from "./status-badge";
import { Card } from "./card";
import { FieldError } from "./field-error";
import { ErrorBanner } from "./error-banner";
import { SelectField } from "./select-field";
import { SideNav } from "./side-nav";

// docs/specs/design-system.md AC-6 の表が名指しする色トークンの割当
// （6-1 / 6-2 / 6-4 / 6-6 / 6-7 / 6-12 / 6-13。検証手段は AC-10-3-b）。
//
// 各コンポーネントを描画し、表が名指しした対象要素の class に、該当
// トークン --X を指すユーティリティ（bg-X / text-X / border-X）が現れる
// ことを読む。Button は variant の3値すべて、StatusBadge は state の3値
// すべてで描画し、自分の組だけを持つ（他 variant / 他 state の組が
// 現れない）ことまで読む（割当の取り違えが緑にならない形にする）。

afterEach(() => {
  cleanup();
});

function classesOf(element: Element): string[] {
  return element.className.split(/\s+/).filter(Boolean);
}

describe("色トークンの割当 - AC-6（AC-10-3-b）", () => {
  describe("Button（6-2）", () => {
    const VARIANT_TOKEN_CLASSES: Record<ButtonVariant, string[]> = {
      // 6-2: primary は地色 --primary に、--primary（始点）から --et-grad-to（終点）への
      // グラデーションを重ねる（from-primary / to-et-grad-to）。
      primary: ["bg-primary", "text-primary-foreground", "from-primary", "to-et-grad-to"],
      danger: ["bg-danger", "text-danger-foreground"],
      // 6-2: secondary のホバーの地色は --et-hover。
      secondary: ["bg-surface", "text-surface-foreground", "border-border", "hover:bg-et-hover"],
    };
    const ALL_TOKEN_CLASSES = Array.from(
      new Set(Object.values(VARIANT_TOKEN_CLASSES).flat()),
    );

    function expectOwnTokensOnly(classes: string[], variant: ButtonVariant): void {
      const expected = VARIANT_TOKEN_CLASSES[variant];

      for (const token of expected) {
        expect(classes).toContain(token);
      }
      const forbidden = ALL_TOKEN_CLASSES.filter((c) => !expected.includes(c));
      for (const token of forbidden) {
        expect(classes).not.toContain(token);
      }
    }

    it.each(Object.keys(VARIANT_TOKEN_CLASSES) as ButtonVariant[])(
      "variant=%s は自分の組のトークンだけを持つ（他 variant の組は現れない）",
      (variant) => {
        render(<Button variant={variant}>操作</Button>);
        expectOwnTokensOnly(classesOf(screen.getByRole("button", { name: "操作" })), variant);
      },
    );

    // AC-10-3-b: variant を省略した描画も読む。3値それぞれを明示した描画だけでは
    // 既定値を変える変更（6-2 の「既定 secondary」を別の値へ替える形）がどの
    // ケースにも掛からず素通りする。期待する既定値は条文（AC-6 の表の 6-2）の
    // 字面から取る —— テスト側に書いた値だけを期待値にすると、条文と乖離した
    // まま緑になる（10-2-a / 10-3-d と同じ理由）。
    it("variant を省略した描画は、条文が定める既定値の組だけを持つ（6-2）", () => {
      const specPath = path.join(
        process.cwd(),
        "..",
        "..",
        "docs",
        "specs",
        "design-system.md",
      );
      const hint =
        `仕様書 ${specPath} の AC-6 の表から 6-2 の既定値を読めなかった。条文の体裁を` +
        `変えたのなら、既定値の変更ではないのでこの抽出側を追随させること（10-3-b は` +
        `抽出手段を仕様で固定していない）。`;
      const spec = readFileSync(specPath, "utf8");
      const rowMatch = spec.match(/^\|\s*6-2\s*\|.*$/m);
      expect(rowMatch, hint).not.toBeNull();
      const defaultMatch = rowMatch![0].match(/既定\s*`([\w-]+)`/);
      expect(defaultMatch, `${hint} 「既定 \`…\`」を切り出せなかった。`).not.toBeNull();
      const defaultVariant = defaultMatch![1] as ButtonVariant;
      expect(
        Object.keys(VARIANT_TOKEN_CLASSES),
        `${hint} 条文が述べる既定値 ${defaultVariant} が variant の3値に無い。`,
      ).toContain(defaultVariant);

      render(<Button>操作</Button>);
      expectOwnTokensOnly(
        classesOf(screen.getByRole("button", { name: "操作" })),
        defaultVariant,
      );
    });
  });

  describe("StatusBadge（6-1）", () => {
    const STATE_LABELS: Record<StatusBadgeState, string> = {
      Draft: "下書き",
      PendingApproval: "締め済",
      Approved: "承認済",
    };
    const STATE_TOKEN_CLASSES: Record<StatusBadgeState, string[]> = {
      Draft: ["bg-state-draft", "text-state-draft-foreground"],
      PendingApproval: [
        "bg-state-pending-approval",
        "text-state-pending-approval-foreground",
      ],
      Approved: ["bg-state-approved", "text-state-approved-foreground"],
    };
    const ALL_TOKEN_CLASSES = Array.from(
      new Set(Object.values(STATE_TOKEN_CLASSES).flat()),
    );

    it.each(Object.keys(STATE_TOKEN_CLASSES) as StatusBadgeState[])(
      "state=%s は自分の組のトークンだけを持つ（他 state の組は現れない）",
      (state) => {
        render(<StatusBadge state={state} />);
        const classes = classesOf(screen.getByText(STATE_LABELS[state]));
        const expected = STATE_TOKEN_CLASSES[state];

        for (const token of expected) {
          expect(classes).toContain(token);
        }
        const forbidden = ALL_TOKEN_CLASSES.filter((c) => !expected.includes(c));
        for (const token of forbidden) {
          expect(classes).not.toContain(token);
        }
      },
    );
  });

  it("Card（6-4）は --surface / --surface-foreground / --border を読む", () => {
    render(<Card>本文</Card>);
    const classes = classesOf(screen.getByText("本文"));
    expect(classes).toContain("bg-surface");
    expect(classes).toContain("text-surface-foreground");
    expect(classes).toContain("border-border");
  });

  it("FieldError（6-6）は --danger を読む", () => {
    render(<FieldError id="hours-error">必須です</FieldError>);
    const classes = classesOf(screen.getByRole("alert"));
    expect(classes).toContain("text-danger");
  });

  // 10-3-b: ErrorBanner（6-7）は、role="alert" で問い合わせられる要素の class に bg-et-danger-soft と
  // border-danger が現れ、テキストが `エラー` と完全に一致する要素（6-7-i）の class に text-danger が
  // 現れる。2026-10-10 に承認したテーマ B の改定（3-6-b・3-6-c）。
  describe("ErrorBanner（6-7）", () => {
    it("role='alert' の要素の地色は --et-danger-soft、境界線は --danger", () => {
      render(<ErrorBanner>失敗しました</ErrorBanner>);
      const classes = classesOf(screen.getByRole("alert"));
      expect(classes).toContain("bg-et-danger-soft");
      expect(classes).toContain("border-danger");
    });

    it("種別表示（テキストが `エラー` と完全に一致する要素）の文字色は --danger", () => {
      render(<ErrorBanner>失敗しました</ErrorBanner>);
      const kind = within(screen.getByRole("alert")).getByText("エラー");
      expect(classesOf(kind)).toContain("text-danger");
    });

    // 6-7: 理由文の文字色は --foreground（3-6-c の組）。理由文を含む要素から role="alert" の
    // 要素までのどこかに text-foreground が当たっていること（どの要素に当てるかは固定しない）。
    it("理由文の文字色は --foreground（理由文の要素から alert の要素までのいずれかに当たる）", () => {
      render(<ErrorBanner>失敗しました</ErrorBanner>);
      const alert = screen.getByRole("alert");
      const chain: Element[] = [];
      for (let el: Element | null = screen.getByText("失敗しました"); el !== null; el = el.parentElement) {
        chain.push(el);
        if (el === alert) break;
      }
      expect(chain[chain.length - 1]).toBe(alert);
      expect(chain.some((el) => classesOf(el).includes("text-foreground"))).toBe(true);
    });
  });

  // 10-3-b: SelectField（6-12）は、選択の要素の class に、invalid が偽のとき border-et-input-border が
  // 現れ border-danger が現れないこと、真のとき border-danger が現れることを読む。
  describe("SelectField（6-12）", () => {
    const options = [{ value: "9", label: "9" }];

    it("invalid が偽のとき --et-input-border を指し、--danger を指さない", () => {
      render(<SelectField id="h" label="時" value="9" options={options} onChange={() => {}} />);
      const classes = classesOf(screen.getByRole("combobox"));
      expect(classes).toContain("border-et-input-border");
      expect(classes).not.toContain("border-danger");
    });

    it("invalid が真のとき --danger を指す", () => {
      render(<SelectField id="h" label="時" value="9" options={options} onChange={() => {}} invalid />);
      expect(classesOf(screen.getByRole("combobox"))).toContain("border-danger");
    });
  });

  // 10-3-b: SideNav（6-13）は、navigation のロールの要素か部品の出力の最も外側の要素のいずれかの
  // class に bg-et-sidebar が現れ、current が真の項目の class に bg-et-sidebar-active が現れ、
  // 偽の項目には（hover: を伴わない形で）現れないことを読む。
  describe("SideNav（6-13）", () => {
    function renderNav() {
      return render(
        <SideNav
          items={[
            { label: "勤務月", href: "/work-months", current: true },
            { label: "契約", href: "/contracts", current: false },
          ]}
          footerItems={[{ label: "ヘルプ", href: "/help", current: false }]}
        />,
      );
    }
    /** 項目の要素（リンク、またはそれを包む li）の class 群。 */
    function itemClasses(name: string): string[][] {
      const link = screen.getByRole("link", { name });
      const item = link.closest("li");
      return [classesOf(link), ...(item !== null ? [classesOf(item)] : [])];
    }

    it("navigation の要素か出力の最も外側の要素の地色は --et-sidebar", () => {
      const { container } = renderNav();
      const nav = screen.getByRole("navigation");
      const outermost = container.firstElementChild as Element;
      expect([...classesOf(nav), ...classesOf(outermost)]).toContain("bg-et-sidebar");
    });

    it("current が真の項目の地色は --et-sidebar-active", () => {
      renderNav();
      expect(itemClasses("勤務月").some((classes) => classes.includes("bg-et-sidebar-active"))).toBe(true);
    });

    it.each(["契約", "ヘルプ"])("current が偽の項目（%s）には bg-et-sidebar-active が現れない（hover: を伴う形は可）", (name) => {
      renderNav();
      expect(itemClasses(name).some((classes) => classes.includes("bg-et-sidebar-active"))).toBe(false);
    });
  });
});
