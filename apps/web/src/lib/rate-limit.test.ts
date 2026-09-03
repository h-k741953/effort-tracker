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
import {
  createRateLimiter,
  resolveRateLimitKey,
  tooManyRequestsResponse,
  withRateLimit,
  withRequestRateLimit,
} from "./rate-limit";

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

// --- AC-11-6 (iii): 境界判定を掃除から独立に検査する（検査 11-e の補強） ----
//
// 既存の 11-e（t0+60秒ちょうど）は、掃除（AC-11-7 (iii)）と**互いにマスクし
// 合う**：check は先に掃除を呼び、初回 check(T0) で掃除の基準時刻が T0 に立つ
// ため、t0+60秒 では掃除側が先に当該キーを消してしまう。よって
// 「check 側の境界判定を『以上』から『超過』へ変える」ミューテーションでも、
// 「掃除側の削除を消す」ミューテーションでも、もう一方が同じ結果を出して緑の
// まま通る（tester 工程でコードを読んで確認）。
//
// 本 describe は**掃除が走らない時刻**に境界を置くことで、通るかどうかを
// **check 側の境界判定だけ**で決まるようにする。
describe("createRateLimiter - 境界判定を掃除から独立に検査する（AC-11-6 (iii)）", () => {
  it("掃除が走らない時刻に置いた t0+60秒ちょうどの要求は通り、件数が1に戻る", () => {
    const limiter = createRateLimiter();
    const target = "203.0.113.70";

    // (1) 掃除の基準時刻を T0 に立てる（初回の check で立つ）。
    limiter.check("203.0.113.71", T0);

    // (2) 対象キーのウィンドウを T0+10秒 から始める（掃除は走らない: 10 < 60）。
    limiter.check(target, secondsAfter(T0, 10));

    // (3) T0+60秒 に掃除を走らせ、基準時刻を T0+60秒 へ進める。対象キーは
    //     60-10=50 < 60 なので、この掃除では消えない。
    limiter.check("203.0.113.71", secondsAfter(T0, 60));

    // (4) 対象キーを同一ウィンドウ内で 60 件まで満たす（掃除は走らない）。
    for (let i = 0; i < 59; i += 1) {
      limiter.check(target, secondsAfter(T0, 60));
    }

    // (5) 対象キーの t0（T0+10秒）から**ちょうど 60 秒**の T0+70秒。掃除の
    //     基準時刻は T0+60秒 であり 70-60=10 < 60 なので**掃除は走らない**。
    //     したがって、ここで通るかどうかは check 側の境界判定だけが決める
    //     ——「以上」を「超過」に変えると、この行が赤になる。
    const boundary = limiter.check(target, secondsAfter(T0, 70));
    expect(boundary).toEqual({ allowed: true });

    // 新しいウィンドウで件数が1に戻っていれば、59件追加しても60件目まで通る。
    let lastResult = boundary;
    for (let i = 0; i < 59; i += 1) {
      lastResult = limiter.check(target, secondsAfter(T0, 70));
    }
    expect(lastResult).toEqual({ allowed: true });
    expect(limiter.check(target, secondsAfter(T0, 70)).allowed).toBe(false);
  });
});

// --- AC-11-7 (iii): 期限の切れた記録を保持し続けない -------------------------
//
// 「キーが際限なく増える形にしない」は、**保持件数を見ないと検査できない**。
// 現行の公開 API（check / reset）だけでは、掃除を丸ごと消しても check の結果が
// 変わらず緑のまま通る。そこで **保持している記録の件数だけ**を返す最小の
// 観測点 trackedKeyCount() を要求する（AC-11-7 (iv) がテストのために reset() を
// 許したのと同型。仕様の変更を要さない）。
//
// **「何でも見える窓」にしない** —— 返すのは件数だけであり、キーの一覧・
// カウンタ・ウィンドウの開始時刻は公開しない。
describe("createRateLimiter - 期限切れ記録の掃除（AC-11-7 (iii)）", () => {
  it("掃除は期限の切れた記録だけを落とし、まだ生きている記録は落とさない", () => {
    const limiter = createRateLimiter();

    // 掃除の基準時刻が T0 に立ち、old のウィンドウは T0 から始まる。
    limiter.check("203.0.113.80", T0);
    // young のウィンドウは T0+30秒 から（ここでは掃除は走らない: 30 < 60）。
    limiter.check("203.0.113.81", secondsAfter(T0, 30));
    expect(limiter.trackedKeyCount()).toBe(2);

    // T0+61秒 で掃除が走る（61 >= 60）。
    // - old  : 61-0  = 61 >= 60 → 期限切れ。**保持し続けない**
    // - young: 61-30 = 31 <  60 → まだ生きている。**落とさない**
    limiter.check("203.0.113.82", secondsAfter(T0, 61));

    // 残るのは young と、いま作られた trigger の 2 件。
    // 掃除の削除を消すと 3 件になり、まとめて捨てる実装だと 1 件になる。
    expect(limiter.trackedKeyCount()).toBe(2);
  });

  it("ウィンドウをまたいで別々のキーが来ても、保持件数は際限なく増えない", () => {
    const limiter = createRateLimiter();
    for (let i = 0; i < 10; i += 1) {
      // 1 件ごとに 120 秒進める。直前のキーは必ず期限切れになる。
      limiter.check(`203.0.113.${100 + i}`, secondsAfter(T0, i * 120));
    }
    // 最後の 1 件だけが生きている状態が期待値。保持し続ける実装なら 10 件。
    expect(limiter.trackedKeyCount()).toBe(1);
  });

  it("reset() は保持している記録を落とす（AC-11-7 (iv)）", () => {
    const limiter = createRateLimiter();
    limiter.check("203.0.113.90", T0);
    limiter.check("203.0.113.91", T0);
    expect(limiter.trackedKeyCount()).toBe(2);

    limiter.reset();

    expect(limiter.trackedKeyCount()).toBe(0);
  });

  // 上の2本は t0+61秒 を使っており、61 >= 60 も 61 > 60 も真になるため、
  // 掃除側の境界式（rate-limit.ts 81行目）を「以上」から「超過」へ緩めても
  // 緑のまま通ってしまう（AI が実測で確認済み）。掃除側の境界判定を、
  // check 自身の境界判定（AC-11-6 (iii)・95行目）から独立に突く。
  it("掃除は t0+60秒ちょうどでも期限切れとみなす（境界の判定を check 自身の境界判定から独立に検査する）", () => {
    const limiter = createRateLimiter();
    const keyA = "203.0.113.83";
    const keyB = "203.0.113.84";

    // (1) A を T0 で check する。A のウィンドウは T0 から始まり、掃除の
    //     基準時刻 lastSweepMillis も T0 に立つ（初回 check で立つ）。
    limiter.check(keyA, T0);

    // (2) T0+60秒 ちょうどに、別のキー B で check する。
    //     60-0 >= 60 なので掃除は走る（AC-11-6 (iii) と同じ「以上」の境界を
    //     AC-11-7 (iii) の掃除も用いる）。掃除が A を見るとき、比較になるのは
    //     60000-0 と WINDOW_MILLISECONDS。「以上」なら A は消え、「超過」なら
    //     残る。B は windows に未登録（新規）なので check 側の境界判定
    //     （current === undefined の短絡で新規ウィンドウ扱いになる）は評価
    //     されず、ここでの結果は**掃除側の境界判定だけ**で決まる。
    limiter.check(keyB, secondsAfter(T0, 60));

    // 掃除が正しく「以上」で判定していれば A は消え、残るのは B の1件だけ。
    // 掃除側の境界式だけを「超過」に緩めると A が残って2件になる
    // （check 側の境界式（95行目）は今回の呼び出しでは短絡され評価されない
    // ため、この it は掃除側だけの変異で赤くなる）。
    expect(limiter.trackedKeyCount()).toBe(1);
  });
});

// --- AC-11-8: 超過時の応答（検査 11-b / 11-c） ------------------------------
//
// tooManyRequestsResponse / withRequestRateLimit にはテストが 1 本も無かった。
// Retry-After を削っても、429 を 200 にしても緑のままだった（reviewer 指摘）。

const DUMMY_SESSION_TOKEN = "dummy.session.token-0123456789";
const DUMMY_IP = "203.0.113.200";

function requestHeaders(): Pick<Headers, "get"> {
  return headersOf({
    "cloudfront-viewer-address": `${DUMMY_IP}:443`,
    cookie: `et_session=${DUMMY_SESSION_TOKEN}`,
  });
}

describe("tooManyRequestsResponse - 超過時の応答（AC-11-8 (ii)(iii)）", () => {
  it("429 を返し、Retry-After に与えた残り秒数を添える", () => {
    const response = tooManyRequestsResponse({
      allowed: false,
      retryAfterSeconds: 48,
    });

    expect(response.status).toBe(429);
    expect(response.headers.get("Retry-After")).toBe("48");
  });

  it("Retry-After は 1 以上の整数である", () => {
    const response = tooManyRequestsResponse({
      allowed: false,
      retryAfterSeconds: 1,
    });

    const retryAfter = response.headers.get("Retry-After");
    expect(retryAfter).not.toBeNull();
    const parsed = Number(retryAfter);
    expect(Number.isInteger(parsed)).toBe(true);
    expect(parsed).toBeGreaterThanOrEqual(1);
  });
});

describe("withRequestRateLimit - 入口での超過（AC-11-8・検査 11-b / 11-c / 11-j）", () => {
  it("超過した要求は 429 になり、Retry-After が必ず付く", async () => {
    const limiter = createRateLimiter();
    const headers = requestHeaders();
    const handler = async () => new Response("ok", { status: 200 });

    for (let i = 0; i < 60; i += 1) {
      const passed = await withRequestRateLimit(headers, T0, handler, limiter);
      expect(passed.status).toBe(200);
    }

    const response = await withRequestRateLimit(headers, T0, handler, limiter);

    expect(response.status).toBe(429);
    const retryAfter = response.headers.get("Retry-After");
    expect(retryAfter).not.toBeNull();
    // 61 件目は t0 と同時刻なので、残りはウィンドウ丸ごとの 60 秒。
    expect(retryAfter).toBe("60");
    expect(Number.isInteger(Number(retryAfter))).toBe(true);
    expect(Number(retryAfter)).toBeGreaterThanOrEqual(1);
  });

  it("超過した要求では本来の処理が実行されない（検査 11-j）", async () => {
    const limiter = createRateLimiter();
    const headers = requestHeaders();
    let callCount = 0;
    const handler = async () => {
      callCount += 1;
      return new Response("ok", { status: 200 });
    };

    for (let i = 0; i < 60; i += 1) {
      await withRequestRateLimit(headers, T0, handler, limiter);
    }
    expect(callCount).toBe(60);

    const response = await withRequestRateLimit(headers, T0, handler, limiter);

    expect(response.status).toBe(429);
    expect(callCount).toBe(60);
  });

  it("429 の本文・ヘッダにキー（IP）もトークンも載せない（AC-11-8 (iii)・AC-4-1）", async () => {
    const limiter = createRateLimiter();
    const headers = requestHeaders();
    const handler = async () => new Response("ok", { status: 200 });

    for (let i = 0; i < 61; i += 1) {
      await withRequestRateLimit(headers, T0, handler, limiter);
    }
    const response = await withRequestRateLimit(headers, T0, handler, limiter);
    expect(response.status).toBe(429);

    // ヘッダは空ではない（Retry-After が必ず居る＝上の it）。空集合に対する
    // 「含まない」で緑にならないよう、まず中身があることを確かめる。
    const headerPairs = [...response.headers.entries()];
    expect(headerPairs.length).toBeGreaterThan(0);
    const serializedHeaders = headerPairs
      .map(([name, value]) => `${name}: ${value}`)
      .join("\n");

    expect(serializedHeaders).not.toContain(DUMMY_IP);
    expect(serializedHeaders).not.toContain(DUMMY_SESSION_TOKEN);

    const body = await response.text();
    expect(body).not.toContain(DUMMY_IP);
    expect(body).not.toContain(DUMMY_SESSION_TOKEN);
  });
});
