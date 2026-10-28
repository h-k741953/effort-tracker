import { beforeEach, describe, expect, it } from "vitest";
// docs/specs/bff-wiring.md のうち、処理（src/lib/ 側）を lambda-client の Fake
// 越しに呼んで、Fake が受け取った要求と応答を見る検査:
//   AC-4（操作者）の 4-a〜4-f／AC-5（入口の順序）の 5-b〜5-i／AC-6 の
//   6-a〜6-g・6-j／AC-8-s（Cache-Control）の入口側。
//
// AC-10-2・10-3: 処理は src/lib/ 配下にあり、lambda-client は呼ぶ側が宣言する
// 最小のインターフェース越しに受ける。レート制限の入口も処理の側に持つ
// （5-b が「処理を Fake 越しに呼んで Fake が呼ばれない」ことを見るため。
// Route Handler の配線は business-routes.test.ts が 5-a で見る）。
//
// AC-11-3: 外部へ通信しない（公開鍵の取得・lambda-client は手書きの Fake）。
// レート制限の状態はテストごとに初期化する（終端仕様 AC-11-7 (iv)）。
// AC-10-4: Vitest 標準の expect だけを使う。
import { routeHandlerRateLimiter } from "./rate-limit";
import { handleSignInCallback } from "./auth-handlers";
import {
  ALL_BUSINESS_HANDLERS,
  BUSINESS_HANDLERS,
  DEMO_ENGINEER_ID,
  DEMO_HANDLER,
  businessOptions,
  fakeClient,
  requestFor,
} from "../test-support/bff-wiring-handlers";
import {
  ENVIRONMENT,
  NOW,
  SUB,
  browserRequest,
  buildIdTokenWithBadSignature,
  environmentWithEmpty,
  environmentWithout,
  exhaustSharedLimiterFor,
  expectAuthCookiesExpired,
  fakePublicKeySource,
  signedRoleCookieValue,
  tamperedRoleCookieValue,
} from "../test-support/bff-wiring";

const R1 = BUSINESS_HANDLERS.filter((h) => h.route === "R-1")[0];
const R2 = BUSINESS_HANDLERS.filter((h) => h.route === "R-2")[0];
const R3 = BUSINESS_HANDLERS.filter((h) => h.route === "R-3")[0];
const R10 = BUSINESS_HANDLERS.filter((h) => h.route === "R-10")[0];
const R11 = BUSINESS_HANDLERS.filter((h) => h.route === "R-11")[0];
const UPDATES = BUSINESS_HANDLERS.filter((h) => h.updates);
const GUESTABLE = [R1, R3];

beforeEach(() => {
  // 終端仕様 AC-11-7 (iv): テスト間で制限器の状態が漏れないようにする。
  routeHandlerRateLimiter.reset();
});

describe("母集団（AC-5-6: R-4 を含む R-1〜R-11 の 11 個）", () => {
  it("検査の表は 11 個の処理を持つ（0件・取りこぼしを一致とみなさない。2026-10-27 改定（#56）: 旧 10）", () => {
    expect(ALL_BUSINESS_HANDLERS.map((h) => h.route)).toEqual([
      "R-1", "R-2", "R-3", "R-4", "R-5", "R-6", "R-7", "R-8", "R-9", "R-10", "R-11",
    ]);
    expect(UPDATES).toHaveLength(5);
  });

  it("5-h の表は R-4 を除く 10 個のまま（R-4 はロールを使わない。2026-10-27 人間決定（#56））", () => {
    expect(BUSINESS_HANDLERS.map((h) => h.route)).toEqual([
      "R-1", "R-2", "R-3", "R-5", "R-6", "R-7", "R-8", "R-9", "R-10", "R-11",
    ]);
  });
});

describe("AC-4: 操作者ヘッダの組み立て（Fake が受け取った操作者を見る。R-1）", () => {
  it("4-a: 検証済みセッションあり・ロール Cookie なし → 操作者は sub と Engineer", async () => {
    const { client, calls } = fakeClient();

    await R1.invoke(requestFor(R1), businessOptions(client));

    expect(calls).toHaveLength(1);
    expect(calls[0].actor).toEqual({ id: SUB, role: "Engineer" });
  });

  it("4-b: 検証済みセッションあり・正当に署名された Approver のロール Cookie → 操作者は sub と Approver", async () => {
    const { client, calls } = fakeClient();

    await R1.invoke(
      requestFor(R1, { roleCookieValue: signedRoleCookieValue("Approver") }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    expect(calls[0].actor).toEqual({ id: SUB, role: "Approver" });
  });

  it("4-c: Approver を主張する改竄されたロール Cookie → 操作者は sub と Engineer（Approver にならない）", async () => {
    const { client, calls } = fakeClient();

    await R1.invoke(
      requestFor(R1, { roleCookieValue: tamperedRoleCookieValue("Approver") }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    expect(calls[0].actor).toEqual({ id: SUB, role: "Engineer" });
  });

  it("4-d: セッション Cookie なし（参照系 R-1）→ 操作者はなし", async () => {
    const { client, calls } = fakeClient();

    await R1.invoke(requestFor(R1, { session: null }), businessOptions(client));

    expect(calls).toHaveLength(1);
    expect(calls[0].actor).toBeNull();
  });

  it("4-e: セッション Cookie なしでブラウザが X-Actor-Id: 別人 と X-Actor-Role: Approver を送る → 操作者はなし", async () => {
    const { client, calls } = fakeClient();

    await R1.invoke(
      requestFor(R1, {
        session: null,
        headers: { "X-Actor-Id": "another-user", "X-Actor-Role": "Approver" },
      }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    expect(calls[0].actor).toBeNull();
  });

  it("4-f: 検証済みセッションありでブラウザが X-Actor-Id: 別人 を送る → 操作者の識別子は sub", async () => {
    const { client, calls } = fakeClient();

    await R1.invoke(
      requestFor(R1, { headers: { "X-Actor-Id": "another-user" } }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    expect(calls[0].actor?.id).toBe(SUB);
  });
});

describe("AC-5-1: 入口のレート制限（検査 5-b）", () => {
  it.each(ALL_BUSINESS_HANDLERS)(
    "5-b: $route は上限を使い切ったキーでは 429 と Retry-After で終わり、lambda-client の Fake を1度も呼ばない",
    async (handler) => {
      const { client, calls } = fakeClient();
      const request = requestFor(handler);
      exhaustSharedLimiterFor(request);

      const response = await handler.invoke(request, businessOptions(client));

      expect(response.status).toBe(429);
      expect(response.headers.get("Retry-After")).not.toBeNull();
      expect(response.headers.get("Cache-Control")).toBe("no-store");
      expect(calls).toHaveLength(0);
    },
  );

  it.each(ALL_BUSINESS_HANDLERS)(
    "5-b（対）: $route は上限に達していなければ 429 にならない（常に 429 を返す実装を緑にしない）",
    async (handler) => {
      const { client } = fakeClient();

      const response = await handler.invoke(requestFor(handler), businessOptions(client));

      expect(response.status).not.toBe(429);
    },
  );
});

describe("AC-5-2: 更新系のゲスト（検査 5-c・5-d・5-e）", () => {
  it.each(UPDATES)("5-c: $route はセッション Cookie なしで 401 になり、Fake が呼ばれない", async (handler) => {
    const { client, calls } = fakeClient();

    const response = await handler.invoke(
      requestFor(handler, { session: null }),
      businessOptions(client),
    );

    expect(response.status).toBe(401);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(calls).toHaveLength(0);
  });

  it.each(UPDATES)(
    "5-d: $route は検証に失敗するセッション Cookie で 401 になり、Fake が呼ばれず、セッション Cookie とロール Cookie を失効させる",
    async (handler) => {
      const { client, calls } = fakeClient();

      const response = await handler.invoke(
        requestFor(handler, { session: buildIdTokenWithBadSignature() }),
        businessOptions(client),
      );

      expect(response.status).toBe(401);
      expect(calls).toHaveLength(0);
      expectAuthCookiesExpired(response);
    },
  );

  it.each(UPDATES)(
    "5-e: $route は検証済みセッションありなら Fake がちょうど1度呼ばれる（常に 401 を返す実装を緑にしない）",
    async (handler) => {
      const { client, calls } = fakeClient();

      const response = await handler.invoke(requestFor(handler), businessOptions(client));

      expect(calls).toHaveLength(1);
      expect(response.status).not.toBe(401);
    },
  );
});

describe("AC-5-3: 参照系のゲスト（検査 5-f）", () => {
  it.each(GUESTABLE)(
    "5-f: $route は検証に失敗するセッション Cookie でも Fake が操作者なしで1度呼ばれ、Cookie を失効させる",
    async (handler) => {
      const { client, calls } = fakeClient();

      const response = await handler.invoke(
        requestFor(handler, { session: buildIdTokenWithBadSignature() }),
        businessOptions(client),
      );

      expect(calls).toHaveLength(1);
      expect(calls[0].actor).toBeNull();
      expectAuthCookiesExpired(response);
    },
  );
});

describe("AC-5-4・5-5: 構成と署名鍵の欠落（検査 5-g・5-h）", () => {
  const missingUrl = [
    ["未設定", environmentWithout("DOMAIN_API_FUNCTION_URL")],
    ["空", environmentWithEmpty("DOMAIN_API_FUNCTION_URL")],
  ] as const;

  const missingKey = [
    ["未設定", environmentWithout("ROLE_COOKIE_SIGNING_KEY")],
    ["空", environmentWithEmpty("ROLE_COOKIE_SIGNING_KEY")],
  ] as const;

  for (const [label, environment] of missingUrl) {
    // R-4 はセッションなしで呼び、デモの技術者識別子は与えたまま呼び先だけを欠かす
    // （bff-wiring.md 5-g。2026-10-27 改定（#56）: 旧 10）。
    it.each(ALL_BUSINESS_HANDLERS)(
      `5-g: 呼び先が${label}のとき $route は 5xx（401・429 でない）・本文が空・Fake が呼ばれない・no-store`,
      async (handler) => {
        const { client, calls } = fakeClient();

        const response = await handler.invoke(
          requestFor(handler, handler.route === "R-4" ? { session: null } : {}),
          businessOptions(client, environment),
        );

        expect(response.status).toBeGreaterThanOrEqual(500);
        expect(response.status).toBeLessThan(600);
        expect(await response.text()).toBe("");
        expect(response.headers.get("Cache-Control")).toBe("no-store");
        expect(calls).toHaveLength(0);
      },
    );
  }

  for (const [label, environment] of missingKey) {
    it.each(BUSINESS_HANDLERS)(
      `5-h: 検証済みセッションありで署名鍵が${label}のとき $route は 5xx・本文が空・Fake が呼ばれない（Engineer として呼ばれない）`,
      async (handler) => {
        const { client, calls } = fakeClient();

        const response = await handler.invoke(
          requestFor(handler),
          businessOptions(client, environment),
        );

        expect(response.status).toBeGreaterThanOrEqual(500);
        expect(response.status).toBeLessThan(600);
        expect(await response.text()).toBe("");
        expect(calls).toHaveLength(0);
      },
    );
  }
});

describe("AC-5-5 (R-4): 署名鍵が欠けていても R-4 は通る（検査 5-h の (R-4)。2026-10-27 追加、#56）", () => {
  const missingKey = [
    ["未設定", environmentWithout("ROLE_COOKIE_SIGNING_KEY")],
    ["空", environmentWithEmpty("ROLE_COOKIE_SIGNING_KEY")],
  ] as const;
  // 契約 E-2 の成功の形の本文（値はダミー。Fake が返したものと深く等しいかだけを見る）。
  const E2_BODY = {
    items: [{ contractId: "contract-dummy-1", yearMonth: "2026-10", state: "Draft" }],
    total: 1,
  };

  for (const [label, environment] of missingKey) {
    it(`5-h (R-4): 検証済みセッションありで署名鍵が${label}でも Fake がちょうど1度呼ばれ、200 で、本文は Fake の本文と深く等しい`, async () => {
      const { client, calls } = fakeClient({ kind: "success", status: 200, body: E2_BODY });

      const response = await DEMO_HANDLER.invoke(
        requestFor(DEMO_HANDLER),
        businessOptions(client, environment),
      );

      expect(calls).toHaveLength(1);
      expect(calls[0]).toStrictEqual({
        endpoint: "E-2",
        query: { engineerId: DEMO_ENGINEER_ID },
        actor: null,
      });
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual(E2_BODY);
    });
  }
});

describe("AC-5-7: 401 の本文は終端仕様の検証失敗の 401 と同じ（検査 5-i）", () => {
  it("5-i: 5-c の 401・6-d の 401 の本文が、サインインの戻りで検証に失敗したときの 401 の本文と一致する", async () => {
    const { client } = fakeClient();
    const updateGuest = await UPDATES[0].invoke(
      requestFor(UPDATES[0], { session: null }),
      businessOptions(client),
    );
    const listGuest = await R2.invoke(
      requestFor(R2, { session: null }),
      businessOptions(client),
    );
    // 終端仕様 AC-11-2 (ii): コードもセッションも持たない戻りは検証に失敗して 401。
    const callback = await handleSignInCallback(
      new Request("https://public.example.test/api/auth/callback"),
      { now: NOW, environment: ENVIRONMENT, publicKeyResolver: fakePublicKeySource },
    );

    expect(callback.status).toBe(401);
    expect(updateGuest.status).toBe(401);
    expect(listGuest.status).toBe(401);
    const expected = await callback.text();
    expect(await updateGuest.text()).toBe(expected);
    expect(await listGuest.text()).toBe(expected);
  });
});

describe("AC-6: 契約のエンドポイントとの対応（Fake が受け取った要求を見る）", () => {
  it.each(BUSINESS_HANDLERS.filter((h) => !["R-2", "R-3", "R-10", "R-11"].includes(h.route)))(
    "6-a: $route は契約形式の識別子で $endpoint を呼ぶ",
    async (handler) => {
      const { client, calls } = fakeClient();

      await handler.invoke(requestFor(handler), businessOptions(client));

      expect(calls).toHaveLength(1);
      const call = calls[0];
      expect(call.endpoint).toBe(handler.endpoint);
      expect(call).toMatchObject({ contractId: "c-001", yearMonth: "2026-07" });
      if (call.endpoint === "E-3" || call.endpoint === "E-4") {
        expect(call).toMatchObject({ date: "2026-07-15" });
      }
    },
  );

  it("6-b: R-2 は要求に engineerId=other を含めても、Fake が受け取った engineerId は sub", async () => {
    const { client, calls } = fakeClient();

    await R2.invoke(
      requestFor(R2, { search: "?engineerId=other&state=Draft&limit=10&offset=0" }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    const call = calls[0];
    expect(call.endpoint).toBe("E-2");
    if (call.endpoint !== "E-2") throw new Error("unreachable");
    expect(call.query.engineerId).toBe(SUB);
    // state・limit・offset は要求から渡す（AC-6 の R-2 の行）。
    expect(call.query).toMatchObject({ state: "Draft", limit: "10", offset: "0" });
  });

  it("6-c: R-3 は要求に engineerId=other と state=Approved を含めても、engineerId を送らず state は PendingApproval", async () => {
    const { client, calls } = fakeClient();

    await R3.invoke(
      requestFor(R3, { search: "?engineerId=other&state=Approved&limit=5&offset=10" }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    const call = calls[0];
    expect(call.endpoint).toBe("E-2");
    if (call.endpoint !== "E-2") throw new Error("unreachable");
    expect(call.query.engineerId).toBeUndefined();
    expect(call.query.state).toBe("PendingApproval");
    // limit・offset は要求から渡す（AC-6 の R-3 の行）。
    expect(call.query).toMatchObject({ limit: "5", offset: "10" });
  });

  it("6-d: R-2 はセッション Cookie なし・engineerId=other でも 401（200 でない）で Fake が呼ばれない", async () => {
    const { client, calls } = fakeClient();

    const response = await R2.invoke(
      requestFor(R2, { session: null, search: "?engineerId=other" }),
      businessOptions(client),
    );

    expect(response.status).toBe(401);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(calls).toHaveLength(0);
  });

  it("6-e: R-5 は業務的に不正な本文（開始が終了より後）でも Fake が1度呼ばれる（BFF が先回りして弾かない）", async () => {
    const { client, calls } = fakeClient();
    const body = JSON.stringify({
      startTime: { hour: 18, minute: 0 },
      endTime: { hour: 9, minute: 0 },
    });

    await BUSINESS_HANDLERS.filter((h) => h.route === "R-5")[0].invoke(
      browserRequest("PUT", { body }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    expect(calls[0].endpoint).toBe("E-3");
  });

  it.each(UPDATES)(
    "6-f: $route を GET で要求しても、状態を変える処理に到達しない（Fake が呼ばれない）",
    async (handler) => {
      const { client, calls } = fakeClient();

      await handler.invoke(browserRequest("GET"), businessOptions(client));

      expect(calls).toHaveLength(0);
    },
  );

  it("6-g: R-2 と R-3 は検証済みセッションありなら、それぞれ Fake がちょうど1度呼ばれる", async () => {
    const own = fakeClient();
    const pending = fakeClient();

    await R2.invoke(requestFor(R2), businessOptions(own.client));
    await R3.invoke(requestFor(R3), businessOptions(pending.client));

    expect(own.calls).toHaveLength(1);
    expect(pending.calls).toHaveLength(1);
  });

  it("6-j: R-2 は検証に失敗するセッション Cookie・engineerId=other で 401、Fake が呼ばれず、Cookie を失効させる", async () => {
    const { client, calls } = fakeClient();

    const response = await R2.invoke(
      requestFor(R2, { session: buildIdTokenWithBadSignature(), search: "?engineerId=other" }),
      businessOptions(client),
    );

    expect(response.status).toBe(401);
    expect(calls).toHaveLength(0);
    expectAuthCookiesExpired(response);
  });
});

// --- E-8・E-9 の読む口（R-10・R-11。2026-10-21 追加。D 節 #109） -----------------

const REFERENCE = [R10, R11];

describe("AC-5-3 順3・5-j: R-10・R-11 のセッションの扱い（検査 5-j）", () => {
  it("母集団: R-10 は E-8、R-11 は E-9 を呼ぶ処理として表に載っている（0件を一致とみなさない）", () => {
    expect(REFERENCE.map((h) => [h.route, h.endpoint, h.method])).toEqual([
      ["R-10", "E-8", "GET"],
      ["R-11", "E-9", "GET"],
    ]);
  });

  it.each(REFERENCE)(
    "5-j (i): $route はセッション Cookie なしで 401 になり、Fake が呼ばれず、本文は 5-i の本文と一致する",
    async (handler) => {
      const { client, calls } = fakeClient();

      const response = await handler.invoke(
        requestFor(handler, { session: null }),
        businessOptions(client),
      );
      const callback = await handleSignInCallback(
        new Request("https://public.example.test/api/auth/callback"),
        { now: NOW, environment: ENVIRONMENT, publicKeyResolver: fakePublicKeySource },
      );

      expect(response.status).toBe(401);
      expect(response.headers.get("Cache-Control")).toBe("no-store");
      expect(calls).toHaveLength(0);
      expect(callback.status).toBe(401);
      expect(await response.text()).toBe(await callback.text());
    },
  );

  it.each(REFERENCE)(
    "5-j (ii): $route は検証に失敗するセッション Cookie で 401 になり、Fake が呼ばれず、Cookie を失効させ、本文は 5-i の本文と一致する",
    async (handler) => {
      const { client, calls } = fakeClient();

      const response = await handler.invoke(
        requestFor(handler, { session: buildIdTokenWithBadSignature() }),
        businessOptions(client),
      );
      const callback = await handleSignInCallback(
        new Request("https://public.example.test/api/auth/callback"),
        { now: NOW, environment: ENVIRONMENT, publicKeyResolver: fakePublicKeySource },
      );

      expect(response.status).toBe(401);
      expect(calls).toHaveLength(0);
      expectAuthCookiesExpired(response);
      expect(await response.text()).toBe(await callback.text());
    },
  );

  it.each(REFERENCE)(
    "5-j (iii): $route は検証済みセッションありなら Fake がちょうど1度呼ばれる（常に 401 を返す実装を緑にしない）",
    async (handler) => {
      const { client, calls } = fakeClient();

      const response = await handler.invoke(requestFor(handler), businessOptions(client));

      expect(calls).toHaveLength(1);
      expect(response.status).toBe(200);
    },
  );
});

describe("AC-6-1: 1対1（検査表の 6-1）", () => {
  it("各 R はちょうど1つの E を呼び、E-1・E-3〜E-9 にはそれぞれちょうど1つの R が対応する（E-2 は R-2・R-3）", async () => {
    const perEndpoint = new Map<string, string[]>();
    for (const handler of BUSINESS_HANDLERS) {
      const { client, calls } = fakeClient();

      await handler.invoke(requestFor(handler), businessOptions(client));

      expect(`${handler.route}: ${calls.length}`).toBe(`${handler.route}: 1`);
      expect(calls[0].endpoint).toBe(handler.endpoint);
      perEndpoint.set(calls[0].endpoint, [...(perEndpoint.get(calls[0].endpoint) ?? []), handler.route]);
    }

    expect(Object.fromEntries(perEndpoint)).toEqual({
      "E-1": ["R-1"],
      "E-2": ["R-2", "R-3"],
      "E-3": ["R-5"],
      "E-4": ["R-6"],
      "E-5": ["R-7"],
      "E-6": ["R-8"],
      "E-7": ["R-9"],
      "E-8": ["R-10"],
      "E-9": ["R-11"],
    });
  });
});

describe("AC-6-4・6-9・6-k・6-l: R-10・R-11 が渡す値", () => {
  it("6-k: R-10 は要求に engineerId=other と yearMonth=2026-10 を含めても、E-8 を engineerId=sub・yearMonth=2026-10 で呼ぶ", async () => {
    const { client, calls } = fakeClient();

    await R10.invoke(
      requestFor(R10, { search: "?engineerId=other&yearMonth=2026-10" }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    const call = calls[0];
    expect(call.endpoint).toBe("E-8");
    if (call.endpoint !== "E-8") throw new Error("unreachable");
    expect(call.query).toEqual({ engineerId: SUB, yearMonth: "2026-10" });
    expect(Object.keys(call.query).sort()).toEqual(["engineerId", "yearMonth"]);
    expect(call.actor).toEqual({ id: SUB, role: "Engineer" });
  });

  it("6-k: R-11 は要求に engineerId=other を含めても、E-9 を engineerId=sub のキーだけで呼ぶ", async () => {
    const { client, calls } = fakeClient();

    await R11.invoke(
      requestFor(R11, { search: "?engineerId=other&yearMonth=2026-10&state=Draft" }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    const call = calls[0];
    expect(call.endpoint).toBe("E-9");
    if (call.endpoint !== "E-9") throw new Error("unreachable");
    expect(Object.keys(call.query)).toEqual(["engineerId"]);
    expect(call.query.engineerId).toBe(SUB);
    expect(call.actor).toEqual({ id: SUB, role: "Engineer" });
  });

  it("6-l: R-10 は yearMonth を含めない要求でも Fake が1度呼ばれ、query に yearMonth が無い（補完も拒否もしない）", async () => {
    const { client, calls } = fakeClient();

    const response = await R10.invoke(
      requestFor(R10, { search: "?engineerId=other" }),
      businessOptions(client),
    );

    expect(calls).toHaveLength(1);
    expect(response.status).not.toBe(400);
    const call = calls[0];
    if (call.endpoint !== "E-8") throw new Error("unreachable");
    expect(call.query.yearMonth).toBeUndefined();
    expect(call.query.engineerId).toBe(SUB);
  });

  it("6-l（対）: クエリが全く無い要求でも同じ", async () => {
    const { client, calls } = fakeClient();

    await R10.invoke(requestFor(R10), businessOptions(client));

    expect(calls).toHaveLength(1);
    const call = calls[0];
    if (call.endpoint !== "E-8") throw new Error("unreachable");
    expect(call.query.yearMonth).toBeUndefined();
  });
});

describe("AC-6-10・6-m: ロールで先回りしない（R-10・R-11 を Approver で呼ぶ）", () => {
  it.each(REFERENCE)(
    "6-m: $route は Approver のセッションでも Fake が1度呼ばれ（操作者のロールは Approver）、403 の FORBIDDEN_NOT_OWNER を error.code だけで写す",
    async (handler) => {
      const { client, calls } = fakeClient({
        kind: "contract-error",
        status: 403,
        code: "FORBIDDEN_NOT_OWNER",
      });

      const response = await handler.invoke(
        requestFor(handler, { roleCookieValue: signedRoleCookieValue("Approver") }),
        businessOptions(client),
      );

      expect(calls).toHaveLength(1);
      expect(calls[0].actor).toEqual({ id: SUB, role: "Approver" });
      expect(response.status).toBe(403);
      expect(await response.json()).toEqual({ error: { code: "FORBIDDEN_NOT_OWNER" } });
    },
  );
});
