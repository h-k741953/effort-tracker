// 検証済みセッションの保持（docs/specs/bff-auth-termination.md AC-5-8・Q-C = (a)）。
//
// - AC-5-8 (i): HttpOnly + Secure + SameSite の 3 属性をいずれも欠かさない。
//   SameSite は**サインインの戻り（AC-11-2 (ii)）が成立する範囲で最も狭いもの**
//   として Lax を選ぶ —— Strict では Cognito からの遷移で Cookie が送られず、
//   コールバックの照合（state / PKCE）が成立しない。
// - AC-5-8 (ii)(iii): 外部のセッションストアを作らない。サーバー側のメモリにも
//   セッションを持たない。保持先は Cookie だけである。
// - AC-4-3: ブラウザのスクリプトから読める場所へ複製しない（HttpOnly を外さない）。
// - AC-5-9: 保持するトークンの種類は ID トークン 1 つに絞る（Cookie のサイズ
//   上限に触れうることは方式の代償。実際に収まるかは apply 後にしか分からない
//   ＝限界 10-12）。
import type { AuthConfig } from "./auth-config";
import type { PublicKeyResolver, VerifyTokenResult } from "./jwt-verifier";
import { verifyToken } from "./jwt-verifier";
import { ROLE_COOKIE_NAME } from "./role-cookie";

/** 検証済みセッション（ID トークン）を保持する Cookie。 */
export const SESSION_COOKIE_NAME = "et_session";
/** コールバックの照合に使う一時 Cookie（AC-11-2 (ii)）。 */
export const OAUTH_STATE_COOKIE_NAME = "et_oauth_state";
export const OAUTH_CODE_VERIFIER_COOKIE_NAME = "et_oauth_code_verifier";

export interface CookieOptions {
  readonly maxAgeSeconds: number;
}

/**
 * Set-Cookie の値を組み立てる。**3 属性（HttpOnly / Secure / SameSite）を
 * 引数で外せる形にしない**（AC-5-8 (i)・AC-4-3）。
 */
export function serializeCookie(
  name: string,
  value: string,
  options: CookieOptions,
): string {
  return [
    `${name}=${value}`,
    "Path=/",
    "HttpOnly",
    "Secure",
    "SameSite=Lax",
    `Max-Age=${options.maxAgeSeconds}`,
  ].join("; ");
}

/** Cookie を破棄する Set-Cookie（AC-3-4 (ii)・AC-5-8 (v)）。 */
export function expireCookie(name: string): string {
  return serializeCookie(name, "", { maxAgeSeconds: 0 });
}

/**
 * 検証に失敗したセッションと、それに紐づくロール切替の状態を同時に破棄する
 * （AC-3-4 (ii)）。無効な値を保持したまま次の要求へ持ち越さない。
 */
export function expiredAuthCookies(): readonly string[] {
  return [
    expireCookie(SESSION_COOKIE_NAME),
    expireCookie(ROLE_COOKIE_NAME),
    expireCookie(OAUTH_STATE_COOKIE_NAME),
    expireCookie(OAUTH_CODE_VERIFIER_COOKIE_NAME),
  ];
}

/** Cookie ヘッダを名前→値へ分解する。解釈できない断片は無視する。 */
export function parseCookieHeader(
  cookieHeader: string | null,
): ReadonlyMap<string, string> {
  const cookies = new Map<string, string>();
  if (typeof cookieHeader !== "string" || cookieHeader.length === 0) {
    return cookies;
  }

  for (const fragment of cookieHeader.split(";")) {
    const separatorIndex = fragment.indexOf("=");
    if (separatorIndex <= 0) {
      continue;
    }
    const name = fragment.slice(0, separatorIndex).trim();
    const value = fragment.slice(separatorIndex + 1).trim();
    if (name.length > 0) {
      cookies.set(name, value);
    }
  }
  return cookies;
}

/**
 * セッション Cookie のトークンを検証して現在の利用者を確定させる
 * （AC-5-1・AC-5-2）。**クライアントが送ってきた識別子を採用しない** ——
 * 出所は検証済みトークンの sub だけである。
 *
 * Cookie が無い要求は**ゲスト（未認証）**として扱い、拒否の結果を返す
 * （AC-3-1。例外にしない）。
 */
export async function resolveVerifiedSession(params: {
  cookieHeader: string | null;
  now: Date;
  config: AuthConfig;
  publicKeyResolver: PublicKeyResolver;
}): Promise<VerifyTokenResult> {
  const { cookieHeader, now, config, publicKeyResolver } = params;

  const token = parseCookieHeader(cookieHeader).get(SESSION_COOKIE_NAME);
  if (token === undefined || token.length === 0) {
    return { ok: false };
  }

  return verifyToken({
    token,
    now,
    issuer: config.issuer,
    clientId: config.clientId,
    publicKeyResolver,
  });
}
