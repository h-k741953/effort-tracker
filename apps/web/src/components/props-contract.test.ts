import type {
  ButtonHTMLAttributes,
  ComponentProps,
  ComponentPropsWithoutRef,
  ForwardRefExoticComponent,
  HTMLAttributes,
  JSX,
  ReactNode,
  RefAttributes,
} from "react";
import { readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import type { AppHeader } from "./app-header";
import type { Button } from "./button";
import type { Card } from "./card";
import type { ConfirmDialog } from "./confirm-dialog";
import type { EmptyState } from "./empty-state";
import type { ErrorBanner } from "./error-banner";
import type { FieldError } from "./field-error";
import type { NumberField, NumberFieldProps } from "./number-field";
import type { Pagination } from "./pagination";
import type { RoleSwitcher } from "./role-switcher";
import type { StatusBadge, StatusBadgeProps } from "./status-badge";

// docs/specs/design-system.md AC-6 / AC-10-5（限界は AC-11-7）。
//
// AC-5-1 の台帳の11本それぞれについて、公開する props 型の独自のキーの集合（ネイティブ属性の
// 型を継承している場合は、継承した分を除いたもの）が AC-6 の表の「公開 props」欄に挙がる
// 名前の集合と一致することを、tsc（`npx tsc --noEmit`）が型レベルで読む。キーが足りなくても
// 余っても型エラーになる。vitest は型を読まないため、この検査の本体は型検査の側にある。
//
// 期待するキーの集合と継承先の要素はテスト側に書いた写しであり、tsc は AC-6 の表の字面を
// 読まない（AC-11-7）。継承したネイティブ属性の中身（Button の `children` 等）は読まない。

/** 継承先の要素（null は継承しない）。除くのはその要素の標準属性のキーだけ（全要素の属性の和ではない）。 */
type InheritFrom = keyof JSX.IntrinsicElements | null;
/** 継承した分として除くキー。`key` / `ref` は継承の有無によらず常に除く。 */
type Inherited<T extends InheritFrom> = (T extends keyof JSX.IntrinsicElements ? keyof ComponentPropsWithoutRef<T> : never) | "key" | "ref";
/** A と B が同じ文字列リテラルの和であるとき true。 */
type SameKeys<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
/**
 * P（コンポーネントの引数型）の独自のキーが E（表の名前）と一致するとき true。T は継承先の要素で、
 * 継承する型ではその要素の標準属性のキーを両側から除く（表の名前のうち同名のもの —— Button の
 * `children` —— は比べない）。継承しない型（T = null）からは `key` / `ref` 以外を何も除かない。
 */
type Matches<P, E extends string, T extends InheritFrom> = SameKeys<Exclude<keyof P, Inherited<T>>, Exclude<E, Inherited<T>>>;

// 述語 Matches の両側（10-5。10-3-g と同じ形）。`true satisfies X` は X が true でないとき、
// `false satisfies X` は X が false でないとき型エラーになる。
type Variant = "primary" | "secondary" | "danger";
interface ButtonExtends extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  children?: ReactNode; // 宣言し直しても比べる集合に現れない
}
interface ButtonOmitStyle extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "style"> {
  variant?: Variant;
}
type ButtonWithoutRef = ComponentPropsWithoutRef<"button"> & { variant?: Variant };
type NumberFieldForwardRef = ComponentProps<ForwardRefExoticComponent<NumberFieldProps & RefAttributes<HTMLInputElement>>>;
interface CardPick extends Pick<HTMLAttributes<HTMLDivElement>, "className"> {
  children: ReactNode;
  title?: ReactNode;
}
type FieldErrorOmit = Omit<HTMLAttributes<HTMLParagraphElement>, "id" | "children" | "role"> & {
  id: string;
  children: ReactNode;
};
type NoInherit = { id: string; title?: string; children?: ReactNode; role: string; onChange: () => void };

// 偽 Red にならない側（条文どおりの形は true）。継承元の書き方を替えた形・継承元から一部を除いた形・
// forwardRef（`ref`）・継承せず標準属性と同名の独自キーを持つ形。
const _predTrue = [
  true satisfies Matches<ButtonExtends, "variant" | "children", "button">,
  true satisfies Matches<ButtonOmitStyle, "variant" | "children", "button">,
  true satisfies Matches<ButtonWithoutRef, "variant" | "children", "button">,
  true satisfies Matches<NumberFieldForwardRef, "id" | "label" | "value" | "onChange" | "min" | "max" | "step" | "errorId" | "invalid", null>,
  true satisfies Matches<CardPick, "children" | "title", "div">,
  true satisfies Matches<FieldErrorOmit, "id" | "children", "p">,
  true satisfies Matches<StatusBadgeProps & RefAttributes<HTMLSpanElement>, "state", null>,
  true satisfies Matches<NoInherit, "id" | "title" | "children" | "role" | "onChange", null>,
];
// すり抜けない側（キーが余る・足りない形は false）。
const _predFalse = [
  // 継承先の要素に無い属性名のキーが余る（全要素の属性の和にはある名前を含む）
  false satisfies Matches<ButtonExtends & { size?: "sm" | "md" }, "variant" | "children", "button">,
  false satisfies Matches<ButtonExtends & { href?: string }, "variant" | "children", "button">,
  false satisfies Matches<ButtonExtends & { loading?: boolean }, "variant" | "children", "button">,
  false satisfies Matches<ButtonExtends & { label?: string }, "variant" | "children", "button">,
  false satisfies Matches<ButtonExtends & { open?: boolean }, "variant" | "children", "button">,
  false satisfies Matches<ButtonWithoutRef & { extra?: 1 }, "variant" | "children", "button">,
  false satisfies Matches<CardPick & { extra?: 1 }, "children" | "title", "div">,
  // 継承しない型へのインライン追加
  false satisfies Matches<StatusBadgeProps & { viewerRole?: "Engineer" | "Approver" }, "state", null>,
  // キーが足りない（継承している型から表の名前を欠く）
  false satisfies Matches<ButtonHTMLAttributes<HTMLButtonElement>, "variant" | "children", "button">,
  false satisfies Matches<ButtonWithoutRef, "variant" | "children" | "bar", "button">,
  // 継承していない型からは何も除かない（標準属性と同名のキーが余る・足りないとき false）
  false satisfies Matches<NoInherit, "title" | "children" | "role" | "onChange", null>,
  false satisfies Matches<NoInherit, "id" | "title" | "children" | "role" | "onChange" | "extra", null>,
  false satisfies Matches<{ id: string }, never, null>,
  false satisfies Matches<{ foo: string; extra: 1 }, "foo", null>,
];
void _predTrue;
void _predFalse;

// 値の `true satisfies Matches<...>` は、集合が一致しないとき（型が false になるため）型エラーになる。
// 比べる型はコンポーネントの引数型 `ComponentProps<typeof X>`（エクスポートした XxxProps ではない。
// 引数へインラインで足した形をすり抜けないため）。
// 継承先の要素は AC-6 の表に合わせる。標準属性の透過を挙げるのは 6-2 の Button（`button`）だけで、
// 他の10本は null（継承しない）。
// キーはファイル名であり、台帳の11本（= components/ 直下の実装 *.tsx）と過不足なく対応することを
// 下の it が読む。1本を落とすと、その props の検査が消えるため。
const CHECKS = {
  "app-header.tsx": true satisfies Matches<ComponentProps<typeof AppHeader>, "right", null>,
  "button.tsx": true satisfies Matches<ComponentProps<typeof Button>, "variant" | "children", "button">,
  "card.tsx": true satisfies Matches<ComponentProps<typeof Card>, "children" | "title", null>,
  "confirm-dialog.tsx": true satisfies Matches<
    ComponentProps<typeof ConfirmDialog>,
    "open" | "title" | "description" | "confirmLabel" | "confirmVariant" | "onConfirm" | "onCancel",
    null
  >,
  "empty-state.tsx": true satisfies Matches<ComponentProps<typeof EmptyState>, "title" | "description" | "action", null>,
  "error-banner.tsx": true satisfies Matches<ComponentProps<typeof ErrorBanner>, "children" | "onRetry", null>,
  "field-error.tsx": true satisfies Matches<ComponentProps<typeof FieldError>, "id" | "children", null>,
  "number-field.tsx": true satisfies Matches<
    ComponentProps<typeof NumberField>,
    "id" | "label" | "value" | "onChange" | "min" | "max" | "step" | "errorId" | "invalid",
    null
  >,
  "pagination.tsx": true satisfies Matches<ComponentProps<typeof Pagination>, "page" | "pageCount" | "onPageChange", null>,
  "role-switcher.tsx": true satisfies Matches<ComponentProps<typeof RoleSwitcher>, "role" | "onChange", null>,
  "status-badge.tsx": true satisfies Matches<ComponentProps<typeof StatusBadge>, "state", null>,
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
