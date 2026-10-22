// docs/specs/bff-wiring.md の検査のうち、業務の処理（R-1〜R-3・R-5〜R-9）を表で
// 回すものが共有する補助。アプリの実装からは import しない（検査専用）。
import type { Environment } from "../lib/auth-config";
import type { PublicKeyResolver } from "../lib/jwt-verifier";
import type { LambdaClient } from "../lib/lambda-client";
import {
  handleApproveWorkMonth,
  handleCloseWorkMonth,
  handleDeleteDailyRecord,
  handleGetWorkMonth,
  handleListContracts,
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
  | "R-1" | "R-2" | "R-3" | "R-5" | "R-6" | "R-7" | "R-8" | "R-9" | "R-10" | "R-11";

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
