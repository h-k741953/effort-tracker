// ホストされたサインイン画面への送り出しと、認可コードのトークン交換
// （docs/specs/bff-auth-termination.md AC-11-2 (i)(ii)）。
//
// - AC-1-1 / AC-1-2: 交換も検証も**サーバー側（BFF）でのみ**行う。ブラウザが
//   Cognito のトークンを直接扱う経路を作らない。
// - AC-4-2: トークンを URL（クエリ・パス）へ載せない。
// - AC-6-1: Cognito のトークンを AWS の認証へ用いない（Identity Pool を使わない）。
// - AC-11-13 / AC-7-1 (i): npm 依存を足さない。PKCE・state は Node 標準の
//   crypto で組み立てる。
import { createHash, randomBytes } from "node:crypto";
import type { AuthConfig } from "./auth-config";

/** サインインの戻り（コールバック）の経路。AC-11-3: パス文字列は実装側が持つ。 */
export const CALLBACK_PATH = "/api/auth/callback";

/**
 * 認可要求のスコープ。構成側（aws_cognito_user_pool_client.allowed_oauth_scopes）
 * と揃える。**一致は機械検査されない**（限界 10-10 と同じ性質）。
 */
const AUTHORIZE_SCOPES = "openid email";

export interface PkcePair {
  /** トークン交換時にだけサーバー側から送る（ブラウザへ渡さない）。 */
  readonly codeVerifier: string;
  readonly codeChallenge: string;
}

/** PKCE（S256）の組を作る。認可コードの横取りに対する備え。 */
export function createPkcePair(): PkcePair {
  const codeVerifier = randomBytes(32).toString("base64url");
  const codeChallenge = createHash("sha256")
    .update(codeVerifier)
    .digest("base64url");
  return { codeVerifier, codeChallenge };
}

/** CSRF 対策の state を作る（コールバックで Cookie の値と突き合わせる）。 */
export function createOAuthState(): string {
  return randomBytes(16).toString("base64url");
}

/**
 * ホストされたサインイン画面の URL を組み立てる（AC-11-2 (i)・Q-1 = (a)）。
 * **3 つの入口の提示そのものは画面の責務**であり、ここでは作らない（login-ui.md）。
 */
export function buildAuthorizeUrl(params: {
  config: AuthConfig;
  redirectUri: string;
  state: string;
  codeChallenge: string;
}): string {
  const { config, redirectUri, state, codeChallenge } = params;

  const url = new URL("/oauth2/authorize", config.hostedUiBaseUrl);
  url.searchParams.set("response_type", "code"); // 認可コードのみ（implicit を使わない）
  url.searchParams.set("client_id", config.clientId);
  url.searchParams.set("redirect_uri", redirectUri);
  url.searchParams.set("scope", AUTHORIZE_SCOPES);
  url.searchParams.set("state", state);
  url.searchParams.set("code_challenge", codeChallenge);
  url.searchParams.set("code_challenge_method", "S256");
  return url.toString();
}

/**
 * サインインの戻り先（redirect_uri）を組み立てる。
 *
 * **戻り先は Cognito 側の許可リスト（構成側の `cognito_callback_urls`）で
 * 検証される**ため、要求ヘッダから導いた出所が登録されていなければ Cognito が
 * サインインを拒否する。**この対応づけは機械検査されず、ずれは apply 後の
 * サインインで初めて現れる**（限界 10-10）。
 */
export function resolveRedirectUri(
  headers: Pick<Headers, "get">,
  requestUrl: string,
): string {
  const forwardedHost = headers.get("x-forwarded-host");
  const forwardedProto = headers.get("x-forwarded-proto");
  const base =
    typeof forwardedHost === "string" && forwardedHost.length > 0
      ? `${forwardedProto ?? "https"}://${forwardedHost}`
      : new URL(requestUrl).origin;
  return new URL(CALLBACK_PATH, base).toString();
}

export interface TokenExchanged {
  readonly ok: true;
  /** AC-5-9: 保持するのは ID トークンだけ。他のトークンは持ち回らない。 */
  readonly idToken: string;
}

export interface TokenExchangeFailed {
  readonly ok: false;
}

export type TokenExchangeResult = TokenExchanged | TokenExchangeFailed;

/**
 * 認可コードを ID トークンへ交換する（AC-11-2 (ii)）。**失敗は理由を持たない
 * 結果で表し**、応答・ログへ内部的な理由やトークンを出さない（AC-3-4 (iii)・
 * AC-4-1）。アプリクライアントはシークレットを持たないため（構成側の決定）、
 * PKCE の code_verifier で証明する。
 */
export async function exchangeAuthorizationCode(params: {
  config: AuthConfig;
  code: string;
  redirectUri: string;
  codeVerifier: string;
  fetchImpl?: typeof fetch;
}): Promise<TokenExchangeResult> {
  const { config, code, redirectUri, codeVerifier } = params;
  const fetchImpl = params.fetchImpl ?? fetch;

  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: config.clientId,
    code,
    redirect_uri: redirectUri,
    code_verifier: codeVerifier,
  });

  try {
    const response = await fetchImpl(`${config.hostedUiBaseUrl}/oauth2/token`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString(),
      cache: "no-store",
    });
    if (!response.ok) {
      return { ok: false };
    }

    const payload: unknown = await response.json();
    const idToken =
      typeof payload === "object" && payload !== null
        ? (payload as Record<string, unknown>)["id_token"]
        : undefined;
    if (typeof idToken !== "string" || idToken.length === 0) {
      return { ok: false };
    }
    return { ok: true, idToken };
  } catch {
    return { ok: false };
  }
}
