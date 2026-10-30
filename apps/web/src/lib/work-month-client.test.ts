import { afterEach, describe, expect, it, vi } from "vitest";
import {
  OPERATOR_OWNERSHIP_KEY,
} from "./work-month";
import {
  approveWorkMonth,
  closeWorkMonth,
  deleteDailyRecord,
  fetchWorkMonth,
  rejectWorkMonth,
  saveDailyRecord,
  workMonthApiPath,
  type DailyRecordInput,
} from "./work-month-client";
import { errorBody, jsonResponse, workMonthBody } from "@/test-support/work-month";

// docs/specs/work-month-screen-ui.md AC-6、daily-record-entry-ui.md 3-6-c、bff-wiring.md 6-6・6-7。
// 設計 M-2・M-3・M-11・T-0・T-3。6つの関数へ手書きの Fake の fetchImpl を渡して読む。
// fetchImpl を省く行だけ、globalThis.fetch に手書きの Fake を置く。

interface Call {
  url: URL;
  method: string;
  body: unknown;
  headers: Headers;
}

type Responder = (call: Call) => Response | Promise<Response>;

function fakeFetchImpl(respond: Responder) {
  const calls: Call[] = [];
  const impl = (async (input: unknown, init?: RequestInit) => {
    const call: Call = {
      url: new URL(String(input), "http://localhost"),
      method: (init?.method ?? "GET").toUpperCase(),
      body: init?.body,
      headers: new Headers(init?.headers),
    };
    calls.push(call);
    return respond(call);
  }) as typeof fetch;
  return { impl, calls };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const INPUT: DailyRecordInput = {
  startTime: { hour: 9, minute: 0 },
  endTime: { hour: 18, minute: 0 },
  break: { start: { hour: 12, minute: 0 }, end: { hour: 13, minute: 0 } },
};
const INPUT_NO_BREAK: DailyRecordInput = { ...INPUT, break: null };

type Invoke = (fetchImpl: typeof fetch) => ReturnType<typeof fetchWorkMonth>;

const OPERATIONS: Array<[string, Invoke, string, string]> = [
  ["fetchWorkMonth（R-1）", (f) => fetchWorkMonth("c-001", "2026-07", f), "GET", "/api/work-months/c-001/2026-07"],
  [
    "saveDailyRecord（R-5）",
    (f) => saveDailyRecord("c-001", "2026-07", "2026-07-03", INPUT, f),
    "PUT",
    "/api/work-months/c-001/2026-07/daily-records/2026-07-03",
  ],
  [
    "deleteDailyRecord（R-6）",
    (f) => deleteDailyRecord("c-001", "2026-07", "2026-07-03", f),
    "DELETE",
    "/api/work-months/c-001/2026-07/daily-records/2026-07-03",
  ],
  ["closeWorkMonth（R-7）", (f) => closeWorkMonth("c-001", "2026-07", f), "POST", "/api/work-months/c-001/2026-07/close"],
  [
    "approveWorkMonth（R-8）",
    (f) => approveWorkMonth("c-001", "2026-07", f),
    "POST",
    "/api/work-months/c-001/2026-07/approve",
  ],
  [
    "rejectWorkMonth（R-9）",
    (f) => rejectWorkMonth("c-001", "2026-07", f),
    "POST",
    "/api/work-months/c-001/2026-07/reject",
  ],
];

describe("workMonthApiPath（M-11）", () => {
  it("/api ＋ E-1 のパス", () => {
    expect(workMonthApiPath("c-001", "2026-07")).toBe("/api/work-months/c-001/2026-07");
  });

  it("/・?・# を含む値は1つのセグメントに符号化される", () => {
    expect(workMonthApiPath("a/b", "2026-07?x#y")).toBe("/api/work-months/a%2Fb/2026-07%3Fx%23y");
  });
});

describe("要求のメソッドとパス（bff-wiring.md 6-7）", () => {
  it.each(OPERATIONS)("%s は %s・%s", async (_name, invoke, method, pathname) => {
    const { impl, calls } = fakeFetchImpl(() => jsonResponse(workMonthBody()));
    await invoke(impl);
    expect(calls).toHaveLength(1);
    expect(calls[0].method).toBe(method);
    expect(calls[0].url.pathname).toBe(pathname);
  });

  it("R-5・R-6 の日付も1つのセグメントとして符号化される", async () => {
    const { impl, calls } = fakeFetchImpl(() => jsonResponse(workMonthBody()));
    await saveDailyRecord("c/1", "2026-07", "2026-07-0?", INPUT, impl);
    await deleteDailyRecord("c/1", "2026-07", "2026-07-0?", impl);
    for (const call of calls) {
      expect(call.url.pathname).toBe("/api/work-months/c%2F1/2026-07/daily-records/2026-07-0%3F");
    }
  });
});

describe("R-5 の本文（入力 UI 3-6-c。稼働時間を送らない）", () => {
  it.each([
    ["休憩あり", INPUT],
    ["休憩なし（break: null のときもキーがある）", INPUT_NO_BREAK],
  ])("%s: JSON として読んだ値が input と深く等しく、キーがちょうど3つ", async (_name, input) => {
    const { impl, calls } = fakeFetchImpl(() => jsonResponse(workMonthBody()));
    await saveDailyRecord("c-001", "2026-07", "2026-07-03", input, impl);
    const sent = JSON.parse(calls[0].body as string) as Record<string, unknown>;
    expect(sent).toEqual(input);
    expect(Object.keys(sent).sort()).toEqual(["break", "endTime", "startTime"]);
    expect("workingHours" in sent).toBe(false);
    expect(calls[0].headers.get("content-type")).toBe("application/json");
  });

  it.each([
    ["R-1", (f: typeof fetch) => fetchWorkMonth("c-001", "2026-07", f)],
    ["R-6", (f: typeof fetch) => deleteDailyRecord("c-001", "2026-07", "2026-07-03", f)],
    ["R-7", (f: typeof fetch) => closeWorkMonth("c-001", "2026-07", f)],
    ["R-8", (f: typeof fetch) => approveWorkMonth("c-001", "2026-07", f)],
    ["R-9", (f: typeof fetch) => rejectWorkMonth("c-001", "2026-07", f)],
  ])("%s は本文を持たない", async (_name, invoke) => {
    const { impl, calls } = fakeFetchImpl(() => jsonResponse(workMonthBody()));
    await invoke(impl);
    expect(calls[0].body === undefined || calls[0].body === null).toBe(true);
  });
});

describe("応答の分類（M-2 の表）", () => {
  describe.each(OPERATIONS)("%s", (_name, invoke) => {
    it("200 の表現 → ok（value が表現）", async () => {
      const body = workMonthBody({ state: "PendingApproval" });
      const { impl } = fakeFetchImpl(() => jsonResponse(body));
      const result = await invoke(impl);
      expect(result.kind).toBe("ok");
      if (result.kind === "ok") {
        expect(result.value).toEqual(body);
        expect(OPERATOR_OWNERSHIP_KEY in result.value).toBe(false);
      }
    });

    it("401（本文を問わない）→ refresh", async () => {
      for (const response of [
        jsonResponse(errorBody("UNAUTHENTICATED"), 401),
        new Response("not json", { status: 401 }),
        new Response(null, { status: 401 }),
      ]) {
        const { impl } = fakeFetchImpl(() => response);
        expect(await invoke(impl)).toEqual({ kind: "refresh" });
      }
    });

    it.each([400, 403, 404, 409])("%i で {error:{code}} → rejected（status と code）", async (status) => {
      const { impl } = fakeFetchImpl(() => jsonResponse(errorBody("SOME_CODE"), status));
      expect(await invoke(impl)).toEqual({ kind: "rejected", status, code: "SOME_CODE" });
    });

    it.each([429, 500, 502])("%i → failed", async (status) => {
      const { impl } = fakeFetchImpl(() => jsonResponse(errorBody("SOME_CODE"), status));
      expect(await invoke(impl)).toEqual({ kind: "failed" });
    });

    it("422 は {error:{code}} の本文でも failed（rejected にするのは 400・403・404・409 だけ）", async () => {
      const { impl } = fakeFetchImpl(() => jsonResponse(errorBody("SOME_CODE"), 422));
      expect(await invoke(impl)).toEqual({ kind: "failed" });
    });

    it("fetch が例外 → failed", async () => {
      const impl = (async () => {
        throw new Error("network");
      }) as unknown as typeof fetch;
      expect(await invoke(impl)).toEqual({ kind: "failed" });
    });

    it("JSON でない本文の 200 → failed", async () => {
      const { impl } = fakeFetchImpl(() => new Response("<html>", { status: 200 }));
      expect(await invoke(impl)).toEqual({ kind: "failed" });
    });

    it("code の無い 403・形の違う 404 → failed", async () => {
      for (const response of [
        jsonResponse({ error: {} }, 403),
        jsonResponse({}, 404),
        new Response("not json", { status: 409 }),
      ]) {
        const { impl } = fakeFetchImpl(() => response);
        expect(await invoke(impl)).toEqual({ kind: "failed" });
      }
    });

    it("dailyRecords の無い 200 → failed", async () => {
      const { dailyRecords: _omitted, ...withoutRecords } = workMonthBody();
      void _omitted;
      const { impl } = fakeFetchImpl(() => jsonResponse(withoutRecords));
      expect(await invoke(impl)).toEqual({ kind: "failed" });
    });

    it("contractId が文字列でない 200 → failed", async () => {
      const { impl } = fakeFetchImpl(() => jsonResponse({ ...workMonthBody(), contractId: 1 }));
      expect(await invoke(impl)).toEqual({ kind: "failed" });
    });
  });
});

describe("ok の ownership（M-3。6つの関数のいずれでも同じ）", () => {
  describe.each(OPERATIONS)("%s", (_name, invoke) => {
    it.each([
      [true, "own"],
      [false, "other"],
    ])("本文のキーが %s → ownership が %s", async (flag, expected) => {
      const { impl } = fakeFetchImpl(() => jsonResponse(workMonthBody({ ownership: flag })));
      const result = await invoke(impl);
      expect(result.kind === "ok" && result.ownership).toBe(expected);
    });

    it("キーが無い → ownership は unknown で、ok のまま（failed にしない）", async () => {
      const { impl } = fakeFetchImpl(() => jsonResponse(workMonthBody()));
      const result = await invoke(impl);
      expect(result.kind).toBe("ok");
      expect(result.kind === "ok" && result.ownership).toBe("unknown");
    });

    it("キーの値が真偽でない → ownership は unknown で、ok のまま", async () => {
      const { impl } = fakeFetchImpl(() => jsonResponse({ ...workMonthBody(), [OPERATOR_OWNERSHIP_KEY]: "true" }));
      const result = await invoke(impl);
      expect(result.kind).toBe("ok");
      expect(result.kind === "ok" && result.ownership).toBe("unknown");
    });
  });
});

describe("fetchImpl を省いたとき（M-11）", () => {
  it("呼び出しの時点の globalThis.fetch を使う", async () => {
    const first = fakeFetchImpl(() => jsonResponse(workMonthBody({ ownership: true })));
    vi.stubGlobal("fetch", first.impl);
    expect((await fetchWorkMonth("c-001", "2026-07")).kind).toBe("ok");
    expect(first.calls).toHaveLength(1);

    const second = fakeFetchImpl(() => jsonResponse(errorBody("CONTRACT_NOT_FOUND"), 404));
    vi.stubGlobal("fetch", second.impl);
    expect(await fetchWorkMonth("c-001", "2026-07")).toEqual({
      kind: "rejected",
      status: 404,
      code: "CONTRACT_NOT_FOUND",
    });
    expect(second.calls).toHaveLength(1);
    expect(first.calls).toHaveLength(1);
  });

  it.each(OPERATIONS.slice(1))("%s も globalThis.fetch を使う", async (_name, invoke) => {
    const fake = fakeFetchImpl(() => jsonResponse(workMonthBody()));
    vi.stubGlobal("fetch", fake.impl);
    await invoke(undefined as unknown as typeof fetch);
    expect(fake.calls).toHaveLength(1);
  });
});
