import { readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { beforeEach, describe, expect, it } from "vitest";
// docs/specs/bff-wiring.md の Route Handler（src/app/api/**/route.ts）に対する
// ファイル走査の検査:
//   5-a（すべての Route Handler が入口でレート制限を通る）／
//   6-h（契約の E-1・E-3〜E-7 に対応する面がちょうど1つ）／
//   6-g の route.ts 側（R-2・R-3 が別の route.ts）／6-6（更新系を GET で受けない）。
//
// 母集団は手書きで列挙せず、ファイルの走査で得る（AC-1 検査の前文・AC-11-4）。
// **既存の手書き列挙（auth/route-rate-limit.test.ts）に業務の経路を書き足す形で
// 代えない**（5-a）。
//
// AC-11-3: 外部へ通信しない。ここでは入口で 429 になるところまでしか進まない。
import { routeHandlerRateLimiter } from "../../lib/rate-limit";
import {
  PATH_PARAMS,
  VIEWER_ADDRESS,
  exhaustSharedLimiterFor,
} from "../../test-support/bff-wiring";

const apiDir = fileURLToPath(new URL(".", import.meta.url));

const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"] as const;
type HttpMethod = (typeof HTTP_METHODS)[number];

interface RouteSurface {
  readonly file: string;
  /** `/api/...` ＋ 動的セグメントを `*` に正規化したパス。 */
  readonly normalizedPath: string;
  /** 動的セグメントの名前（context.params に与える）。 */
  readonly paramNames: readonly string[];
  readonly method: HttpMethod;
  readonly handler: (request: Request, context: unknown) => Promise<Response>;
}

function listRouteFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...listRouteFiles(full));
    } else if (entry.name === "route.ts") {
      files.push(full);
    }
  }
  return files.sort();
}

function pathOf(file: string): { normalizedPath: string; paramNames: string[] } {
  const relative = path.relative(apiDir, path.dirname(file));
  const segments = relative === "" ? [] : relative.split(path.sep);
  const paramNames: string[] = [];
  const normalized = segments
    // ルートグループ（括弧）は URL に現れない。
    .filter((segment) => !/^\(.+\)$/.test(segment))
    .map((segment) => {
      const dynamic = /^\[(.+)\]$/.exec(segment);
      if (dynamic !== null) {
        paramNames.push(dynamic[1]);
        return "*";
      }
      return segment;
    });
  return { normalizedPath: `/api/${normalized.join("/")}`.replace(/\/$/, ""), paramNames };
}

async function scanSurfaces(): Promise<RouteSurface[]> {
  const surfaces: RouteSurface[] = [];
  for (const file of listRouteFiles(apiDir)) {
    const loaded = (await import(/* @vite-ignore */ pathToFileURL(file).href)) as Record<
      string,
      unknown
    >;
    const { normalizedPath, paramNames } = pathOf(file);
    for (const method of HTTP_METHODS) {
      const exported = loaded[method];
      if (typeof exported === "function") {
        surfaces.push({
          file,
          normalizedPath,
          paramNames,
          method,
          handler: exported as RouteSurface["handler"],
        });
      }
    }
  }
  return surfaces;
}

/** 契約のパスを同じ正規化（`{x}` → `*`）にかけ、`/api` を前置する（6-7 (i)）。 */
function bffPathOf(contractPath: string): string {
  return `/api${contractPath.replace(/\{[^}]+\}/g, "*")}`;
}

// 出所: docs/specs/domain-api-http-contract.md「エンドポイント一覧」の E-1・E-3〜E-7
// （E-2 は R-2・R-3 が別のパスで持つ＝6-7 (ii) のため、この表に含めない）。
const CONTRACT_ENDPOINTS: ReadonlyArray<{ id: string; method: HttpMethod; path: string }> = [
  { id: "E-1", method: "GET", path: "/work-months/{contractId}/{yearMonth}" },
  { id: "E-3", method: "PUT", path: "/work-months/{contractId}/{yearMonth}/daily-records/{date}" },
  { id: "E-4", method: "DELETE", path: "/work-months/{contractId}/{yearMonth}/daily-records/{date}" },
  { id: "E-5", method: "POST", path: "/work-months/{contractId}/{yearMonth}/close" },
  { id: "E-6", method: "POST", path: "/work-months/{contractId}/{yearMonth}/approve" },
  { id: "E-7", method: "POST", path: "/work-months/{contractId}/{yearMonth}/reject" },
];

const isBusiness = (surface: RouteSurface) => !surface.normalizedPath.startsWith("/api/auth/");

function buildRequest(surface: RouteSurface): Request {
  const hasBody = surface.method === "PUT";
  return new Request(
    `https://public.example.test${surface.normalizedPath.replace(/\*/g, "x")}`,
    {
      method: surface.method,
      headers: { "CloudFront-Viewer-Address": VIEWER_ADDRESS },
      body: hasBody ? "{}" : undefined,
    },
  );
}

function contextFor(surface: RouteSurface): { params: Promise<Record<string, string>> } {
  const params: Record<string, string> = {};
  for (const name of surface.paramNames) {
    params[name] = (PATH_PARAMS as Record<string, string>)[name] ?? "dummy";
  }
  return { params: Promise.resolve(params) };
}

describe("AC-5-1・5-6: すべての Route Handler が入口でレート制限を通る（検査 5-a）", () => {
  beforeEach(() => {
    routeHandlerRateLimiter.reset();
  });

  it("5-a: 走査で列挙したすべてのメソッドが、上限を使い切ったキーで 429・Retry-After・no-store になる。業務のメソッドは 8 以上ある", async () => {
    const surfaces = await scanSurfaces();

    // 0件・取りこぼしの一致を一致とみなさない。
    expect(surfaces.filter(isBusiness).length).toBeGreaterThanOrEqual(8);
    expect(surfaces.filter((s) => !isBusiness(s)).length).toBeGreaterThanOrEqual(3);

    for (const surface of surfaces) {
      routeHandlerRateLimiter.reset();
      const request = buildRequest(surface);
      exhaustSharedLimiterFor(request);

      const response = await surface.handler(request, contextFor(surface));

      expect(`${surface.method} ${surface.normalizedPath}: ${response.status}`).toBe(
        `${surface.method} ${surface.normalizedPath}: 429`,
      );
      expect(response.headers.get("Retry-After")).not.toBeNull();
      expect(response.headers.get("Cache-Control")).toBe("no-store");
    }
  });
});

describe("AC-6-1・6-7: 契約のエンドポイントに対応する面（検査 6-h・6-g・6-6）", () => {
  it("6-h: 契約の E-1・E-3〜E-7 の各行について、/api ＋ 当該パスと当該メソッドの組がちょうど1つ存在する", async () => {
    const surfaces = await scanSurfaces();
    expect(surfaces.length).toBeGreaterThan(0);

    for (const endpoint of CONTRACT_ENDPOINTS) {
      const matches = surfaces.filter(
        (s) => s.method === endpoint.method && s.normalizedPath === bffPathOf(endpoint.path),
      );
      expect(`${endpoint.id}: ${matches.length}`).toBe(`${endpoint.id}: 1`);
    }
  });

  it("6-g（route.ts 側）: R-2・R-3 は互いに別のパスの GET で、E-1 のパスとも別（業務の GET が E-1 を含め 3 つ以上ある）", async () => {
    const surfaces = await scanSurfaces();
    const e1 = bffPathOf(CONTRACT_ENDPOINTS[0].path);

    const businessGets = surfaces.filter((s) => isBusiness(s) && s.method === "GET");
    const listPaths = new Set(
      businessGets.filter((s) => s.normalizedPath !== e1).map((s) => s.normalizedPath),
    );

    expect(businessGets.length).toBeGreaterThanOrEqual(3);
    expect(listPaths.size).toBeGreaterThanOrEqual(2);
  });

  it("6-6: 状態を変える R-5〜R-9 の面（E-3〜E-7 のパス）を GET で受けない", async () => {
    const surfaces = await scanSurfaces();
    const updatePaths = CONTRACT_ENDPOINTS.filter((e) => e.method !== "GET").map((e) =>
      bffPathOf(e.path),
    );

    // 母集団の陽性側: 更新系のパスの面が実際に走査で見つかっている。
    expect(surfaces.filter((s) => updatePaths.includes(s.normalizedPath)).length).toBeGreaterThanOrEqual(5);
    const gets = surfaces.filter(
      (s) => s.method === "GET" && updatePaths.includes(s.normalizedPath),
    );
    expect(gets.map((s) => `${s.method} ${s.normalizedPath}`)).toEqual([]);
  });

  it("6-3・6-7 (iii): 業務の面は契約の E-1・E-3〜E-7 と R-2・R-3 の GET 2 つだけで、ブラウザが engineerId を指定するパスの口を持たない", async () => {
    const surfaces = await scanSurfaces();
    const contractPairs = new Set(
      CONTRACT_ENDPOINTS.map((e) => `${e.method} ${bffPathOf(e.path)}`),
    );

    const extra = surfaces
      .filter(isBusiness)
      .filter((s) => !contractPairs.has(`${s.method} ${s.normalizedPath}`));

    // R-2・R-3 の 2 つだけ（それ以外のドメイン API を呼ぶ Route Handler を作らない＝AC-6 前文）。
    expect(extra.map((s) => s.method)).toEqual(["GET", "GET"]);
    expect(new Set(extra.map((s) => s.normalizedPath)).size).toBe(2);
    for (const surface of extra) {
      expect(surface.paramNames.join(",")).not.toMatch(/engineer/i);
      expect(surface.normalizedPath).not.toMatch(/engineer/i);
    }
  });
});
