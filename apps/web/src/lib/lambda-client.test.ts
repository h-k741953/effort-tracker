import { describe, expect, it } from "vitest";
// docs/specs/bff-wiring.md AC-2（SigV4 署名）・AC-3（呼び先と資格情報）・
// AC-6-2（メソッドとパス）・AC-8-1（結果の3分類。検査 8-a〜8-h）の検査。
//
// AC-11-3: 外部へ通信しない。送信は手書きの Fake、時刻は固定値、資格情報は
// ダミー。プロセスの環境変数を書き換えず、環境変数の集まりは引数で与える。
// AC-10-4: Vitest 標準の expect だけを使う（モック／アサーションの追加なし）。
//
// lambda-client.ts が公開するもの（本テストが要求する形。名前の根拠は各所に記す）:
//   createLambdaClient / loadCredentials / signRequest と、型 LambdaClient・
//   SendFunction・DomainApiRequest・LambdaClientResult。
import {
  ENVIRONMENT,
  NOW,
  SECRET_MARK,
  TOKEN_MARK,
  DOMAIN_API_URL,
  environmentWithEmpty,
  environmentWithout,
  headerOf,
  jsonResponse,
  recordingSend,
  signedHeaderNames,
  testClient,
} from "../test-support/bff-wiring";
import type { DomainApiRequest } from "./lambda-client";
import { createLambdaClient, loadCredentials, signRequest } from "./lambda-client";

const ACTOR = { id: "user-a", role: "Engineer" } as const;

const ENCODER = new TextEncoder();

function getWorkMonth(actor: DomainApiRequest["actor"] = ACTOR): DomainApiRequest {
  return { endpoint: "E-1", contractId: "c-001", yearMonth: "2026-07", actor };
}

function putDailyRecord(
  body: string,
  actor: DomainApiRequest["actor"] = ACTOR,
): DomainApiRequest {
  return {
    endpoint: "E-3",
    contractId: "c-001",
    yearMonth: "2026-07",
    date: "2026-07-15",
    body: ENCODER.encode(body),
    actor,
  };
}

// --- AC-2-7 / 検査 2-i・2-j: AWS 公式の SigV4 テストスイート -----------------
//
// 出所: AWS が公開する SigV4 のテストスイート（aws-sig-v4-test-suite）。値は
// smithy-typescript の packages/signature-v4/src/suite.fixture.ts に収録された
// 当該ケースから写した（取得日 2026-10-16。テスト実行時には取得しない＝2-i (ii)）。
//   - ケース名 get-vanilla（セッショントークンを含まない GET の基本形）
//   - ケース名 post-sts-header-before（X-Amz-Security-Token を署名の前に付けて
//     署名対象に含める）
// 資格情報はスイートが例示用に公開しているダミー値（9-3）。
// 一致しないときに期待値・入力を書き換えて合わせない（2-i (iii)）。
const SUITE_REGION = "us-east-1";
const SUITE_SERVICE = "service";
const SUITE_NOW = new Date("2015-08-30T12:36:00Z");
const SUITE_CREDENTIALS = {
  accessKeyId: "AKIDEXAMPLE",
  secretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
};
const SUITE_SESSION_TOKEN =
  "AQoDYXdzEPT//////////wEXAMPLEtc764bNrC9SAPBSM22wDOk4x4HIZ8j4FZTwdQWLWsKWHGBuFqwAeMicRXmxfpSPfIeoIYRqTflfKD8YUuwthAx7mSEI/qkPpKPi/kMcGdQrmGdeehM4IC1NtBmUpp2wUE8phUZampKsburEDy0KPkyQDYwT7WZ0wq5VSXDvp75YU9HFvlRd8Tx6q6fE8YQcHNVXAkiY9q6d+xo0rKwT38xVqr7ZD0u0iPPkUL64lIZbqBAz+scqKmlzm8FDrypNC9Yjc8fPOLn9FX9KSYvKTr4rvx3iSIlTJabIQwj2ICCR/oLxBA==";

function authorizationOf(headers: Readonly<Record<string, string>>): string | undefined {
  return headerOf(headers, "authorization");
}

describe("AC-2-7: 署名を単体で呼べる関数（公式テストベクタ）", () => {
  it("2-i: get-vanilla（セッショントークンなし）の Authorization がスイートの期待値と一致する", async () => {
    const signed = await signRequest({
      service: SUITE_SERVICE,
      region: SUITE_REGION,
      credentials: SUITE_CREDENTIALS,
      now: SUITE_NOW,
      request: { method: "GET", url: "https://example.amazonaws.com/", headers: {} },
    });

    expect(authorizationOf(signed)).toBe(
      "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31",
    );
  });

  it("2-j: post-sts-header-before（トークンを署名の前に付けて署名対象に含める）の Authorization がスイートの期待値と一致する", async () => {
    const signed = await signRequest({
      service: SUITE_SERVICE,
      region: SUITE_REGION,
      credentials: { ...SUITE_CREDENTIALS, sessionToken: SUITE_SESSION_TOKEN },
      now: SUITE_NOW,
      request: { method: "POST", url: "https://example.amazonaws.com/", headers: {} },
    });

    expect(authorizationOf(signed)).toBe(
      "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=85d96828115b5dc0cfc3bd16ad9e210dd772bbebba041836c64533a82be05ead",
    );
  });
});

// --- AC-2 検査 2-a〜2-h（送信関数の Fake 越し） -------------------------------

describe("AC-2: 署名（lambda-client の送信経路）", () => {
  async function sendOne(
    request: DomainApiRequest,
    environment = ENVIRONMENT,
  ): Promise<Readonly<Record<string, string>>> {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    await testClient({ send, environment }).call(request);
    expect(sent).toHaveLength(1);
    return sent[0].init.headers;
  }

  it("2-a: 同じ要求・同じ資格情報・同じ時刻で2回署名すると Authorization が一致する", async () => {
    const first = await sendOne(getWorkMonth());
    const second = await sendOne(getWorkMonth());

    expect(authorizationOf(first)).toBeDefined();
    expect(authorizationOf(first)).toBe(authorizationOf(second));
  });

  it("2-b: X-Actor-Id の値だけが異なる2つの要求は署名が異なる", async () => {
    const a = await sendOne(getWorkMonth({ id: "user-a", role: "Engineer" }));
    const b = await sendOne(getWorkMonth({ id: "user-b", role: "Engineer" }));

    expect(authorizationOf(a)).not.toBe(authorizationOf(b));
  });

  it("2-c: X-Actor-Role の値だけが異なる2つの要求は署名が異なる", async () => {
    const engineer = await sendOne(getWorkMonth({ id: "user-a", role: "Engineer" }));
    const approver = await sendOne(getWorkMonth({ id: "user-a", role: "Approver" }));

    expect(authorizationOf(engineer)).not.toBe(authorizationOf(approver));
  });

  it("2-d: 操作者ありの要求は SignedHeaders が x-actor-id と x-actor-role を含む（値は送った要求に載る）", async () => {
    const headers = await sendOne(getWorkMonth({ id: "user-a", role: "Approver" }));

    const names = signedHeaderNames(authorizationOf(headers));
    expect(names).toContain("x-actor-id");
    expect(names).toContain("x-actor-role");
    expect(headerOf(headers, "x-actor-id")).toBe("user-a");
    expect(headerOf(headers, "x-actor-role")).toBe("Approver");
  });

  it("2-e: 操作者なし（ゲスト）の要求は X-Actor-Id・X-Actor-Role を送らず、SignedHeaders にも現れない", async () => {
    const headers = await sendOne(getWorkMonth(null));

    expect(headerOf(headers, "x-actor-id")).toBeUndefined();
    expect(headerOf(headers, "x-actor-role")).toBeUndefined();
    const names = signedHeaderNames(authorizationOf(headers));
    expect(names.length).toBeGreaterThan(0);
    expect(names).not.toContain("x-actor-id");
    expect(names).not.toContain("x-actor-role");
  });

  it("2-f: E-3 で本文だけが異なる2つの要求は署名が異なる", async () => {
    const a = await sendOne(putDailyRecord('{"a":1}'));
    const b = await sendOne(putDailyRecord('{"a":2}'));

    expect(authorizationOf(a)).not.toBe(authorizationOf(b));
  });

  it("2-g: 送った要求の X-Amz-Date が与えた時刻を表す", async () => {
    const headers = await sendOne(getWorkMonth());

    // NOW = 2026-10-16T01:02:03Z
    expect(headerOf(headers, "x-amz-date")).toBe("20261016T010203Z");
  });

  it("2-h: セッショントークンが X-Amz-Security-Token として付き署名対象に含まれ、トークンの値だけが違えば署名も違う", async () => {
    const headers = await sendOne(getWorkMonth());
    const other = await sendOne(getWorkMonth(), {
      ...ENVIRONMENT,
      AWS_SESSION_TOKEN: "DUMMY-OTHER-TOKEN",
    });

    expect(headerOf(headers, "x-amz-security-token")).toBe(TOKEN_MARK);
    expect(signedHeaderNames(authorizationOf(headers))).toContain(
      "x-amz-security-token",
    );
    expect(authorizationOf(headers)).not.toBe(authorizationOf(other));
  });

  it("2-2: host と x-amz-date も署名対象に含まれる", async () => {
    const headers = await sendOne(getWorkMonth());

    const names = signedHeaderNames(authorizationOf(headers));
    expect(names).toContain("host");
    expect(names).toContain("x-amz-date");
  });
});

// --- AC-3 検査 3-a〜3-h ------------------------------------------------------

describe("AC-3: 呼び先と資格情報の構成", () => {
  const fullRequests: DomainApiRequest[] = [
    getWorkMonth(),
    { endpoint: "E-2", query: { engineerId: "user-a" }, actor: null },
    putDailyRecord("{}"),
    { endpoint: "E-4", contractId: "c-001", yearMonth: "2026-07", date: "2026-07-15", actor: ACTOR },
    { endpoint: "E-5", contractId: "c-001", yearMonth: "2026-07", actor: ACTOR },
    { endpoint: "E-6", contractId: "c-001", yearMonth: "2026-07", actor: ACTOR },
    { endpoint: "E-7", contractId: "c-001", yearMonth: "2026-07", actor: ACTOR },
  ];

  it.each([
    ["3-a 未設定", environmentWithout("DOMAIN_API_FUNCTION_URL")],
    ["3-b 空文字", environmentWithEmpty("DOMAIN_API_FUNCTION_URL")],
  ])("%s: 送信関数が1度も呼ばれず、結果は失敗（契約外）", async (_name, environment) => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));

    const result = await testClient({ send, environment }).call(getWorkMonth());

    expect(sent).toHaveLength(0);
    expect(result.kind).toBe("unexpected");
  });

  it("3-c: 資格情報の取得が失敗する Fake（失敗の結果を返す）では送信関数が1度も呼ばれない", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    const client = createLambdaClient({
      environment: ENVIRONMENT,
      send,
      now: () => NOW,
      timeoutMilliseconds: 1000,
      getCredentials: async () => ({ ok: false, missing: ["AWS_ACCESS_KEY_ID"] }),
    });

    const result = await client.call(getWorkMonth());

    expect(sent).toHaveLength(0);
    expect(result.kind).toBe("unexpected");
  });

  it("3-c: 資格情報の取得が例外を投げる Fake でも送信関数は呼ばれず、例外は外へ出ない（8-3）", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    const client = createLambdaClient({
      environment: ENVIRONMENT,
      send,
      now: () => NOW,
      timeoutMilliseconds: 1000,
      getCredentials: async () => {
        throw new Error("credential provider failure");
      },
    });

    const result = await client.call(getWorkMonth());

    expect(sent).toHaveLength(0);
    expect(result.kind).toBe("unexpected");
  });

  it("3-d: 構成と資格情報がそろっていれば送信関数がちょうど1度呼ばれ、宛先のオリジンが構成の Function URL のオリジンに一致する", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));

    await testClient({ send }).call(getWorkMonth());

    expect(sent).toHaveLength(1);
    expect(new URL(sent[0].url).origin).toBe(new URL(DOMAIN_API_URL).origin);
  });

  it("3-e: 送った全要求（E-1〜E-7）が SigV4 の Authorization を持つ", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));
    const client = testClient({ send });

    for (const request of fullRequests) {
      await client.call(request);
    }

    expect(sent).toHaveLength(fullRequests.length);
    for (const entry of sent) {
      expect(authorizationOf(entry.init.headers)).toMatch(/^AWS4-HMAC-SHA256 /);
    }
  });

  it("3-f: 3つの値がそろった環境変数の集まりから、与えた3つの値がそのまま資格情報になる", () => {
    const result = loadCredentials(ENVIRONMENT);

    expect(result).toEqual({
      ok: true,
      value: {
        accessKeyId: ENVIRONMENT.AWS_ACCESS_KEY_ID,
        secretAccessKey: SECRET_MARK,
        sessionToken: TOKEN_MARK,
      },
    });
  });

  const missingAccessOrSecret = [
    ["アクセスキー ID が未設定", environmentWithout("AWS_ACCESS_KEY_ID")],
    ["アクセスキー ID が空", environmentWithEmpty("AWS_ACCESS_KEY_ID")],
    ["シークレットアクセスキーが未設定", environmentWithout("AWS_SECRET_ACCESS_KEY")],
    ["シークレットアクセスキーが空", environmentWithEmpty("AWS_SECRET_ACCESS_KEY")],
  ] as const;

  it.each(missingAccessOrSecret)(
    "3-g: %s のとき資格情報は得られず、送信関数は1度も呼ばれない",
    async (_name, environment) => {
      expect(loadCredentials(environment).ok).toBe(false);
      const { send, sent } = recordingSend(() => jsonResponse(200, {}));

      const result = await testClient({ send, environment }).call(getWorkMonth());

      expect(sent).toHaveLength(0);
      expect(result.kind).toBe("unexpected");
    },
  );

  const missingToken = [
    ["セッショントークンが未設定", environmentWithout("AWS_SESSION_TOKEN")],
    ["セッショントークンが空", environmentWithEmpty("AWS_SESSION_TOKEN")],
  ] as const;

  it.each(missingToken)(
    "3-h: %s のとき資格情報は得られず（拒否する）、送信関数は1度も呼ばれない",
    async (_name, environment) => {
      expect(loadCredentials(environment).ok).toBe(false);
      const { send, sent } = recordingSend(() => jsonResponse(200, {}));

      const result = await testClient({ send, environment }).call(getWorkMonth());

      expect(sent).toHaveLength(0);
      expect(result.kind).toBe("unexpected");
    },
  );
});

// --- AC-6-2: lambda-client が送るメソッドとパス ------------------------------

describe("AC-6-2: lambda-client が送る要求のメソッドとパス（契約のエンドポイント一覧どおり）", () => {
  // 出所: docs/specs/domain-api-http-contract.md「エンドポイント一覧」の E-1〜E-7
  // （プレフィックスを付けない＝契約 AC-1-1）。
  const table: Array<[string, DomainApiRequest, string, string]> = [
    ["E-1", getWorkMonth(), "GET", "/work-months/c-001/2026-07"],
    ["E-2", { endpoint: "E-2", query: {}, actor: null }, "GET", "/work-months"],
    ["E-3", putDailyRecord("{}"), "PUT", "/work-months/c-001/2026-07/daily-records/2026-07-15"],
    [
      "E-4",
      { endpoint: "E-4", contractId: "c-001", yearMonth: "2026-07", date: "2026-07-15", actor: ACTOR },
      "DELETE",
      "/work-months/c-001/2026-07/daily-records/2026-07-15",
    ],
    [
      "E-5",
      { endpoint: "E-5", contractId: "c-001", yearMonth: "2026-07", actor: ACTOR },
      "POST",
      "/work-months/c-001/2026-07/close",
    ],
    [
      "E-6",
      { endpoint: "E-6", contractId: "c-001", yearMonth: "2026-07", actor: ACTOR },
      "POST",
      "/work-months/c-001/2026-07/approve",
    ],
    [
      "E-7",
      { endpoint: "E-7", contractId: "c-001", yearMonth: "2026-07", actor: ACTOR },
      "POST",
      "/work-months/c-001/2026-07/reject",
    ],
  ];

  it.each(table)("%s のメソッドとパス", async (_name, request, method, pathname) => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));

    await testClient({ send }).call(request);

    expect(sent).toHaveLength(1);
    expect(sent[0].init.method).toBe(method);
    expect(new URL(sent[0].url).pathname).toBe(pathname);
  });

  it("E-2 のクエリ（engineerId・state・limit・offset）が値として送られる", async () => {
    const { send, sent } = recordingSend(() => jsonResponse(200, {}));

    await testClient({ send }).call({
      endpoint: "E-2",
      query: { engineerId: "user-a", state: "Draft", limit: "10", offset: "20" },
      actor: null,
    });

    const params = new URL(sent[0].url).searchParams;
    expect(params.get("engineerId")).toBe("user-a");
    expect(params.get("state")).toBe("Draft");
    expect(params.get("limit")).toBe("10");
    expect(params.get("offset")).toBe("20");
  });
});

// --- AC-8 検査 8-a〜8-h（結果の分類） -----------------------------------------

describe("AC-8-1: 結果の3分類（送信関数の Fake の応答を変えて分類を見る）", () => {
  async function classify(
    respond: () => Response | Promise<Response>,
  ) {
    const { send } = recordingSend(respond);
    return testClient({ send }).call(getWorkMonth());
  }

  const WORK_MONTH = { contractId: "c-001", yearMonth: "2026-07", state: "Draft" };

  it("8-a: 200・JSON は (i) 契約の成功", async () => {
    const result = await classify(() => jsonResponse(200, WORK_MONTH));

    expect(result).toEqual({ kind: "success", status: 200, body: WORK_MONTH });
  });

  it("8-b: 409・INVALID_STATE_FOR_CLOSE は (ii) 契約のエラーで code が取り出せる", async () => {
    const result = await classify(() =>
      jsonResponse(409, { error: { code: "INVALID_STATE_FOR_CLOSE", message: "x" } }),
    );

    expect(result).toEqual({
      kind: "contract-error",
      status: 409,
      code: "INVALID_STATE_FOR_CLOSE",
    });
  });

  it("8-c: 403・IAM の拒否の形は (iii) 契約外", async () => {
    const result = await classify(() => jsonResponse(403, { Message: "Forbidden" }));

    expect(result.kind).toBe("unexpected");
  });

  it("8-d: 403・表に無い code は (iii) 契約外", async () => {
    const result = await classify(() =>
      jsonResponse(403, { error: { code: "UNKNOWN_CODE", message: "x" } }),
    );

    expect(result.kind).toBe("unexpected");
  });

  it("8-e: 409・FORBIDDEN_NOT_OWNER（ステータスと code が表と食い違う）は (iii) 契約外", async () => {
    const result = await classify(() =>
      jsonResponse(409, { error: { code: "FORBIDDEN_NOT_OWNER", message: "x" } }),
    );

    expect(result.kind).toBe("unexpected");
  });

  it("8-f: 429・本文なしは (iii) 契約外で、429 であったことが区別される", async () => {
    const result = await classify(() => new Response(null, { status: 429 }));

    expect(result.kind).toBe("unexpected");
    expect(result).toMatchObject({ rateLimited: true });
  });

  it("8-c・8-d・8-g の契約外は 429 として区別されない（8-f と対）", async () => {
    const results = await Promise.all([
      classify(() => jsonResponse(403, { Message: "Forbidden" })),
      classify(() => jsonResponse(403, { error: { code: "UNKNOWN_CODE", message: "x" } })),
      classify(() => new Response("not json", { status: 200 })),
    ]);

    for (const result of results) {
      expect(result).toMatchObject({ kind: "unexpected", rateLimited: false });
    }
  });

  it("8-g: 200・JSON でない本文は (iii) 契約外", async () => {
    const result = await classify(() => new Response("not json", { status: 200 }));

    expect(result.kind).toBe("unexpected");
  });

  it("8-h: 送信関数が例外を投げても (iii) 契約外で返り、例外は lambda-client の外へ出ない", async () => {
    const { send } = recordingSend(() => {
      throw new Error("network failure");
    });

    const result = await testClient({ send }).call(getWorkMonth());

    expect(result.kind).toBe("unexpected");
  });
});
