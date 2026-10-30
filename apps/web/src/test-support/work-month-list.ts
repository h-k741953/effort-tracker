// docs/specs/web-screens-implementation-design.md T-2。画面2（勤務月一覧）のテストが共有する補助。
//
// 手書きの Fake だけを置く（モックライブラリを使わない。T-0-1）。ブラウザの大域の fetch を
// vi.stubGlobal で差し替える。使うテストが afterEach で vi.unstubAllGlobals() を呼ぶこと。
// 行・契約の形は domain-api-http-contract.md AC-10-2（一覧の行）・AC-10-4（契約一覧）の
// キー名（contractId / contractDisplayName / yearMonth / state）に従う。

import { vi } from "vitest";

export type TestState = "Draft" | "PendingApproval" | "Approved";

export interface TestRow {
  contractId: string;
  contractDisplayName: string;
  yearMonth: string;
  state: TestState;
}

export interface TestContract {
  contractId: string;
  contractDisplayName: string;
}

export const CONTRACT_A: TestContract = { contractId: "c-1", contractDisplayName: "A社 / 保守" };
export const CONTRACT_B: TestContract = { contractId: "c-2", contractDisplayName: "B社 / 開発" };
export const CONTRACT_C: TestContract = { contractId: "c-3", contractDisplayName: "C社 / 運用" };

export function row(contract: TestContract, yearMonth: string, state: TestState): TestRow {
  return { ...contract, yearMonth, state };
}

/** 対象年月の降順（同じ年月は契約識別子の昇順）に並んだ、3つの状態を含む行。 */
export const OWN_ROWS: TestRow[] = [
  row(CONTRACT_A, "2026-10", "PendingApproval"),
  row(CONTRACT_B, "2026-10", "Draft"),
  row(CONTRACT_A, "2026-09", "Approved"),
  row(CONTRACT_A, "2026-08", "Approved"),
];

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** E-2 の 200 応答（AC-10-2）。 */
export function listBody(items: TestRow[], total: number, offset: number) {
  return { items, total, limit: items.length, offset };
}

/** offset クエリに従って rows を chunk 件ずつ返す、ページングする一覧の応答。 */
export function pagedListHandler(rows: TestRow[], chunk: number): (url: URL) => Response {
  return (url) => {
    const offset = Number(url.searchParams.get("offset") ?? "0");
    return jsonResponse(listBody(rows.slice(offset, offset + chunk), rows.length, offset));
  };
}

/** E-8 の 200 応答（AC-10-4）。契約期間などは画面2が読まないため、キーの一部だけを持つ。 */
export function contractsBody(contracts: TestContract[]) {
  return { items: contracts };
}

export const OWN_PATH = "/api/work-months/own";
export const PENDING_PATH = "/api/work-months/pending-approval";
export const DEMO_PATH = "/api/work-months/demo";
export const CONTRACTS_PATH = "/api/contracts";

export type FakeHandler = (url: URL) => Response | Promise<Response>;

export interface FakeFetch {
  /** 受けた要求のURL（時系列）。 */
  urls: URL[];
  /** 受けた要求のメソッド（時系列。未指定は GET）。 */
  methods: string[];
  /** パス名ごとの要求数。 */
  count(pathname: string): number;
  /** パス名ごとの offset クエリの列。 */
  offsets(pathname: string): Array<string | null>;
}

/** パス名ごとの応答を返す fetch を大域へ置く。未登録のパスは例外にして見逃さない。 */
export function installFakeFetch(handlers: Record<string, FakeHandler>): FakeFetch {
  const fake: FakeFetch = {
    urls: [],
    methods: [],
    count: (pathname) => fake.urls.filter((u) => u.pathname === pathname).length,
    offsets: (pathname) =>
      fake.urls.filter((u) => u.pathname === pathname).map((u) => u.searchParams.get("offset")),
  };
  vi.stubGlobal("fetch", async (input: unknown, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost");
    fake.urls.push(url);
    fake.methods.push((init?.method ?? "GET").toUpperCase());
    const handler = handlers[url.pathname];
    if (handler === undefined) {
      throw new Error(`unexpected request: ${url.pathname}`);
    }
    return handler(url);
  });
  return fake;
}
