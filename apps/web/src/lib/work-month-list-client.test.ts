import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CONTRACTS_PATH,
  CONTRACT_A,
  CONTRACT_B,
  DEMO_PATH,
  OWN_PATH,
  OWN_ROWS,
  PENDING_PATH,
  contractsBody,
  installFakeFetch,
  jsonResponse,
  listBody,
  pagedListHandler,
  row,
  type TestRow,
} from "@/test-support/work-month-list";
import {
  fetchAllWorkMonths,
  fetchDemoWorkMonths,
  fetchOwnContracts,
  fetchOwnWorkMonths,
  fetchPendingApprovalWorkMonths,
} from "./work-month-list-client";

// docs/specs/work-month-listing-ui.md AC-1-1・7-3。設計 W-2・W-3・W-8・T-0・T-2。
// 取得関数へ手書きの Fake の fetchImpl を渡して読む。fetchImpl を省く行だけ、globalThis.fetch に手書きの Fake を置く。
// 一覧の取得の要求は GET で本文を持たない（W-8）。

interface Call {
  url: URL;
  method: string;
  body: unknown;
}

type Responder = (url: URL, index: number) => Response | Promise<Response>;

/** 要求を記録して respond の応答を返す、手書きの fetch。 */
function fakeFetchImpl(respond: Responder) {
  const calls: Call[] = [];
  const impl = (async (input: unknown, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost");
    calls.push({ url, method: (init?.method ?? "GET").toUpperCase(), body: init?.body });
    return respond(url, calls.length - 1);
  }) as typeof fetch;
  return { impl, calls };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("分類（W-2・W-8。7-3）", () => {
  const OK_BODY = listBody(OWN_ROWS, OWN_ROWS.length, 0);

  it("200 → ok（行を返す）", async () => {
    const { impl } = fakeFetchImpl(() => jsonResponse(OK_BODY));
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind: "ok", value: OWN_ROWS });
  });

  it.each([401, 403])("%i → refresh", async (status) => {
    const { impl } = fakeFetchImpl(() => jsonResponse({}, status));
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind: "refresh" });
  });

  it.each([429, 500, 404, 502])("%i → failed", async (status) => {
    const { impl } = fakeFetchImpl(() => jsonResponse({}, status));
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind: "failed" });
  });

  it("fetch が例外 → failed", async () => {
    const impl = (async () => {
      throw new TypeError("network");
    }) as typeof fetch;
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind: "failed" });
  });

  it("200 でも本文が JSON として読めない → failed", async () => {
    const { impl } = fakeFetchImpl(() => new Response("<html>", { status: 200 }));
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind: "failed" });
  });

  it.each([
    ["items が無い", { total: 0 }],
    ["items が配列でない", { items: "x", total: 0 }],
    ["本文が null", null],
  ])("200 でも %s → failed", async (_name, body) => {
    const { impl } = fakeFetchImpl(() => jsonResponse(body));
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind: "failed" });
  });
});

describe("fetchAllWorkMonths の全件の取得（W-3。1-1）", () => {
  it("offset=0 から GET し、limit・state・engineerId を送らず、本文を持たない", async () => {
    const { impl, calls } = fakeFetchImpl(pagedListHandler(OWN_ROWS, 1000));
    await fetchAllWorkMonths(OWN_PATH, impl);
    expect(calls).toHaveLength(1);
    expect(calls[0].url.pathname).toBe(OWN_PATH);
    expect(calls[0].url.searchParams.get("offset")).toBe("0");
    for (const key of ["limit", "state", "engineerId"]) {
      expect(calls[0].url.searchParams.has(key)).toBe(false);
    }
    expect(calls[0].method).toBe("GET");
    expect(calls[0].body).toBeUndefined();
  });

  it("offset を受け取った件数ずつ進め、total に届いたら止める（2ページ）", async () => {
    const rows = [...OWN_ROWS]; // 4件
    const { impl, calls } = fakeFetchImpl(pagedListHandler(rows, 2));
    const result = await fetchAllWorkMonths(OWN_PATH, impl);
    expect(calls.map((c) => c.url.searchParams.get("offset"))).toEqual(["0", "2"]);
    expect(result).toEqual({ kind: "ok", value: rows });
  });

  it("端数のページを含む全件（3ページ）を応答の順で連結する", async () => {
    const rows = [...OWN_ROWS, row(CONTRACT_A, "2026-07", "Draft")]; // 5件
    const { impl, calls } = fakeFetchImpl(pagedListHandler(rows, 2));
    const result = await fetchAllWorkMonths(OWN_PATH, impl);
    expect(calls.map((c) => c.url.searchParams.get("offset"))).toEqual(["0", "2", "4"]);
    expect(result).toEqual({ kind: "ok", value: rows });
  });

  it("items が空の応答で止まる（total に届かなくても無限に要求しない）", async () => {
    const { impl, calls } = fakeFetchImpl((url) => {
      const offset = Number(url.searchParams.get("offset") ?? "0");
      const items = offset === 0 ? OWN_ROWS.slice(0, 2) : [];
      return jsonResponse({ items, total: 10, limit: 2, offset });
    });
    const result = await fetchAllWorkMonths(OWN_PATH, impl);
    expect(calls.map((c) => c.url.searchParams.get("offset"))).toEqual(["0", "2"]);
    expect(result).toEqual({ kind: "ok", value: OWN_ROWS.slice(0, 2) });
  });

  it("0件の一覧は1回の要求で ok の空配列", async () => {
    const { impl, calls } = fakeFetchImpl(pagedListHandler([], 1000));
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind: "ok", value: [] });
    expect(calls).toHaveLength(1);
  });

  it("(契約, 対象年月) が重複した行は、先に来たものを残す", async () => {
    const dup = row(CONTRACT_A, "2026-10", "Approved"); // OWN_ROWS[0] と同じ契約・年月で状態が違う
    const rows: TestRow[] = [OWN_ROWS[0], OWN_ROWS[1], dup, OWN_ROWS[2]];
    const { impl } = fakeFetchImpl(pagedListHandler(rows, 2));
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({
      kind: "ok",
      value: [OWN_ROWS[0], OWN_ROWS[1], OWN_ROWS[2]],
    });
  });

  it.each<[string, number]>([
    ["refresh", 401],
    ["failed", 500],
  ])("2ページ目が ok でなければ、その分類（%s）を返し、受け取った行を捨てて止める", async (kind, status) => {
    const { impl, calls } = fakeFetchImpl((url) => {
      const offset = Number(url.searchParams.get("offset") ?? "0");
      return offset === 0
        ? jsonResponse(listBody(OWN_ROWS.slice(0, 2), OWN_ROWS.length, 0))
        : jsonResponse({}, status);
    });
    expect(await fetchAllWorkMonths(OWN_PATH, impl)).toEqual({ kind });
    expect(calls).toHaveLength(2);
  });
});

describe("本人一覧・承認待ち一覧・デモ勤務月群（W-8）", () => {
  it("fetchOwnWorkMonths: R-2 のパスで全件を取り、engineerId を付けない", async () => {
    const { impl, calls } = fakeFetchImpl(pagedListHandler(OWN_ROWS, 2));
    expect(await fetchOwnWorkMonths(impl)).toEqual({ kind: "ok", value: OWN_ROWS });
    expect(new Set(calls.map((c) => c.url.pathname))).toEqual(new Set([OWN_PATH]));
    expect(calls.map((c) => c.url.searchParams.get("offset"))).toEqual(["0", "2"]);
    expect(calls.every((c) => !c.url.searchParams.has("engineerId") && c.method === "GET")).toBe(true);
  });

  it("fetchPendingApprovalWorkMonths: R-3 のパスで全件を取り、state を付けない", async () => {
    const pending = [row(CONTRACT_A, "2026-10", "PendingApproval"), row(CONTRACT_B, "2026-10", "PendingApproval")];
    const { impl, calls } = fakeFetchImpl(pagedListHandler(pending, 1));
    expect(await fetchPendingApprovalWorkMonths(impl)).toEqual({ kind: "ok", value: pending });
    expect(new Set(calls.map((c) => c.url.pathname))).toEqual(new Set([PENDING_PATH]));
    expect(calls.map((c) => c.url.searchParams.get("offset"))).toEqual(["0", "1"]);
    expect(calls.every((c) => !c.url.searchParams.has("state") && c.method === "GET")).toBe(true);
  });

  it("fetchDemoWorkMonths: R-4 を1回だけ、クエリなしで GET する（total が行より大きくても送らない）", async () => {
    const { impl, calls } = fakeFetchImpl(() => jsonResponse(listBody(OWN_ROWS.slice(0, 2), 10, 0)));
    expect(await fetchDemoWorkMonths(impl)).toEqual({ kind: "ok", value: OWN_ROWS.slice(0, 2) });
    expect(calls).toHaveLength(1);
    expect(calls[0].url.pathname).toBe(DEMO_PATH);
    expect([...calls[0].url.searchParams.keys()]).toEqual([]);
    expect(calls[0].method).toBe("GET");
    expect(calls[0].body).toBeUndefined();
  });

  it.each([
    ["fetchOwnWorkMonths", fetchOwnWorkMonths],
    ["fetchPendingApprovalWorkMonths", fetchPendingApprovalWorkMonths],
    ["fetchDemoWorkMonths", fetchDemoWorkMonths],
  ])("%s: 401 → refresh、429 → failed", async (_name, fetchFn) => {
    expect(await fetchFn(fakeFetchImpl(() => jsonResponse({}, 401)).impl)).toEqual({ kind: "refresh" });
    expect(await fetchFn(fakeFetchImpl(() => jsonResponse({}, 429)).impl)).toEqual({ kind: "failed" });
  });
});

describe("fetchOwnContracts（R-10。W-3・W-8）", () => {
  const contracts = [CONTRACT_A, CONTRACT_B];

  it.each([
    ["2026-10-31T14:59:59Z", "2026-10"],
    ["2026-10-31T15:00:00Z", "2026-11"],
    ["2026-12-31T15:00:00Z", "2027-01"],
  ])("now=%s → yearMonth=%s（JST の年月）を付けて GET する", async (iso, yearMonth) => {
    const { impl, calls } = fakeFetchImpl(() => jsonResponse(contractsBody(contracts)));
    await fetchOwnContracts(new Date(iso), impl);
    expect(calls).toHaveLength(1);
    expect(calls[0].url.pathname).toBe(CONTRACTS_PATH);
    expect(calls[0].url.searchParams.get("yearMonth")).toBe(yearMonth);
    expect(calls[0].method).toBe("GET");
    expect(calls[0].body).toBeUndefined();
  });

  it("200 → ok（items を応答の順のまま返す）", async () => {
    const { impl } = fakeFetchImpl(() => jsonResponse(contractsBody([CONTRACT_B, CONTRACT_A])));
    expect(await fetchOwnContracts(new Date("2026-10-01T00:00:00Z"), impl)).toEqual({
      kind: "ok",
      value: [CONTRACT_B, CONTRACT_A],
    });
  });

  it.each<[string, () => Response, string]>([
    ["401", () => jsonResponse({}, 401), "refresh"],
    ["403", () => jsonResponse({}, 403), "refresh"],
    ["429", () => jsonResponse({}, 429), "failed"],
    ["500", () => jsonResponse({}, 500), "failed"],
    ["items が配列でない", () => jsonResponse({ items: {} }), "failed"],
    ["JSON でない本文", () => new Response("x", { status: 200 }), "failed"],
  ])("%s → %s", async (_name, respond, kind) => {
    const { impl } = fakeFetchImpl(respond);
    expect(await fetchOwnContracts(new Date("2026-10-01T00:00:00Z"), impl)).toEqual({ kind });
  });

  it("fetch が例外 → failed", async () => {
    const impl = (async () => {
      throw new TypeError("network");
    }) as typeof fetch;
    expect(await fetchOwnContracts(new Date("2026-10-01T00:00:00Z"), impl)).toEqual({ kind: "failed" });
  });
});

describe("fetchImpl を省いたとき（W-8。T-0-1）", () => {
  it("呼び出しの時点の globalThis.fetch を使う（読み込みの後に差し替えた Fake が呼ばれる）", async () => {
    const fake = installFakeFetch({
      [OWN_PATH]: pagedListHandler(OWN_ROWS, 1000),
      [PENDING_PATH]: pagedListHandler([], 1000),
      [DEMO_PATH]: pagedListHandler([], 1000),
      [CONTRACTS_PATH]: () => jsonResponse(contractsBody([CONTRACT_A])),
    });
    expect(await fetchOwnWorkMonths()).toEqual({ kind: "ok", value: OWN_ROWS });
    expect(await fetchPendingApprovalWorkMonths()).toEqual({ kind: "ok", value: [] });
    expect(await fetchDemoWorkMonths()).toEqual({ kind: "ok", value: [] });
    expect(await fetchOwnContracts(new Date("2026-10-01T00:00:00Z"))).toEqual({
      kind: "ok",
      value: [CONTRACT_A],
    });
    expect(fake.count(OWN_PATH)).toBe(1);
    expect(fake.count(PENDING_PATH)).toBe(1);
    expect(fake.count(DEMO_PATH)).toBe(1);
    expect(fake.count(CONTRACTS_PATH)).toBe(1);
  });

  it("fetchImpl を渡したときは、globalThis.fetch を呼ばない", async () => {
    const fake = installFakeFetch({});
    const { impl } = fakeFetchImpl(pagedListHandler(OWN_ROWS, 1000));
    await fetchOwnWorkMonths(impl);
    expect(fake.urls).toHaveLength(0);
  });
});
