// Route Handler の中身（docs/specs/bff-auth-termination.md AC-11-2）。
//
// - AC-1-4 / AC-11-11: **Route Handler の内側にだけ存在する形にしない。** 処理は
//   この lib 側に置き、`src/app/api/` の Route Handler は薄い配線に留める。
// - AC-11-12: 3 つの経路はいずれも入口でレート制限を通る（配線は Route Handler
//   側の withRequestRateLimit）。
// - AC-3-4 (iii) / AC-4-1 / AC-4-2: 応答の本文・ヘッダに、トークン・その一部・
//   失敗の内部的な理由を載せない。
// - AC-9: ここで作るのは AC-11-2 の 3 つだけ。業務エンドポイントを発明しない。
//   lambda-client（SigV4）はスタブも置かない。
import type { Environment } from "./auth-config";
import { loadAuthConfig, loadRoleCookieSigningKey } from "./auth-config";
import {
  buildAuthorizeUrl,
  createOAuthState,
  createPkcePair,
  exchangeAuthorizationCode,
  resolveRedirectUri,
} from "./cognito-oidc";
import type { PublicKeyResolver } from "./jwt-verifier";
import { createCognitoPublicKeyResolver, verifyToken } from "./jwt-verifier";
import type { Role } from "./role-cookie";
import { ROLE_COOKIE_NAME, signRoleCookie } from "./role-cookie";
import {
  OAUTH_CODE_VERIFIER_COOKIE_NAME,
  OAUTH_STATE_COOKIE_NAME,
  SESSION_COOKIE_NAME,
  expiredAuthCookies,
  expireCookie,
  parseCookieHeader,
  resolveVerifiedSession,
  serializeCookie,
} from "./session-cookie";

/** 認可要求の往復に使う一時 Cookie の寿命（秒）。 */
const OAUTH_HANDSHAKE_MAX_AGE_SECONDS = 600;

/**
 * セッション Cookie の寿命（秒）。構成側の id_token_validity（60 分）と揃える。
 * **一致は機械検査されない**（限界 10-2 と同じ性質）。
 */
const SESSION_MAX_AGE_SECONDS = 3600;

/** サインイン後の戻り先（画面は login-ui.md の担当。ここでは入口へ戻すだけ）。 */
const SIGN_IN_LANDING_PATH = "/";

function textResponse(status: number, body: string, headers?: Headers): Response {
  const responseHeaders = headers ?? new Headers();
  responseHeaders.set("Cache-Control", "no-store");
  return new Response(body, { status, headers: responseHeaders });
}

function redirectResponse(location: string, setCookies: readonly string[]): Response {
  const headers = new Headers();
  headers.set("Location", location);
  headers.set("Cache-Control", "no-store");
  for (const cookie of setCookies) {
    headers.append("Set-Cookie", cookie);
  }
  return new Response(null, { status: 302, headers });
}

/**
 * AC-3-4: 検証に失敗した要求への応答。**401 を返し**（(i)）、**検証に失敗した
 * セッション Cookie とロール Cookie を破棄する**（(ii)）。理由は載せない（(iii)）。
 */
function unauthenticatedResponse(): Response {
  const headers = new Headers();
  for (const cookie of expiredAuthCookies()) {
    headers.append("Set-Cookie", cookie);
  }
  return textResponse(401, "Unauthorized", headers);
}

/** 構成が欠けている状態を、既定値へ落ちずに失敗として表す（AC-7-5）。 */
function misconfiguredResponse(): Response {
  return textResponse(500, "Service Unavailable");
}

/**
 * (i) サインインの開始（AC-11-2 (i)）。ホストされたサインイン画面へ送り出す。
 */
export async function handleSignInStart(
  request: Request,
  options: { environment?: Environment } = {},
): Promise<Response> {
  const config = loadAuthConfig(options.environment);
  if (!config.ok) {
    return misconfiguredResponse();
  }

  const redirectUri = resolveRedirectUri(request.headers, request.url);
  const state = createOAuthState();
  const pkce = createPkcePair();

  return redirectResponse(
    buildAuthorizeUrl({
      config: config.value,
      redirectUri,
      state,
      codeChallenge: pkce.codeChallenge,
    }),
    [
      serializeCookie(OAUTH_STATE_COOKIE_NAME, state, {
        maxAgeSeconds: OAUTH_HANDSHAKE_MAX_AGE_SECONDS,
      }),
      serializeCookie(OAUTH_CODE_VERIFIER_COOKIE_NAME, pkce.codeVerifier, {
        maxAgeSeconds: OAUTH_HANDSHAKE_MAX_AGE_SECONDS,
      }),
    ],
  );
}

/**
 * (ii) サインインの戻り（AC-11-2 (ii)）。認可コードを**サーバー側で**トークンへ
 * 交換し、AC-2 の検証を通してから検証済みセッションの Cookie を確立する。
 * 検証に失敗したら AC-3-4 に従う（401・Cookie の破棄・理由を載せない）。
 */
export async function handleSignInCallback(
  request: Request,
  options: {
    now: Date;
    environment?: Environment;
    publicKeyResolver?: PublicKeyResolver;
  },
): Promise<Response> {
  const config = loadAuthConfig(options.environment);
  if (!config.ok) {
    return misconfiguredResponse();
  }

  const requestUrl = new URL(request.url);
  const code = requestUrl.searchParams.get("code");
  const state = requestUrl.searchParams.get("state");
  const cookies = parseCookieHeader(request.headers.get("cookie"));
  const expectedState = cookies.get(OAUTH_STATE_COOKIE_NAME);
  const codeVerifier = cookies.get(OAUTH_CODE_VERIFIER_COOKIE_NAME);

  if (
    typeof code !== "string" ||
    code.length === 0 ||
    typeof state !== "string" ||
    expectedState === undefined ||
    state !== expectedState ||
    codeVerifier === undefined
  ) {
    return unauthenticatedResponse();
  }

  const exchanged = await exchangeAuthorizationCode({
    config: config.value,
    code,
    redirectUri: resolveRedirectUri(request.headers, request.url),
    codeVerifier,
  });
  if (!exchanged.ok) {
    return unauthenticatedResponse();
  }

  // AC-1-1: 検証は BFF でのみ行う。ブラウザ側の申告を採用しない。
  const verified = await verifyToken({
    token: exchanged.idToken,
    now: options.now,
    issuer: config.value.issuer,
    clientId: config.value.clientId,
    publicKeyResolver:
      options.publicKeyResolver ?? createCognitoPublicKeyResolver(),
  });
  if (!verified.ok) {
    return unauthenticatedResponse();
  }

  // AC-4-2: トークンを URL へ載せない。運ぶのは Cookie（AC-5-8）だけである。
  return redirectResponse(SIGN_IN_LANDING_PATH, [
    serializeCookie(SESSION_COOKIE_NAME, exchanged.idToken, {
      maxAgeSeconds: SESSION_MAX_AGE_SECONDS,
    }),
    expireCookie(OAUTH_STATE_COOKIE_NAME),
    expireCookie(OAUTH_CODE_VERIFIER_COOKIE_NAME),
  ]);
}

/**
 * (iii) デモ用ロール切替（AC-11-2 (iii)・AC-5-5）。**署名済み Cookie を発行する
 * 唯一の経路**である。
 *
 * 要求が運べるのは「どちらへ切り替えたいか」の申告までであり、**認可の根拠に
 * なるのは BFF が署名した Cookie を BFF 自身が検証して得た値だけ**である
 * （AC-5-5 (i)(ii)）。申告は許可リスト（Engineer / Approver）で検査し、
 * 署名できたときにだけ切替が成立する。
 */
export async function handleRoleSwitch(
  request: Request,
  options: {
    now: Date;
    environment?: Environment;
    publicKeyResolver?: PublicKeyResolver;
  },
): Promise<Response> {
  const config = loadAuthConfig(options.environment);
  if (!config.ok) {
    return misconfiguredResponse();
  }

  // AC-5-5 (viii) / AC-7-5: 鍵が未設定・空なら**切替を成立させない**。
  // 既定値へ落ちない・インスタンスごとに鍵を生成して代替しない。
  const signingKey = loadRoleCookieSigningKey(options.environment);
  if (!signingKey.ok) {
    return misconfiguredResponse();
  }

  // AC-5-5 (v): ゲスト（未認証）には切替を与えない。
  const session = await resolveVerifiedSession({
    cookieHeader: request.headers.get("cookie"),
    now: options.now,
    config: config.value,
    publicKeyResolver:
      options.publicKeyResolver ?? createCognitoPublicKeyResolver(),
  });
  if (!session.ok) {
    return unauthenticatedResponse();
  }

  const requestedRole = await readRequestedRole(request);
  if (requestedRole === undefined) {
    return textResponse(400, "Bad Request");
  }

  // AC-5-5 (iii): Engineer / Approver 以外は成立させない（判定は role-cookie 側）。
  const signed = signRoleCookie({
    role: requestedRole as Role,
    signingKey: signingKey.value,
  });
  if (!signed.ok) {
    return textResponse(400, "Bad Request");
  }

  // AC-5-5 (iv): 切替はロールにのみ効き、利用者識別子（セッション）を書き換えない。
  const headers = new Headers();
  headers.append(
    "Set-Cookie",
    serializeCookie(ROLE_COOKIE_NAME, signed.value, {
      maxAgeSeconds: SESSION_MAX_AGE_SECONDS,
    }),
  );
  headers.set("Cache-Control", "no-store");
  return new Response(null, { status: 204, headers });
}

/**
 * 切替の申告を要求から読む。**ここで読むのは申告に過ぎず**、採用されるのは
 * 許可リストを通り署名できた値だけである（AC-5-5 (i)(iii)）。
 */
async function readRequestedRole(request: Request): Promise<string | undefined> {
  const contentType = request.headers.get("content-type") ?? "";

  try {
    if (contentType.includes("application/json")) {
      const payload: unknown = await request.json();
      if (typeof payload === "object" && payload !== null) {
        const role = (payload as Record<string, unknown>)["role"];
        return typeof role === "string" ? role : undefined;
      }
      return undefined;
    }

    if (
      contentType.includes("application/x-www-form-urlencoded") ||
      contentType.includes("multipart/form-data")
    ) {
      const form = await request.formData();
      const role = form.get("role");
      return typeof role === "string" ? role : undefined;
    }
  } catch {
    return undefined;
  }

  return undefined;
}
