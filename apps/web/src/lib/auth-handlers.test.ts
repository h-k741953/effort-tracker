import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  createHash,
  generateKeyPairSync,
  sign as cryptoSign,
} from "node:crypto";
// AC-12（docs/specs/bff-auth-termination.md）の検査表 12-a〜12-k をそのまま
// 入出力に落とす。あわせて AC-7-6（公開オリジンの読み取り）を 12-i / 12-j が
// 検査する。
//
// AC-8-2: Red を先に踏む。
// AC-8-1 / AC-7-3: Vitest 標準の expect だけを使う。モックライブラリを入れない。
// AC-8-3: 決定的であること。時刻は引数で与え（NOW）、**生成する値は差し替えて
//   与える**（12-7 (iii)）。外部へ通信しない —— トークンエンドポイントへの
//   要求は**手書きのインメモリ Fake**（fakeTokenEndpoint）で受け、公開鍵の取得も
//   手書きの Fake（fakePublicKeySource）で差し替える。実際の Cognito・実際の
//   JWKS を呼ばない。
// AC-8-4 / AC-4-4: 鍵とトークンはテスト内で組み立て、固定値は明らかにダミーと
//   読める形にする（実在しそうな URL を書かない＝example.test 系）。
import type { Environment } from "./auth-config";
import { handleSignInCallback, handleSignInStart } from "./auth-handlers";
import type { Jwk, PublicKeyResolver } from "./jwt-verifier";
import {
  OAUTH_CODE_VERIFIER_COOKIE_NAME,
  OAUTH_STATE_COOKIE_NAME,
  SESSION_COOKIE_NAME,
} from "./session-cookie";

// --- 構成（AC-7-5・AC-7-6: いずれも環境変数から受け取る） -------------------
//
// 公開オリジンの環境変数の名前は、**本仕様が持たない**（AC-7-6 (iii)「名前の
// 実体は構成側と実装側が持ち、本仕様は役割で参照する」）。ここで固定する
// PUBLIC_ORIGIN は**このテストが暫定的に置くインターフェース**であり、実装側は
// 同じ名前で作ってよいし、都合が悪ければテストごと見直す（Terraform 側の変数名
// は `public_origin`＝`cognito-auth-infra.md` AC-8-8。**名の一致は機械検査
// されない**）。
const PUBLIC_ORIGIN = "https://public.example.test";
const REGION = "ap-northeast-1";
const USER_POOL_ID = "ap-northeast-1_dummyPool";
const CLIENT_ID = "dummy0000000000000000000000client";
const ISSUER = `https://cognito-idp.${REGION}.amazonaws.com/${USER_POOL_ID}`;
const SUB = "11111111-1111-1111-1111-111111111111";

const ENVIRONMENT: Environment = {
  COGNITO_REGION: REGION,
  COGNITO_USER_POOL_ID: USER_POOL_ID,
  COGNITO_CLIENT_ID: CLIENT_ID,
  COGNITO_DOMAIN_PREFIX: "effort-tracker-dummy",
  PUBLIC_ORIGIN,
};

/** 12-i: 要求が名乗る別のオリジン（構成側の公開オリジンとは異なる）。 */
const REQUEST_ORIGIN = "https://attacker.example.test";

// AC-2-11 / AC-8-3: 現在時刻は呼ぶ側から与える。
const NOW = new Date("2026-09-01T00:00:00.000Z");
const NOW_EPOCH_SECONDS = Math.floor(NOW.getTime() / 1000);

// --- 差し替える値（12-7 (iii)） ---------------------------------------------

const STATE = "dummy-state-0123456789";
const CODE_VERIFIER = "dummy-code-verifier-0123456789abcdefghij";
const OTHER_CODE_VERIFIER = "dummy-code-verifier-zyxwvutsrq9876543210";
const AUTHORIZATION_CODE = "dummy-authorization-code-0001";

// --- 鍵とトークン（AC-8-4） -------------------------------------------------

const { publicKey, privateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
});
const PUBLIC_JWK = publicKey.export({ format: "jwk" }) as unknown as Jwk;
const KID = "dummy-key-id-0001";

function base64url(input: string | Buffer): string {
  return (typeof input === "string" ? Buffer.from(input) : input).toString(
    "base64url",
  );
}

function buildIdToken(
  payloadOverrides: Record<string, unknown> = {},
): string {
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

/** AC-7-2: 公開鍵の取得は手書きの Fake で差し替える（ネットへ出ない）。 */
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

// --- トークンエンドポイントの手書き Fake（AC-8-3・AC-7-3） ------------------
//
// globalThis.fetch を**手書きの関数**へ差し替える。モックライブラリは使わない。
// これにより、実装が交換へ進んだかどうか（進まなかったかどうか）を観測でき、
// かつ**どの経路を通っても外部へ出ない**ことが保証される。

interface RecordedTokenRequest {
  readonly url: string;
  readonly body: URLSearchParams;
}

let tokenRequests: RecordedTokenRequest[] = [];
/** 交換の応答を差し替える（既定は成功して正当な ID トークンを返す）。 */
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

// --- 呼び出しの薄い包み（テストが暫定的に固定するインターフェース） ----------
//
// 12-7 (iii): サインインの開始が使う値（state・code_verifier）は、**呼ぶ側から
// 差し替えられる形**でなければならない。generateOAuthValues がその差し替え点で
// あり、名前は本テストが暫定的に固定する（AC-1-3 と同型。実装側は同じ名前で
// 作ってよいし、都合が悪ければテストごと見直す）。

interface OAuthValues {
  readonly state: string;
  readonly codeVerifier: string;
}

interface SignInStartOptions {
  environment: Environment;
  generateOAuthValues?: () => OAuthValues;
}

function startSignIn(
  request: Request,
  options: SignInStartOptions,
): Promise<Response> {
  return handleSignInStart(request, options);
}

function signInRequest(
  options: { origin?: string; forwardedHost?: string } = {},
): Request {
  const origin = options.origin ?? PUBLIC_ORIGIN;
  const headers = new Headers();
  if (options.forwardedHost !== undefined) {
    // 12-i: ブラウザから与えられた値（要求由来）。
    headers.set("X-Forwarded-Host", options.forwardedHost);
    headers.set("X-Forwarded-Proto", "https");
  }
  return new Request(`${origin}/api/auth/sign-in`, { method: "GET", headers });
}

function callbackRequest(
  options: {
    code?: string | null;
    state?: string | null;
    stateCookie?: string | null;
    verifierCookie?: string | null;
    origin?: string;
    forwardedHost?: string;
  } = {},
): Request {
  const origin = options.origin ?? PUBLIC_ORIGIN;
  const url = new URL(`${origin}/api/auth/callback`);
  const code = options.code === undefined ? AUTHORIZATION_CODE : options.code;
  const state = options.state === undefined ? STATE : options.state;
  if (code !== null) {
    url.searchParams.set("code", code);
  }
  if (state !== null) {
    url.searchParams.set("state", state);
  }

  const cookies: string[] = [];
  const stateCookie =
    options.stateCookie === undefined ? STATE : options.stateCookie;
  const verifierCookie =
    options.verifierCookie === undefined
      ? CODE_VERIFIER
      : options.verifierCookie;
  if (stateCookie !== null) {
    cookies.push(`${OAUTH_STATE_COOKIE_NAME}=${stateCookie}`);
  }
  if (verifierCookie !== null) {
    cookies.push(`${OAUTH_CODE_VERIFIER_COOKIE_NAME}=${verifierCookie}`);
  }

  const headers = new Headers();
  if (cookies.length > 0) {
    headers.set("Cookie", cookies.join("; "));
  }
  if (options.forwardedHost !== undefined) {
    headers.set("X-Forwarded-Host", options.forwardedHost);
    headers.set("X-Forwarded-Proto", "https");
  }
  return new Request(url.toString(), { method: "GET", headers });
}

function runCallback(
  request: Request,
  environment: Environment = ENVIRONMENT,
): Promise<Response> {
  return handleSignInCallback(request, {
    now: NOW,
    environment,
    publicKeyResolver: fakePublicKeySource,
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

/** セッションが確立された（空でない値の Cookie が発行された）か。 */
function establishedSession(response: Response): boolean {
  const value = cookieValue(response, SESSION_COOKIE_NAME);
  return typeof value === "string" && value.length > 0;
}

/** 12-h: 一時 Cookie が破棄されているか（空の値 + 失効の指示）。 */
function isDiscarded(response: Response, name: string): boolean {
  const header = findSetCookie(response, name);
  if (header === undefined) {
    return false;
  }
  const value = header.slice(name.length + 1).split(";")[0];
  return (
    value.length === 0 &&
    (/;\s*max-age=0\s*(;|$)/i.test(header) || /;\s*expires=/i.test(header))
  );
}

/** 送り出し先（ホストされたサインイン画面）の URL。 */
function authorizeUrl(response: Response): URL {
  const location = response.headers.get("Location");
  expect(location).not.toBeNull();
  return new URL(location as string);
}

function expectedRedirectUri(): string {
  return new URL("/api/auth/callback", PUBLIC_ORIGIN).toString();
}

// --- 12-a -------------------------------------------------------------------

describe("12-a: サインインの開始（送り出しの URL）", () => {
  it("変換後の値と state が載り、検証値そのものは URL のどこにも現れない（12-6）", async () => {
    const response = await startSignIn(signInRequest(), {
      environment: ENVIRONMENT,
      generateOAuthValues: () => ({
        state: STATE,
        codeVerifier: CODE_VERIFIER,
      }),
    });

    const url = authorizeUrl(response);
    expect(url.searchParams.get("state")).toBe(STATE);

    const challenge = url.searchParams.get("code_challenge");
    expect(typeof challenge).toBe("string");
    expect(challenge).not.toBe("");

    // 12-6: 検証値そのものは URL のどこ（クエリ・パス）にも現れない。
    expect(url.toString()).not.toContain(CODE_VERIFIER);
  });
});

// --- 12-b -------------------------------------------------------------------

describe("12-b: 検証値から変換後の値への変換（12-2 (iii)）", () => {
  async function challengeFor(codeVerifier: string): Promise<string> {
    const response = await startSignIn(signInRequest(), {
      environment: ENVIRONMENT,
      generateOAuthValues: () => ({ state: STATE, codeVerifier }),
    });
    const challenge = authorizeUrl(response).searchParams.get("code_challenge");
    expect(challenge).not.toBeNull();
    return challenge as string;
  }

  it("異なる2つの検証値からは、それぞれ異なる変換後の値になる", async () => {
    const first = await challengeFor(CODE_VERIFIER);
    const second = await challengeFor(OTHER_CODE_VERIFIER);
    expect(first).not.toBe(second);
  });

  it("同じ検証値からは同じ変換後の値になる", async () => {
    const first = await challengeFor(CODE_VERIFIER);
    const second = await challengeFor(CODE_VERIFIER);
    expect(first).toBe(second);
  });

  it("変換後の値が検証値と同一でない（plain 相当を選ばない）", async () => {
    const challenge = await challengeFor(CODE_VERIFIER);
    expect(challenge).not.toBe(CODE_VERIFIER);
    // 12-2 (iii): 変換は一方向のハッシュ（S256 相当）である。
    expect(challenge).toBe(
      createHash("sha256").update(CODE_VERIFIER).digest("base64url"),
    );
  });
});

// --- 12-c -------------------------------------------------------------------

describe("12-c: 戻り（受理側）", () => {
  it("state が一致し、一時 Cookie の検証値があり、認可コードがあるときはコード交換へ進む", async () => {
    const response = await runCallback(callbackRequest());

    expect(tokenRequests).toHaveLength(1);
    // 12-2 (ii): 交換の要求には開始時の検証値そのものが添えられている。
    expect(tokenRequests[0].body.get("code_verifier")).toBe(CODE_VERIFIER);
    expect(tokenRequests[0].body.get("code")).toBe(AUTHORIZATION_CODE);
    // 受理側が実際に成立していること（拒否側の検査を空虚にしない）。
    expect(establishedSession(response)).toBe(true);
  });
});

// --- 12-d / 12-e / 12-f -----------------------------------------------------

describe("12-d / 12-e / 12-f: 戻り（拒否側）", () => {
  const cases: Array<{ name: string; request: () => Request }> = [
    {
      name: "12-d: state が一致しない",
      request: () => callbackRequest({ state: "dummy-state-mismatched" }),
    },
    {
      name: "12-e (1): 戻りのクエリに state が無い",
      request: () => callbackRequest({ state: null }),
    },
    {
      name: "12-e (2): 一時 Cookie（state）が無い",
      request: () => callbackRequest({ stateCookie: null }),
    },
    {
      name: "12-e (3): 一時 Cookie（検証値）が無い",
      request: () => callbackRequest({ verifierCookie: null }),
    },
    {
      name: "12-e (4): state が空（クエリ・Cookie とも空）",
      request: () => callbackRequest({ state: "", stateCookie: "" }),
    },
    {
      name: "12-e (5): 一時 Cookie の検証値が空",
      request: () => callbackRequest({ verifierCookie: "" }),
    },
    {
      name: "12-f (1): 認可コードが無い",
      request: () => callbackRequest({ code: null }),
    },
    {
      name: "12-f (2): 認可コードが空",
      request: () => callbackRequest({ code: "" }),
    },
  ];

  it.each(cases)(
    "$name はコード交換へ進まず、セッション Cookie を発行しない",
    async ({ request }) => {
      const response = await runCallback(request());

      expect(tokenRequests).toHaveLength(0);
      expect(establishedSession(response)).toBe(false);
    },
  );
});

// --- 12-g -------------------------------------------------------------------

describe("12-g: 交換は成立したが、得たトークンが AC-2 の検証を通らない", () => {
  const cases: Array<{ name: string; idToken: () => string }> = [
    {
      name: "有効期限（exp）を過ぎた ID トークン",
      idToken: () => buildIdToken({ exp: NOW_EPOCH_SECONDS - 1 }),
    },
    {
      name: "宛先（aud）が当該アプリクライアントでない ID トークン",
      idToken: () =>
        buildIdToken({ aud: "dummy1111111111111111111111client" }),
    },
  ];

  it.each(cases)("$name ではセッションを確立しない", async ({ idToken }) => {
    tokenEndpointResponse = () => jsonResponse({ id_token: idToken() });

    const response = await runCallback(callbackRequest());

    // 交換自体は成立している（この検査が「交換に進まなかったから緑」に
    // ならないことを、ここで確かめる）。
    expect(tokenRequests).toHaveLength(1);
    expect(establishedSession(response)).toBe(false);
    // 3-4 (ii): 検証に失敗した状態を次の要求へ持ち越さない。
    expect(isDiscarded(response, SESSION_COOKIE_NAME)).toBe(true);
  });
});

// --- 12-h -------------------------------------------------------------------

describe("12-h: 一時 Cookie の破棄（5-10 (iv)）", () => {
  it("戻りの処理が成功した場合でも一時 Cookie が破棄されている", async () => {
    const response = await runCallback(callbackRequest());

    expect(establishedSession(response)).toBe(true);
    expect(isDiscarded(response, OAUTH_STATE_COOKIE_NAME)).toBe(true);
    expect(isDiscarded(response, OAUTH_CODE_VERIFIER_COOKIE_NAME)).toBe(true);
  });

  it("戻りが拒否された場合でも一時 Cookie が破棄されている", async () => {
    const response = await runCallback(
      callbackRequest({ state: "dummy-state-mismatched" }),
    );

    expect(establishedSession(response)).toBe(false);
    expect(isDiscarded(response, OAUTH_STATE_COOKIE_NAME)).toBe(true);
    expect(isDiscarded(response, OAUTH_CODE_VERIFIER_COOKIE_NAME)).toBe(true);
  });
});

// --- 12-i（＝AC-7-6 (i)） ---------------------------------------------------

describe("12-i: 戻り先の出所は構成側の公開オリジンだけ（12-9・7-6 (i)）", () => {
  it("開始: X-Forwarded-Host / Host に別のオリジンを与えても戻り先が変わらない", async () => {
    const response = await startSignIn(
      signInRequest({
        origin: REQUEST_ORIGIN,
        forwardedHost: "forwarded.example.test",
      }),
      { environment: ENVIRONMENT },
    );

    const redirectUri = authorizeUrl(response).searchParams.get("redirect_uri");
    expect(redirectUri).toBe(expectedRedirectUri());
    expect(redirectUri).not.toContain("attacker.example.test");
    expect(redirectUri).not.toContain("forwarded.example.test");
  });

  it("戻り: X-Forwarded-Host / Host に別のオリジンを与えても、交換に添える戻り先が変わらない", async () => {
    await runCallback(
      callbackRequest({
        origin: REQUEST_ORIGIN,
        forwardedHost: "forwarded.example.test",
      }),
    );

    expect(tokenRequests).toHaveLength(1);
    const redirectUri = tokenRequests[0].body.get("redirect_uri");
    // 12-9 (i): 開始で送り出す戻り先と、交換で添える戻り先は同じ値である。
    expect(redirectUri).toBe(expectedRedirectUri());
    expect(redirectUri).not.toContain("attacker.example.test");
    expect(redirectUri).not.toContain("forwarded.example.test");
  });
});

// --- 12-j（＝AC-7-6 (ii)） --------------------------------------------------

describe("12-j: 公開オリジンが未設定／空（7-6 (ii)・12-9 (ii)）", () => {
  const withoutOrigin: Environment = {
    ...ENVIRONMENT,
    PUBLIC_ORIGIN: undefined,
  };
  const emptyOrigin: Environment = { ...ENVIRONMENT, PUBLIC_ORIGIN: "" };

  const environments: Array<{ name: string; environment: Environment }> = [
    { name: "未設定", environment: withoutOrigin },
    { name: "空", environment: emptyOrigin },
  ];

  it.each(environments)(
    "開始: 公開オリジンが$name のとき、既定値へ落ちず・要求元のホストへも落ちず、送り出しが成立しない",
    async ({ environment }) => {
      const response = await startSignIn(
        signInRequest({ origin: REQUEST_ORIGIN }),
        { environment },
      );

      expect(response.headers.get("Location")).toBeNull();
      expect(response.status).not.toBe(302);
    },
  );

  it.each(environments)(
    "戻り: 公開オリジンが$name のとき、コード交換へ進まず、セッションを確立しない",
    async ({ environment }) => {
      const response = await runCallback(
        callbackRequest({ origin: REQUEST_ORIGIN }),
        environment,
      );

      expect(tokenRequests).toHaveLength(0);
      expect(establishedSession(response)).toBe(false);
    },
  );
});

// --- 12-k -------------------------------------------------------------------

describe("12-k: 一時 Cookie の属性（5-10 (ii)）", () => {
  it("HttpOnly / Secure / SameSite の3属性をいずれも欠かない", async () => {
    const response = await startSignIn(signInRequest(), {
      environment: ENVIRONMENT,
    });

    const names = [OAUTH_STATE_COOKIE_NAME, OAUTH_CODE_VERIFIER_COOKIE_NAME];
    for (const name of names) {
      const header = findSetCookie(response, name);
      expect(header).toBeDefined();
      expect(header).toMatch(/;\s*HttpOnly\s*(;|$)/i);
      expect(header).toMatch(/;\s*Secure\s*(;|$)/i);
      expect(header).toMatch(/;\s*SameSite=[A-Za-z]+\s*(;|$)/i);
    }
  });
});

// --- 5-h ----------------------------------------------------------------

// 検査（Vitest）— セッション Cookie（5-8 (i)）とロール Cookie（5-5 (vi)）の属性
// （bff-auth-termination.md）。5-i（ロール Cookie 側）は
// auth-handlers-role-switch.test.ts が持つ（同じ要求を両所へ置かない＝ADR 0004）。
//
// 見るのは HttpOnly / Secure / SameSite の3属性がいずれも欠けていないことまで
// であり、SameSite の具体の値は実装側が持つため検査しない（5-8 (i)）。
// 12-k と同じく、応答が実際に送出する Set-Cookie を直接見る —— 共通の組み立て
// （serializeCookie）だけを呼んで確かめる形にしない。理由は、関数が属性を
// 付けていても、経路がその関数を通らなければ属性は失われ、関数だけを見る検査は
// 緑のまま残るためである。

describe("5-h: サインインの戻りが成功した応答が送出する、セッション Cookie の Set-Cookie の属性（5-8 (i)）", () => {
  it("HttpOnly / Secure / SameSite の3属性をいずれも欠かない", async () => {
    const response = await runCallback(callbackRequest());

    const header = findSetCookie(response, SESSION_COOKIE_NAME);
    expect(header).toBeDefined();
    expect(header).toMatch(/;\s*HttpOnly\s*(;|$)/i);
    expect(header).toMatch(/;\s*Secure\s*(;|$)/i);
    expect(header).toMatch(/;\s*SameSite=[A-Za-z]+\s*(;|$)/i);
  });
});
