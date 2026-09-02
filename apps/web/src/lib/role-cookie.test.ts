import { describe, expect, it } from "vitest";
import { createHmac } from "node:crypto";
// AC-8-2（docs/specs/bff-auth-termination.md）: 実装より先にテストを書き、
// Red を確認してから実装へ入る。本ファイルが対象とするモジュール
// （./role-cookie）はまだ存在しない。これは意図した Red である。
//
// ファイル名・関数名・型名・Cookie の内部形式はこのテストが暫定的に固定する
// インターフェースである（AC-1-3 と同型）。実装側は同じ名前・同じ形式で
// 作ってよいし、都合が悪ければテストごと見直す。
//
// AC-5-5（BFF が発行する署名済み Cookie。Q-B = (a)）と、その署名鍵（(viii)・
// Q-F = (b)）を対象にする。署名鍵の実値はテストでも明らかなダミーに限る
// （AC-4-4・AC-8-4）。
import type { Role } from "./role-cookie";
import { signRoleCookie, verifyRoleCookie } from "./role-cookie";

const SIGNING_KEY = "dummy-role-cookie-signing-key-0123456789";
const OTHER_SIGNING_KEY = "dummy-other-signing-key-0000000000";

// role-cookie の Cookie 値は `${role}.${base64url(HMAC-SHA256(signingKey, role))}`
// という形を、このテストが暫定的なワイヤーフォーマットとして固定する
// （AC-5-5 の「BFF が署名した Cookie を BFF 自身が検証して得た値だけから
// 決める」を検査するには、正当な署名だが許可外のロール値を持つ Cookie を
// 組み立てて渡す必要があるため）。
function buildCookieValue(role: string, key: string): string {
  const signature = createHmac("sha256", key).update(role).digest("base64url");
  return `${role}.${signature}`;
}

// --- AC-5-5: 受理（正当なロール・正当な署名） -------------------------------

describe("signRoleCookie / verifyRoleCookie - 受理（AC-5-5 (i)(iii)）", () => {
  it.each<{ role: Role }>([{ role: "Engineer" }, { role: "Approver" }])(
    "$role を署名し、同じ鍵で検証すると受理される",
    ({ role }) => {
      const signed = signRoleCookie({ role, signingKey: SIGNING_KEY });
      expect(signed.ok).toBe(true);
      if (!signed.ok) return;

      const verified = verifyRoleCookie({
        cookieValue: signed.value,
        signingKey: SIGNING_KEY,
      });
      expect(verified).toEqual({ ok: true, role });
    },
  );
});

// --- AC-5-5 / AC-8-5: 拒否側（受理側と同じだけ検査する） --------------------

describe("verifyRoleCookie - 拒否（AC-5-5 (i)(iii)(viii)・AC-8-5）", () => {
  it("署名が検証できない Cookie（改竄された署名）を拒否する", () => {
    const signed = signRoleCookie({
      role: "Engineer",
      signingKey: SIGNING_KEY,
    });
    expect(signed.ok).toBe(true);
    if (!signed.ok) return;

    const tampered = `${signed.value}x`;
    const verified = verifyRoleCookie({
      cookieValue: tampered,
      signingKey: SIGNING_KEY,
    });
    expect(verified).toEqual({ ok: false });
  });

  it("署名の欠けた Cookie（ロールのみ）を拒否する", () => {
    const verified = verifyRoleCookie({
      cookieValue: "Engineer",
      signingKey: SIGNING_KEY,
    });
    expect(verified).toEqual({ ok: false });
  });

  it("別の鍵で署名された Cookie を拒否する", () => {
    const signedWithOtherKey = buildCookieValue("Engineer", OTHER_SIGNING_KEY);
    const verified = verifyRoleCookie({
      cookieValue: signedWithOtherKey,
      signingKey: SIGNING_KEY,
    });
    expect(verified).toEqual({ ok: false });
  });

  it("Engineer / Approver 以外の値（署名は正当）を拒否する", () => {
    const signedForbiddenRole = buildCookieValue("Admin", SIGNING_KEY);
    const verified = verifyRoleCookie({
      cookieValue: signedForbiddenRole,
      signingKey: SIGNING_KEY,
    });
    expect(verified).toEqual({ ok: false });
  });

  it("鍵が未設定（空文字）のときは検証を成立させない", () => {
    const signed = signRoleCookie({
      role: "Engineer",
      signingKey: SIGNING_KEY,
    });
    expect(signed.ok).toBe(true);
    if (!signed.ok) return;

    const verified = verifyRoleCookie({
      cookieValue: signed.value,
      signingKey: "",
    });
    expect(verified).toEqual({ ok: false });
  });
});

describe("signRoleCookie - 拒否（AC-5-5 (iii)(viii)・AC-7-5）", () => {
  it("Engineer / Approver 以外のロール値は署名を成立させない", () => {
    const signed = signRoleCookie({
      role: "Admin" as Role,
      signingKey: SIGNING_KEY,
    });
    expect(signed.ok).toBe(false);
  });

  it("鍵が未設定（空文字）のときは署名を成立させない（AC-7-5・既定値へ黙って落ちない）", () => {
    const signed = signRoleCookie({ role: "Engineer", signingKey: "" });
    expect(signed.ok).toBe(false);
  });
});
