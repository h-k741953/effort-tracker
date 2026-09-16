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
// 正解を2箇所に作らないため、期待値リテラルを2つ書かず、layout.tsx の
// metadata.title を import して見出しのテキストと突き合わせる
// （片方だけが変われば落ち、両方を同時に変えれば緑のまま通る。AC-11-13）。

afterEach(() => {
  cleanup();
});

describe("AppHeader - AC-6-11", () => {
  it("<header> を出力し、アプリ名を見出し要素として含む", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    expect(within(header).getByRole("heading")).toBeTruthy();
  });

  it("6-11-i: header 内の見出しのテキストが layout.tsx の metadata.title と一致する", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    const heading = within(header).getByRole("heading");
    expect(heading.textContent).toBe(metadata.title);
  });

  it("right に渡した内容を差し込む", () => {
    render(<AppHeader right={<span>差込内容</span>} />);
    expect(screen.getByText("差込内容")).toBeTruthy();
  });
});
