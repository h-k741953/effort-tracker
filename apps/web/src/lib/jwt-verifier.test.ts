import { describe, expect, it } from "vitest";
import { generateKeyPairSync, sign as cryptoSign } from "node:crypto";
// AC-8-2（docs/specs/bff-auth-termination.md）: 実装より先にテストを書き、
// Red を確認してから実装へ入る。本ファイルが対象とするモジュール
// （./jwt-verifier）はまだ存在しない。これは意図した Red である。
//
// ファイル名・関数名・型名はこのテストが暫定的に固定するインターフェース
// である（`docs/specs/bff-auth-termination.md` AC-1-3 は「具体のファイル名は
// 本仕様で固定しない」としているため、tester が単体で呼べる最小の形を仮に
// 決める）。実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。
//
// AC-7-2: 公開鍵の取得は「呼ぶ側が宣言する最小のインターフェース」越しに
// 受け取る。ここでは PublicKeyResolver がそれであり、テストは手書きの
// インメモリ Fake（fakePublicKeySource）で差し替える。ネットワークへは
// 出ない（AC-8-3）。
import type { Jwk, PublicKeyResolver } from "./jwt-verifier";
import { verifyToken } from "./jwt-verifier";

// --- テスト用の鍵とトークンの組み立て（AC-8-4: 明らかなダミー） -----------

const { publicKey, privateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
});
const { privateKey: otherPrivateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
});

const PUBLIC_JWK = publicKey.export({ format: "jwk" }) as unknown as Jwk;

const KID = "dummy-key-id-0001";
const ISSUER =
  "https://cognito-idp.ap-northeast-1.amazonaws.com/ap-northeast-1_dummyPool";
const OTHER_ISSUER =
  "https://cognito-idp.ap-northeast-1.amazonaws.com/ap-northeast-1_otherPool";
const CLIENT_ID = "dummy0000000000000000000000client";
const OTHER_CLIENT_ID = "dummy1111111111111111111111client";
const SUB = "11111111-1111-1111-1111-111111111111";

// AC-2-11: 現在時刻は呼ぶ側から与える。テストはこの固定時刻だけを使う。
const NOW = new Date("2026-09-01T00:00:00.000Z");
const NOW_EPOCH_SECONDS = Math.floor(NOW.getTime() / 1000);

function base64url(input: string | Buffer): string {
  return (typeof input === "string" ? Buffer.from(input) : input).toString(
    "base64url",
  );
}

interface BuildJwtOptions {
  headerOverrides?: Record<string, unknown>;
  payloadOverrides?: Record<string, unknown>;
  signWith?: typeof privateKey;
  corruptSignature?: boolean;
  /**
   * AC-2-3: 許可リストを検査するには、**その alg で正しく署名された**トークンが
   * 要る。署名が壊れているだけのトークンでは、署名検証の側で落ちてしまい
   * 「こちら側の許可リスト」を一度も通らないためである。
   */
  signDigest?: string;
}

function buildJwt(options: BuildJwtOptions = {}): string {
  const header = {
    alg: "RS256",
    typ: "JWT",
    kid: KID,
    ...options.headerOverrides,
  };
  const payload = {
    iss: ISSUER,
    aud: CLIENT_ID,
    sub: SUB,
    token_use: "id",
    iat: NOW_EPOCH_SECONDS - 10,
    exp: NOW_EPOCH_SECONDS + 3600,
    ...options.payloadOverrides,
  };
  const encodedHeader = base64url(JSON.stringify(header));
  const encodedPayload = base64url(JSON.stringify(payload));
  const signingInput = `${encodedHeader}.${encodedPayload}`;

  if (header.alg === "none") {
    // AC-2-3: alg=none・署名部が空のトークン。
    return `${signingInput}.`;
  }

  const signature = cryptoSign(
    options.signDigest ?? "RSA-SHA256",
    Buffer.from(signingInput),
    options.signWith ?? privateKey,
  );
  const encodedSignature = options.corruptSignature
    ? base64url(Buffer.concat([signature, Buffer.from("x")]))
    : base64url(signature);
  return `${signingInput}.${encodedSignature}`;
}

function fakePublicKeySource(jwk: Jwk = PUBLIC_JWK): PublicKeyResolver {
  return {
    async getKey({ issuer, kid }: { issuer: string; kid: string | undefined }) {
      if (issuer !== ISSUER) {
        throw new Error(
          `fakePublicKeySource: 想定外の issuer で鍵を要求された（issuer=${issuer}, kid=${kid}）。` +
            "AC-2-10「鍵の取得先をブラウザからの入力で決めない」に反する疑いがある。",
        );
      }
      return jwk;
    },
  };
}

async function verify(
  token: string,
  resolver: PublicKeyResolver = fakePublicKeySource(),
) {
  return verifyToken({
    token,
    now: NOW,
    issuer: ISSUER,
    clientId: CLIENT_ID,
    publicKeyResolver: resolver,
  });
}

// --- AC-2-1: 受理 -----------------------------------------------------------

describe("verifyToken - 受理（AC-2-1）", () => {
  it("署名・発行者・宛先・有効期限・種別のいずれも正しいトークンを受理し、sub を取り出せる", async () => {
    const result = await verify(buildJwt());
    expect(result).toEqual({ ok: true, sub: SUB });
  });
});

// --- AC-2-2〜AC-2-9: 拒否（テーブル駆動。受理側と同じだけ検査する＝AC-8-5）---

describe("verifyToken - 拒否（AC-2-2〜AC-2-9）", () => {
  const cases: Array<{
    name: string;
    token: () => string;
  }> = [
    {
      name: "AC-2-2: 署名が対応しない鍵で作られたトークン（改竄・別の鍵）",
      token: () => buildJwt({ signWith: otherPrivateKey }),
    },
    {
      name: "AC-2-2b: 署名部を末尾で改竄したトークン",
      token: () => buildJwt({ corruptSignature: true }),
    },
    {
      name: "AC-2-3: 署名アルゴリズムを none にし、署名部が空のトークン",
      token: () => buildJwt({ headerOverrides: { alg: "none" } }),
    },
    // AC-2-3 の後段「**許可するアルゴリズムを許可リストで固定し、トークン側の
    // 申告に従わない**」を検査する行。alg=none だけでは足りない —— 採用した
    // 検証ライブラリは alg=none を自ら拒む（tester 工程の実測）ため、その 1 行
    // が緑でも**こちら側の許可リストは一度も通らない**。
    //
    // 実測（tester 工程）: 検証ライブラリの署名検証は RS256 / RS384 / RS512 を
    // いずれも受理する。したがって **RS384 / RS512 で正しく署名したトークン**が
    // 拒否されることは、こちら側の許可リスト（RS256 のみ）が効いていることを
    // 示す。許可リストの判定を外すと、この 2 行だけが赤になる。
    {
      name: "AC-2-3: alg=RS384 で正しく署名されたトークン（許可リストの外）",
      token: () =>
        buildJwt({
          headerOverrides: { alg: "RS384" },
          signDigest: "RSA-SHA384",
        }),
    },
    {
      name: "AC-2-3: alg=RS512 で正しく署名されたトークン（許可リストの外）",
      token: () =>
        buildJwt({
          headerOverrides: { alg: "RS512" },
          signDigest: "RSA-SHA512",
        }),
    },
    {
      name: "AC-2-4: 発行者（iss）が当該 User Pool でないトークン",
      token: () => buildJwt({ payloadOverrides: { iss: OTHER_ISSUER } }),
    },
    {
      name: "AC-2-5: 宛先（aud）が当該アプリクライアントでないトークン",
      token: () => buildJwt({ payloadOverrides: { aud: OTHER_CLIENT_ID } }),
    },
    {
      name: "AC-2-6: 有効期限（exp）を過ぎたトークン",
      token: () =>
        buildJwt({ payloadOverrides: { exp: NOW_EPOCH_SECONDS - 1 } }),
    },
    {
      name: "AC-2-7: 利用開始時刻（nbf）より前に検証されたトークン",
      token: () =>
        buildJwt({ payloadOverrides: { nbf: NOW_EPOCH_SECONDS + 3600 } }),
    },
    {
      name: "AC-2-8: 種別（token_use 相当）が想定と異なるトークン",
      token: () => buildJwt({ payloadOverrides: { token_use: "access" } }),
    },
    {
      name: "AC-2-9a: トークンが空文字",
      token: () => "",
    },
    {
      name: "AC-2-9b: 形式が JWT でない文字列",
      token: () => "this-is-not-a-jwt",
    },
  ];

  it.each(cases)("$name は拒否される", async ({ token }) => {
    const result = await verify(token());
    expect(result.ok).toBe(false);
  });

  it("AC-2-9: 例外で異常終了させず、未認証として扱える（拒否の戻り値で表す）", async () => {
    await expect(verify("")).resolves.toEqual({ ok: false });
  });
});

// --- AC-2-10: 鍵の取得先をブラウザからの入力で決めない -----------------------

describe("verifyToken - 鍵の取得先（AC-2-10）", () => {
  it("公開鍵の取得は、呼ぶ側が与えた issuer（サーバー側の構成）に基づく。トークン内の iss の申告では鍵を取得しない", async () => {
    // iss を別の値へ差し替えたトークンで検証しても、fakePublicKeySource は
    // サーバー側の ISSUER（呼ぶ側が verify() へ渡した issuer）以外での
    // 呼び出しを例外にする。したがって、この呼び出しが例外を投げずに
    // 拒否の戻り値になることは、鍵の取得先がトークン側の申告に従っていない
    // ことの根拠になる。
    const forged = buildJwt({ payloadOverrides: { iss: OTHER_ISSUER } });
    const result = await verify(forged);
    expect(result.ok).toBe(false);
  });
});
