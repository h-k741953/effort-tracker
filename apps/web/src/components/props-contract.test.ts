import type { AllHTMLAttributes, ButtonHTMLAttributes, ClassAttributes, ComponentProps, ComponentPropsWithoutRef, HTMLAttributes, ReactNode } from "react";
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
import type { NumberField } from "./number-field";
import type { Pagination } from "./pagination";
import type { RoleSwitcher } from "./role-switcher";
import type { StatusBadge } from "./status-badge";

// docs/specs/design-system.md AC-6 / AC-10-5（限界は AC-11-7）。
//
// AC-5-1 の台帳の11本それぞれについて、公開する props 型の独自のキーの集合（ネイティブ属性の
// 型を継承している場合は、継承した分を除いたもの）が AC-6 の表の「公開 props」欄に挙がる
// 名前の集合と一致することを、tsc（`npx tsc --noEmit`）が型レベルで読む。キーが足りなくても
// 余っても型エラーになる。vitest は型を読まないため、この検査の本体は型検査の側にある。
//
// 期待するキーの集合はテスト側に書いた写しであり、tsc は AC-6 の表の字面を読まない（AC-11-7）。
// 継承したネイティブ属性の中身（Button の `children` 等）は読まない。

/** 標準属性のキー（継承した分として扱う名前の母集団）。 */
type NativeKeys =
  | keyof AllHTMLAttributes<HTMLElement>
  | keyof ButtonHTMLAttributes<HTMLButtonElement>
  | keyof ClassAttributes<HTMLElement>; // key / ref
/** 標準属性を継承している型だけが全部持つキー。AC-6 の表はこれらを公開 props に挙げない。 */
type InheritMarkers = "className" | "style" | "tabIndex" | "onClick";
/** 継承しているか否かを、型（キーの集合）からコンポーネントごとに読む。 */
type Inherits<P> = [InheritMarkers] extends [keyof P] ? true : false;
/** 独自のキー。継承している型からだけ標準属性のキーを除き、継承していない型からは何も除かない。 */
type OwnKeys<P> = Inherits<P> extends true ? Exclude<keyof P, NativeKeys> : keyof P;
/** 期待するキー。継承している型では、表の名前のうち標準属性と同名のものは比べない。 */
type ExpectedKeys<P, E extends string> = Inherits<P> extends true ? Exclude<E, NativeKeys> : E;
/** A と B が同じ文字列リテラルの和であるとき true。 */
type SameKeys<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
/** P（コンポーネントの引数型）の独自のキーが E と一致するとき true。 */
type Matches<P, E extends string> = SameKeys<OwnKeys<P>, ExpectedKeys<P, E>>;

// 述語 Matches の両側（10-5。10-3-g と同じ形）。`true satisfies X` は X が true でないとき、
// `false satisfies X` は X が false でないとき型エラーになる。
interface InheritsHtml extends HTMLAttributes<HTMLDivElement> {
  foo: string;
}
interface InheritsButton extends ButtonHTMLAttributes<HTMLButtonElement> {
  foo?: string;
  children?: ReactNode; // 宣言し直しても比べる集合に現れない
}
type InheritsPropsWithoutRef = ComponentPropsWithoutRef<"button"> & { foo?: string };
type InheritsOmit = Omit<HTMLAttributes<HTMLParagraphElement>, "id" | "children" | "role"> & {
  id: string;
  children: ReactNode;
};
type NoInherit = { id: string; title?: string; children?: ReactNode; role: string; onChange: () => void };

// 偽 Red にならない側（条文どおりの形は true）。継承元の書き方を替えた3形 + 継承元から一部を除いた形。
const _predTrue = [
  true satisfies Matches<InheritsHtml, "foo">,
  true satisfies Matches<InheritsButton, "foo" | "children">,
  true satisfies Matches<InheritsPropsWithoutRef, "foo">,
  true satisfies Matches<InheritsOmit, "id" | "children">,
  // 継承せず標準属性と同名の独自キーを持つ形は、そのキーを独自のキーとして比べる。
  true satisfies Matches<NoInherit, "id" | "title" | "children" | "role" | "onChange">,
];
// すり抜けない側（キーが余る・足りない形は false）。
const _predFalse = [
  // インライン追加でキーが余る（3形すべて）
  false satisfies Matches<InheritsHtml & { extra: 1 }, "foo">,
  false satisfies Matches<InheritsButton & { extra?: 1 }, "foo" | "children">,
  false satisfies Matches<InheritsPropsWithoutRef & { extra?: 1 }, "foo">,
  false satisfies Matches<InheritsOmit & { extra?: 1 }, "id" | "children">,
  // キーが足りない
  false satisfies Matches<InheritsHtml, "foo" | "bar">,
  false satisfies Matches<InheritsPropsWithoutRef, "foo" | "bar">,
  // 継承していない型からは何も除かない（標準属性と同名のキーが余る・足りないとき false）
  false satisfies Matches<NoInherit, "title" | "children" | "role" | "onChange">,
  false satisfies Matches<NoInherit, "id" | "title" | "children" | "role" | "onChange" | "extra">,
  false satisfies Matches<{ id: string }, never>,
  false satisfies Matches<{ foo: string; extra: 1 }, "foo">,
];
void _predTrue;
void _predFalse;

// 値の `true satisfies Matches<...>` は、集合が一致しないとき（型が false になるため）型エラーになる。
// 比べる型はコンポーネントの引数型 `ComponentProps<typeof X>`（エクスポートした XxxProps ではない。
// 引数へインラインで足した形をすり抜けないため）。
// キーはファイル名であり、台帳の11本（= components/ 直下の実装 *.tsx）と過不足なく対応することを
// 下の it が読む。1本を落とすと、その props の検査が消えるため。
const CHECKS = {
  "app-header.tsx": true satisfies Matches<ComponentProps<typeof AppHeader>, "right">,
  "button.tsx": true satisfies Matches<ComponentProps<typeof Button>, "variant">,
  "card.tsx": true satisfies Matches<ComponentProps<typeof Card>, "children" | "title">,
  "confirm-dialog.tsx": true satisfies Matches<
    ComponentProps<typeof ConfirmDialog>,
    "open" | "title" | "description" | "confirmLabel" | "confirmVariant" | "onConfirm" | "onCancel"
  >,
  "empty-state.tsx": true satisfies Matches<ComponentProps<typeof EmptyState>, "title" | "description" | "action">,
  "error-banner.tsx": true satisfies Matches<ComponentProps<typeof ErrorBanner>, "children" | "onRetry">,
  "field-error.tsx": true satisfies Matches<ComponentProps<typeof FieldError>, "id" | "children">,
  "number-field.tsx": true satisfies Matches<
    ComponentProps<typeof NumberField>,
    "id" | "label" | "value" | "onChange" | "min" | "max" | "step" | "errorId" | "invalid"
  >,
  "pagination.tsx": true satisfies Matches<ComponentProps<typeof Pagination>, "page" | "pageCount" | "onPageChange">,
  "role-switcher.tsx": true satisfies Matches<ComponentProps<typeof RoleSwitcher>, "role" | "onChange">,
  "status-badge.tsx": true satisfies Matches<ComponentProps<typeof StatusBadge>, "state">,
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
