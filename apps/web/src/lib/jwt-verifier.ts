// BFF でのトークン検証（docs/specs/bff-auth-termination.md AC-1・AC-2・AC-7）。
//
// - AC-1-3 / AC-7-4: 実装は src/lib/ 配下に閉じる。新しい層を作らない。
// - AC-1-4 / AC-11-11: HTTP の要求・応答を組み立てずに単体で呼べる形にする。
// - AC-7-1: 署名検証は Cognito 専用の検証ライブラリ 1 本（aws-jwt-verify）に
//   委ね、自前実装しない。2 本目の JWT / JOSE ライブラリを足さない。
// - AC-7-2: 公開鍵の取得は、呼ぶ側が宣言する最小のインターフェース
//   （PublicKeyResolver）越しに受け取る。ライブラリ固有の型を公開 API へ
//   露出させない。
// - AC-4-1 / AC-4-2: 失敗の理由・トークンの一部をログにもエラーにも載せない。
//   拒否は理由を持たない戻り値（{ ok: false }）で表す。
import { SimpleJwksCache } from "aws-jwt-verify/jwk";
import type { Jwk as LibraryJwk } from "aws-jwt-verify/jwk";
import { decomposeUnverifiedJwt } from "aws-jwt-verify/jwt";
import { verifyJwtSync } from "aws-jwt-verify/jwt-verifier";

/**
 * 公開鍵（JSON Web Key）。AC-7-2 ③ によりライブラリ固有の型を公開 API へ
 * 出さないため、呼ぶ側から見える形をここで最小限に宣言する。
 */
export type Jwk = { readonly [member: string]: unknown };

/**
 * 公開鍵の取得（AC-2-10）。**呼ぶ側が宣言する最小のインターフェース**であり、
 * メソッドは AC-2 を満たす 1 つに限る（AC-7-2 ①②）。
 *
 * `issuer` はサーバー側の構成から与えられる値であり、**トークン内の申告では
 * ない**（AC-2-10「鍵の取得先をブラウザからの入力で決めない」）。
 */
export interface PublicKeyResolver {
  getKey(params: { issuer: string; kid: string | undefined }): Promise<Jwk>;
}

/** 受理。AC-2-1 の「利用者識別子を取り出せる」を満たす。 */
export interface VerifiedToken {
  readonly ok: true;
  /** AC-5-2: X-Actor-Id に載せる値は検証済みトークンの sub からのみ決める。 */
  readonly sub: string;
}

/** 拒否。AC-2 前文により拒否の理由は持たない（文言を検査しない・露出しない）。 */
export interface RejectedToken {
  readonly ok: false;
}

export type VerifyTokenResult = VerifiedToken | RejectedToken;

/**
 * AC-2-3: 許可する署名アルゴリズムを**こちら側の許可リストで固定**し、
 * トークン側の申告（alg）に従わない。Cognito の署名は RS256 である。
 */
const ALLOWED_SIGNATURE_ALGORITHMS: readonly string[] = ["RS256"];

/**
 * AC-2-8: 受け付ける種別を 1 つに固定する。別の種別を代替として受理しない。
 */
const ACCEPTED_TOKEN_USE = "id";

export interface VerifyTokenParams {
  /** 検証対象のトークン。空文字・JWT でない文字列でも例外にしない（AC-2-9）。 */
  token: string;
  /** AC-2-11: 有効期限の判定に用いる時刻は呼ぶ側から与える。 */
  now: Date;
  /** AC-2-4: 期待する発行者。サーバー側の構成が持つ（AC-7-5）。 */
  issuer: string;
  /** AC-2-5: 期待する宛先（アプリクライアント）。サーバー側の構成が持つ。 */
  clientId: string;
  /** AC-2-10 / AC-7-2: 公開鍵の取得口。 */
  publicKeyResolver: PublicKeyResolver;
}

/**
 * トークンを検証する。受理なら sub を伴い、拒否なら理由を持たない結果を返す。
 * **例外を投げない**（AC-2-9・AC-3-1「例外で応答を壊さない」）。
 */
export async function verifyToken(
  params: VerifyTokenParams,
): Promise<VerifyTokenResult> {
  const { token, now, issuer, clientId, publicKeyResolver } = params;

  try {
    if (typeof token !== "string" || token.length === 0) {
      return { ok: false }; // AC-2-9
    }

    // ここで得られるのは**未検証**の header である。kid の取り出しにのみ使う。
    const decomposed = decomposeUnverifiedJwt(token);

    const alg = decomposed.header.alg;
    if (typeof alg !== "string" || !ALLOWED_SIGNATURE_ALGORITHMS.includes(alg)) {
      return { ok: false }; // AC-2-3（alg=none を含む）
    }

    // AC-2-10: 鍵の取得先は**呼ぶ側が与えた issuer**（サーバー側の構成）で
    // 決める。トークン内の iss を鍵の取得先にしない。
    const jwk = await publicKeyResolver.getKey({
      issuer,
      kid: decomposed.header.kid,
    });

    // 署名・発行者・宛先の検証はライブラリに委ねる（AC-7-1 (iii)）。
    // 期限の判定だけは now（AC-2-11）で行うため、下の graceSeconds で
    // ライブラリ側の実時刻による判定を無効化する。
    const payload = verifyJwtSync(token, jwk as unknown as LibraryJwk, {
      issuer, // AC-2-4
      audience: clientId, // AC-2-5
      graceSeconds: clockGraceSecondsForLibrary(now),
    });

    const nowSeconds = now.getTime() / 1000;

    // AC-2-6: exp を過ぎたトークンは拒否する。exp が無いトークンも受理しない。
    const exp = payload.exp;
    if (typeof exp !== "number" || nowSeconds >= exp) {
      return { ok: false };
    }

    // AC-2-7: nbf より前の検証は拒否する。
    const nbf = payload.nbf;
    if (nbf !== undefined && (typeof nbf !== "number" || nowSeconds < nbf)) {
      return { ok: false };
    }

    // AC-2-8: 種別が想定と異なるトークンは拒否する。
    if (payload.token_use !== ACCEPTED_TOKEN_USE) {
      return { ok: false };
    }

    // AC-2-1 / AC-5-2: sub を取り出せないトークンは受理しない。
    const sub = payload.sub;
    if (typeof sub !== "string" || sub.length === 0) {
      return { ok: false };
    }

    return { ok: true, sub };
  } catch {
    // AC-2-9 / AC-3-1: 例外で異常終了させず、未認証として扱う。
    // AC-4-1: 例外の内容（署名不一致か期限切れかの別を含む）を残さない。
    return { ok: false };
  }
}

/**
 * ライブラリ側の exp / nbf 判定は実時刻（Date.now）を読む。本実装は AC-2-11 に
 * より **呼ぶ側が与えた now** だけで期限を判定するため、実時刻と now のずれを
 * 打ち消す猶予を与えて**ライブラリ側の時刻判定を無効化**し、判定を
 * verifyToken 内の now による検査へ一本化する。
 *
 * これは判定を緩めるものではない —— now による exp / nbf の検査は
 * verifyToken 側で必ず行われる（AC-8-5「常に受理する実装」にならない）。
 */
function clockGraceSecondsForLibrary(now: Date): number {
  const skewSeconds = Math.abs(Date.now() - now.getTime()) / 1000;
  return Math.ceil(skewSeconds) + 1;
}

/**
 * 本番の公開鍵取得（AC-2-10）。**サーバー側で取得し、再取得を要求ごとに
 * 行わない** —— モジュールのライフタイムでキャッシュを共有し、kid が見つから
 * ないときだけ取得し直す（鍵のローテーション時）。
 *
 * テストはこの実装を使わず、手書きの Fake を PublicKeyResolver として渡す
 * （AC-7-2 ④・AC-8-3「外部へ通信しない」）。
 */
const jwksCache = new SimpleJwksCache();

export function createCognitoPublicKeyResolver(): PublicKeyResolver {
  return {
    async getKey({ issuer, kid }) {
      if (typeof kid !== "string" || kid.length === 0) {
        // 鍵を特定できない。呼び出し側（verifyToken）が拒否として扱う。
        // AC-4-1: 例外にトークンの内容を載せない。
        throw new Error("public key id is missing");
      }
      const jwksUri = `${issuer}/.well-known/jwks.json`;
      const jwk = await jwksCache.getJwk(jwksUri, {
        header: { kid },
        payload: {},
      });
      return jwk as Jwk;
    },
  };
}
