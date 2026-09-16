/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, within } from "@testing-library/react";
import { ErrorBanner } from "./error-banner";

// docs/specs/design-system.md AC-6-7（検証手段は AC-10-6）。
//
// 固定文字（6-7-i）は role="alert" の内側にテキストが「エラー」と完全に
// 一致する要素として出ること。children に依存せず常に出ることを確認する
// ため、「エラー」を含まない理由文で描画する（AC-10-6-d）。

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

  it("6-7-i: role='alert' の内側に『エラー』と完全一致する要素が children に依存せず出る（onRetry を与えない描画でも満たす）", () => {
    // 「エラー」を含まない理由文を渡し、固定文字が children とは別の層で
    // 出ることを検証する（10-6-d）。
    render(<ErrorBanner>ネットワークに接続できませんでした</ErrorBanner>);
    const alert = screen.getByRole("alert");
    expect(within(alert).getByText("エラー")).toBeTruthy();
  });
});
