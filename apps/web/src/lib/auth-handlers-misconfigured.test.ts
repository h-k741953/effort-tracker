import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { generateKeyPairSync, sign as cryptoSign, createHmac } from "node:crypto";
// docs/specs/bff-auth-termination.md AC-7-7（構成が欠けているときの応答の形）の
// 検査表 7-a〜7-d をそのまま入出力に落とす。
//
// AC-8-2: Red を先に踏む。
// AC-8-1 / AC-7-3: Vitest 標準の expect だけを使う。モックライブラリを入れない。
// AC-8-3: 決定的であること。時刻は引数で与え、外部へ通信しない
//   （トークンエンドポイントは手書きの Fake、公開鍵の取得も手書きの Fake）。
// AC-8-4 / AC-4-4: 鍵とトークンはテスト内で組み立てる／明らかなダミー値にする。
//
// 検査表（7-a〜7-d）が要求すること —— 「拒否の文言は検査しない」（AC-2 前文）。
// **本文の文字列を期待値として突き合わせない**（AC-7-7 (iv)）。**検査は「本文が
// 空であること」に留める**。ステータスも 5xx というクラスまでで、具体値を
// 固定しない（AC-7-7 (v)・限界 10-17）。
//
// **受理側と対で検査する**（7-d） —— 失敗側だけを検査すると、常に失敗を返す
// 実装が緑になる（AC-8-5 と同じ理由）。
import type { Environment } from "./auth-config";
import {
  handleRoleSwitch,
  handleSignInCallback,
  handleSignInStart,
} from "./auth-handlers";
import type { Jwk, PublicKeyResolver } from "./jwt-verifier";
import type { Role } from "./role-cookie";
import { ROLE_COOKIE_NAME } from "./role-cookie";
import {
  OAUTH_CODE_VERIFIER_COOKIE_NAME,
  OAUTH_STATE_COOKIE_NAME,
  SESSION_COOKIE_NAME,
} from "./session-cookie";

// --- 構成（フルセット。7-d はこのまま、7-a〜7-c はいずれか1つを欠かせる） ----

const REGION = "ap-northeast-1";
const USER_POOL_ID = "ap-northeast-1_dummyPool";
const CLIENT_ID = "dummy0000000000000000000000client";
const ISSUER = `https://cognito-idp.${REGION}.amazonaws.com/${USER_POOL_ID}`;
const SUB = "11111111-1111-1111-1111-111111111111";
const PUBLIC_ORIGIN = "https://public.example.test";
const SIGNING_KEY = "dummy-role-cookie-signing-key-0123456789";

const FULL_ENVIRONMENT: Environment = {
  COGNITO_REGION: REGION,
  COGNITO_USER_POOL_ID: USER_POOL_ID,
  COGNITO_CLIENT_ID: CLIENT_ID,
  COGNITO_DOMAIN_PREFIX: "effort-tracker-dummy",
  PUBLIC_ORIGIN,
  ROLE_COOKIE_SIGNING_KEY: SIGNING_KEY,
};

function withoutVar(name: keyof typeof FULL_ENVIRONMENT): Environment {
  return { ...FULL_ENVIRONMENT, [name]: undefined };
}

// --- 一時値（サインインの戻りに使う。正当な値を固定する） -------------------

const STATE = "dummy-state-0123456789";
const CODE_VERIFIER = "dummy-code-verifier-0123456789abcdefghij";
const AUTHORIZATION_CODE = "dummy-authorization-code-0001";

// --- 鍵とトークン（AC-8-4） -------------------------------------------------

const { publicKey, privateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
});
const PUBLIC_JWK = publicKey.export({ format: "jwk" }) as unknown as Jwk;
const KID = "dummy-key-id-0001";

const NOW = new Date("2026-09-01T00:00:00.000Z");
const NOW_EPOCH_SECONDS = Math.floor(NOW.getTime() / 1000);

function base64url(input: string | Buffer): string {
  return (typeof input === "string" ? Buffer.from(input) : input).toString(
    "base64url",
  );
}

function buildIdToken(payloadOverrides: Record<string, unknown> = {}): string {
  const header = { alg: "RS256", typ: "JWT", kid: KID };
  const payload = {
    iss: ISSUER,
    aud: CLIENT_ID,
    sub: SUB,
    token_use: "id",
    iat: NOW_EPOCH_SECONDS - 10,
    exp: NOW_EPOCH_SECONDS + 3600,
    ...payloadOverrides,
  };
  const signingInput = `${base64url(JSON.stringify(header))}.${base64url(
    JSON.stringify(payload),
  )}`;
  const signature = cryptoSign(
    "RSA-SHA256",
    Buffer.from(signingInput),
    privateKey,
  );
  return `${signingInput}.${base64url(signature)}`;
}

const fakePublicKeySource: PublicKeyResolver = {
  async getKey({ issuer }: { issuer: string; kid: string | undefined }) {
    if (issuer !== ISSUER) {
      throw new Error(
        "fakePublicKeySource: 想定外の issuer で鍵を要求された（AC-2-10）",
      );
    }
    return PUBLIC_JWK;
  },
};

/** 検証済みロール Cookie を組み立てる（role-cookie.test.ts と同じワイヤー形式）。 */
function signedRoleCookieValue(role: Role): string {
  const signature = createHmac("sha256", SIGNING_KEY)
    .update(role)
    .digest("base64url");
  return `${role}.${signature}`;
}

// --- トークンエンドポイントの手書き Fake（AC-8-3・AC-7-3） ------------------

interface RecordedTokenRequest {
  readonly url: string;
  readonly body: URLSearchParams;
}

let tokenRequests: RecordedTokenRequest[] = [];
let tokenEndpointResponse: () => Response = () =>
  jsonResponse({ id_token: buildIdToken() });

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const originalFetch = globalThis.fetch;

async function fakeTokenEndpoint(
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> {
  const url = typeof input === "string" ? input : input.toString();
  const body = typeof init?.body === "string" ? init.body : "";
  tokenRequests.push({ url, body: new URLSearchParams(body) });
  return tokenEndpointResponse();
}

beforeEach(() => {
  tokenRequests = [];
  tokenEndpointResponse = () => jsonResponse({ id_token: buildIdToken() });
  globalThis.fetch = fakeTokenEndpoint as unknown as typeof fetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

// --- 要求の組み立て ---------------------------------------------------------

function signInRequest(): Request {
  return new Request(`${PUBLIC_ORIGIN}/api/auth/sign-in`, { method: "GET" });
}

function callbackRequest(): Request {
  const url = new URL(`${PUBLIC_ORIGIN}/api/auth/callback`);
  url.searchParams.set("code", AUTHORIZATION_CODE);
  url.searchParams.set("state", STATE);
  const headers = new Headers();
  headers.set(
    "Cookie",
    `${OAUTH_STATE_COOKIE_NAME}=${STATE}; ${OAUTH_CODE_VERIFIER_COOKIE_NAME}=${CODE_VERIFIER}`,
  );
  return new Request(url.toString(), { method: "GET", headers });
}

/** ロール切替は検証済みセッションを与えたうえで呼ぶ（未認証の 401 と混ざらないため）。 */
function roleSwitchRequest(): Request {
  const idToken = buildIdToken();
  const headers = new Headers();
  headers.set(
    "Cookie",
    `${SESSION_COOKIE_NAME}=${idToken}; ${ROLE_COOKIE_NAME}=${signedRoleCookieValue("Engineer")}`,
  );
  return new Request(`${PUBLIC_ORIGIN}/api/auth/role`, {
    method: "POST",
    headers,
  });
}

// --- 応答の読み取り ---------------------------------------------------------

function setCookieHeaders(response: Response): readonly string[] {
  return response.headers.getSetCookie();
}

function findSetCookie(
  response: Response,
  name: string,
): string | undefined {
  return setCookieHeaders(response).find((cookie) =>
    cookie.startsWith(`${name}=`),
  );
}

function cookieValue(response: Response, name: string): string | undefined {
  const header = findSetCookie(response, name);
  if (header === undefined) {
    return undefined;
  }
  return header.slice(name.length + 1).split(";")[0];
}

function establishedSession(response: Response): boolean {
  const value = cookieValue(response, SESSION_COOKIE_NAME);
  return typeof value === "string" && value.length > 0;
}

function issuedRoleCookie(response: Response): boolean {
  const value = cookieValue(response, ROLE_COOKIE_NAME);
  return typeof value === "string" && value.length > 0;
}

// --- 7-b / 7-c: 応答の形の共通検査 ------------------------------------------

/**
 * AC-7-7 (i)・7-b: サーバー側の構成の失敗を表す 5xx。401（未認証）・429（超過）
 * と同じ値にならない。
 * AC-7-7 (iv)・7-c: 応答の本文が空である。文字列を期待値として突き合わせない。
 */
async function assertMisconfiguredResponseShape(
  response: Response,
): Promise<void> {
  expect(response.status).toBeGreaterThanOrEqual(500);
  expect(response.status).toBeLessThan(600);
  expect(response.status).not.toBe(401);
  expect(response.status).not.toBe(429);

  const body = await response.text();
  expect(body).toBe("");
}

// --- 7-a〜7-c: 経路ごとの「本来の処理が成立しないこと」 ----------------------

const START_AND_CALLBACK_MISSING_VARS: ReadonlyArray<
  keyof typeof FULL_ENVIRONMENT
> = ["COGNITO_REGION", "PUBLIC_ORIGIN"];
const ROLE_SWITCH_MISSING_VARS: ReadonlyArray<keyof typeof FULL_ENVIRONMENT> =
  ["COGNITO_REGION", "ROLE_COOKIE_SIGNING_KEY"];

describe("7-a〜7-c: サインインの開始（構成欠落時。7-5 / 7-6 のいずれか1つが未設定）", () => {
  it.each(START_AND_CALLBACK_MISSING_VARS.map((missing) => ({ missing })))(
    "$missing が未設定のとき、送り出し（302）が起きず、5xx を返し、本文が空である",
    async ({ missing }) => {
      const response = await handleSignInStart(signInRequest(), {
        environment: withoutVar(missing),
      });

      // 7-a (7-7 (ii)): 送り出しが成立しない。
      expect(response.headers.get("Location")).toBeNull();
      expect(response.status).not.toBe(302);

      // 7-b / 7-c
      await assertMisconfiguredResponseShape(response);
    },
  );
});

describe("7-a〜7-c: サインインの戻り（構成欠落時。7-5 / 7-6 のいずれか1つが未設定）", () => {
  it.each(START_AND_CALLBACK_MISSING_VARS.map((missing) => ({ missing })))(
    "$missing が未設定のとき、コード交換へ進まず、セッション Cookie を発行せず、5xx を返し、本文が空である",
    async ({ missing }) => {
      const response = await handleSignInCallback(callbackRequest(), {
        now: NOW,
        environment: withoutVar(missing),
        publicKeyResolver: fakePublicKeySource,
      });

      // 7-a (7-7 (ii)): コード交換へ進まず、セッション Cookie を発行しない。
      expect(tokenRequests).toHaveLength(0);
      expect(establishedSession(response)).toBe(false);

      // 7-b / 7-c
      await assertMisconfiguredResponseShape(response);
    },
  );
});

describe("7-a〜7-c: デモ用ロール切替（構成欠落時。7-5 / 7-6 のいずれか1つが未設定）", () => {
  it.each(ROLE_SWITCH_MISSING_VARS.map((missing) => ({ missing })))(
    "$missing が未設定のとき、ロール Cookie を発行せず、5xx を返し、本文が空である",
    async ({ missing }) => {
      const response = await handleRoleSwitch(roleSwitchRequest(), {
        now: NOW,
        environment: withoutVar(missing),
        publicKeyResolver: fakePublicKeySource,
      });

      // 7-a (7-7 (ii)): ロール Cookie を発行しない。
      expect(issuedRoleCookie(response)).toBe(false);

      // 7-b / 7-c
      await assertMisconfiguredResponseShape(response);
    },
  );
});

// --- 7-d: 構成がそろっている状態（受理側との対。AC-8-5） --------------------
//
// 失敗側だけを検査すると、常に失敗を返す実装が緑になる。構成がそろっている
// ときは、7-a〜7-c の結果にならないこと（送り出し・コード交換・Cookie の発行
// が成立し、ステータスが 5xx にならないこと）を確かめる。

describe("7-d: サインインの開始（構成がそろっている）", () => {
  it("送り出し（302）が成立し、ステータスが 5xx にならない", async () => {
    const response = await handleSignInStart(signInRequest(), {
      environment: FULL_ENVIRONMENT,
    });

    expect(response.status).toBe(302);
    expect(response.headers.get("Location")).not.toBeNull();
    expect(response.status).not.toBeGreaterThanOrEqual(500);
  });
});

describe("7-d: サインインの戻り（構成がそろっている）", () => {
  it("コード交換が成立し、セッション Cookie が発行され、ステータスが 5xx にならない", async () => {
    const response = await handleSignInCallback(callbackRequest(), {
      now: NOW,
      environment: FULL_ENVIRONMENT,
      publicKeyResolver: fakePublicKeySource,
    });

    expect(tokenRequests).toHaveLength(1);
    expect(establishedSession(response)).toBe(true);
    expect(response.status).not.toBeGreaterThanOrEqual(500);
  });
});

describe("7-d: デモ用ロール切替（構成がそろっている）", () => {
  it("ロール Cookie が発行され、ステータスが 5xx にならない", async () => {
    const response = await handleRoleSwitch(roleSwitchRequest(), {
      now: NOW,
      environment: FULL_ENVIRONMENT,
      publicKeyResolver: fakePublicKeySource,
    });

    expect(issuedRoleCookie(response)).toBe(true);
    expect(response.status).not.toBeGreaterThanOrEqual(500);
  });
});
