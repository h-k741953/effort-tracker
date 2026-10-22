// ドメイン API（Lambda Function URL）を呼ぶ唯一の集約点
// （docs/specs/bff-wiring.md AC-1-1・P-1）。
//
// - AC-2: SigV4 で署名する。署名は @smithy/signature-v4、ハッシュは
//   @aws-crypto/sha256-js に任せ、署名の計算を自前で実装しない（D 節 U-1・U-1d）。
// - AC-3: 呼び先は環境変数の Function URL だけ。資格情報は Lambda ランタイムが
//   実行ロールから与える環境変数だけから読む（D 節 U-1b）。読む対象の環境変数の
//   集まりは引数で受ける。欠落・セッショントークンなしは失敗として扱い、送信しない。
// - AC-8: 結果を契約の成功／契約のエラー／契約外に分ける。例外を外へ漏らさない。
//   自動の再試行をしない。時間切れを持つ。
// - 契約（エンドポイント・code とステータスの組）の正解は
//   docs/specs/domain-api-http-contract.md。本ファイルは書き写さず、AC-9 の表を
//   判定に要る分だけ持つ（bff-wiring.md 10-7）。
import { Sha256 } from "@aws-crypto/sha256-js";
import { SignatureV4 } from "@smithy/signature-v4";
import type { ConfigResult, Environment } from "./auth-config";

/** Function URL を受け取る環境変数の名。Terraform の注入と揃える（12-38）。 */
const DOMAIN_API_URL_VAR = "DOMAIN_API_FUNCTION_URL";
/** 署名のリージョン。名前の持ち主は Lambda ランタイム（D 節）。 */
const REGION_VAR = "AWS_REGION";
const ACCESS_KEY_ID_VAR = "AWS_ACCESS_KEY_ID";
const SECRET_ACCESS_KEY_VAR = "AWS_SECRET_ACCESS_KEY";
const SESSION_TOKEN_VAR = "AWS_SESSION_TOKEN";

/** Function URL のサービス名（2-1）。 */
const SIGNING_SERVICE = "lambda";

/**
 * 本番の時間切れ（8-8・D 節 U-5b）。BFF・SSR Lambda 自身の時間切れより先に来る
 * 趣旨の値。その関係は機械検査されない（限界 13-8）。
 */
const DEFAULT_TIMEOUT_MILLISECONDS = 2000;

export type ActorRole = "Engineer" | "Approver";

/** 操作者。識別子とロールの組、またはなし（ゲスト）。片方だけを表せない（4-4）。 */
export type Actor = { readonly id: string; readonly role: ActorRole } | null;

export interface ListQuery {
  readonly engineerId?: string;
  readonly state?: string;
  readonly limit?: string;
  readonly offset?: string;
}

interface WorkMonthKey {
  readonly contractId: string;
  readonly yearMonth: string;
  readonly actor: Actor;
}

/** 契約のエンドポイント E-1〜E-9 を表す値（1-5: 呼び先のオリジンは持たない）。 */
export type DomainApiRequest =
  | ({ readonly endpoint: "E-1" } & WorkMonthKey)
  | { readonly endpoint: "E-2"; readonly query: ListQuery; readonly actor: Actor }
  | ({
      readonly endpoint: "E-3";
      readonly date: string;
      readonly body: Uint8Array;
    } & WorkMonthKey)
  | ({ readonly endpoint: "E-4"; readonly date: string } & WorkMonthKey)
  | ({ readonly endpoint: "E-5" } & WorkMonthKey)
  | ({ readonly endpoint: "E-6" } & WorkMonthKey)
  | ({ readonly endpoint: "E-7" } & WorkMonthKey)
  | {
      readonly endpoint: "E-8";
      readonly query: { readonly engineerId: string; readonly yearMonth?: string };
      readonly actor: Actor;
    }
  | {
      readonly endpoint: "E-9";
      readonly query: { readonly engineerId: string };
      readonly actor: Actor;
    };

/** AC-8-1 の3分類。 */
export type LambdaClientResult =
  | { readonly kind: "success"; readonly status: number; readonly body: unknown }
  | {
      readonly kind: "contract-error";
      readonly status: number;
      readonly code: string;
    }
  | {
      readonly kind: "unexpected";
      /** 上流のステータスが 429 であったか（8-1・8-7 (iii)）。 */
      readonly rateLimited: boolean;
      /** 上流が返した Retry-After（429 のときだけ）。 */
      readonly retryAfter?: string;
    };

export interface LambdaClient {
  call(request: DomainApiRequest): Promise<LambdaClientResult>;
}

/** 送信関数（fetch 相当）の最小のインターフェース（3-5）。 */
export type SendFunction = (
  url: string,
  init: {
    readonly method: string;
    readonly headers: Readonly<Record<string, string>>;
    readonly body?: Uint8Array;
    readonly signal?: AbortSignal;
  },
) => Promise<Response>;

export interface Credentials {
  readonly accessKeyId: string;
  readonly secretAccessKey: string;
  readonly sessionToken?: string;
}

export type CredentialsResult =
  | { readonly ok: true; readonly value: Required<Credentials> }
  | { readonly ok: false; readonly missing: readonly string[] };

function read(environment: Environment, name: string): string | undefined {
  const value = environment[name];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

/**
 * 呼び先の Function URL を読む（3-1・3-2）。未設定・空は失敗。要求から導かない。
 * 失敗の結果に載せるのは変数の名前だけ。
 */
export function loadDomainApiUrl(
  environment: Environment = process.env,
): ConfigResult<string> {
  const value = read(environment, DOMAIN_API_URL_VAR);
  if (value === undefined) {
    return { ok: false, missing: [DOMAIN_API_URL_VAR] };
  }
  return { ok: true, value };
}

/**
 * 実行ロールの一時資格情報を環境変数から読む（3-3）。アクセスキー ID・
 * シークレットアクセスキー・セッショントークンのいずれかが未設定・空なら失敗
 * （3-4・3-6）。失敗の結果に値は載せない。
 */
export function loadCredentials(
  environment: Environment = process.env,
): CredentialsResult {
  const accessKeyId = read(environment, ACCESS_KEY_ID_VAR);
  const secretAccessKey = read(environment, SECRET_ACCESS_KEY_VAR);
  const sessionToken = read(environment, SESSION_TOKEN_VAR);

  const missing: string[] = [];
  if (accessKeyId === undefined) missing.push(ACCESS_KEY_ID_VAR);
  if (secretAccessKey === undefined) missing.push(SECRET_ACCESS_KEY_VAR);
  if (sessionToken === undefined) missing.push(SESSION_TOKEN_VAR);
  if (
    accessKeyId === undefined ||
    secretAccessKey === undefined ||
    sessionToken === undefined
  ) {
    return { ok: false, missing };
  }
  return { ok: true, value: { accessKeyId, secretAccessKey, sessionToken } };
}

export interface SignRequestParams {
  readonly service: string;
  readonly region: string;
  readonly credentials: Credentials;
  /** 署名に用いる時刻は呼ぶ側から与える（2-5）。 */
  readonly now: Date;
  readonly request: {
    readonly method: string;
    readonly url: string;
    readonly headers: Readonly<Record<string, string>>;
    readonly body?: Uint8Array;
  };
}

/**
 * 送信を伴わない署名（2-7）。署名した結果の**ヘッダ一式**（host・x-amz-date・
 * セッショントークン・authorization を含む）を返す。署名した後に書き換えない（2-4）。
 */
export async function signRequest(
  params: SignRequestParams,
): Promise<Record<string, string>> {
  const { service, region, credentials, now, request } = params;
  const url = new URL(request.url);

  const query: Record<string, string> = {};
  for (const [name, value] of url.searchParams) {
    query[name] = value;
  }

  const signer = new SignatureV4({
    service,
    region,
    credentials,
    sha256: Sha256,
    // x-amz-content-sha256 を署名対象に足さない（SignedHeaders を要求の
    // ヘッダだけにする）。本文のハッシュは正規リクエストに入る（2-3）。
    applyChecksum: false,
  });

  const signed = await signer.sign(
    {
      method: request.method,
      protocol: url.protocol,
      hostname: url.hostname,
      port: url.port === "" ? undefined : Number(url.port),
      path: url.pathname,
      query,
      headers: { ...request.headers, host: url.host },
      body: request.body,
    },
    { signingDate: now },
  );

  return { ...signed.headers };
}

/** RFC 3986 のクエリ・パスの符号化（予約文字を残さない）。 */
function encodeStrict(value: string): string {
  return encodeURIComponent(value).replace(
    /[!'()*]/g,
    (character) => `%${character.charCodeAt(0).toString(16).toUpperCase()}`,
  );
}

/**
 * パスの1セグメントとして符号化する（7-1）。値そのものが `.` `..` `""` のものは送れない。
 * `.` `..` は、符号化しても WHATWG URL がドットセグメントとして畳むため。
 * `""` は、空のセグメント（`//`）になり、受け側が畳みうるうえ署名の正規パスと食い違いうるため（理由が異なる）。
 * 呼び出し側（call の catch）が契約外として返す。理由・値は載せない（AC-9）。
 * 対象はこの3値だけで、それ以外（空白だけの値を含む）は書式を判定せず符号化して送る（7-3）。
 */
function encodeSegment(value: string): string {
  if (value === "." || value === ".." || value === "") {
    throw new Error("rejected segment");
  }
  return encodeStrict(value);
}

interface Target {
  readonly method: string;
  readonly path: string;
  readonly query: string;
  readonly body?: Uint8Array;
  readonly actor: Actor;
}

/** 値が undefined のものを除き、与えた順にクエリ文字列へ符号化する（7-2）。 */
function queryString(entries: ReadonlyArray<readonly [string, string | undefined]>): string {
  const pairs: string[] = [];
  for (const [name, value] of entries) {
    if (value !== undefined) {
      pairs.push(`${name}=${encodeStrict(value)}`);
    }
  }
  return pairs.length === 0 ? "" : `?${pairs.join("&")}`;
}

/** 契約のエンドポイント一覧（E-1〜E-9）に従ってメソッドとパスを決める（6-2）。 */
function targetOf(request: DomainApiRequest): Target {
  if (request.endpoint === "E-2") {
    const { engineerId, state, limit, offset } = request.query;
    return {
      method: "GET",
      path: "/work-months",
      query: queryString([
        ["engineerId", engineerId],
        ["state", state],
        ["limit", limit],
        ["offset", offset],
      ]),
      actor: request.actor,
    };
  }
  if (request.endpoint === "E-8") {
    return {
      method: "GET",
      path: "/contracts",
      query: queryString([
        ["engineerId", request.query.engineerId],
        ["yearMonth", request.query.yearMonth],
      ]),
      actor: request.actor,
    };
  }
  if (request.endpoint === "E-9") {
    return {
      method: "GET",
      path: "/work-trends",
      query: queryString([["engineerId", request.query.engineerId]]),
      actor: request.actor,
    };
  }

  const month = `/work-months/${encodeSegment(request.contractId)}/${encodeSegment(request.yearMonth)}`;
  switch (request.endpoint) {
    case "E-1":
      return { method: "GET", path: month, query: "", actor: request.actor };
    case "E-3":
      return {
        method: "PUT",
        path: `${month}/daily-records/${encodeSegment(request.date)}`,
        query: "",
        body: request.body,
        actor: request.actor,
      };
    case "E-4":
      return {
        method: "DELETE",
        path: `${month}/daily-records/${encodeSegment(request.date)}`,
        query: "",
        actor: request.actor,
      };
    case "E-5":
      return { method: "POST", path: `${month}/close`, query: "", actor: request.actor };
    case "E-6":
      return { method: "POST", path: `${month}/approve`, query: "", actor: request.actor };
    case "E-7":
      return { method: "POST", path: `${month}/reject`, query: "", actor: request.actor };
  }
}

/**
 * 契約のエラーのステータスと code の組（契約 AC-9 の表。出所はそちら）。
 * 組が食い違う応答は契約外として扱う（8-1 (iii)・8-2）。
 */
const CONTRACT_ERROR_STATUS: Readonly<Record<string, number>> = {
  INVALID_REQUEST: 400,
  TIME_OF_DAY_OUT_OF_RANGE: 400,
  INVALID_TIME_COMBINATION: 400,
  FUTURE_DATE_NOT_ALLOWED: 400,
  DATE_OUT_OF_WORK_MONTH: 400,
  UNAUTHENTICATED: 401,
  FORBIDDEN_NOT_OWNER: 403,
  FORBIDDEN_NOT_APPROVER: 403,
  FORBIDDEN_SELF_APPROVAL: 403,
  CONTRACT_NOT_FOUND: 404,
  WORK_MONTH_NOT_FOUND: 404,
  NOT_FOUND: 404,
  WORK_MONTH_NOT_EDITABLE: 409,
  INVALID_STATE_FOR_CLOSE: 409,
  INVALID_STATE_FOR_APPROVE: 409,
  INVALID_STATE_FOR_REJECT: 409,
  INTERNAL_ERROR: 500,
};

const UNEXPECTED: LambdaClientResult = { kind: "unexpected", rateLimited: false };

function classify(status: number, text: string, retryAfter: string | null): LambdaClientResult {
  if (status === 429) {
    return retryAfter === null
      ? { kind: "unexpected", rateLimited: true }
      : { kind: "unexpected", rateLimited: true, retryAfter };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return UNEXPECTED;
  }

  if (status >= 200 && status < 300) {
    return { kind: "success", status, body: parsed };
  }

  const code = (parsed as { error?: { code?: unknown } } | null)?.error?.code;
  if (
    typeof code === "string" &&
    Object.hasOwn(CONTRACT_ERROR_STATUS, code) &&
    CONTRACT_ERROR_STATUS[code] === status
  ) {
    return { kind: "contract-error", status, code };
  }
  return UNEXPECTED;
}

export interface CreateLambdaClientOptions {
  /** 呼び先・リージョンを読む環境変数の集まり（3-5）。 */
  readonly environment?: Environment;
  readonly send?: SendFunction;
  readonly now?: () => Date;
  /** 時間切れ（8-8）。呼ぶ側から与えられる形。 */
  readonly timeoutMilliseconds?: number;
  readonly getCredentials?: () => Promise<CredentialsResult>;
}

const defaultSend: SendFunction = (url, init) =>
  fetch(url, {
    method: init.method,
    headers: { ...init.headers },
    body: init.body === undefined ? undefined : Buffer.from(init.body),
    signal: init.signal,
    redirect: "manual",
  });

/**
 * lambda-client を作る。呼び先と資格情報が得られない・署名に失敗したときは
 * 送信せず契約外として返す（2-6・3-7）。自動の再試行をしない（8-6）。
 */
export function createLambdaClient(
  options: CreateLambdaClientOptions = {},
): LambdaClient {
  const environment = options.environment ?? process.env;
  const send = options.send ?? defaultSend;
  const now = options.now ?? (() => new Date());
  const timeoutMilliseconds = options.timeoutMilliseconds ?? DEFAULT_TIMEOUT_MILLISECONDS;
  const getCredentials =
    options.getCredentials ?? (async () => loadCredentials(environment));

  async function exchange(request: DomainApiRequest): Promise<LambdaClientResult> {
    const baseUrl = loadDomainApiUrl(environment);
    const region = read(environment, REGION_VAR);
    if (!baseUrl.ok || region === undefined) {
      return UNEXPECTED;
    }
    const credentials = await getCredentials();
    if (!credentials.ok) {
      return UNEXPECTED;
    }

    const target = targetOf(request);
    const url = `${new URL(baseUrl.value).origin}${target.path}${target.query}`;

    // ヘッダは一から組み立てる（4-6）。ブラウザ要求のヘッダを渡さない。
    const headers: Record<string, string> = {};
    if (target.body !== undefined) {
      headers["content-type"] = "application/json";
    }
    if (target.actor !== null) {
      headers["x-actor-id"] = target.actor.id;
      headers["x-actor-role"] = target.actor.role;
    }

    const signedHeaders = await signRequest({
      service: SIGNING_SERVICE,
      region,
      credentials: credentials.value,
      now: now(),
      request: { method: target.method, url, headers, body: target.body },
    });

    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const timedOut = new Promise<LambdaClientResult>((resolve) => {
      timer = setTimeout(() => {
        controller.abort();
        resolve(UNEXPECTED);
      }, timeoutMilliseconds);
    });

    const attempt = (async (): Promise<LambdaClientResult> => {
      const response = await send(url, {
        method: target.method,
        headers: signedHeaders,
        body: target.body,
        signal: controller.signal,
      });
      const text = await response.text();
      return classify(response.status, text, response.headers.get("retry-after"));
    })();
    // 時間切れの後に失敗しても未処理の拒否にしない。
    attempt.catch(() => undefined);

    try {
      return await Promise.race([attempt, timedOut]);
    } finally {
      clearTimeout(timer);
    }
  }

  return {
    async call(request) {
      try {
        return await exchange(request);
      } catch {
        // 8-3: 例外を漏らさない。詳細は載せない（AC-9）。
        return UNEXPECTED;
      }
    },
  };
}
