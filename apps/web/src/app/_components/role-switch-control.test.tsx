/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { RoleSwitchControl } from "./role-switch-control";

// docs/specs/login-ui.md AC-6-1・6-2・6-6・7-2・9-5。設計 L-4。
// next/navigation は差し替える（refresh / push / replace の呼び出しを数える）。
// fetch は手書きの Fake をグローバルへ置く（通信しない）。

const router = vi.hoisted(() => ({
  refresh: vi.fn(),
  push: vi.fn(),
  replace: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => router,
}));

interface FetchCall {
  input: unknown;
  init: RequestInit | undefined;
}
let fetchCalls: FetchCall[];

function installFetch(respond: () => Response | Promise<Response>) {
  fetchCalls = [];
  vi.stubGlobal("fetch", async (input: unknown, init?: RequestInit) => {
    fetchCalls.push({ input, init });
    return respond();
  });
}

beforeEach(() => {
  router.refresh.mockReset();
  router.push.mockReset();
  router.replace.mockReset();
  document.documentElement.removeAttribute("data-theme");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  document.documentElement.removeAttribute("data-theme");
  document.cookie = "et_theme=; Max-Age=0; Path=/";
});

const FAILURE_TEXT = "ロールを切り替えられませんでした。";

describe("表示（6-1・L-4: ロールはサーバーが持ち、props で受ける）", () => {
  it.each([
    ["Engineer", "技術者", "承認者"],
    ["Approver", "承認者", "技術者"],
  ] as const)("role=%s のとき %s が選択済み", (role, checked, unchecked) => {
    installFetch(() => new Response(null, { status: 204 }));
    render(<RoleSwitchControl role={role} />);
    expect(screen.getByRole("radiogroup", { name: "ロール切替" })).toBeTruthy();
    expect(screen.getByRole("radio", { name: checked, checked: true })).toBeTruthy();
    expect(screen.getByRole("radio", { name: unchecked, checked: false })).toBeTruthy();
  });

  it("初期状態ではエラーを出さない", () => {
    installFetch(() => new Response(null, { status: 204 }));
    render(<RoleSwitchControl role="Engineer" />);
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("成功（6-2・6-6）", () => {
  it("204: POST を1回送り、router.refresh がちょうど1回・push/replace は0回", async () => {
    installFetch(() => new Response(null, { status: 204 }));
    render(<RoleSwitchControl role="Engineer" />);
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    expect(fetchCalls).toHaveLength(1);
    expect(String(fetchCalls[0].input)).toBe("/api/auth/role");
    expect(fetchCalls[0].init?.method).toBe("POST");
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("成功しても、表示のロールは先回りして書き換えない（props のまま。サーバーの描き直しが決める）", async () => {
    installFetch(() => new Response(null, { status: 204 }));
    render(<RoleSwitchControl role="Engineer" />);
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("radio", { name: "技術者", checked: true })).toBeTruthy();
  });

  it("401（signed-out）: router.refresh がちょうど1回・push/replace は0回・エラーを出さない", async () => {
    installFetch(() => new Response(null, { status: 401 }));
    render(<RoleSwitchControl role="Engineer" />);
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("失敗（7-2）", () => {
  it.each([
    ["429", () => new Response(null, { status: 429 })],
    ["500", () => new Response(null, { status: 500 })],
    [
      "例外",
      () => {
        throw new TypeError("network down");
      },
    ],
  ] as const)("%s: role=alert が理由文を持って出て、refresh 0回・push/replace 0回・ロール表示は変わらない", async (_l, respond) => {
    installFetch(respond);
    render(<RoleSwitchControl role="Engineer" />);
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(FAILURE_TEXT);
    expect(router.refresh).not.toHaveBeenCalled();
    expect(router.push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
    expect(screen.getByRole("radio", { name: "技術者", checked: true })).toBeTruthy();
  });
});

describe("通信中の再入（L-4: 反転なので二重送信しない）", () => {
  it("通信が終わるまでの2回目の操作は無視され、送信は1回・refresh は1回", async () => {
    let release: (response: Response) => void = () => {};
    installFetch(
      () =>
        new Promise<Response>((resolve) => {
          release = resolve;
        }),
    );
    render(<RoleSwitchControl role="Engineer" />);
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    await waitFor(() => expect(fetchCalls).toHaveLength(1));
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    fireEvent.click(screen.getByRole("radio", { name: "技術者" }));
    expect(fetchCalls).toHaveLength(1);
    release(new Response(null, { status: 204 }));
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    expect(fetchCalls).toHaveLength(1);
  });
});

describe("9-5: テーマに触れない", () => {
  it("成功・失敗のどちらでも et_theme と data-theme を変えない", async () => {
    document.cookie = "et_theme=dark; Path=/";
    document.documentElement.setAttribute("data-theme", "dark");
    expect(document.cookie).toContain("et_theme=dark");

    installFetch(() => new Response(null, { status: 204 }));
    const first = render(<RoleSwitchControl role="Engineer" />);
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    expect(document.cookie).toContain("et_theme=dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    first.unmount();

    installFetch(() => new Response(null, { status: 500 }));
    render(<RoleSwitchControl role="Engineer" />);
    fireEvent.click(screen.getByRole("radio", { name: "承認者" }));
    await screen.findByRole("alert");
    expect(document.cookie).toContain("et_theme=dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });
});
