import type { ButtonHTMLAttributes } from "react";
import { readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import type { AppHeaderProps } from "./app-header";
import type { ButtonProps } from "./button";
import type { CardProps } from "./card";
import type { ConfirmDialogProps } from "./confirm-dialog";
import type { EmptyStateProps } from "./empty-state";
import type { ErrorBannerProps } from "./error-banner";
import type { FieldErrorProps } from "./field-error";
import type { NumberFieldProps } from "./number-field";
import type { PaginationProps } from "./pagination";
import type { RoleSwitcherProps } from "./role-switcher";
import type { StatusBadgeProps } from "./status-badge";

// docs/specs/design-system.md AC-6 / AC-10-5（限界は AC-11-7）。
//
// AC-5-1 の台帳の11本それぞれについて、公開する props 型の独自のキーの集合（ネイティブ属性の
// 型を継承している場合は、継承した分を除いたもの）が AC-6 の表の「公開 props」欄に挙がる
// 名前の集合と一致することを、tsc（`npx tsc --noEmit`）が型レベルで読む。キーが足りなくても
// 余っても型エラーになる。vitest は型を読まないため、この検査の本体は型検査の側にある。
//
// 期待するキーの集合はテスト側に書いた写しであり、tsc は AC-6 の表の字面を読まない（AC-11-7）。
// 継承したネイティブ属性の中身（Button の `children` 等）は読まない。

type OwnKeys<P, Native = object> = Exclude<keyof P, keyof Native>;
/** A と B が同じ文字列リテラルの和であるとき true。 */
type SameKeys<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;

// 値の `true satisfies SameKeys<...>` は、集合が一致しないとき（型が false になるため）型エラーになる。
// キーはファイル名であり、台帳の11本（= components/ 直下の実装 *.tsx）と過不足なく対応することを
// 下の it が読む。1本を落とすと、その props の検査が消えるため。
const CHECKS = {
  "app-header.tsx": true satisfies SameKeys<OwnKeys<AppHeaderProps>, "right">,
  "button.tsx": true satisfies SameKeys<OwnKeys<ButtonProps, ButtonHTMLAttributes<HTMLButtonElement>>, "variant">,
  "card.tsx": true satisfies SameKeys<OwnKeys<CardProps>, "children" | "title">,
  "confirm-dialog.tsx": true satisfies SameKeys<
    OwnKeys<ConfirmDialogProps>,
    "open" | "title" | "description" | "confirmLabel" | "confirmVariant" | "onConfirm" | "onCancel"
  >,
  "empty-state.tsx": true satisfies SameKeys<OwnKeys<EmptyStateProps>, "title" | "description" | "action">,
  "error-banner.tsx": true satisfies SameKeys<OwnKeys<ErrorBannerProps>, "children" | "onRetry">,
  "field-error.tsx": true satisfies SameKeys<OwnKeys<FieldErrorProps>, "id" | "children">,
  "number-field.tsx": true satisfies SameKeys<
    OwnKeys<NumberFieldProps>,
    "id" | "label" | "value" | "onChange" | "min" | "max" | "step" | "errorId" | "invalid"
  >,
  "pagination.tsx": true satisfies SameKeys<OwnKeys<PaginationProps>, "page" | "pageCount" | "onPageChange">,
  "role-switcher.tsx": true satisfies SameKeys<OwnKeys<RoleSwitcherProps>, "role" | "onChange">,
  "status-badge.tsx": true satisfies SameKeys<OwnKeys<StatusBadgeProps>, "state">,
};

const componentsDir = fileURLToPath(new URL(".", import.meta.url));

describe("公開 props のキーの集合 - AC-6 / AC-10-5", () => {
  it("型の検査が components/ 直下の実装 *.tsx の全件を覆う", () => {
    const actual = readdirSync(componentsDir, { withFileTypes: true })
      .filter((e) => e.isFile() && e.name.endsWith(".tsx") && !e.name.endsWith(".test.tsx"))
      .map((e) => e.name)
      .sort();
    expect(actual.length).toBeGreaterThan(0);
    expect(Object.keys(CHECKS).sort()).toEqual(actual);
  });
});
