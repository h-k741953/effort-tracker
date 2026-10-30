/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { SiteHeader } from "./site-header";
import type { Viewer } from "@/lib/viewer";

// docs/specs/login-ui.md AC-6-1・6-5・9-2・9-3・9-4・9-6・9-7。設計 L-3 の表。
// HeaderControls は SiteHeader の右側に差し込まれる。HeaderControls の props と
// headerSlots の戻り値の形は設計が定めていないため、描画した DOM（SiteHeader 経由）の
// 並びで L-3 の表を読む（申告事項）。ログインへのリンクは名前「ログイン」・行き先 `/`。

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}));

afterEach(() => {
  cleanup();
});

type Slot = "role" | "theme" | "login-link";

/** ヘッダーの中で、出ているものを文書順に返す。 */
function slotsInOrder(): Slot[] {
  const found: Array<[Slot, Element]> = [];
  const role = screen.queryAllByRole("radiogroup", { name: "ロール切替" });
  const theme = screen.queryAllByRole("radiogroup", { name: "テーマ切替" });
  const login = screen.queryAllByRole("link", { name: "ログイン" });
  for (const el of role) found.push(["role", el]);
  for (const el of theme) found.push(["theme", el]);
  for (const el of login) found.push(["login-link", el]);
  found.sort((a, b) => {
    const pos = a[1].compareDocumentPosition(b[1]);
    return pos & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1;
  });
  return found.map(([slot]) => slot);
}

const GUEST: Viewer = { kind: "guest" };
const ENGINEER: Viewer = { kind: "signed-in", role: "Engineer" };
const APPROVER: Viewer = { kind: "signed-in", role: "Approver" };

const TABLE: ReadonlyArray<[string, Viewer, "login" | "app", Slot[]]> = [
  ["ゲスト × login", GUEST, "login", ["theme"]],
  ["ゲスト × app", GUEST, "app", ["theme", "login-link"]],
  ["技術者 × login", ENGINEER, "login", ["theme"]],
  ["技術者 × app", ENGINEER, "app", ["role", "theme"]],
  ["承認者 × login", APPROVER, "login", ["theme"]],
  ["承認者 × app", APPROVER, "app", ["role", "theme"]],
];

describe("L-3 の表（出すものと並び。9-7）", () => {
  it.each(TABLE)("%s → %j の順", (_label, viewer, screenKind, expected) => {
    render(<SiteHeader viewer={viewer} theme="light" screen={screenKind} />);
    expect(slotsInOrder()).toEqual(expected);
  });
});

describe("個別の AC", () => {
  it("6-5・9-2: ゲストにロール切替を出さない", () => {
    render(<SiteHeader viewer={GUEST} theme="light" screen="app" />);
    expect(screen.queryByRole("radiogroup", { name: "ロール切替" })).toBeNull();
  });

  it("9-2: ログイン画面にロール切替を出さない", () => {
    render(<SiteHeader viewer={ENGINEER} theme="light" screen="login" />);
    expect(screen.queryByRole("radiogroup", { name: "ロール切替" })).toBeNull();
  });

  it("9-3・9-4: テーマ切替は全区分・全画面に1つ出る（ゲスト・ログイン画面を含む）", () => {
    for (const [, viewer, screenKind] of TABLE) {
      const { unmount } = render(<SiteHeader viewer={viewer} theme="light" screen={screenKind} />);
      expect(screen.getAllByRole("radiogroup", { name: "テーマ切替" })).toHaveLength(1);
      unmount();
    }
  });

  it("6-1: ログイン中（app）のロール切替は技術者・承認者の2択で、現在のロールが選択済み", () => {
    render(<SiteHeader viewer={APPROVER} theme="light" screen="app" />);
    expect(screen.getByRole("radio", { name: "承認者", checked: true })).toBeTruthy();
    expect(screen.getByRole("radio", { name: "技術者", checked: false })).toBeTruthy();
  });

  it("9-6: ゲスト × app のログインへのリンクは名前「ログイン」・行き先 /", () => {
    render(<SiteHeader viewer={GUEST} theme="light" screen="app" />);
    const links = screen.getAllByRole("link", { name: "ログイン" });
    expect(links).toHaveLength(1);
    expect(links[0].getAttribute("href")).toBe("/");
  });

  it("9-6: ログイン中・ログイン画面にはログインへのリンクを出さない", () => {
    for (const [viewer, screenKind] of [
      [ENGINEER, "app"],
      [APPROVER, "app"],
      [GUEST, "login"],
    ] as const) {
      const { unmount } = render(<SiteHeader viewer={viewer} theme="light" screen={screenKind} />);
      expect(screen.queryByRole("link", { name: "ログイン" })).toBeNull();
      unmount();
    }
  });

  it("9-3: テーマ切替の選択は props の theme を反映する", () => {
    render(<SiteHeader viewer={GUEST} theme="dark" screen="app" />);
    expect(screen.getByRole("radio", { name: "ダーク", checked: true })).toBeTruthy();
  });
});
