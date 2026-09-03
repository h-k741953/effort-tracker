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
import {
  loadAuthConfig,
  loadPublicOrigin,
  loadRoleCookieSigningKey,
} from "./auth-config";
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
import { ROLE_COOKIE_NAME, resolveEffectiveRole, signRoleCookie } from "./role-cookie";
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
 * AC-12-7 (iii): サインインの開始が使う一時的な値（state・code_verifier）は
 * 呼ぶ側から差し替えられる形にする（P-7・AC-8-3。テストが決定的であるため）。
 * **本番の既定は暗号論的に安全な乱数から作る**（差し替え可能にしたことを理由に
 * 既定を弱くしない）。
 */
interface OAuthValues {
  readonly state: string;
  readonly codeVerifier: string;
}

function defaultOAuthValues(): OAuthValues {
  return { state: createOAuthState(), codeVerifier: createPkcePair().codeVerifier };
}

/**
 * (i) サインインの開始（AC-11-2 (i)）。ホストされたサインイン画面へ送り出す。
 */
export async function handleSignInStart(
  request: Request,
  options: {
    environment?: Environment;
    generateOAuthValues?: () => OAuthValues;
  } = {},
): Promise<Response> {
  const config = loadAuthConfig(options.environment);
  if (!config.ok) {
    return misconfiguredResponse();
  }
  // AC-7-6: 戻り先は構成側から受け取った公開オリジンだけから決める。要求
  // ヘッダ（X-Forwarded-Host 等）からは導かない。未設定・空なら失敗として扱う。
  const publicOrigin = loadPublicOrigin(options.environment);
  if (!publicOrigin.ok) {
    return misconfiguredResponse();
  }

  const redirectUri = resolveRedirectUri(publicOrigin.value);
  const { state, codeVerifier } = (options.generateOAuthValues ?? defaultOAuthValues)();
  const pkce = createPkcePair(codeVerifier);

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
  // AC-7-6: 戻り先は構成側から受け取った公開オリジンだけから決める。
  const publicOrigin = loadPublicOrigin(options.environment);
  if (!publicOrigin.ok) {
    return misconfiguredResponse();
  }

  const requestUrl = new URL(request.url);
  const code = requestUrl.searchParams.get("code");
  const state = requestUrl.searchParams.get("state");
  const cookies = parseCookieHeader(request.headers.get("cookie"));
  const expectedState = cookies.get(OAUTH_STATE_COOKIE_NAME);
  const codeVerifier = cookies.get(OAUTH_CODE_VERIFIER_COOKIE_NAME);

  // AC-12-3 / AC-12-4: 欠落・不一致・空はすべて拒否側に含める。空の state
  // 同士が一致してしまう／空の検証値が undefined 判定を抜けることのないよう、
  // 明示的に長さ 0 も拒否する。
  if (
    typeof code !== "string" ||
    code.length === 0 ||
    typeof state !== "string" ||
    state.length === 0 ||
    expectedState === undefined ||
    expectedState.length === 0 ||
    state !== expectedState ||
    codeVerifier === undefined ||
    codeVerifier.length === 0
  ) {
    return unauthenticatedResponse();
  }

  const exchanged = await exchangeAuthorizationCode({
    config: config.value,
    code,
    redirectUri: resolveRedirectUri(publicOrigin.value),
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

  // AC-5-5 (ii): 要求ヘッダ・クエリ・本文で与えられたロールを採用しない。
  // 採用するのは、BFF が発行し BFF 自身が検証した現在のロール Cookie だけで
  // あり、それを反転させる（AC-5-11。正当な Cookie が無ければ反転元は
  // Engineer）。
  const cookies = parseCookieHeader(request.headers.get("cookie"));
  const currentRole = resolveEffectiveRole({
    cookieValue: cookies.get(ROLE_COOKIE_NAME),
    signingKey: signingKey.value,
  });
  const nextRole: Role = currentRole === "Engineer" ? "Approver" : "Engineer";

  const signed = signRoleCookie({ role: nextRole, signingKey: signingKey.value });
  if (!signed.ok) {
    return misconfiguredResponse();
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
