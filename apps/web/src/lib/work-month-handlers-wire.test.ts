import { beforeEach, describe, expect, it } from "vitest";
// docs/specs/bff-wiring.md のうち、処理を **送信関数の Fake を差し込んだ本物の
// lambda-client** 越しに呼び、送った要求とブラウザへの応答を見る検査:
//   4-g（転送しない）／6-i（本文の中継）／AC-7（7-a〜7-c）／
//   AC-8 の写像（8-i〜8-s）／AC-9（9-a・9-b）。
//
// AC-11-3: 外部へ通信しない。時間切れは小さい値を与えて検査する（AC-8-8:
// 値は呼ぶ側から与えられる形）。応答を返さない Fake は解決しない Promise。
// AC-10-4: Vitest 標準の expect だけを使う。
import { routeHandlerRateLimiter } from "./rate-limit";
import {
  BUSINESS_HANDLERS,
  businessOptions,
  requestFor,
} from "../test-support/bff-wiring-handlers";
import {
  ACCESS_KEY_ID_VALUE,
  DOMAIN_API_URL,
  ENVIRONMENT,
  FUNCTION_URL_MARK,
  PATH_PARAMS,
  SECRET_MARK,
  SUB,
  TOKEN_MARK,
  browserRequest,
  headerOf,
  jsonResponse,
  recordingSend,
  testClient,
} from "../test-support/bff-wiring";
import {
  handleDeleteDailyRecord,
  handleGetWorkMonth,
  handlePutDailyRecord,
} from "./work-month-handlers";

const R1 = BUSINESS_HANDLERS.filter((h) => h.route === "R-1")[0];
const R2 = BUSINESS_HANDLERS.filter((h) => h.route === "R-2")[0];
const R5 = BUSINESS_HANDLERS.filter((h) => h.route === "R-5")[0];

beforeEach(() => {
  routeHandlerRateLimiter.reset();
});

// --- 4-g・6-a（メソッドとパス）・6-i ----------------------------------------

describe("AC-4-6: ブラウザの Cookie・Authorization を転送しない（検査 4-g）", () => {
  it("4-g: ブラウザが Cookie と Authorization: Bearer を送っても、送った要求に Cookie が無く Authorization は SigV4 の値", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));

    await R1.invoke(
      requestFor(R1, { headers: { Authorization: "Bearer dummy-bearer-token" } }),
      businessOptions(testClient({ send })),
    );

    expect(sent).toHaveLength(1);
    const headers = sent[0].init.headers;
    expect(headerOf(headers, "cookie")).toBeUndefined();
    const authorization = headerOf(headers, "authorization");
    expect(authorization).toMatch(/^AWS4-HMAC-SHA256 /);
    expect(authorization?.startsWith("Bearer")).toBe(false);
  });
});

describe("AC-6-2・6-8: 処理から本物の lambda-client を経た要求のメソッドとパス", () => {
  // 出所: docs/specs/domain-api-http-contract.md「エンドポイント一覧」の E-1・E-3〜E-7。
  const expected: Record<string, [string, string]> = {
    "R-1": ["GET", "/work-months/c-001/2026-07"],
    "R-5": ["PUT", "/work-months/c-001/2026-07/daily-records/2026-07-15"],
    "R-6": ["DELETE", "/work-months/c-001/2026-07/daily-records/2026-07-15"],
    "R-7": ["POST", "/work-months/c-001/2026-07/close"],
    "R-8": ["POST", "/work-months/c-001/2026-07/approve"],
    "R-9": ["POST", "/work-months/c-001/2026-07/reject"],
  };

  it.each(BUSINESS_HANDLERS.filter((h) => h.route in expected))(
    "6-a: $route が lambda-client を経て送るメソッドとパスが契約の一覧どおり",
    async (handler) => {
      const { send, sent } = recordingSend(() => jsonResponse(200, {}));

      await handler.invoke(requestFor(handler), businessOptions(testClient({ send })));

      expect(sent).toHaveLength(1);
      const [method, pathname] = expected[handler.route];
      expect(sent[0].init.method).toBe(method);
      expect(new URL(sent[0].url).pathname).toBe(pathname);
      expect(new URL(sent[0].url).origin).toBe(new URL(DOMAIN_API_URL).origin);
    },
  );
});

describe("AC-6-8: 本文はそのままドメイン API へ送る（検査 6-i）", () => {
  const bodies: Array<[string, Uint8Array]> = [
    ["JSON として不正な ASCII", new TextEncoder().encode("{not-json")],
    ["UTF-8 として不正なバイト列を含む", Uint8Array.from([0x7b, 0xff, 0xfe, 0x00, 0x80, 0x7d])],
    ["空白と改行を含む正当な JSON", new TextEncoder().encode('{ "a" :\n 1 }  ')],
  ];

  it.each(bodies)(
    "6-i: 本文（%s）が、バイト単位で一致して送られ、Content-Type は契約の application/json",
    async (_name, bytes) => {
      const { send, sent } = recordingSend(() => jsonResponse(200, {}));

      await handlePutDailyRecord(
        browserRequest("PUT", { body: new Uint8Array(bytes), headers: { "Content-Type": "text/plain" } }),
        PATH_PARAMS,
        businessOptions(testClient({ send })),
      );

      expect(sent).toHaveLength(1);
      const sentBody = sent[0].init.body;
      expect(sentBody).toBeDefined();
      expect(Buffer.from(sentBody as Uint8Array).equals(Buffer.from(bytes))).toBe(true);
      expect(headerOf(sent[0].init.headers, "content-type")).toBe("application/json");
    },
  );
});

// --- AC-7 -------------------------------------------------------------------

describe("AC-7: ブラウザ入力の扱い（パス・クエリの構造を変えさせない）", () => {
  it("7-a: contractId が a/../../work-months でも、E-1 の形のまま contractId の位置に符号化された1つのセグメントが入る", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    const value = "a/../../work-months";

    await handleGetWorkMonth(
      requestFor(R1),
      { contractId: value, yearMonth: "2026-07" },
      businessOptions(testClient({ send })),
    );

    expect(sent).toHaveLength(1);
    const url = new URL(sent[0].url);
    const segments = url.pathname.split("/");
    expect(segments).toHaveLength(4);
    expect(segments[1]).toBe("work-months");
    expect(decodeURIComponent(segments[2])).toBe(value);
    expect(segments[3]).toBe("2026-07");
  });

  it("7-b: yearMonth が 2026-07?engineerId=x でも、クエリは空で値は符号化されてパスの中に留まる", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    const value = "2026-07?engineerId=x";

    await handleGetWorkMonth(
      requestFor(R1),
      { contractId: "c-001", yearMonth: value },
      businessOptions(testClient({ send })),
    );

    expect(sent).toHaveLength(1);
    const url = new URL(sent[0].url);
    expect(url.search).toBe("");
    const segments = url.pathname.split("/");
    expect(segments).toHaveLength(4);
    expect(decodeURIComponent(segments[3])).toBe(value);
  });

  it("7-c: R-2 の state が Draft&engineerId=other でも、送った engineerId は sub だけ（1つ）で state は符号化された1つの値", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    const state = "Draft&engineerId=other";

    await R2.invoke(
      requestFor(R2, { search: `?state=${encodeURIComponent(state)}` }),
      businessOptions(testClient({ send })),
    );

    expect(sent).toHaveLength(1);
    const params = new URL(sent[0].url).searchParams;
    expect(params.getAll("engineerId")).toEqual([SUB]);
    expect(params.getAll("state")).toEqual([state]);
  });
});

describe("AC-7-1 補足・7-d (i): 値が . / .. / 空文字のパラメータを Route Handler に与える（5xx・本文空・no-store・送信 0 回）", () => {
  const R6 = BUSINESS_HANDLERS.filter((h) => h.route === "R-6")[0];

  type Case = [string, string, (value: string) => Promise<{ response: Response; sentCount: number }>];
  const cases: Case[] = [];
  const rejectedSegmentValues = [".", "..", ""];

  async function run(
    handler: (request: Request, options: ReturnType<typeof businessOptions>) => Promise<Response>,
    request: Request,
  ) {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    const response = await handler(request, businessOptions(testClient({ send })));
    return { response, sentCount: sent.length };
  }

  for (const value of rejectedSegmentValues) {
    cases.push([
      "R-1 contractId",
      value,
      (v) =>
        run(
          (request, options) =>
            handleGetWorkMonth(request, { contractId: v, yearMonth: "2026-07" }, options),
          requestFor(R1),
        ),
    ]);
    cases.push([
      "R-1 yearMonth",
      value,
      (v) =>
        run(
          (request, options) =>
            handleGetWorkMonth(request, { contractId: "c-001", yearMonth: v }, options),
          requestFor(R1),
        ),
    ]);
    cases.push([
      "R-5 date",
      value,
      (v) =>
        run(
          (request, options) => handlePutDailyRecord(request, { ...PATH_PARAMS, date: v }, options),
          requestFor(R5),
        ),
    ]);
    cases.push([
      "R-6 date",
      value,
      (v) =>
        run(
          (request, options) =>
            handleDeleteDailyRecord(request, { ...PATH_PARAMS, date: v }, options),
          requestFor(R6),
        ),
    ]);
  }

  it("件数の下限: 4 位置 × 3 値 = 12 組", () => {
    expect(cases.length).toBeGreaterThanOrEqual(12);
  });

  it.each(cases)("7-d (i): %s が %j → 5xx（401・429 でない）・本文空・no-store・値を載せない・送信 0 回", async (_name, value, exec) => {
    const { response, sentCount } = await exec(value);

    expect(sentCount).toBe(0);
    expect(response.status).toBeGreaterThanOrEqual(500);
    expect(response.status).toBeLessThan(600);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(await response.text()).toBe("");
    for (const [, headerValue] of response.headers) {
      expect(headerValue).not.toBe(value);
      if (value === "..") expect(headerValue).not.toContain("..");
    }
  });
});

// --- AC-8 写像 ---------------------------------------------------------------

/** R-1 を検証済みセッションありで、与えた Fake の応答で呼ぶ。 */
async function callR1(respond: () => Response | Promise<Response>, timeoutMilliseconds?: number) {
  const { send, sent } = recordingSend(respond);
  const response = await R1.invoke(
    requestFor(R1),
    businessOptions(testClient({ send, timeoutMilliseconds })),
  );
  return { response, sent };
}

describe("AC-8-7: ブラウザへの写像（検査 8-i〜8-q。R-1 を検証済みセッションありで呼ぶ）", () => {
  it("8-i: 200・JSON の勤務月 → 200 で、本文を JSON として読んだ値が Fake の値と深く等しい", async () => {
    const workMonth = { contractId: "c-001", yearMonth: "2026-07", state: "Draft", dailyRecords: [] };

    const { response } = await callR1(() => jsonResponse(200, workMonth));

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual(workMonth);
    expect(headerOf(Object.fromEntries(response.headers), "content-type")).toBe(
      "application/json; charset=utf-8",
    );
  });

  it("8-j: 409・契約のエラー → 409 で、本文は error.code だけを持ち message が現れない", async () => {
    const { response } = await callR1(() =>
      jsonResponse(409, { error: { code: "INVALID_STATE_FOR_CLOSE", message: "MESSAGE-MARK" } }),
    );

    expect(response.status).toBe(409);
    const text = await response.text();
    expect(text).not.toContain("MESSAGE-MARK");
    expect(JSON.parse(text)).toEqual({ error: { code: "INVALID_STATE_FOR_CLOSE" } });
  });

  it("8-k: 500・INTERNAL_ERROR → 500 で、本文は error.code だけ", async () => {
    const { response } = await callR1(() =>
      jsonResponse(500, { error: { code: "INTERNAL_ERROR", message: "x" } }),
    );

    expect(response.status).toBe(500);
    expect(await response.json()).toEqual({ error: { code: "INTERNAL_ERROR" } });
  });

  it("8-l: 401・UNAUTHENTICATED（R-3 をゲストで呼ぶ）→ 401 で、本文は error.code だけ（5-3）", async () => {
    const { send, sent } = recordingSend(() =>
      jsonResponse(401, { error: { code: "UNAUTHENTICATED", message: "x" } }),
    );
    const R3 = BUSINESS_HANDLERS.filter((h) => h.route === "R-3")[0];

    const response = await R3.invoke(
      requestFor(R3, { session: null }),
      businessOptions(testClient({ send })),
    );

    // ゲストとして続け、操作者なしでドメイン API を呼ぶ。
    expect(sent).toHaveLength(1);
    expect(headerOf(sent[0].init.headers, "x-actor-id")).toBeUndefined();
    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({ error: { code: "UNAUTHENTICATED" } });
  });

  it("8-m: 429・Retry-After: 7・本文 RATE-MARK → 429 で Retry-After が 7、本文が空", async () => {
    const { response } = await callR1(
      () => new Response("RATE-MARK", { status: 429, headers: { "Retry-After": "7" } }),
    );

    expect(response.status).toBe(429);
    expect(response.headers.get("Retry-After")).toBe("7");
    expect(await response.text()).toBe("");
  });

  it("8-n: 429・Retry-After なし → 429 で Retry-After が無く、本文が空", async () => {
    const { response } = await callR1(() => new Response(null, { status: 429 }));

    expect(response.status).toBe(429);
    expect(response.headers.get("Retry-After")).toBeNull();
    expect(await response.text()).toBe("");
  });

  it("8-o: 403・IAM の拒否の形 → 5xx（401・429 でない）で本文が空", async () => {
    const { response } = await callR1(() => jsonResponse(403, { Message: "Forbidden" }));

    expect(response.status).toBeGreaterThanOrEqual(500);
    expect(response.status).toBeLessThan(600);
    expect(await response.text()).toBe("");
  });

  it("8-p: 送信関数が例外を投げる → 5xx で本文が空", async () => {
    const { response } = await callR1(() => {
      throw new Error("network failure");
    });

    expect(response.status).toBeGreaterThanOrEqual(500);
    expect(response.status).toBeLessThan(600);
    expect(await response.text()).toBe("");
  });

  it("8-q: 送信関数が応答を返さない → 時間切れで 5xx・本文が空。時間切れの後に Fake が応答を返しても応答は変わらない", async () => {
    let release: (response: Response) => void = () => {};
    const never = new Promise<Response>((resolve) => {
      release = resolve;
    });

    const { response, sent } = await callR1(() => never, 20);

    expect(response.status).toBeGreaterThanOrEqual(500);
    expect(response.status).toBeLessThan(600);
    expect(await response.clone().text()).toBe("");

    // 時間切れの後に届いた応答を結果に用いない。
    release(jsonResponse(200, { late: true }));
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(response.status).toBeGreaterThanOrEqual(500);
    expect(await response.text()).toBe("");
    expect(sent).toHaveLength(1);
  });
});

describe("AC-8-6: 自動の再試行をしない（検査 8-r）", () => {
  const cases: Array<[string, () => Response | Promise<Response>, number | undefined]> = [
    ["8-j 409", () => jsonResponse(409, { error: { code: "INVALID_STATE_FOR_CLOSE", message: "x" } }), undefined],
    ["8-k 500", () => jsonResponse(500, { error: { code: "INTERNAL_ERROR", message: "x" } }), undefined],
    ["8-m 429", () => new Response("RATE-MARK", { status: 429, headers: { "Retry-After": "7" } }), undefined],
    ["8-o 403 IAM", () => jsonResponse(403, { Message: "Forbidden" }), undefined],
    [
      "8-p 例外",
      () => {
        throw new Error("network failure");
      },
      undefined,
    ],
    ["8-q 時間切れ", () => new Promise<Response>(() => {}), 20],
  ];

  it.each(cases)("8-r: %s でも送信関数はちょうど1度だけ呼ばれる", async (_name, respond, timeout) => {
    const { sent } = await callR1(respond, timeout);

    expect(sent).toHaveLength(1);
  });
});

describe("AC-8-7 (v): すべての応答に Cache-Control: no-store（検査 8-s）", () => {
  const responses: Array<[string, () => Response | Promise<Response>]> = [
    ["8-i 200", () => jsonResponse(200, {})],
    ["8-j 409", () => jsonResponse(409, { error: { code: "INVALID_STATE_FOR_CLOSE", message: "x" } })],
    ["8-k 500", () => jsonResponse(500, { error: { code: "INTERNAL_ERROR", message: "x" } })],
    ["8-l 401", () => jsonResponse(401, { error: { code: "UNAUTHENTICATED", message: "x" } })],
    ["8-m 429", () => new Response("RATE-MARK", { status: 429, headers: { "Retry-After": "7" } })],
    ["8-n 429", () => new Response(null, { status: 429 })],
    ["8-o 403", () => jsonResponse(403, { Message: "Forbidden" })],
    [
      "8-p 例外",
      () => {
        throw new Error("network failure");
      },
    ],
  ];

  it.each(responses)("8-s: %s の応答が no-store", async (_name, respond) => {
    const { response } = await callR1(respond);

    expect(response.headers.get("Cache-Control")).toBe("no-store");
  });

  it("8-s: 8-q 時間切れの応答も no-store", async () => {
    const { response } = await callR1(() => new Promise<Response>(() => {}), 20);

    expect(response.headers.get("Cache-Control")).toBe("no-store");
  });
});

// --- AC-9 -------------------------------------------------------------------

describe("AC-9-1: 機密をブラウザへの応答に載せない（検査 9-a・9-b）", () => {
  const MARKS = [FUNCTION_URL_MARK, SECRET_MARK, TOKEN_MARK, ACCESS_KEY_ID_VALUE];

  const upstreams: Array<[string, () => Response | Promise<Response>]> = [
    ["8-a 200", () => jsonResponse(200, { state: "Draft" })],
    ["8-b 409", () => jsonResponse(409, { error: { code: "INVALID_STATE_FOR_CLOSE", message: "x" } })],
    ["8-c 403", () => jsonResponse(403, { Message: "Forbidden" })],
    [
      "8-h 例外",
      () => {
        throw new Error(`failure at ${DOMAIN_API_URL} with ${SECRET_MARK} ${TOKEN_MARK}`);
      },
    ],
  ];

  function assertNoSecrets(text: string, headers: Headers): void {
    const haystack = `${text}\n${JSON.stringify(Object.fromEntries(headers))}`.toLowerCase();
    for (const mark of MARKS) {
      expect(haystack).not.toContain(mark.toLowerCase());
    }
    // 署名（Authorization の値）も載せない。
    expect(haystack).not.toContain("aws4-hmac-sha256");
  }

  it("前提: 構成の環境変数に印が入っている（印が空でない）", () => {
    expect(String(ENVIRONMENT.DOMAIN_API_FUNCTION_URL).toLowerCase()).toContain(
      FUNCTION_URL_MARK.toLowerCase(),
    );
    expect(ENVIRONMENT.AWS_SECRET_ACCESS_KEY).toBe(SECRET_MARK);
    expect(ENVIRONMENT.AWS_SESSION_TOKEN).toBe(TOKEN_MARK);
  });

  for (const handler of [R1, R5]) {
    it.each(upstreams)(
      `9-a: ${handler.route} を Fake の応答 %s で呼んだブラウザへの応答の本文とヘッダに、いずれの印も現れない`,
      async (_name, respond) => {
        const { send } = recordingSend(respond);

        const response = await handler.invoke(
          requestFor(handler),
          businessOptions(testClient({ send })),
        );

        assertNoSecrets(await response.text(), response.headers);
      },
    );
  }

  it("9-b: 8-c の上流本文に印 UPSTREAM-BODY-MARK を含めても、ブラウザへの応答に現れない", async () => {
    const { response } = await callR1(() =>
      jsonResponse(403, { Message: "Forbidden", detail: "UPSTREAM-BODY-MARK" }),
    );

    const text = await response.text();
    expect(text).not.toContain("UPSTREAM-BODY-MARK");
    expect(JSON.stringify(Object.fromEntries(response.headers))).not.toContain("UPSTREAM-BODY-MARK");
  });
});
