// docs/specs/web-screens-implementation-design.md C-3。
// 判定の順は work-month-handlers.ts の handle() の順2〜順4 と同じ。
// Cookie を書き換えない（ページの描画からは書けない。Q-4 (i) = (a)）。

import { loadAuthConfig, loadRoleCookieSigningKey, type Environment } from "./auth-config";
import { createCognitoPublicKeyResolver, type PublicKeyResolver } from "./jwt-verifier";
import { ROLE_COOKIE_NAME, resolveEffectiveRole, type Role } from "./role-cookie";
import { parseCookieHeader, resolveVerifiedSession } from "./session-cookie";

export type Viewer = { kind: "guest" } | { kind: "signed-in"; role: Role };
export type ResolveViewerResult = Viewer | { kind: "misconfigured" };

export async function resolveViewer(params: {
  cookieHeader: string | null;
  now: Date;
  environment?: Environment;
  publicKeyResolver?: PublicKeyResolver;
}): Promise<ResolveViewerResult> {
  const environment = params.environment ?? process.env;

  // (1) 認証の構成が欠けていれば misconfigured。
  const config = loadAuthConfig(environment);
  if (!config.ok) {
    return { kind: "misconfigured" };
  }

  // (2) セッションが成立しなければ guest。
  const session = await resolveVerifiedSession({
    cookieHeader: params.cookieHeader,
    now: params.now,
    config: config.value,
    publicKeyResolver: params.publicKeyResolver ?? createCognitoPublicKeyResolver(),
  });
  if (!session.ok) {
    return { kind: "guest" };
  }

  // (3) 成立したのに署名鍵が欠けていれば misconfigured（Engineer へ落とさない）。
  const signingKey = loadRoleCookieSigningKey(environment);
  if (!signingKey.ok) {
    return { kind: "misconfigured" };
  }

  // (4) ロールの確定。
  const roleCookie = parseCookieHeader(params.cookieHeader).get(ROLE_COOKIE_NAME);
  return {
    kind: "signed-in",
    role: resolveEffectiveRole({ cookieValue: roleCookie, signingKey: signingKey.value }),
  };
}
