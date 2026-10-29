/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ThemeControl } from "./theme-control";
import { serializeThemeCookie } from "@/lib/theme-cookie";

// docs/specs/login-ui.md AC-9-4、design-system.md AC-12-3。設計 L-5。
// jsdom の既定の http の URL は Secure 付き Cookie を保存しないため、document.cookie の
// 書き込みを手書きの差し替えで観測する（保存の文字列そのものを読む）。

let cookieWrites: string[];
let fetchCalls: number;

beforeEach(() => {
  cookieWrites = [];
  fetchCalls = 0;
  Object.defineProperty(document, "cookie", {
    configurable: true,
    get: () => "",
    set: (value: string) => {
      cookieWrites.push(value);
    },
  });
  vi.stubGlobal("fetch", async () => {
    fetchCalls += 1;
    return new Response(null, { status: 204 });
  });
  document.documentElement.removeAttribute("data-theme");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  delete (document as unknown as { cookie?: string }).cookie;
  document.documentElement.removeAttribute("data-theme");
});

describe("ThemeControl", () => {
  it.each([
    ["light", "ライト", "ダーク"],
    ["dark", "ダーク", "ライト"],
  ] as const)("初期値 %s: %s が選択済み", (initialTheme, checked, unchecked) => {
    render(<ThemeControl initialTheme={initialTheme} />);
    expect(screen.getByRole("radio", { name: checked, checked: true })).toBeTruthy();
    expect(screen.getByRole("radio", { name: unchecked, checked: false })).toBeTruthy();
  });

  it("12-3: ダークを選ぶと data-theme が dark になり、保存の文字列が serializeThemeCookie('dark') で、表示も更新される", () => {
    render(<ThemeControl initialTheme="light" />);
    fireEvent.click(screen.getByRole("radio", { name: "ダーク" }));
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(cookieWrites).toContain(serializeThemeCookie("dark"));
    expect(screen.getByRole("radio", { name: "ダーク", checked: true })).toBeTruthy();
  });

  it("12-3: ライトへ戻すと data-theme が light になり、保存の文字列が serializeThemeCookie('light')", () => {
    render(<ThemeControl initialTheme="dark" />);
    fireEvent.click(screen.getByRole("radio", { name: "ライト" }));
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(cookieWrites).toContain(serializeThemeCookie("light"));
    expect(screen.getByRole("radio", { name: "ライト", checked: true })).toBeTruthy();
  });

  it("12-3: サーバーへ要求を送らない（再読み込みもしない）", () => {
    render(<ThemeControl initialTheme="light" />);
    fireEvent.click(screen.getByRole("radio", { name: "ダーク" }));
    expect(fetchCalls).toBe(0);
  });

  it("9-4: 書き込む Cookie はテーマの Cookie だけ（認証の Cookie に触れない）", () => {
    render(<ThemeControl initialTheme="light" />);
    fireEvent.click(screen.getByRole("radio", { name: "ダーク" }));
    expect(cookieWrites.length).toBeGreaterThan(0);
    for (const write of cookieWrites) {
      expect(write.startsWith("et_theme=")).toBe(true);
    }
  });
});
