import { describe, expect, it } from "vitest";
import { requestRoleSwitch } from "./role-switch-client";

// docs/specs/login-ui.md AC-6-9-b・7-2。設計 L-4。
// 送る要求は POST /api/auth/role・本文なし（ロールを申告しない）。
// 応答の対応: 204 → switched、401 → signed-out、それ以外の応答・例外 → failed。
// fetch は手書きの Fake（呼び出しを記録する）。

function fakeFetch(respond: () => Response | Promise<Response>) {
  const calls: Array<{ input: unknown; init: RequestInit | undefined }> = [];
  const fetchImpl = (async (input: unknown, init?: RequestInit) => {
    calls.push({ input, init });
    return respond();
  }) as unknown as typeof fetch;
  return { fetchImpl, calls };
}

describe("送る要求", () => {
  it("POST /api/auth/role をちょうど1回、本文なしで送る", async () => {
    const { fetchImpl, calls } = fakeFetch(() => new Response(null, { status: 204 }));
    await requestRoleSwitch(fetchImpl);
    expect(calls).toHaveLength(1);
    expect(String(calls[0].input)).toBe("/api/auth/role");
    expect(calls[0].init?.method).toBe("POST");
    expect(calls[0].init?.body ?? null).toBeNull();
  });
});

describe("結果の分類", () => {
  it.each([
    [204, "switched"],
    [401, "signed-out"],
    [429, "failed"],
    [500, "failed"],
    [502, "failed"],
    [403, "failed"],
    [200, "failed"],
  ] as const)("%i → %s", async (status, expected) => {
    const { fetchImpl } = fakeFetch(() => new Response(null, { status }));
    expect(await requestRoleSwitch(fetchImpl)).toBe(expected);
  });

  it("fetch が例外を投げる → failed（例外を外へ出さない）", async () => {
    const { fetchImpl } = fakeFetch(() => {
      throw new TypeError("network down");
    });
    expect(await requestRoleSwitch(fetchImpl)).toBe("failed");
  });

  it("fetch が reject する → failed", async () => {
    const { fetchImpl } = fakeFetch(() => Promise.reject(new Error("aborted")));
    expect(await requestRoleSwitch(fetchImpl)).toBe("failed");
  });
});
