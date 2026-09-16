/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { ErrorBanner } from "./error-banner";

// docs/specs/design-system.md AC-6-7（検証手段は AC-10-6）。

afterEach(() => {
  cleanup();
});

describe("ErrorBanner - AC-6-7", () => {
  it("role='alert' で children を出力する", () => {
    render(<ErrorBanner>エラーが発生しました</ErrorBanner>);
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("エラーが発生しました");
  });

  it("onRetry があるとき再試行の Button を出す", () => {
    render(<ErrorBanner onRetry={vi.fn()}>失敗しました</ErrorBanner>);
    expect(screen.getByRole("button")).toBeTruthy();
  });

  it("onRetry が無いとき Button を出さない", () => {
    render(<ErrorBanner>失敗しました</ErrorBanner>);
    expect(screen.queryByRole("button")).toBeNull();
  });
});
