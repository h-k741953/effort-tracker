import { describe, expect, it } from "vitest";
import { ROLE_COOKIE_NAME } from "./role-cookie";
import { SESSION_COOKIE_NAME, expiredAuthCookies } from "./session-cookie";
import { THEME_COOKIE_NAME } from "./theme-cookie";

// docs/specs/login-ui.md AC-9-5（design-system.md AC-12-3）。
// 認証の Cookie を破棄しても、利用者が選んだテーマの Cookie には触れない。
// 注意: 実装が既に満たしているため、この検査は書いた時点で緑である（Red を踏めない）。
// 陽性対照として、破棄の対象（セッション・ロール）が含まれることも同時に読む。

describe("expiredAuthCookies（AC-9-5）", () => {
  const cookies = expiredAuthCookies();

  it("セッションとロールの Cookie の破棄を含む（陽性対照）", () => {
    expect(cookies.some((c) => c.startsWith(`${SESSION_COOKIE_NAME}=;`))).toBe(true);
    expect(cookies.some((c) => c.startsWith(`${ROLE_COOKIE_NAME}=;`))).toBe(true);
  });

  it("テーマの Cookie（et_theme）を含まない", () => {
    expect(THEME_COOKIE_NAME).toBe("et_theme");
    expect(cookies.filter((c) => c.startsWith(`${THEME_COOKIE_NAME}=`))).toEqual([]);
    expect(cookies.filter((c) => c.includes(THEME_COOKIE_NAME))).toEqual([]);
  });
});
