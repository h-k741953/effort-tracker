// docs/specs/bff-wiring.md の検査（AC-1〜AC-9）が共有する補助。アプリの実装からは
// import しない（検査専用。src/test-support/ の規則は test-support-boundary.test.ts）。
//
// - AC-11-3: 外部へ通信しない。公開鍵の取得・送信は手書きの Fake。時刻は固定値。
//   プロセスの環境変数を書き換えず、環境変数の集まりは引数で与える。
// - AC-9-3 / 終端仕様 AC-4-4: 鍵・トークン・資格情報はテスト内で組み立てるか、
//   明らかなダミー値にする。
// - 本ファイルは lambda-client を import する。1-a の走査（lambda-client を
//   import する実装ファイルの場所）は、テストと test-support を母集団から除く。
import {
  createHmac,
  generateKeyPairSync,
  sign as cryptoSign,
} from "node:crypto";
import { expect } from "vitest";
import type { Environment } from "../lib/auth-config";
import type { Jwk, PublicKeyResolver } from "../lib/jwt-verifier";
import type { SendFunction } from "../lib/lambda-client";
import { createLambdaClient, loadCredentials } from "../lib/lambda-client";
import { routeHandlerRateLimiter, resolveRateLimitKey } from "../lib/rate-limit";
import type { Role } from "../lib/role-cookie";
import { ROLE_COOKIE_NAME } from "../lib/role-cookie";
import { SESSION_COOKIE_NAME } from "../lib/session-cookie";

// --- 構成・資格情報（ダミー。識別できる印を含める＝9-a） -----------------

export const REGION = "ap-northeast-1";
const USER_POOL_ID = "ap-northeast-1_dummyPool";
const CLIENT_ID = "dummy0000000000000000000000client";
export const ISSUER = `https://cognito-idp.${REGION}.amazonaws.com/${USER_POOL_ID}`;
export const SUB = "11111111-1111-1111-1111-111111111111";
export const SIGNING_KEY = "dummy-role-cookie-signing-key-0123456789";

export const FUNCTION_URL_MARK = "DUMMY-FUNCTION-URL-MARK";
export const SECRET_MARK = "DUMMY-SECRET-MARK";
export const TOKEN_MARK = "DUMMY-TOKEN-MARK";
export const ACCESS_KEY_ID_VALUE = "DUMMY-ACCESS-KEY-ID";

export const DOMAIN_API_URL = `https://${FUNCTION_URL_MARK.toLowerCase()}.lambda-url.${REGION}.on.aws/`;

/**
 * 構成と資格情報がそろった環境変数の集まり。
 *
 * 名前の根拠 —— 資格情報の 3 つと AWS_REGION は Lambda ランタイムが実行ロールの
 * 一時資格情報とともに載せる環境変数で、名前の持ち主は Lambda ランタイム
 * （bff-wiring.md AC-3-3・D 節 U-1b）。呼び先の変数名は仕様が持たない
 * （AC-3-1）ため、本テストが DOMAIN_API_FUNCTION_URL に固定する（役割どおりの
 * 名。Terraform 側の名との一致は検査されない＝AC-13-2・infra-terraform.md 12-38）。
 */
export const ENVIRONMENT: Environment = {
  COGNITO_REGION: REGION,
  COGNITO_USER_POOL_ID: USER_POOL_ID,
  COGNITO_CLIENT_ID: CLIENT_ID,
  COGNITO_DOMAIN_PREFIX: "effort-tracker-dummy",
  PUBLIC_ORIGIN: "https://public.example.test",
  ROLE_COOKIE_SIGNING_KEY: SIGNING_KEY,
  DOMAIN_API_FUNCTION_URL: DOMAIN_API_URL,
  AWS_REGION: REGION,
  AWS_ACCESS_KEY_ID: ACCESS_KEY_ID_VALUE,
  AWS_SECRET_ACCESS_KEY: SECRET_MARK,
  AWS_SESSION_TOKEN: TOKEN_MARK,
};

/** ENVIRONMENT から指定した変数を取り除く／空にした集まり。 */
export function environmentWithout(...names: string[]): Environment {
  const copy: Environment = { ...ENVIRONMENT };
  for (const name of names) {
    delete copy[name];
  }
  return copy;
}

export function environmentWithEmpty(...names: string[]): Environment {
  const copy: Environment = { ...ENVIRONMENT };
  for (const name of names) {
    copy[name] = "";
  }
  return copy;
}

// AC-2-5 / 終端仕様 AC-2-11: 現在時刻は呼ぶ側から与える。
export const NOW = new Date("2026-10-16T01:02:03.000Z");
const NOW_EPOCH_SECONDS = Math.floor(NOW.getTime() / 1000);

// --- 鍵とトークン ------------------------------------------------------------

const { publicKey, privateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
});
const { privateKey: otherPrivateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
});
const PUBLIC_JWK = publicKey.export({ format: "jwk" }) as unknown as Jwk;

function base64url(input: string | Buffer): string {
  return (typeof input === "string" ? Buffer.from(input) : input).toString(
    "base64url",
  );
}

function buildToken(
  signingPrivateKey: typeof privateKey,
  payloadOverrides: Record<string, unknown>,
): string {
  const header = { alg: "RS256", typ: "JWT", kid: "dummy-key-id-0001" };
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
  return `${signingInput}.${base64url(
    cryptoSign("RSA-SHA256", Buffer.from(signingInput), signingPrivateKey),
  )}`;
}

/** 検証に成功する ID トークン。 */
export function buildIdToken(): string {
  return buildToken(privateKey, {});
}

/** 署名が合わない（別の鍵で署名された）ID トークン。検証に失敗する。 */
export function buildIdTokenWithBadSignature(): string {
  return buildToken(otherPrivateKey, {});
}

export const fakePublicKeySource: PublicKeyResolver = {
  async getKey({ issuer }: { issuer: string; kid: string | undefined }) {
    if (issuer !== ISSUER) {
      throw new Error(
        "fakePublicKeySource: 想定外の issuer で鍵を要求された（終端仕様 AC-2-10）",
      );
    }
    return PUBLIC_JWK;
  },
};

/** role-cookie.test.ts が固定しているワイヤーフォーマットに合わせる。 */
export function signedRoleCookieValue(role: Role): string {
  const signature = createHmac("sha256", SIGNING_KEY)
    .update(role)
    .digest("base64url");
  return `${role}.${signature}`;
}

/** 改竄された署名（Approver を主張するが検証できない）。 */
export function tamperedRoleCookieValue(role: Role): string {
  return `${signedRoleCookieValue(role)}x`;
}

// --- ブラウザの要求 ----------------------------------------------------------

export const VIEWER_ADDRESS = "203.0.113.250:443";
export const PATH_PARAMS = {
  contractId: "c-001",
  yearMonth: "2026-07",
  date: "2026-07-15",
} as const;

export interface BrowserRequestOptions {
  /** セッション Cookie の値。省略時は検証に成功するトークン。null で持たせない。 */
  readonly session?: string | null;
  /** ロール Cookie の生の値。省略時は持たせない。 */
  readonly roleCookieValue?: string;
  /** 追加のヘッダ（ブラウザが勝手に付けるもの）。 */
  readonly headers?: Readonly<Record<string, string>>;
  readonly body?: BodyInit;
  /** `?` 以降。 */
  readonly search?: string;
}

export function browserRequest(
  method: string,
  options: BrowserRequestOptions = {},
): Request {
  const session = options.session === undefined ? buildIdToken() : options.session;
  const cookies: string[] = [];
  if (session !== null) {
    cookies.push(`${SESSION_COOKIE_NAME}=${session}`);
  }
  if (options.roleCookieValue !== undefined) {
    cookies.push(`${ROLE_COOKIE_NAME}=${options.roleCookieValue}`);
  }

  const headers = new Headers({ "CloudFront-Viewer-Address": VIEWER_ADDRESS });
  if (cookies.length > 0) {
    headers.set("Cookie", cookies.join("; "));
  }
  for (const [name, value] of Object.entries(options.headers ?? {})) {
    headers.set(name, value);
  }

  return new Request(
    `https://public.example.test/api/dummy${options.search ?? ""}`,
    { method, headers, body: options.body },
  );
}

/** 共有の制限器を、当該要求のキーについて上限まで使い切らせる（AC-11-6: 60 件）。 */
export function exhaustSharedLimiterFor(request: Request): void {
  const key = resolveRateLimitKey(request.headers);
  for (let i = 0; i < 60; i += 1) {
    routeHandlerRateLimiter.check(key, new Date());
  }
}

/**
 * セッション Cookie とロール Cookie を失効させる Set-Cookie を持つこと
 * （終端仕様 AC-3-4 (ii)・本仕様 5-d・5-f・6-j）。
 */
export function expectAuthCookiesExpired(response: Response): void {
  const setCookies = response.headers.getSetCookie();
  for (const name of [SESSION_COOKIE_NAME, ROLE_COOKIE_NAME]) {
    const expired = setCookies.filter(
      (cookie) => cookie.startsWith(`${name}=;`) && /Max-Age=0/.test(cookie),
    );
    expect(expired.length).toBeGreaterThanOrEqual(1);
  }
}

// --- 送信関数の Fake とクライアントの組み立て（AC-3-5・AC-11-3） ---------------

/** Fake の送信関数が受け取った 1 件。 */
export interface SentRequest {
  readonly url: string;
  readonly init: {
    readonly method: string;
    readonly headers: Readonly<Record<string, string>>;
    readonly body?: Uint8Array;
  };
}

/** 受け取った要求を記録し、与えた関数が返す応答を返す手書きの Fake。 */
export function recordingSend(
  respond: (sent: SentRequest) => Response | Promise<Response>,
): { readonly send: SendFunction; readonly sent: SentRequest[] } {
  const sent: SentRequest[] = [];
  const send: SendFunction = async (url, init) => {
    const entry: SentRequest = { url, init };
    sent.push(entry);
    return respond(entry);
  };
  return { send, sent };
}

/** 契約の成功（200・JSON）を返す Fake の応答。 */
export function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8", ...headers },
  });
}

/** ヘッダ名を大文字小文字を区別せずに引く。 */
export function headerOf(
  headers: Readonly<Record<string, string>>,
  name: string,
): string | undefined {
  const found = Object.entries(headers).find(
    ([key]) => key.toLowerCase() === name.toLowerCase(),
  );
  return found?.[1];
}

/** `Authorization` の SignedHeaders（小文字・; 区切り）を配列で返す。 */
export function signedHeaderNames(authorization: string | undefined): string[] {
  const match = /SignedHeaders=([^,\s]+)/.exec(authorization ?? "");
  return match === null ? [] : match[1].split(";");
}

/**
 * 本物の lambda-client を、Fake の送信関数と ENVIRONMENT の資格情報で組み立てる。
 * 資格情報は loadCredentials（3-f・3-g・3-h の対象）を通して得る。
 */
export function testClient(options: {
  readonly send: SendFunction;
  readonly environment?: Environment;
  readonly timeoutMilliseconds?: number;
}) {
  const environment = options.environment ?? ENVIRONMENT;
  return createLambdaClient({
    environment,
    send: options.send,
    now: () => NOW,
    timeoutMilliseconds: options.timeoutMilliseconds ?? 1000,
    getCredentials: async () => loadCredentials(environment),
  });
}
