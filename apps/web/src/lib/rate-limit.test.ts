import { describe, expect, it } from "vitest";
// AC-8-2（docs/specs/bff-auth-termination.md）: 実装より先にテストを書き、
// Red を確認してから実装へ入る。本ファイルが対象とするモジュール
// （./rate-limit）はまだ存在しない。これは意図した Red である。
//
// ファイル名・関数名・型名はこのテストが暫定的に固定するインターフェース
// である（AC-1-3・AC-11-11 と同型。「HTTP の要求・応答を組み立てずに単体で
// 呼べる形にする」ため、Route Handler を経由せず直接呼べるように設計する）。
// 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。
//
// AC-11-5〜11-9（U-1 の a〜e。2026-09-01 人間決定）と、検査テーブル
// 11-a〜11-j（236行〜）がそのまま本ファイルの入出力になる。
import { createRateLimiter, resolveRateLimitKey, withRateLimit } from "./rate-limit";

// AC-11-10: ウィンドウの判定に用いる現在時刻は呼ぶ側から与える。
const T0 = new Date("2026-09-01T00:00:00.000Z");
function secondsAfter(base: Date, seconds: number): Date {
  return new Date(base.getTime() + seconds * 1000);
}

function headersOf(entries: Record<string, string>): Pick<Headers, "get"> {
  const map = new Map(Object.entries(entries));
  return {
    get(name: string) {
      return map.get(name) ?? null;
    },
  };
}

// --- AC-11-6 (b): 上限とウィンドウ ------------------------------------------

describe("createRateLimiter - 上限とウィンドウ（AC-11-6・検査 11-a〜11-c, 11-f）", () => {
  it("11-a: 同一キー・同一ウィンドウの60件目までは通す", () => {
    const limiter = createRateLimiter();
    let lastResult;
    for (let i = 0; i < 60; i += 1) {
      lastResult = limiter.check("203.0.113.10", T0);
    }
    expect(lastResult).toEqual({ allowed: true });
  });

  it("11-b: 同一キー・同一ウィンドウの61件目は超過になる", () => {
    const limiter = createRateLimiter();
    for (let i = 0; i < 60; i += 1) {
      limiter.check("203.0.113.11", T0);
    }
    const result = limiter.check("203.0.113.11", T0);
    expect(result.allowed).toBe(false);
  });

  it("11-c: 61件目の Retry-After はウィンドウの残り秒数（切り上げ、1以上）", () => {
    const limiter = createRateLimiter();
    const key = "203.0.113.12";
    // 最初の要求（t0）から 12.4 秒後に 61 件そろえる。
    for (let i = 0; i < 60; i += 1) {
      limiter.check(key, i === 0 ? T0 : secondsAfter(T0, 12.4));
    }
    const result = limiter.check(key, secondsAfter(T0, 12.4));
    expect(result.allowed).toBe(false);
    if (result.allowed) return;
    // 残り = 60 - 12.4 = 47.6 → 切り上げで 48。
    expect(result.retryAfterSeconds).toBe(48);
    expect(Number.isInteger(result.retryAfterSeconds)).toBe(true);
    expect(result.retryAfterSeconds).toBeGreaterThanOrEqual(1);
  });

  it("11-f: t0+59秒に与えた61件目の要求は超過になる（境界の内側）", () => {
    const limiter = createRateLimiter();
    const key = "203.0.113.13";
    for (let i = 0; i < 60; i += 1) {
      limiter.check(key, T0);
    }
    const result = limiter.check(key, secondsAfter(T0, 59));
    expect(result.allowed).toBe(false);
  });
});

// --- AC-11-6 (iv): キーが違えば枠は独立 -------------------------------------

describe("createRateLimiter - キーの独立性（検査 11-d）", () => {
  it("11-d: 一方が上限に達していても、別のキーからの要求は通す", () => {
    const limiter = createRateLimiter();
    for (let i = 0; i < 61; i += 1) {
      limiter.check("203.0.113.20", T0);
    }
    const result = limiter.check("203.0.113.21", T0);
    expect(result).toEqual({ allowed: true });
  });
});

// --- AC-11-6 (iii): t0+60秒ちょうどは新しいウィンドウ（検査 11-e） ----------

describe("createRateLimiter - ウィンドウの境界（検査 11-e）", () => {
  it("11-e: t0+60秒ちょうどの要求は通り、件数が1に戻る（59件追加しても60件目まで通る）", () => {
    const limiter = createRateLimiter();
    const key = "203.0.113.30";
    for (let i = 0; i < 60; i += 1) {
      limiter.check(key, T0);
    }
    // 前のウィンドウで上限に達している状態から、ちょうど60秒後。
    const boundaryResult = limiter.check(key, secondsAfter(T0, 60));
    expect(boundaryResult).toEqual({ allowed: true });

    // 新しいウィンドウで件数が1に戻っていれば、59件追加しても60件目まで通る。
    let lastResult = boundaryResult;
    for (let i = 0; i < 59; i += 1) {
      lastResult = limiter.check(key, secondsAfter(T0, 60));
    }
    expect(lastResult).toEqual({ allowed: true });

    // 新しいウィンドウの61件目は超過になる。
    const overResult = limiter.check(key, secondsAfter(T0, 60));
    expect(overResult.allowed).toBe(false);
  });
});

// --- AC-11-7 (iv): テストから状態を明示的に初期化できる ---------------------

describe("createRateLimiter - 明示的なリセット（AC-11-7 (iv)）", () => {
  it("reset() の後は、同じキーでも上限に達していない状態から数え直す", () => {
    const limiter = createRateLimiter();
    const key = "203.0.113.40";
    for (let i = 0; i < 61; i += 1) {
      limiter.check(key, T0);
    }
    expect(limiter.check(key, T0).allowed).toBe(false);

    limiter.reset();

    expect(limiter.check(key, T0)).toEqual({ allowed: true });
  });
});

// --- AC-11-5 (a): キーの取り方（検査 11-g〜11-i） ---------------------------

describe("resolveRateLimitKey - キーの取り方（AC-11-5・検査 11-g〜11-i）", () => {
  it("11-g: CloudFront-Viewer-Address のポートだけが異なる要求は同じキーになる", () => {
    const keyA = resolveRateLimitKey(
      headersOf({ "cloudfront-viewer-address": "203.0.113.50:443" }),
    );
    const keyB = resolveRateLimitKey(
      headersOf({ "cloudfront-viewer-address": "203.0.113.50:65000" }),
    );
    expect(keyA).toBe(keyB);
  });

  it("11-g: アドレスが異なれば別のキーになる（ポート正規化の副作用でキーが1つに潰れていないことの確認）", () => {
    const keyA = resolveRateLimitKey(
      headersOf({ "cloudfront-viewer-address": "203.0.113.50:443" }),
    );
    const keyC = resolveRateLimitKey(
      headersOf({ "cloudfront-viewer-address": "203.0.113.51:443" }),
    );
    expect(keyA).not.toBe(keyC);
  });

  it("11-h: CloudFront-Viewer-Address が無く、X-Forwarded-For だけが（異なる値で）与えられた要求は、すべて同一のキーになる", () => {
    const noHeaderKey = resolveRateLimitKey(headersOf({}));
    const xffKeyA = resolveRateLimitKey(
      headersOf({ "x-forwarded-for": "198.51.100.1" }),
    );
    const xffKeyB = resolveRateLimitKey(
      headersOf({ "x-forwarded-for": "198.51.100.2, 198.51.100.3" }),
    );
    expect(xffKeyA).toBe(noHeaderKey);
    expect(xffKeyB).toBe(noHeaderKey);
  });

  it("11-i: CloudFront-Viewer-Address が解釈できない形の要求は、例外を投げずに（無い場合と）同一のキーになる", () => {
    const noHeaderKey = resolveRateLimitKey(headersOf({}));
    const unparseableKey = resolveRateLimitKey(
      headersOf({ "cloudfront-viewer-address": "not-an-address" }),
    );
    expect(() => unparseableKey).not.toThrow();
    expect(unparseableKey).toBe(noHeaderKey);
  });
});

// --- AC-11-8 (i): 超過した要求は本来の処理を実行しない（検査 11-j） --------

describe("withRateLimit - 超過時に本来の処理を実行しない（検査 11-j）", () => {
  it("11-j: 超過した要求では、ラップされたハンドラが呼ばれない（通過の副作用が起きていないことを見る）", () => {
    const limiter = createRateLimiter();
    const key = "203.0.113.60";
    let callCount = 0;
    const handler = () => {
      callCount += 1;
      return "handled";
    };

    for (let i = 0; i < 60; i += 1) {
      withRateLimit(limiter, key, T0, handler);
    }
    expect(callCount).toBe(60);

    const result = withRateLimit(limiter, key, T0, handler);

    // 429 を返すだけで内部的にはハンドラを呼んでしまう実装は、ここで緑に
    // ならない（callCount が61のままなら Red）。
    expect(callCount).toBe(60);
    expect(result).not.toBe("handled");
  });
});
