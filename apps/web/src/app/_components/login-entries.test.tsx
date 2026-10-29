/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { LoginEntries } from "./login-entries";

// docs/specs/login-ui.md AC-1-1・1-2・2-3・3-1・4-1・4-3。設計 L-2。
// (b)(c) は同じ href を持つため、入口はアクセシブル名で引いて区別する。

afterEach(() => {
  cleanup();
});

const ENTRIES = [
  ["ゲストとして続行（閲覧のみ）", "/demo"],
  ["メールアドレスでログイン", "/api/auth/sign-in"],
  ["Google でログイン", "/api/auth/sign-in"],
] as const;

describe("LoginEntries", () => {
  it("1-1・1-2: 入口（リンク）はちょうど3つ", () => {
    render(<LoginEntries />);
    expect(screen.getAllByRole("link")).toHaveLength(3);
  });

  it.each(ENTRIES)("1-1: 「%s」がちょうど1つあり、行き先は %s", (name, href) => {
    render(<LoginEntries />);
    const links = screen.getAllByRole("link", { name });
    expect(links).toHaveLength(1);
    expect(links[0].getAttribute("href")).toBe(href);
  });

  it("4-1: ゲストの入口は Route Handler（/api/）を経由しない", () => {
    render(<LoginEntries />);
    const guest = screen.getByRole("link", { name: "ゲストとして続行（閲覧のみ）" });
    expect(guest.getAttribute("href")?.startsWith("/api/")).toBe(false);
  });

  it("4-3: ゲストの注記「入力・締め・承認はできません。」が出る", () => {
    const { container } = render(<LoginEntries />);
    expect(container.textContent).toContain("入力・締め・承認はできません。");
  });

  it("1-2: SMS による入口・案内を出さない", () => {
    const { container } = render(<LoginEntries />);
    expect(container.textContent).not.toMatch(/SMS/i);
  });

  it("L-2（Q-1）: 「新規登録」を文言に付けない", () => {
    const { container } = render(<LoginEntries />);
    expect(container.textContent).not.toContain("新規登録");
  });

  it("2-3: フォーム・入力欄（パスワードを含む）を持たない", () => {
    const { container } = render(<LoginEntries />);
    expect(container.querySelector("form")).toBeNull();
    expect(container.querySelector("input")).toBeNull();
    expect(container.querySelector('input[type="password"]')).toBeNull();
  });
});
