import { describe, expect, it } from "vitest";
import {
  createHmac,
  generateKeyPairSync,
  sign as cryptoSign,
} from "node:crypto";
// デモ用ロール切替（docs/specs/bff-auth-termination.md AC-11-2 (iii)・AC-5-5）の
// うち、**要求が運んできたロールを採用しないこと**（5-5 (ii)）を検査する。
//
// AC-5-5 (ii) は「**切替を要求できるのは BFF の経路だけ**とし、**要求ヘッダ・
// クエリ・本文で与えられたロールを採用しない**」である。したがって、切替の
// 結果は要求が名乗った値に左右されてはならない。
//
// 切替後の値が一意に決まる根拠 —— (1) ロールは `Engineer` / `Approver` の
// **2つだけ**（5-3。`Guest` というロール値を作らない）、(2) 要求はロールの値を
// 運べない（5-5 (ii)）、(3) 本経路は**切替**である（11-2 (iii)・`login-ui.md`
// AC-6-1「`Engineer` / `Approver` を切り替えられる」）。値が2つしかない集合で、
// 要求から値を受け取らずに「切り替える」なら、**現在の検証済みロールのもう
// 一方**へ移る以外にない。**切り替わらない実装（常に同じロールを発行する）は
// 11-2 (iii) を満たさない**。
//
// AC-8-1 / AC-7-3: Vitest 標準の expect だけを使う。モックライブラリを入れない。
// AC-8-3: 時刻は引数で与え、外部へ通信しない（公開鍵の取得は手書きの Fake）。
// AC-8-4 / AC-4-4: 鍵・トークンはテスト内で組み立てるか明らかなダミーにする。
import type { Environment } from "./auth-config";
import { handleRoleSwitch } from "./auth-handlers";
import type { Jwk, PublicKeyResolver } from "./jwt-verifier";
import type { Role } from "./role-cookie";
import { ROLE_COOKIE_NAME, verifyRoleCookie } from "./role-cookie";
import { SESSION_COOKIE_NAME } from "./session-cookie";

const REGION = "ap-northeast-1";
const USER_POOL_ID = "ap-northeast-1_dummyPool";
const CLIENT_ID = "dummy0000000000000000000000client";
const ISSUER = `https://cognito-idp.${REGION}.amazonaws.com/${USER_POOL_ID}`;
const SUB = "11111111-1111-1111-1111-111111111111";
const SIGNING_KEY = "dummy-role-cookie-signing-key-0123456789";
/** AC-5-11 検査（5-c・5-d）専用: 別の鍵で署名された Cookie を作るためだけに使う。 */
const OTHER_SIGNING_KEY = "dummy-other-signing-key-0000000000";

const ENVIRONMENT: Environment = {
  COGNITO_REGION: REGION,
  COGNITO_USER_POOL_ID: USER_POOL_ID,
  COGNITO_CLIENT_ID: CLIENT_ID,
  COGNITO_DOMAIN_PREFIX: "effort-tracker-dummy",
  PUBLIC_ORIGIN: "https://public.example.test",
  ROLE_COOKIE_SIGNING_KEY: SIGNING_KEY,
};

const ROLE_SWITCH_URL = "https://public.example.test/api/auth/role";

// AC-2-11: 現在時刻は呼ぶ側から与える。
const NOW = new Date("2026-09-01T00:00:00.000Z");
const NOW_EPOCH_SECONDS = Math.floor(NOW.getTime() / 1000);

// --- 鍵とトークン（AC-8-4） -------------------------------------------------

const { publicKey, privateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
});
const PUBLIC_JWK = publicKey.export({ format: "jwk" }) as unknown as Jwk;

function base64url(input: string | Buffer): string {
  return (typeof input === "string" ? Buffer.from(input) : input).toString(
    "base64url",
  );
}

function buildIdToken(payloadOverrides: Record<string, unknown> = {}): string {
  const header = { alg: "RS256", typ: "JWT", kid: "dummy-key-id-0001" };
  const payload = {
    iss: ISSUER,
    aud: CLIENT_ID,
    sub: SUB,
    token_use: "id",
    iat: NOW_EPOCH_SECONDS - 10,
    exp: NOW_EPOCH_SECONDS + 3600,
    ...payloadOverrides,
  };
  const signingInput = `${base64url(JSON.stringify(header))}.${base64url(
    JSON.stringify(payload),
  )}`;
  return `${signingInput}.${base64url(
    cryptoSign("RSA-SHA256", Buffer.from(signingInput), privateKey),
  )}`;
}

const fakePublicKeySource: PublicKeyResolver = {
  async getKey({ issuer }: { issuer: string; kid: string | undefined }) {
    if (issuer !== ISSUER) {
      throw new Error(
        "fakePublicKeySource: 想定外の issuer で鍵を要求された（AC-2-10）",
      );
    }
    return PUBLIC_JWK;
  },
};

/** role-cookie.test.ts が固定しているワイヤーフォーマットに合わせる。 */
function signedRoleCookieValue(role: Role): string {
  const signature = createHmac("sha256", SIGNING_KEY)
    .update(role)
    .digest("base64url");
  return `${role}.${signature}`;
}

// --- AC-5-11 検査用: 検証できないロール Cookie の生の値（5-c・5-d） ---------
//
// 「Approver を主張するが検証できない」の3通り（5-5 (i)(iii)。role-cookie.test.ts
// の buildCookieValue と同じ組み立て方）。

/** 改竄された署名。 */
function tamperedRoleCookieValue(role: Role): string {
  return `${signedRoleCookieValue(role)}x`;
}

/** 別の鍵で署名された。 */
function roleCookieValueSignedWithOtherKey(role: Role): string {
  const signature = createHmac("sha256", OTHER_SIGNING_KEY)
    .update(role)
    .digest("base64url");
  return `${role}.${signature}`;
}

/** 署名が欠けた（ロールのみ）。 */
function roleCookieValueWithoutSignature(role: Role): string {
  return role;
}

// --- 要求の組み立て ---------------------------------------------------------

interface RoleSwitchRequestOptions {
  /** 検証済みセッション（省略時は正当な ID トークン）。null で持たせない。 */
  readonly idToken?: string | null;
  /** 現在の（署名済み）ロール。null で持たせない。 */
  readonly currentRole?: Role | null;
  /**
   * AC-5-11（5-c・5-d）検査用: 正当な署名を経ない、生のロール Cookie 値を
   * そのまま載せる。指定された場合、currentRole より優先する。
   */
  readonly rawRoleCookieValue?: string;
  /** 要求が名乗るロール（5-5 (ii)。採用されてはならない）。 */
  readonly declaredRole?: string;
  /** 名乗りをどこへ載せるか。 */
  readonly declaredVia?: "json" | "form" | "header" | "query";
}

function roleSwitchRequest(options: RoleSwitchRequestOptions = {}): Request {
  const idToken = options.idToken === undefined ? buildIdToken() : options.idToken;
  const currentRole =
    options.currentRole === undefined ? "Engineer" : options.currentRole;

  const cookies: string[] = [];
  if (idToken !== null) {
    cookies.push(`${SESSION_COOKIE_NAME}=${idToken}`);
  }
  if (options.rawRoleCookieValue !== undefined) {
    cookies.push(`${ROLE_COOKIE_NAME}=${options.rawRoleCookieValue}`);
  } else if (currentRole !== null) {
    cookies.push(`${ROLE_COOKIE_NAME}=${signedRoleCookieValue(currentRole)}`);
  }

  const headers = new Headers();
  if (cookies.length > 0) {
    headers.set("Cookie", cookies.join("; "));
  }

  const url = new URL(ROLE_SWITCH_URL);
  let body: BodyInit | undefined;

  if (options.declaredRole !== undefined) {
    switch (options.declaredVia) {
      case "json":
        headers.set("Content-Type", "application/json");
        body = JSON.stringify({ role: options.declaredRole });
        break;
      case "form":
        headers.set("Content-Type", "application/x-www-form-urlencoded");
        body = new URLSearchParams({ role: options.declaredRole }).toString();
        break;
      case "header":
        headers.set("X-Role", options.declaredRole);
        break;
      case "query":
        url.searchParams.set("role", options.declaredRole);
        break;
      default:
        break;
    }
  }

  return new Request(url.toString(), { method: "POST", headers, body });
}

function switchRole(request: Request): Promise<Response> {
  return handleRoleSwitch(request, {
    now: NOW,
    environment: ENVIRONMENT,
    publicKeyResolver: fakePublicKeySource,
  });
}

/** 発行されたロール Cookie を BFF 自身の検証にかけて読む（5-5 (i)）。 */
function issuedRole(response: Response): Role | undefined {
  const header = response.headers
    .getSetCookie()
    .find((cookie) => cookie.startsWith(`${ROLE_COOKIE_NAME}=`));
  if (header === undefined) {
    return undefined;
  }
  const value = header.slice(ROLE_COOKIE_NAME.length + 1).split(";")[0];
  if (value.length === 0) {
    return undefined;
  }
  const verified = verifyRoleCookie({
    cookieValue: value,
    signingKey: SIGNING_KEY,
  });
  return verified.ok ? verified.role : undefined;
}

// --- 切替そのもの（11-2 (iii)・login-ui.md AC-6-1） -------------------------

describe("ロール切替: 要求がロールの値を運ばなくても切り替わる", () => {
  const cases: Array<{ currentRole: Role; expected: Role }> = [
    { currentRole: "Engineer", expected: "Approver" },
    { currentRole: "Approver", expected: "Engineer" },
  ];

  it.each(cases)(
    "現在の検証済みロールが $currentRole のとき、$expected へ切り替わる",
    async ({ currentRole, expected }) => {
      const response = await switchRole(roleSwitchRequest({ currentRole }));

      expect(issuedRole(response)).toBe(expected);
    },
  );
});

// --- 5-5 (ii): 要求で与えられたロールを採用しない ---------------------------

describe("ロール切替: 要求ヘッダ・クエリ・本文で与えられたロールを採用しない（5-5 (ii)）", () => {
  const declaredVia: Array<RoleSwitchRequestOptions["declaredVia"]> = [
    "json",
    "form",
    "header",
    "query",
  ];

  // 名乗りが**現在のロールと同じ**場合を使う。名乗りが採用される実装なら結果は
  // 現在のロールのまま（＝切り替わらない）になり、採用されない実装なら
  // もう一方へ切り替わる。両者が必ず食い違うため、この形は空虚にならない。
  const cases = declaredVia.flatMap((via) => [
    {
      name: `${via}: 現在 Engineer で "Engineer" を名乗る`,
      via,
      currentRole: "Engineer" as Role,
      declaredRole: "Engineer",
      expected: "Approver" as Role,
    },
    {
      name: `${via}: 現在 Approver で "Approver" を名乗る`,
      via,
      currentRole: "Approver" as Role,
      declaredRole: "Approver",
      expected: "Engineer" as Role,
    },
  ]);

  it.each(cases)(
    "$name —— 名乗りは採用されず、$expected が発行される",
    async ({ via, currentRole, declaredRole, expected }) => {
      const response = await switchRole(
        roleSwitchRequest({ currentRole, declaredRole, declaredVia: via }),
      );

      expect(issuedRole(response)).toBe(expected);
    },
  );

  it.each(declaredVia)(
    "%s: 名乗る値だけを変えた2つの要求は、同じ結果になる（名乗りが結果を左右しない）",
    async (via) => {
      const asEngineer = await switchRole(
        roleSwitchRequest({
          currentRole: "Engineer",
          declaredRole: "Engineer",
          declaredVia: via,
        }),
      );
      const asApprover = await switchRole(
        roleSwitchRequest({
          currentRole: "Engineer",
          declaredRole: "Approver",
          declaredVia: via,
        }),
      );

      // 空虚な真にしないため、いずれの結果も「発行されている」ことを見る。
      expect(issuedRole(asEngineer)).toBeDefined();
      expect(issuedRole(asApprover)).toBe(issuedRole(asEngineer));
    },
  );

  it("許可外のロールを名乗っても、切替そのものは名乗りに左右されない（5-5 (iii)）", async () => {
    const response = await switchRole(
      roleSwitchRequest({
        currentRole: "Engineer",
        declaredRole: "Administrator",
        declaredVia: "json",
      }),
    );

    // 発行されるとすれば Approver（切替の結果）だけであり、
    // 名乗った "Administrator" が採用されることはない。
    expect(issuedRole(response)).toBe("Approver");
  });
});

// --- 5-5 (v): ゲスト（未認証）には切替を与えない ---------------------------

describe("ロール切替: 検証済みセッションが無ければ切替を与えない（5-5 (v)）", () => {
  const cases: Array<{ name: string; request: () => Request }> = [
    {
      name: "セッション Cookie が無い（ゲスト）",
      request: () => roleSwitchRequest({ idToken: null, currentRole: null }),
    },
    {
      name: "セッション Cookie が空",
      request: () => roleSwitchRequest({ idToken: "", currentRole: null }),
    },
    {
      name: "セッションのトークンが AC-2 の検証を通らない（期限切れ）",
      request: () =>
        roleSwitchRequest({
          idToken: buildIdToken({ exp: NOW_EPOCH_SECONDS - 1 }),
          currentRole: "Engineer",
        }),
    },
    {
      name: "セッション Cookie は無いが、署名済みロール Cookie だけがある",
      request: () =>
        roleSwitchRequest({ idToken: null, currentRole: "Engineer" }),
    },
  ];

  it.each(cases)("$name のときロール Cookie を発行しない", async ({ request }) => {
    const response = await switchRole(request());

    expect(issuedRole(response)).toBeUndefined();
    // 3-4 (i): 状態を変える操作へ至る Route Handler は未認証に 401 を返す。
    // ここまで見ないと、**申告が無いから 400 になっただけ**の応答と区別できず、
    // セッションの検証を外しても落ちないテストになる（AC-8-5）。
    expect(response.status).toBe(401);
  });
});

// --- AC-5-11: デモ用ロール切替の反転元（既定のロール） ----------------------
//
// 検査（Vitest）— 5-11 の反転元。下表がそのままテストの入出力になる
// （bff-auth-termination.md「検査（Vitest）— 5-11 の反転元」）。5-a・5-c・5-e・
// 5-f（resolveEffectiveRole の直接検査）は role-cookie.test.ts が持つ。ここでは
// 反転そのもの（5-b・5-d）と、ゲストへの不適用（5-g）を handleRoleSwitch 越しに
// 検査する。
//
// **Red を先に踏む**（AC-8-2）。**反転後の値だけを見て緑にしない** ——
// 切替を要求した場合だけを検査すると、Cookie の主張を反転元に採る実装も緑に
// なりうる（AC-8-5 と同じ理由。5-c と 5-d を対で検査する。5-c は role-cookie
// 側にある）。**拒否の文言は検査しない**（AC-2 前文）。

describe("ロール切替: ロール Cookie が無いときの反転元は Engineer（AC-5-11・5-b）", () => {
  // 5-5 (ii) により申告は無視されるべきだが、**申告が採用される実装でも
  // 偶然 Approver が一致して緑になる**ことを避けるため、あえて逆の
  // "Engineer" を名乗らせる。反転元が Engineer から正しく反転していれば、
  // 申告に関わらず Approver になる。
  it("ロール Cookie が無い状態で切替を1回要求すると Approver になる（反転元が Engineer であること）", async () => {
    const response = await switchRole(
      roleSwitchRequest({
        currentRole: null,
        declaredRole: "Engineer",
        declaredVia: "json",
      }),
    );

    expect(issuedRole(response)).toBe("Approver");
  });
});

describe("ロール切替: 検証できない Approver 主張の Cookie を反転元に採らない（AC-5-11・5-d）", () => {
  const cases: Array<{ name: string; rawRoleCookieValue: string }> = [
    {
      name: "改竄された署名",
      rawRoleCookieValue: tamperedRoleCookieValue("Approver"),
    },
    {
      name: "別の鍵で署名された",
      rawRoleCookieValue: roleCookieValueSignedWithOtherKey("Approver"),
    },
    {
      name: "署名が欠けた",
      rawRoleCookieValue: roleCookieValueWithoutSignature("Approver"),
    },
  ];

  it.each(cases)(
    "$name の Approver 主張 Cookie がある状態で切替を1回要求すると Approver になる —— Engineer にならない（5-11 (ii)）",
    async ({ rawRoleCookieValue }) => {
      const response = await switchRole(
        roleSwitchRequest({
          rawRoleCookieValue,
          declaredRole: "Engineer",
          declaredVia: "json",
        }),
      );

      // Engineer になるのは Cookie の主張（Approver）を反転元に採った場合で
      // あり、5-11 (ii) に反する。反転元は Engineer であるべきなので、
      // 反転結果は Approver になる。
      expect(issuedRole(response)).toBe("Approver");
    },
  );
});

describe("ロール切替: ゲストには本行を適用しない（AC-5-11 (v)・5-g）", () => {
  it("検証済みセッションが無い要求は、切替を要求しても既定として Engineer を与えず、ロール Cookie を発行しない", async () => {
    const response = await switchRole(
      roleSwitchRequest({
        idToken: null,
        currentRole: null,
        declaredRole: "Approver",
        declaredVia: "json",
      }),
    );

    expect(issuedRole(response)).toBeUndefined();
    // 5-3・P-3: ゲストは両ヘッダの不在で表す。5-11 (v) はゲストへ既定として
    // Engineer を与えない —— 3-4 (i) の 401 で未認証であることを確認する。
    expect(response.status).toBe(401);
  });
});
