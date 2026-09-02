// Route Handler のレート制限（docs/specs/bff-auth-termination.md AC-11-5〜11-9。
// 値は 2026-09-01 に人間が決定した a〜e であり、AI の仮置きではない）。
//
// - AC-11-11: HTTP の要求・応答を組み立てずに単体で呼べる形にする。Route
//   Handler の内側にだけ存在する形にしない。
// - AC-11-10: ウィンドウの判定に用いる現在時刻は呼ぶ側から与える。
// - AC-11-13: レート制限のために npm 依存を追加しない。
// - AC-11-7: カウンタはインメモリ（近似）。外部ストアを作らない。実効上限が
//   予約同時実行数の分だけ緩むことは限界 10-8 として受け入れ済みであり、
//   実装で埋め合わせない。

/** AC-11-6 (b): 60 req / 60 秒 / キー。 */
const LIMIT_PER_WINDOW = 60;
const WINDOW_SECONDS = 60;
const WINDOW_MILLISECONDS = WINDOW_SECONDS * 1000;

/**
 * AC-11-5 (iii): ヘッダが無い／解釈できない要求は、**すべて同一の 1 つのキー**
 * として数える（別々の枠を与えると、ヘッダを外すだけで回避できる）。
 */
const UNKNOWN_KEY = "unknown";

/** AC-11-5: 信頼するのは CloudFront が付けるこのヘッダだけ。 */
const VIEWER_ADDRESS_HEADER = "cloudfront-viewer-address";

export interface RateLimitAllowed {
  readonly allowed: true;
}

export interface RateLimitExceeded {
  readonly allowed: false;
  /** AC-11-8 (ii): 現在のウィンドウの残り秒数（切り上げ・1 以上の整数）。 */
  readonly retryAfterSeconds: number;
}

export type RateLimitResult = RateLimitAllowed | RateLimitExceeded;

export interface RateLimiter {
  /** AC-11-10: now は呼ぶ側から与える。 */
  check(key: string, now: Date): RateLimitResult;
  /** AC-11-7 (iv): テストから状態を明示的に初期化できる形にする。 */
  reset(): void;
}

interface WindowState {
  /** ウィンドウの開始時刻（キーごとの最初の要求時刻 t0）。 */
  startedAtMillis: number;
  count: number;
}

/**
 * 固定ウィンドウのレート制限器を作る（AC-11-6）。
 * ウィンドウはキーごとに t0 から 60 秒、`t0 + 60秒` ちょうどは新しいウィンドウ
 * （経過の判定は「以上」で行う）。キーが違えば枠は独立である。
 */
export function createRateLimiter(): RateLimiter {
  const windows = new Map<string, WindowState>();
  let lastSweepMillis: number | undefined;

  /**
   * AC-11-7 (iii): 期限の切れた記録を保持し続けない（キーが際限なく増える形に
   * しない）。掃除はウィンドウ 1 つにつき高々 1 回に留める。
   */
  function sweepExpired(nowMillis: number): void {
    if (
      lastSweepMillis !== undefined &&
      nowMillis - lastSweepMillis < WINDOW_MILLISECONDS
    ) {
      return;
    }
    lastSweepMillis = nowMillis;
    for (const [key, state] of windows) {
      if (nowMillis - state.startedAtMillis >= WINDOW_MILLISECONDS) {
        windows.delete(key);
      }
    }
  }

  return {
    check(key: string, now: Date): RateLimitResult {
      const nowMillis = now.getTime();
      sweepExpired(nowMillis);

      const current = windows.get(key);
      if (
        current === undefined ||
        nowMillis - current.startedAtMillis >= WINDOW_MILLISECONDS
      ) {
        // 新しいウィンドウ。件数は 1 に戻る（AC-11-6 (iii)）。
        windows.set(key, { startedAtMillis: nowMillis, count: 1 });
        return { allowed: true };
      }

      if (current.count < LIMIT_PER_WINDOW) {
        current.count += 1;
        return { allowed: true }; // 60 件目までは通す（AC-11-6 (i)）
      }

      // 61 件目以降は超過（AC-11-6 (i)）。
      const remainingMillis =
        current.startedAtMillis + WINDOW_MILLISECONDS - nowMillis;
      return {
        allowed: false,
        retryAfterSeconds: Math.max(1, Math.ceil(remainingMillis / 1000)),
      };
    },

    reset(): void {
      windows.clear();
      lastSweepMillis = undefined;
    },
  };
}

/**
 * 要求ヘッダからレート制限のキーを決める（AC-11-5）。
 *
 * - `CloudFront-Viewer-Address` はアドレスとポートを含む形で与えられるため、
 *   **アドレス部分へ正規化**する（ポートで枠が分かれない＝(ii)）。
 * - **`X-Forwarded-For` を使わない**（左端は詐称可能。フォールバックにもしない
 *   ＝(i)）。
 * - ヘッダが無い／解釈できない要求は**同一の 1 つのキー**として数える（(iii)）。
 *   例外を投げない。
 */
export function resolveRateLimitKey(headers: Pick<Headers, "get">): string {
  const rawViewerAddress = headers.get(VIEWER_ADDRESS_HEADER);
  if (typeof rawViewerAddress !== "string") {
    return UNKNOWN_KEY;
  }

  const address = normalizeViewerAddress(rawViewerAddress);
  if (address === undefined) {
    return UNKNOWN_KEY;
  }

  return `ip:${address}`;
}

/**
 * `203.0.113.10:443` / `2001:db8::1:65000` のような「アドレス:ポート」から
 * アドレス部分を取り出す。解釈できない形なら undefined を返す（例外にしない）。
 */
function normalizeViewerAddress(rawViewerAddress: string): string | undefined {
  const value = rawViewerAddress.trim();
  const portSeparatorIndex = value.lastIndexOf(":");
  if (portSeparatorIndex <= 0) {
    return undefined;
  }

  const address = value.slice(0, portSeparatorIndex);
  const port = value.slice(portSeparatorIndex + 1);
  if (address.length === 0 || !/^\d+$/.test(port)) {
    return undefined;
  }

  return address;
}

/**
 * レート制限を通してから本来の処理を実行する（AC-11-8 (i)・検査 11-j）。
 * **超過した要求では handler を呼ばない** —— 429 を返すだけで内部的には呼ぶ
 * 実装にしない。
 */
export function withRateLimit<T>(
  limiter: RateLimiter,
  key: string,
  now: Date,
  handler: () => T,
): T | RateLimitExceeded {
  const result = limiter.check(key, now);
  if (!result.allowed) {
    return result;
  }
  return handler();
}

/**
 * Route Handler が入口で通す共有の制限器（AC-11-12「例外の経路を作らない」）。
 * AC-11-7 (i): 外部ストアを作らない。インスタンスごとのインメモリに留める。
 */
export const routeHandlerRateLimiter: RateLimiter = createRateLimiter();

/**
 * 超過時の応答（AC-11-8）。429 と `Retry-After` を返し、**本文・ヘッダにキー
 * （IP）・トークン・その一部を載せない**。独自のステータス・独自のヘッダを
 * 作らない（10-11）。
 */
export function tooManyRequestsResponse(result: RateLimitExceeded): Response {
  return new Response("Too Many Requests", {
    status: 429,
    headers: {
      "Retry-After": String(result.retryAfterSeconds),
      "Cache-Control": "no-store",
    },
  });
}

/**
 * Route Handler の入口。ヘッダからキーを決め、超過なら 429 を返し、
 * **本来の処理を実行しない**。Route Handler 側はこれを呼ぶだけの薄い配線に
 * 留める（AC-11-11）。
 */
export async function withRequestRateLimit(
  headers: Pick<Headers, "get">,
  now: Date,
  handler: () => Promise<Response>,
  limiter: RateLimiter = routeHandlerRateLimiter,
): Promise<Response> {
  const outcome = withRateLimit(
    limiter,
    resolveRateLimitKey(headers),
    now,
    () => ({ response: handler() }),
  );

  if ("response" in outcome) {
    return outcome.response;
  }
  return tooManyRequestsResponse(outcome);
}
