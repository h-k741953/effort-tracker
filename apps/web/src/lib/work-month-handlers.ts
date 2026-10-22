// 業務の Route Handler の中身（docs/specs/bff-wiring.md AC-5〜AC-8・AC-10-2）。
//
// - AC-10-2: 処理は src/lib/ 側に置き、src/app/api/ の Route Handler は薄い配線に
//   留める。lambda-client は呼ぶ側が宣言する最小のインターフェース越しに受ける
//   （AC-10-3）。
// - AC-5: 入口の順序は 1 レート制限 → 2 構成 → 3 セッション → 4 ロール →
//   5 ドメイン API の呼び出し。先に当たったところで終える。
// - AC-6-5・7-3: 業務ルールと書式を判定しない。本文は組み替えずに中継する。
// - AC-8-7: ブラウザへの写像。応答はすべて Cache-Control: no-store を付ける。
//   契約外の本文・ヘッダは写さない。
import type { Environment } from "./auth-config";
import { loadAuthConfig, loadRoleCookieSigningKey } from "./auth-config";
import { unauthenticatedResponse } from "./auth-handlers";
import type { PublicKeyResolver } from "./jwt-verifier";
import { createCognitoPublicKeyResolver } from "./jwt-verifier";
import type {
  Actor,
  DomainApiRequest,
  LambdaClient,
  LambdaClientResult,
} from "./lambda-client";
import { loadDomainApiUrl } from "./lambda-client";
import { withRequestRateLimit } from "./rate-limit";
import { ROLE_COOKIE_NAME, resolveEffectiveRole } from "./role-cookie";
import {
  SESSION_COOKIE_NAME,
  expiredAuthCookies,
  parseCookieHeader,
  resolveVerifiedSession,
} from "./session-cookie";

export interface BusinessOptions {
  /** セッション検証に用いる現在時刻（呼ぶ側から与える。終端仕様 AC-2-11）。 */
  readonly now?: Date;
  readonly environment?: Environment;
  /** 本物の lambda-client を渡すのは Route Handler の配線だけ（AC-10-3）。 */
  readonly lambdaClient: LambdaClient;
  readonly publicKeyResolver?: PublicKeyResolver;
}

export interface WorkMonthParams {
  readonly contractId: string;
  readonly yearMonth: string;
}

export interface DailyRecordParams extends WorkMonthParams {
  readonly date: string;
}

/**
 * ゲストの扱い（AC-5 の順3）。
 * - required: 更新系と R-2。検証済みセッションが無ければ 401（5-2・6-4）。
 * - optional: R-1・R-3。ゲストとして続ける（5-3）。
 */
type Access = "required" | "optional";

interface Route {
  readonly method: "GET" | "PUT" | "DELETE" | "POST";
  readonly access: Access;
  /** 検証済みセッションの sub（ゲストは null）と操作者から、呼ぶ要求を作る。 */
  readonly build: (context: {
    readonly actor: Actor;
    readonly request: Request;
  }) => Promise<DomainApiRequest> | DomainApiRequest;
}

const JSON_CONTENT_TYPE = "application/json; charset=utf-8";

/** 本文を持たない応答。ブラウザへは理由を載せない（AC-8-7 (iii)(iv)）。 */
function emptyResponse(status: number, headers: Headers): Response {
  headers.set("Cache-Control", "no-store");
  return new Response(null, { status, headers });
}

function jsonResponse(status: number, body: unknown, headers: Headers): Response {
  headers.set("Cache-Control", "no-store");
  headers.set("Content-Type", JSON_CONTENT_TYPE);
  return new Response(JSON.stringify(body), { status, headers });
}

/** 5xx のうちの値は実装側が持つ（AC-8-7 (iv)）。401・429 と同じ値にしない。 */
const UNEXPECTED_STATUS = 502;

/**
 * 構成の欠落（3-7・5-g）。契約外の拒否（AC-8-7 (iv)・7-1 補足）と同じ形
 * （ステータス・ヘッダ・本文）で返す。値・理由で区別できないようにする（AC-9・7-d (i)）。
 */
function misconfiguredBusinessResponse(): Response {
  return emptyResponse(UNEXPECTED_STATUS, new Headers());
}

function toBrowserResponse(result: LambdaClientResult, headers: Headers): Response {
  switch (result.kind) {
    case "success":
      return jsonResponse(result.status, result.body, headers);
    case "contract-error":
      // message を落とし、code だけを渡す（AC-8-7 (ii)）。
      return jsonResponse(result.status, { error: { code: result.code } }, headers);
    case "unexpected":
      if (result.rateLimited) {
        if (result.retryAfter !== undefined) {
          headers.set("Retry-After", result.retryAfter);
        }
        return emptyResponse(429, headers);
      }
      return emptyResponse(UNEXPECTED_STATUS, headers);
  }
}

async function handle(
  request: Request,
  options: BusinessOptions,
  route: Route,
): Promise<Response> {
  // 入口のレート制限（AC-5-1・5-6）。制限の窓は実時間で数えるため、現在時刻は
  // 呼ぶ側の now ではなく実時刻を用いる。
  return withRequestRateLimit(request.headers, new Date(), async () => {
    // 6-6: 状態を変える処理を想定外のメソッドで受けない。
    if (request.method !== route.method) {
      return emptyResponse(405, new Headers());
    }

    // 順2: 構成（呼び先と、当該経路が読む認証の構成）。
    const environment = options.environment ?? process.env;
    const config = loadAuthConfig(environment);
    if (!config.ok || !loadDomainApiUrl(environment).ok) {
      return misconfiguredBusinessResponse();
    }

    // 順3: セッションの検証。
    const cookieHeader = request.headers.get("cookie");
    const session = await resolveVerifiedSession({
      cookieHeader,
      now: options.now ?? new Date(),
      config: config.value,
      publicKeyResolver: options.publicKeyResolver ?? createCognitoPublicKeyResolver(),
    });

    const responseHeaders = new Headers();
    let actor: Actor = null;
    if (session.ok) {
      // 順4: ロールの確定。署名鍵が無ければ Engineer へ落ちず構成欠落（5-5）。
      const signingKey = loadRoleCookieSigningKey(environment);
      if (!signingKey.ok) {
        return misconfiguredBusinessResponse();
      }
      const roleCookie = parseCookieHeader(cookieHeader).get(ROLE_COOKIE_NAME);
      actor = {
        id: session.sub,
        role: resolveEffectiveRole({ cookieValue: roleCookie, signingKey: signingKey.value }),
      };
    } else if (route.access === "required") {
      return unauthenticatedResponse();
    } else if (parseCookieHeader(cookieHeader).has(SESSION_COOKIE_NAME)) {
      // 5-3: 検証に失敗したセッションは破棄しつつ、ゲストとして続ける。
      for (const cookie of expiredAuthCookies()) {
        responseHeaders.append("Set-Cookie", cookie);
      }
    }

    // 順5: ドメイン API の呼び出し。
    const result = await options.lambdaClient.call(await route.build({ actor, request }));
    return toBrowserResponse(result, responseHeaders);
  });
}

/** R-1: 勤務月の取得（E-1）。ゲストは操作者なしで呼ぶ。 */
export function handleGetWorkMonth(
  request: Request,
  params: WorkMonthParams,
  options: BusinessOptions,
): Promise<Response> {
  return handle(request, options, {
    method: "GET",
    access: "optional",
    build: ({ actor }) => ({ endpoint: "E-1", ...params, actor }),
  });
}

/** R-5: 稼働実績の入力・編集（E-3）。本文は解釈せずそのまま送る（6-8）。 */
export function handlePutDailyRecord(
  request: Request,
  params: DailyRecordParams,
  options: BusinessOptions,
): Promise<Response> {
  return handle(request, options, {
    method: "PUT",
    access: "required",
    build: async ({ actor, request: incoming }) => ({
      endpoint: "E-3",
      contractId: params.contractId,
      yearMonth: params.yearMonth,
      date: params.date,
      body: new Uint8Array(await incoming.arrayBuffer()),
      actor,
    }),
  });
}

/** R-6: 稼働実績の削除（E-4）。 */
export function handleDeleteDailyRecord(
  request: Request,
  params: DailyRecordParams,
  options: BusinessOptions,
): Promise<Response> {
  return handle(request, options, {
    method: "DELETE",
    access: "required",
    build: ({ actor }) => ({
      endpoint: "E-4",
      contractId: params.contractId,
      yearMonth: params.yearMonth,
      date: params.date,
      actor,
    }),
  });
}

function stateChange(endpoint: "E-5" | "E-6" | "E-7") {
  return (
    request: Request,
    params: WorkMonthParams,
    options: BusinessOptions,
  ): Promise<Response> =>
    handle(request, options, {
      method: "POST",
      access: "required",
      build: ({ actor }) => ({
        endpoint,
        contractId: params.contractId,
        yearMonth: params.yearMonth,
        actor,
      }),
    });
}

/** R-7: 締め（E-5）。 */
export const handleCloseWorkMonth = stateChange("E-5");

/** R-8: 承認（E-6）。 */
export const handleApproveWorkMonth = stateChange("E-6");

/** R-9: 差戻し（E-7）。 */
export const handleRejectWorkMonth = stateChange("E-7");

function listParameters(request: Request) {
  const params = new URL(request.url).searchParams;
  return {
    state: params.get("state") ?? undefined,
    limit: params.get("limit") ?? undefined,
    offset: params.get("offset") ?? undefined,
  };
}

/**
 * R-2: 本人の勤務月の一覧（E-2）。engineerId は検証済みセッションの sub だけ
 * （6-4）。ブラウザの engineerId は読まない。
 */
export function handleListOwnWorkMonths(
  request: Request,
  options: BusinessOptions,
): Promise<Response> {
  return handle(request, options, {
    method: "GET",
    access: "required",
    build: ({ actor, request: incoming }) => ({
      endpoint: "E-2",
      query: { engineerId: actor?.id, ...listParameters(incoming) },
      actor,
    }),
  });
}

/**
 * R-3: 承認待ち一覧（E-2）。engineerId を送らず、state は PendingApproval に
 * 固定する（6-c）。ゲストは操作者なしで呼び、契約の 401 を写す（5-3）。
 */
export function handleListPendingApprovals(
  request: Request,
  options: BusinessOptions,
): Promise<Response> {
  return handle(request, options, {
    method: "GET",
    access: "optional",
    build: ({ actor, request: incoming }) => ({
      endpoint: "E-2",
      query: { ...listParameters(incoming), state: "PendingApproval" },
      actor,
    }),
  });
}

/**
 * ログインしている本人の `sub` を取り出す。R-2・R-10・R-11 は access が
 * required で、検証済みセッションが無ければ build の前に 401 で終わる（6-4）。
 */
function sessionSub(actor: Actor): string {
  if (actor === null) {
    throw new Error("a verified session is required");
  }
  return actor.id;
}

/**
 * R-10: 契約一覧（E-8）。engineerId は検証済みセッションの sub だけ（6-4）。
 * yearMonth は要求のクエリの値をそのまま渡し、無ければ補完も拒否もしない
 * （6-9・7-3）。ロールで先回りして弾かない（6-10）。
 */
export function handleListContracts(
  request: Request,
  options: BusinessOptions,
): Promise<Response> {
  return handle(request, options, {
    method: "GET",
    access: "required",
    build: ({ actor, request: incoming }) => ({
      endpoint: "E-8",
      query: {
        engineerId: sessionSub(actor),
        yearMonth: new URL(incoming.url).searchParams.get("yearMonth") ?? undefined,
      },
      actor,
    }),
  });
}

/**
 * R-11: 稼働の推移（E-9）。engineerId は検証済みセッションの sub だけで、
 * 他のクエリを送らない（6-4）。ロールで先回りして弾かない（6-10）。
 */
export function handleListWorkTrends(
  request: Request,
  options: BusinessOptions,
): Promise<Response> {
  return handle(request, options, {
    method: "GET",
    access: "required",
    build: ({ actor }) => ({
      endpoint: "E-9",
      query: { engineerId: sessionSub(actor) },
      actor,
    }),
  });
}
