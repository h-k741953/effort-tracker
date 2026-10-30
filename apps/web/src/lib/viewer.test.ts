import { describe, expect, it } from "vitest";
import { resolveViewer } from "./viewer";
import { ROLE_COOKIE_NAME } from "./role-cookie";
import { SESSION_COOKIE_NAME } from "./session-cookie";
import {
  ENVIRONMENT,
  NOW,
  buildIdToken,
  buildIdTokenWithBadSignature,
  environmentWithEmpty,
  environmentWithout,
  fakePublicKeySource,
  signedRoleCookieValue,
  tamperedRoleCookieValue,
} from "../test-support/bff-wiring";

// docs/specs/login-ui.md AC-5-2・6-5、bff-auth-termination.md AC-5-11 (i)(ii)(v)。
// 設計: web-screens-implementation-design.md C-3（判定の順は work-month-handlers の順2〜順4）。
// 公開鍵の解決は既存のテストと同じく手書きの Fake に差し替える（外部へ通信しない）。

function cookieHeader(parts: { session?: string; role?: string }): string | null {
  const cookies: string[] = [];
  if (parts.session !== undefined) cookies.push(`${SESSION_COOKIE_NAME}=${parts.session}`);
  if (parts.role !== undefined) cookies.push(`${ROLE_COOKIE_NAME}=${parts.role}`);
  return cookies.length === 0 ? null : cookies.join("; ");
}

function resolve(
  header: string | null,
  environment: Record<string, string | undefined> = ENVIRONMENT,
) {
  return resolveViewer({
    cookieHeader: header,
    now: NOW,
    environment,
    publicKeyResolver: fakePublicKeySource,
  });
}

describe("ゲスト（5-2・6-5・5-11 (v)）", () => {
  it("Cookie なし → guest（ロールを持たない）", async () => {
    const result = await resolve(null);
    expect(result).toEqual({ kind: "guest" });
    expect("role" in result).toBe(false);
  });

  it("セッションなしで正当なロール Cookie だけがある → guest のまま（ロールを与えない）", async () => {
    const result = await resolve(cookieHeader({ role: signedRoleCookieValue("Approver") }));
    expect(result).toEqual({ kind: "guest" });
  });

  it("セッションの検証に失敗する（別の鍵の署名）→ guest（Q-4 (i) = (a)）。正当なロール Cookie があっても", async () => {
    const result = await resolve(
      cookieHeader({
        session: buildIdTokenWithBadSignature(),
        role: signedRoleCookieValue("Approver"),
      }),
    );
    expect(result).toEqual({ kind: "guest" });
  });

  it("セッション Cookie の値が壊れている → guest", async () => {
    const result = await resolve(cookieHeader({ session: "not-a-jwt" }));
    expect(result).toEqual({ kind: "guest" });
  });
});

describe("検証済みセッション（5-2・5-11 (i)(ii)）", () => {
  it("ロール Cookie なし → Engineer（反転元の既定）", async () => {
    expect(await resolve(cookieHeader({ session: buildIdToken() }))).toEqual({
      kind: "signed-in",
      role: "Engineer",
    });
  });

  it("正当に署名された Approver → Approver（既定で正当な Cookie を潰さない）", async () => {
    expect(
      await resolve(
        cookieHeader({ session: buildIdToken(), role: signedRoleCookieValue("Approver") }),
      ),
    ).toEqual({ kind: "signed-in", role: "Approver" });
  });

  it("正当に署名された Engineer → Engineer", async () => {
    expect(
      await resolve(
        cookieHeader({ session: buildIdToken(), role: signedRoleCookieValue("Engineer") }),
      ),
    ).toEqual({ kind: "signed-in", role: "Engineer" });
  });

  it("Approver を主張する改竄されたロール Cookie → Engineer（昇格の根拠にしない。5-11 (ii)）", async () => {
    expect(
      await resolve(
        cookieHeader({ session: buildIdToken(), role: tamperedRoleCookieValue("Approver") }),
      ),
    ).toEqual({ kind: "signed-in", role: "Engineer" });
  });

  it("署名の欠けた（ロールだけの）Cookie → Engineer", async () => {
    expect(
      await resolve(cookieHeader({ session: buildIdToken(), role: "Approver" })),
    ).toEqual({ kind: "signed-in", role: "Engineer" });
  });
});

describe("構成の欠落 → misconfigured（C-3。guest にならない。Q-4 (ii) = (a)）", () => {
  it.each([
    ["COGNITO_USER_POOL_ID が無い", environmentWithout("COGNITO_USER_POOL_ID")],
    ["COGNITO_CLIENT_ID が空", environmentWithEmpty("COGNITO_CLIENT_ID")],
    ["COGNITO_REGION が無い", environmentWithout("COGNITO_REGION")],
  ])("認証の構成: %s（Cookie なしでも）", async (_label, environment) => {
    expect(await resolve(null, environment)).toEqual({ kind: "misconfigured" });
  });

  it("認証の構成が欠けていれば、検証済みセッションがあっても misconfigured", async () => {
    expect(
      await resolve(
        cookieHeader({ session: buildIdToken() }),
        environmentWithout("COGNITO_CLIENT_ID"),
      ),
    ).toEqual({ kind: "misconfigured" });
  });

  it.each([
    ["ROLE_COOKIE_SIGNING_KEY が無い", environmentWithout("ROLE_COOKIE_SIGNING_KEY")],
    ["ROLE_COOKIE_SIGNING_KEY が空", environmentWithEmpty("ROLE_COOKIE_SIGNING_KEY")],
  ])("署名鍵欠落（%s）で検証済みセッションあり → misconfigured（Engineer へ落ちない）", async (_label, environment) => {
    expect(await resolve(cookieHeader({ session: buildIdToken() }), environment)).toEqual({
      kind: "misconfigured",
    });
  });

  it("署名鍵が欠けていても、セッションが成立しなければ guest（順2 が順3 より先）", async () => {
    expect(
      await resolve(null, environmentWithout("ROLE_COOKIE_SIGNING_KEY")),
    ).toEqual({ kind: "guest" });
  });
});
