// docs/specs/bff-wiring.md の検査のうち、業務の処理（R-1〜R-3・R-5〜R-9）を表で
// 回すものが共有する補助。アプリの実装からは import しない（検査専用）。
import type { Environment } from "../lib/auth-config";
import type { PublicKeyResolver } from "../lib/jwt-verifier";
import type { DomainApiRequest, LambdaClient, LambdaClientResult } from "../lib/lambda-client";
import {
  handleApproveWorkMonth,
  handleCloseWorkMonth,
  handleDeleteDailyRecord,
  handleGetWorkMonth,
  handleListContracts,
  handleListDemoWorkMonths,
  handleListOwnWorkMonths,
  handleListPendingApprovals,
  handleListWorkTrends,
  handlePutDailyRecord,
  handleRejectWorkMonth,
} from "../lib/work-month-handlers";
import {
  ENVIRONMENT,
  NOW,
  PATH_PARAMS,
  browserRequest,
  fakePublicKeySource,
  type BrowserRequestOptions,
} from "./bff-wiring";

/** 受け取った要求を記録し、与えた結果を返す lambda-client の Fake。 */
export function fakeClient(
  result: LambdaClientResult = { kind: "success", status: 200, body: {} },
): { readonly client: LambdaClient; readonly calls: DomainApiRequest[] } {
  const calls: DomainApiRequest[] = [];
  const client: LambdaClient = {
    async call(request) {
      calls.push(request);
      return result;
    },
  };
  return { client, calls };
}

// --- 業務の処理の表（R-1〜R-3・R-5〜R-9） -----------------------------------

/**
 * 処理へ渡す options。名前の根拠 —— auth-handlers.ts の `options`
 * （now / environment / publicKeyResolver）に揃え、lambda-client を呼ぶ側が
 * 宣言する最小のインターフェース越しに受ける（AC-10-3）。
 */
export interface BusinessOptions {
  readonly now: Date;
  readonly environment: Environment;
  readonly lambdaClient: LambdaClient;
  readonly publicKeyResolver: PublicKeyResolver;
}

export function businessOptions(
  lambdaClient: LambdaClient,
  environment: Environment = ENVIRONMENT,
): BusinessOptions {
  return {
    now: NOW,
    environment,
    lambdaClient,
    publicKeyResolver: fakePublicKeySource,
  };
}

export type RouteId =
  | "R-1" | "R-2" | "R-3" | "R-4" | "R-5" | "R-6" | "R-7" | "R-8" | "R-9" | "R-10" | "R-11";

export interface BusinessHandlerCase {
  readonly route: RouteId;
  /** ブラウザが送るメソッド（AC-6-7: 契約の当該 E と同じ。R-2・R-3・R-10・R-11 は GET）。 */
  readonly method: "GET" | "PUT" | "DELETE" | "POST";
  /** 呼ぶ契約のエンドポイント（AC-6）。 */
  readonly endpoint: "E-1" | "E-2" | "E-3" | "E-4" | "E-5" | "E-6" | "E-7" | "E-8" | "E-9";
  readonly updates: boolean;
  readonly invoke: (
    request: Request,
    options: BusinessOptions,
  ) => Promise<Response>;
}

/**
 * R-1〜R-3・R-5〜R-11 の 10 個（**R-4 を除く**）。ロールを使う処理の表で、5-h など
 * R-4 を対象としない検査が回す（bff-wiring.md AC-5 の 5-h。2026-10-27 人間決定、#56）。
 * R-4 を含む 11 は {@link ALL_BUSINESS_HANDLERS}。
 */
export const BUSINESS_HANDLERS: readonly BusinessHandlerCase[] = [
  {
    route: "R-1",
    method: "GET",
    endpoint: "E-1",
    updates: false,
    invoke: (request, options) => handleGetWorkMonth(request, PATH_PARAMS, options),
  },
  {
    route: "R-2",
    method: "GET",
    endpoint: "E-2",
    updates: false,
    invoke: (request, options) => handleListOwnWorkMonths(request, options),
  },
  {
    route: "R-3",
    method: "GET",
    endpoint: "E-2",
    updates: false,
    invoke: (request, options) => handleListPendingApprovals(request, options),
  },
  {
    route: "R-5",
    method: "PUT",
    endpoint: "E-3",
    updates: true,
    invoke: (request, options) => handlePutDailyRecord(request, PATH_PARAMS, options),
  },
  {
    route: "R-6",
    method: "DELETE",
    endpoint: "E-4",
    updates: true,
    invoke: (request, options) => handleDeleteDailyRecord(request, PATH_PARAMS, options),
  },
  {
    route: "R-7",
    method: "POST",
    endpoint: "E-5",
    updates: true,
    invoke: (request, options) => handleCloseWorkMonth(request, PATH_PARAMS, options),
  },
  {
    route: "R-8",
    method: "POST",
    endpoint: "E-6",
    updates: true,
    invoke: (request, options) => handleApproveWorkMonth(request, PATH_PARAMS, options),
  },
  {
    route: "R-9",
    method: "POST",
    endpoint: "E-7",
    updates: true,
    invoke: (request, options) => handleRejectWorkMonth(request, PATH_PARAMS, options),
  },
  // R-10・R-11（2026-10-21 追加。D 節 #109）。処理関数名は D 節「#109（名前・置き場・パス）」(i)。
  {
    route: "R-10",
    method: "GET",
    endpoint: "E-8",
    updates: false,
    invoke: (request, options) => handleListContracts(request, options),
  },
  {
    route: "R-11",
    method: "GET",
    endpoint: "E-9",
    updates: false,
    invoke: (request, options) => handleListWorkTrends(request, options),
  },
];

/** R-4 を呼ぶとき環境変数に与えるデモの技術者識別子（値はダミー。demo-data-reset.md AC-7-1・7-3）。 */
export const DEMO_ENGINEER_ID = "demo-engineer-dummy-0001";

/**
 * R-4（ゲスト向けデモ勤務月群。E-2）。呼び先の環境変数の検査（5-g）でデモの技術者識別子の
 * 欠落と混ざらないよう、invoke がデモの技術者識別子を与えたうえで呼ぶ
 * （bff-wiring.md 5-g。与え方は仕様工程の定め）。
 */
export const DEMO_HANDLER: BusinessHandlerCase = {
  route: "R-4",
  method: "GET",
  endpoint: "E-2",
  updates: false,
  invoke: (request, options) =>
    handleListDemoWorkMonths(request, {
      ...options,
      environment: { ...options.environment, DEMO_ENGINEER_ID },
    }),
};

/**
 * R-4 を含む R-1〜R-11 の 11 個（bff-wiring.md AC-5 の 5-6・5-a・5-b・5-g の母集団。
 * 2026-10-27 改定、#56）。
 */
export const ALL_BUSINESS_HANDLERS: readonly BusinessHandlerCase[] = [
  ...BUSINESS_HANDLERS.slice(0, 3),
  DEMO_HANDLER,
  ...BUSINESS_HANDLERS.slice(3),
];

/** R-5 の本文（契約 AC-4 の形。値の正否は問わない）。 */
export const PUT_BODY = JSON.stringify({
  startTime: { hour: 9, minute: 0 },
  endTime: { hour: 17, minute: 0 },
  break: { start: { hour: 12, minute: 0 }, end: { hour: 13, minute: 0 } },
});

/** 当該の処理を呼ぶ既定の要求（本文は R-5 だけが持つ）。 */
export function requestFor(
  handler: BusinessHandlerCase,
  options: BrowserRequestOptions = {},
): Request {
  return browserRequest(handler.method, {
    body: handler.route === "R-5" ? PUT_BODY : undefined,
    ...options,
  });
}
