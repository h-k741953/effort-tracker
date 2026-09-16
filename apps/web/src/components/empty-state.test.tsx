/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { EmptyState } from "./empty-state";

// docs/specs/design-system.md AC-6-8（検証手段は AC-10-6）。

afterEach(() => {
  cleanup();
});

describe("EmptyState - AC-6-8", () => {
  it("title を表示する", () => {
    render(<EmptyState title="データがありません" />);
    expect(screen.getByText("データがありません")).toBeTruthy();
  });

  it("description と action を渡すとそれぞれ表示する", () => {
    render(
      <EmptyState
        title="データがありません"
        description="最初の実績を入力してください"
        action={<button type="button">入力する</button>}
      />,
    );
    expect(screen.getByText("最初の実績を入力してください")).toBeTruthy();
    expect(screen.getByRole("button", { name: "入力する" })).toBeTruthy();
  });
});
