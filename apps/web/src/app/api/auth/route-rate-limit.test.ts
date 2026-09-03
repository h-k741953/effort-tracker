import { beforeEach, describe, expect, it } from "vitest";
// AC-11-12（docs/specs/bff-auth-termination.md）:
// 「**11-2 が作る Route Handler は、いずれも入口でレート制限を通ること。
//   例外の経路を作らない**」
//
// これまで rate-limit.ts の入口（withRequestRateLimit）にも、Route Handler の
// 配線にもテストが 1 本も無かった。3 本のうち 1 本だけ配線を外しても緑のまま
// 通る状態だったため、**3 本すべてを個別に**検査する。
//
// AC-8-3: 外部へ通信しない。ここでは**入口で 429 になる**ところまでしか進まない
// ため、Cognito にも JWKS にも到達しない（超過した要求では本来の処理が実行され
// ないこと＝検査 11-j。それ自体は rate-limit.test.ts が持つ）。
//
// AC-8-1 / AC-7-3: Vitest 標準の expect だけを使う。モックライブラリを入れない。
import { GET as callbackGET } from "./callback/route";
import { POST as rolePOST } from "./role/route";
import { GET as signInGET } from "./sign-in/route";
import { resolveRateLimitKey, routeHandlerRateLimiter } from "@/lib/rate-limit";

// AC-11-6: 60 req / 60 秒 / キー。61 件目から超過。
const LIMIT_PER_WINDOW = 60;

const DUMMY_VIEWER_ADDRESS = "203.0.113.250:443";

function buildRequest(url: string, method: "GET" | "POST"): Request {
  return new Request(url, {
    method,
    headers: { "CloudFront-Viewer-Address": DUMMY_VIEWER_ADDRESS },
  });
}

/**
 * Route Handler が共有する制限器を、当該キーについて上限まで使い切らせる。
 *
 * Route Handler は自身の内側で現在時刻を読む（AC-11-10 の「呼ぶ側から与える」
 * を満たす境界は lib 側にあり、Route Handler がその呼ぶ側である）。ここで
 * 使い切らせるのも同じ実時刻の並びであり、**同一ウィンドウ内**で完結する
 * ——ウィンドウはキーごとの最初の要求時刻から 60 秒であって、絶対時刻・
 * タイムゾーンに依存しない（AC-8-3）。
 */
function exhaustSharedLimiterFor(request: Request): void {
  const key = resolveRateLimitKey(request.headers);
  for (let i = 0; i < LIMIT_PER_WINDOW; i += 1) {
    routeHandlerRateLimiter.check(key, new Date());
  }
}

describe("Route Handler の入口（AC-11-12・検査 11-b）", () => {
  beforeEach(() => {
    // AC-11-7 (iv): テスト間で状態が漏れないよう明示的に初期化する。
    routeHandlerRateLimiter.reset();
  });

  const routes: Array<{
    name: string;
    url: string;
    method: "GET" | "POST";
    invoke: (request: Request) => Promise<Response>;
  }> = [
    {
      name: "AC-11-2 (i) サインインの開始",
      url: "https://public.example.test/api/auth/sign-in",
      method: "GET",
      invoke: (request) => signInGET(request),
    },
    {
      name: "AC-11-2 (ii) サインインの戻り",
      url: "https://public.example.test/api/auth/callback?code=dummy-code&state=dummy-state",
      method: "GET",
      invoke: (request) => callbackGET(request),
    },
    {
      name: "AC-11-2 (iii) デモ用ロール切替",
      url: "https://public.example.test/api/auth/role",
      method: "POST",
      invoke: (request) => rolePOST(request),
    },
  ];

  it.each(routes)(
    "$name は入口でレート制限を通る（上限到達後の要求が 429 と Retry-After になる）",
    async ({ url, method, invoke }) => {
      const request = buildRequest(url, method);
      exhaustSharedLimiterFor(request);

      const response = await invoke(request);

      // 入口を通っていない Route Handler は、ここで 429 にならない
      // （構成欠落の 500・未認証の 401・送り出しの 302 のいずれかになる）。
      expect(response.status).toBe(429);
      expect(response.headers.get("Retry-After")).not.toBeNull();
    },
  );

  it.each(routes)(
    "$name は上限に達していなければ 429 にならない（常に 429 を返す実装で緑にしない）",
    async ({ url, method, invoke }) => {
      const request = buildRequest(url, method);

      const response = await invoke(request);

      expect(response.status).not.toBe(429);
    },
  );
});
