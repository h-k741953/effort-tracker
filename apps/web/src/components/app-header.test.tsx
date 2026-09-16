/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup, within } from "@testing-library/react";
import { AppHeader } from "./app-header";
import { metadata } from "@/app/layout";

// docs/specs/design-system.md AC-6-11（検証手段は AC-10-6）。
//
// 「アプリ名」の具体的な DOM 構造（見出しレベル等）は AC-6 の表が名指し
// しておらず AC-11-7 により検査対象外。ただしアプリ名の文字列が
// `apps/web/src/app/layout.tsx` の metadata.title と同一であることは
// 6-11-i の DOM 契約が定めるため、ここで検証する（AC-10-6-c）。
//
// 突き合わせは相互比較（見出しのテキストと metadata.title を互いに比べる
// 形）では行わない。相互比較だと、両方を同時に書き換える一括改名を
// 素通ししてしまう（AC-11-13 が指摘する穴）。下記の定数は正解の写しでは
// なく、6-11-i 本条文が定めた値（effort-tracker）をテストへピン留めした
// ものである。正解は本条文が持ち、実装（見出し・metadata.title）は
// それに従う関係であって、docs の外に正解が増えるわけではない。

afterEach(() => {
  cleanup();
});

describe("AppHeader - AC-6-11", () => {
  it("<header> を出力し、アプリ名を見出し要素として含む", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    expect(within(header).getByRole("heading")).toBeTruthy();
  });

  // 6-11-i の定数（正解の写しではなくテスト側のピン留め。上記コメント参照）。
  const EXPECTED_APP_NAME = "effort-tracker";

  it("6-11-i: header 内の見出しのテキストが定数と一致する", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    const heading = within(header).getByRole("heading");
    expect(heading.textContent).toBe(EXPECTED_APP_NAME);
  });

  it("6-11-i: layout.tsx の metadata.title が定数と一致する", () => {
    expect(metadata.title).toBe(EXPECTED_APP_NAME);
  });

  it("right に渡した内容を差し込む", () => {
    render(<AppHeader right={<span>差込内容</span>} />);
    expect(screen.getByText("差込内容")).toBeTruthy();
  });
});
