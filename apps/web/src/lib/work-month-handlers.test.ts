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
import type { DomainApiRequest, LambdaClient, LambdaClientResult } from "./lambda-client";
import { handleSignInCallback } from "./auth-handlers";
import {
  BUSINESS_HANDLERS,
  businessOptions,
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

/** 受け取った要求を記録し、与えた結果を返す lambda-client の Fake。 */
function fakeClient(
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

const R1 = BUSINESS_HANDLERS.filter((h) => h.route === "R-1")[0];
const R2 = BUSINESS_HANDLERS.filter((h) => h.route === "R-2")[0];
const R3 = BUSINESS_HANDLERS.filter((h) => h.route === "R-3")[0];
const UPDATES = BUSINESS_HANDLERS.filter((h) => h.updates);
const GUESTABLE = [R1, R3];

beforeEach(() => {
  // 終端仕様 AC-11-7 (iv): テスト間で制限器の状態が漏れないようにする。
  routeHandlerRateLimiter.reset();
});

describe("母集団（AC-5-6: R-1〜R-3・R-5〜R-9 の 8 つ）", () => {
  it("検査の表は 8 つの処理を持つ（0件・取りこぼしを一致とみなさない）", () => {
    expect(BUSINESS_HANDLERS.map((h) => h.route)).toEqual([
      "R-1", "R-2", "R-3", "R-5", "R-6", "R-7", "R-8", "R-9",
    ]);
    expect(UPDATES).toHaveLength(5);
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
  it.each(BUSINESS_HANDLERS)(
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

  it.each(BUSINESS_HANDLERS)(
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
    it.each(BUSINESS_HANDLERS)(
      `5-g: 呼び先が${label}のとき $route は 5xx（401・429 でない）・本文が空・Fake が呼ばれない・no-store`,
      async (handler) => {
        const { client, calls } = fakeClient();

        const response = await handler.invoke(
          requestFor(handler),
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
  it.each(BUSINESS_HANDLERS.filter((h) => h.route !== "R-2" && h.route !== "R-3"))(
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
