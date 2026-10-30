// docs/specs/web-screens-implementation-design.md T-0・T-3。画面3（勤務月画面）のテストが共有する補助。
//
// 手書きの Fake だけを置く（モックライブラリを使わない。T-0-1）。ブラウザの大域の fetch を
// vi.stubGlobal で差し替える。使うテストが afterEach で vi.unstubAllGlobals() を呼ぶこと。
// 勤務月の表現のキー名は domain-api-http-contract.md AC-10-1 に従う。
// 本人かの真偽のキー名の字面はここにも書かず、OPERATOR_OWNERSHIP_KEY を輸入して使う（M-3）。

import { vi } from "vitest";
import { OPERATOR_OWNERSHIP_KEY } from "@/lib/work-month";

export type TestWorkMonthState = "Draft" | "PendingApproval" | "Approved";

export interface TestHm {
  hours: number;
  minutes: number;
}

export interface TestTime {
  hour: number;
  minute: number;
}

export interface TestDailyRecord {
  date: string;
  startTime: TestTime;
  endTime: TestTime;
  break: { start: TestTime; end: TestTime } | null;
  workingHours: TestHm;
  roundedWorkingHours: TestHm;
}

export const t = (hour: number, minute: number): TestTime => ({ hour, minute });
export const hm = (hours: number, minutes: number): TestHm => ({ hours, minutes });

/** 稼働実績1日分。稼働時間（workingHours・roundedWorkingHours）は呼び出し側が与える（Fake の応答の値）。 */
export function dailyRecord(
  date: string,
  start: TestTime,
  end: TestTime,
  brk: { start: TestTime; end: TestTime } | null,
  working: TestHm,
): TestDailyRecord {
  return { date, startTime: start, endTime: end, break: brk, workingHours: working, roundedWorkingHours: working };
}

export interface WorkMonthBodyOptions {
  contractId?: string;
  contractDisplayName?: string;
  yearMonth?: string;
  state?: TestWorkMonthState;
  generated?: boolean;
  settlementRange?: { lowerBound: TestHm; upperBound: TestHm };
  totalHours?: TestHm;
  excess?: TestHm | null;
  shortfall?: TestHm | null;
  dailyRecords?: TestDailyRecord[];
  /** 本人かの真偽。undefined のときはキーを持たない本文にする（M-3 の「真偽が無い」）。 */
  ownership?: boolean;
}

export const JULY_RECORDS: TestDailyRecord[] = [
  dailyRecord("2026-07-01", t(9, 0), t(18, 0), { start: t(12, 0), end: t(13, 0) }, hm(8, 0)),
  dailyRecord("2026-07-02", t(9, 0), t(17, 45), null, hm(8, 45)),
];

/** 勤務月の表現（AC-10-1）。既定は 2026-07・Draft・生成済み・稼働実績2件・総稼働時間 16時間45分。 */
export function workMonthBody(options: WorkMonthBodyOptions = {}): Record<string, unknown> {
  const state = options.state ?? "Draft";
  const closed = state !== "Draft";
  const body: Record<string, unknown> = {
    contractId: options.contractId ?? "c-001",
    contractDisplayName: options.contractDisplayName ?? "A社 / 保守",
    yearMonth: options.yearMonth ?? "2026-07",
    state,
    generated: options.generated ?? true,
    settlementRange: options.settlementRange ?? { lowerBound: hm(140, 0), upperBound: hm(180, 0) },
    totalHours: options.totalHours ?? hm(16, 45),
    excess: options.excess !== undefined ? options.excess : closed ? hm(0, 0) : null,
    shortfall: options.shortfall !== undefined ? options.shortfall : closed ? hm(123, 15) : null,
    dailyRecords: options.dailyRecords ?? JULY_RECORDS,
  };
  if (options.ownership !== undefined) {
    body[OPERATOR_OWNERSHIP_KEY] = options.ownership;
  }
  return body;
}

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** 契約の失敗の本文 {"error":{"code":...}}。 */
export function errorBody(code: string) {
  return { error: { code } };
}

export const CONTRACTS_PATH = "/api/contracts";

export function workMonthApi(contractId: string, yearMonth: string): string {
  return `/api/work-months/${contractId}/${yearMonth}`;
}

export interface RecordedRequest {
  method: string;
  pathname: string;
  body: unknown;
}

export type RoutedHandler = (request: RecordedRequest) => Response | Promise<Response>;

export interface RoutedFetch {
  /** 受けた要求（時系列）。本文は JSON として読めるものは値、無ければ undefined。 */
  requests: RecordedRequest[];
  /** 登録の無い要求（"METHOD /path"）。0件であることを各テストの後で読む。 */
  unhandled: string[];
  count(method: string, pathname: string): number;
  /** メソッドが GET 以外の要求の数。 */
  nonGetCount(): number;
}

/** "METHOD /path" ごとの応答を返す fetch を大域へ置く。未登録の要求は記録して例外にする。 */
export function installRoutedFetch(routes: Record<string, RoutedHandler>): RoutedFetch {
  const fake: RoutedFetch = {
    requests: [],
    unhandled: [],
    count: (method, pathname) => fake.requests.filter((r) => r.method === method && r.pathname === pathname).length,
    nonGetCount: () => fake.requests.filter((r) => r.method !== "GET").length,
  };
  vi.stubGlobal("fetch", async (input: unknown, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost");
    const method = (init?.method ?? "GET").toUpperCase();
    let body: unknown = undefined;
    if (typeof init?.body === "string") {
      try {
        body = JSON.parse(init.body);
      } catch {
        body = init.body;
      }
    }
    const request: RecordedRequest = { method, pathname: url.pathname, body };
    fake.requests.push(request);
    const handler = routes[`${method} ${url.pathname}`];
    if (handler === undefined) {
      fake.unhandled.push(`${method} ${url.pathname}`);
      throw new Error(`unexpected request: ${method} ${url.pathname}`);
    }
    return handler(request);
  });
  return fake;
}

/** 解決を呼び出し側が握る応答。保留した応答を後から返すために使う。 */
export function deferredResponse(): { promise: Promise<Response>; resolve: (response: Response) => void } {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}
