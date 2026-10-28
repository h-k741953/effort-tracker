import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { beforeEach, describe, expect, it } from "vitest";
// docs/specs/demo-data-reset.md AC-7-6・7-7（検査は AC-14-4）。bff-wiring.md の R-4
// （ゲスト向けデモ勤務月群。E-2）を、lambda-client の Fake 越しに見る。
// 5-g と同型の検査（7-7）と、6-c と同型の検査（7-6）。
//
// - 技術者識別子の値と環境変数名は構成側（Terraform）が持つ（7-3）。値はダミー。
//   共有の ENVIRONMENT には足さず、本ファイルで組み立てる（他の検査の母集団を動かさない）。
// - 2026-10-27 人間決定（#56。bff-wiring.md D 節 #56 (3)）: R-4 は要求から何も取らない。
//   limit・offset・操作者のヘッダを渡さず、セッションの有無で振る舞いを変えない。
// - AC-11-3: 外部へ通信しない。環境変数は引数で与える。
import { routeHandlerRateLimiter } from "./rate-limit";
import { handleListDemoWorkMonths } from "./work-month-handlers";
import { DEMO_ENGINEER_ID, businessOptions, fakeClient } from "../test-support/bff-wiring-handlers";
import {
  ENVIRONMENT,
  browserRequest,
  buildIdTokenWithBadSignature,
  signedRoleCookieValue,
} from "../test-support/bff-wiring";

const DEMO_ENVIRONMENT = { ...ENVIRONMENT, DEMO_ENGINEER_ID };

beforeEach(() => {
  routeHandlerRateLimiter.reset();
});

const SESSIONS = [
  ["セッションなし（ゲスト）", null],
  ["検証済みセッションあり", undefined],
] as const;

describe("AC-7-6: R-4 は E-2 を、技術者識別子にデモ用の値を指定して呼ぶ", () => {
  for (const [label, session] of SESSIONS) {
    it(`7-6: ${label}でも Fake がちょうど1度 E-2 で呼ばれ、engineerId は環境変数の値、state は送らない`, async () => {
      const { client, calls } = fakeClient();

      const response = await handleListDemoWorkMonths(
        browserRequest("GET", { session }),
        businessOptions(client, DEMO_ENVIRONMENT),
      );

      expect(response.status).toBe(200);
      expect(response.headers.get("Cache-Control")).toBe("no-store");
      expect(calls).toHaveLength(1);
      const call = calls[0];
      expect(call.endpoint).toBe("E-2");
      if (call.endpoint !== "E-2") throw new Error("unreachable");
      expect(call.query.engineerId).toBe(DEMO_ENGINEER_ID);
      expect(call.query.state).toBeUndefined();
    });

    it(`7-6: ${label}で、クエリ・ヘッダ・Cookie に別の engineerId を付けても無視され、state も送らない`, async () => {
      const { client, calls } = fakeClient();

      await handleListDemoWorkMonths(
        browserRequest("GET", {
          session,
          search: "?engineerId=other&state=Approved",
          headers: { "X-Engineer-Id": "other", "X-Actor-Id": "other" },
        }),
        businessOptions(client, DEMO_ENVIRONMENT),
      );

      expect(calls).toHaveLength(1);
      const call = calls[0];
      if (call.endpoint !== "E-2") throw new Error("unreachable");
      expect(call.query.engineerId).toBe(DEMO_ENGINEER_ID);
      expect(call.query.state).toBeUndefined();
    });
  }

  // 7-6（2026-10-27 決定）: 要求に other / Approved / limit=5 / offset=10 を含め、4通りで呼ぶ。
  const HOSTILE_SEARCH = "?engineerId=other&state=Approved&limit=5&offset=10";
  const FOUR_WAYS = [
    ["(i) セッション Cookie なし", { session: null }],
    ["(ii) 検証に失敗するセッション Cookie", { session: buildIdTokenWithBadSignature() }],
    ["(iii) Engineer の検証済みセッション", { roleCookieValue: signedRoleCookieValue("Engineer") }],
    ["(iv) Approver の検証済みセッション", { roleCookieValue: signedRoleCookieValue("Approver") }],
  ] as const;
  const E2_BODY = { items: [{ contractId: "contract-dummy-1", yearMonth: "2026-10" }], total: 1 };

  for (const [label, options] of FOUR_WAYS) {
    it(`7-6: ${label}で、Fake がちょうど1度、{ endpoint: "E-2", query: { engineerId: <デモ値> }, actor: null } と厳密に一致する値で呼ばれる`, async () => {
      const { client, calls } = fakeClient();

      await handleListDemoWorkMonths(
        browserRequest("GET", {
          ...options,
          search: HOSTILE_SEARCH,
          headers: { "X-Engineer-Id": "other", "X-Actor-Id": "other" },
        }),
        businessOptions(client, DEMO_ENVIRONMENT),
      );

      expect(calls).toHaveLength(1);
      expect(calls[0]).toStrictEqual({
        endpoint: "E-2",
        query: { engineerId: DEMO_ENGINEER_ID },
        actor: null,
      });
    });

    it(`7-6: ${label}で、Cookie を失効させる Set-Cookie を持たない`, async () => {
      const { client } = fakeClient();

      const response = await handleListDemoWorkMonths(
        browserRequest("GET", { ...options, search: HOSTILE_SEARCH }),
        businessOptions(client, DEMO_ENVIRONMENT),
      );

      expect(response.status).toBe(200);
      const expiring = response.headers
        .getSetCookie()
        .filter((cookie) => /Max-Age=0|Expires=/i.test(cookie));
      expect(expiring).toEqual([]);
    });
  }

  it("7-6: 4通りの応答は、ステータスも本文も互いに一致する（Fake の本文をそのまま返す）", async () => {
    const results: Array<{ status: number; body: unknown }> = [];
    for (const [, options] of FOUR_WAYS) {
      const { client } = fakeClient({ kind: "success", status: 200, body: E2_BODY });

      const response = await handleListDemoWorkMonths(
        browserRequest("GET", { ...options, search: HOSTILE_SEARCH }),
        businessOptions(client, DEMO_ENVIRONMENT),
      );
      results.push({ status: response.status, body: await response.json() });
    }

    expect(results).toHaveLength(4);
    for (const result of results) {
      expect(result).toEqual({ status: 200, body: E2_BODY });
    }
  });

  it("7-6: 検証に失敗するセッション Cookie でもゲストとして続け、E-2 を1度呼ぶ（401 にしない）", async () => {
    const { client, calls } = fakeClient();

    const response = await handleListDemoWorkMonths(
      browserRequest("GET", { session: buildIdTokenWithBadSignature() }),
      businessOptions(client, DEMO_ENVIRONMENT),
    );

    expect(response.status).toBe(200);
    expect(calls).toHaveLength(1);
  });

  it("7-6: GET 以外のメソッドでは Fake が呼ばれない", async () => {
    const { client, calls } = fakeClient();

    const response = await handleListDemoWorkMonths(
      browserRequest("POST"),
      businessOptions(client, DEMO_ENVIRONMENT),
    );

    expect(response.status).toBe(405);
    expect(calls).toHaveLength(0);
  });
});

describe("AC-7-7: R-4 で技術者識別子の環境変数が未設定・空", () => {
  const missing = [
    ["未設定", ENVIRONMENT],
    ["空", { ...ENVIRONMENT, DEMO_ENGINEER_ID: "" }],
  ] as const;

  for (const [label, environment] of missing) {
    for (const [sessionLabel, session] of SESSIONS) {
      it(`7-7: ${label}・${sessionLabel}のとき 5xx（401・429 でない）で、ドメイン API を呼ばない`, async () => {
        const { client, calls } = fakeClient();

        const response = await handleListDemoWorkMonths(
          browserRequest("GET", { session }),
          businessOptions(client, environment),
        );

        expect(response.status).toBeGreaterThanOrEqual(500);
        expect(response.status).toBeLessThan(600);
        expect(await response.text()).toBe("");
        expect(calls).toHaveLength(0);
      });
    }
  }
});

describe("AC-7-6・7-8: R-4 の Route Handler（パスの文字列は実装側が持つ）", () => {
  const routeFile = fileURLToPath(
    new URL("../app/api/work-months/demo/route.ts", import.meta.url),
  );

  it("GET /api/work-months/demo の route.ts が GET だけを持ち、handleListDemoWorkMonths を1つ呼ぶ", () => {
    const source = readFileSync(routeFile, "utf8")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/\/\/.*$/gm, "");

    expect([...source.matchAll(/export\s+(?:async\s+)?function\s+(\w+)/g)].map((m) => m[1])).toEqual([
      "GET",
    ]);
    expect([...source.matchAll(/\b(handle\w+)\s*\(/g)].map((m) => m[1])).toEqual([
      "handleListDemoWorkMonths",
    ]);
  });
});
