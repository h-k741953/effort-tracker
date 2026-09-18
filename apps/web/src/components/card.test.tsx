/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { Card } from "./card";

// docs/specs/design-system.md AC-6-4（検証手段は AC-10-6）。

afterEach(() => {
  cleanup();
});

describe("Card - AC-6-4", () => {
  it("children を表示する", () => {
    render(<Card>本文</Card>);
    expect(screen.getByText("本文")).toBeTruthy();
  });

  it("title があるときは見出し要素として出す", () => {
    render(<Card title="見出し">本文</Card>);
    expect(screen.getByRole("heading", { name: "見出し" })).toBeTruthy();
  });

  it("title が無いときは見出し要素を出さない", () => {
    render(<Card>本文のみ</Card>);
    expect(screen.queryByRole("heading")).toBeNull();
  });
});
