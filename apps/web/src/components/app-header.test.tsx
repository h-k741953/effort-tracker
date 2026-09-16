/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup, within } from "@testing-library/react";
import { AppHeader } from "./app-header";

// docs/specs/design-system.md AC-6-11（検証手段は AC-10-6）。
//
// 「アプリ名」の具体的な文字列は本仕様の表に無く一意に定まらないため、
// 見出し要素の存在（構造）のみを検証し、文字列そのものは検証しない
// （AC-11-7）。

afterEach(() => {
  cleanup();
});

describe("AppHeader - AC-6-11", () => {
  it("<header> を出力し、アプリ名を見出し要素として含む", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    expect(within(header).getByRole("heading")).toBeTruthy();
  });

  it("right に渡した内容を差し込む", () => {
    render(<AppHeader right={<span>差込内容</span>} />);
    expect(screen.getByText("差込内容")).toBeTruthy();
  });
});
