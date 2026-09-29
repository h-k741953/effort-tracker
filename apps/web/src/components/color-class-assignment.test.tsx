/** @vitest-environment jsdom */
import { readFileSync } from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { Button, type ButtonVariant } from "./button";
import { StatusBadge, type StatusBadgeState } from "./status-badge";
import { Card } from "./card";
import { FieldError } from "./field-error";
import { ErrorBanner } from "./error-banner";

// docs/specs/design-system.md AC-6 の表が名指しする色トークンの割当
// （6-1 / 6-2 / 6-4 / 6-6。検証手段は AC-10-3-b）。
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
      primary: ["bg-primary", "text-primary-foreground"],
      danger: ["bg-danger", "text-danger-foreground"],
      secondary: ["bg-surface", "text-surface-foreground", "border-border"],
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

  it("ErrorBanner（6-7）は role='alert' の要素に --danger を指すユーティリティが少なくとも1つ現れる（10-3-b・AC-11-21）", () => {
    // AC-6 の表の 6-7 は「--danger を用い」としか書かず、どの CSS
    // プロパティ（地色・文字色・境界線）へ当てるかを名指ししていない
    // ため、いずれか1つが現れることだけを読む（プロパティを名指ししない）。
    render(<ErrorBanner>失敗しました</ErrorBanner>);
    const classes = classesOf(screen.getByRole("alert"));
    const dangerClasses = ["bg-danger", "text-danger", "border-danger"];
    expect(classes.some((c) => dangerClasses.includes(c))).toBe(true);
  });
});
