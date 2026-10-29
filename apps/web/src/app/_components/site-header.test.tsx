/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { SiteHeader } from "./site-header";
import type { Viewer } from "@/lib/viewer";

// docs/specs/login-ui.md AC-9-1。設計 L-1（props: viewer・theme・screen）。
// ロール切替（クライアント部品）が useRouter を使うため、next/navigation を差し替える。

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}));

afterEach(() => {
  cleanup();
});

const CASES: ReadonlyArray<[string, Viewer, "login" | "app"]> = [
  ["ゲスト × login", { kind: "guest" }, "login"],
  ["ゲスト × app", { kind: "guest" }, "app"],
  ["技術者 × login", { kind: "signed-in", role: "Engineer" }, "login"],
  ["技術者 × app", { kind: "signed-in", role: "Engineer" }, "app"],
  ["承認者 × app", { kind: "signed-in", role: "Approver" }, "app"],
];

describe("SiteHeader（9-1: ログイン画面を含む全画面にヘッダーを置く）", () => {
  it.each(CASES)("%s: <header>（banner）がちょうど1つ出て、アプリ名の見出しを持つ", (_l, viewer, screenKind) => {
    render(<SiteHeader viewer={viewer} theme="light" screen={screenKind} />);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("heading", { name: "effort-tracker" })).toBeTruthy();
  });
});
