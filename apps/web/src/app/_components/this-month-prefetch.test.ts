import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { describe, expect, it } from "vitest";

// docs/specs/work-month-listing-ui.md AC-10-9（「選んだ時点」）。設計 W-7「先読み」・T-2。
//
// `/this-month` は押された時点の JST の当月で送り先を決める。next/link の先読みが本体を走らせると
// 「先読みした時点」の月で決まってしまうため、先読みされない動的なページに保つ。その条件を、
// ファイルを読んで静的に確かめる（Next を動かさない。実際に先読みされないことは本番の構成でしか起きず、
// ここでは読まない。T-2 の限界）。
//
// ページを import しない（import すると next/headers が入る）。ファイルの字面を TypeScript の構文木で読む。
// 各検査は述語に切り出し、違反の字面を与えると落ちること（陽性対照）を添える。

const srcDir = fileURLToPath(new URL("../..", import.meta.url));
const webDir = path.join(srcDir, "..");
const appDir = path.join(srcDir, "app");

const PAGE_PATH = path.join(appDir, "this-month", "page.tsx");
const NEXT_CONFIG_PATH = path.join(webDir, "next.config.ts");
const SIDE_NAV_PATH = path.join(srcDir, "components", "side-nav.tsx");
const INITIAL_ENTRY_PATH = path.join(appDir, "_components", "initial-entry-action.tsx");

const LOADING_EXTENSIONS = ["tsx", "ts", "jsx", "js"] as const;

function parse(text: string): ts.SourceFile {
  return ts.createSourceFile("source.tsx", text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
}

/** (1) `export const dynamic = "force-dynamic"` を持つ（コメントの中の字面は数えない）。 */
function exportsForceDynamic(text: string): boolean {
  let found = false;
  const visit = (node: ts.Node): void => {
    if (
      ts.isVariableStatement(node) &&
      node.modifiers?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword) === true &&
      (node.declarationList.flags & ts.NodeFlags.Const) !== 0
    ) {
      for (const decl of node.declarationList.declarations) {
        if (
          ts.isIdentifier(decl.name) &&
          decl.name.text === "dynamic" &&
          decl.initializer !== undefined &&
          ts.isStringLiteralLike(decl.initializer) &&
          decl.initializer.text === "force-dynamic"
        ) {
          found = true;
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(parse(text));
  return found;
}

/** (2) dir の直下にある loading.<拡張子> の名前（tsx・ts・jsx・js）。dir が無ければ空。 */
function loadingFilesIn(dir: string): string[] {
  return LOADING_EXTENSIONS.map((ext) => `loading.${ext}`).filter((name) => existsSync(path.join(dir, name)));
}

/** (3) cacheComponents・partialPrefetching を true にしている（入れ子の experimental 等も含む。構文木の全体を読む）。 */
function enablesPrefetchingFeatures(text: string): string[] {
  const enabled: string[] = [];
  const visit = (node: ts.Node): void => {
    if (ts.isPropertyAssignment(node)) {
      const name = ts.isIdentifier(node.name) || ts.isStringLiteralLike(node.name) ? node.name.text : "";
      if (
        (name === "cacheComponents" || name === "partialPrefetching") &&
        node.initializer.kind === ts.SyntaxKind.TrueKeyword
      ) {
        enabled.push(name);
      }
    } else if (
      ts.isShorthandPropertyAssignment(node) &&
      (node.name.text === "cacheComponents" || node.name.text === "partialPrefetching")
    ) {
      // 変数を渡す形は値を静的に読めないため、有効化の疑いとして数える。
      enabled.push(node.name.text);
    }
    ts.forEachChild(node, visit);
  };
  visit(parse(text));
  return enabled;
}

/** (4) 字面に prefetch が現れる（コメントの中も数える。要求は「現れない」こと）。 */
function mentionsPrefetch(text: string): boolean {
  return text.includes("prefetch");
}

describe("検査の述語の陽性対照（違反の字面を与えると落ちること）", () => {
  it("(1) exportsForceDynamic: 正しい字面だけを真とする", () => {
    expect(exportsForceDynamic(`export const dynamic = "force-dynamic";`)).toBe(true);
    expect(exportsForceDynamic(`export const dynamic = 'force-dynamic'`)).toBe(true);
    for (const bad of [
      `export const dynamic = "force-static";`,
      `export const dynamic = "auto";`,
      `const dynamic = "force-dynamic";`,
      `export const revalidate = "force-dynamic";`,
      `// export const dynamic = "force-dynamic";\nexport default function Page() {}`,
      `export let dynamic = "force-dynamic";`,
      ``,
    ]) {
      expect(exportsForceDynamic(bad), bad).toBe(false);
    }
  });

  it("(2) loadingFilesIn: 4つの拡張子の loading を検出し、無いディレクトリでは空を返す", () => {
    const dir = mkdtempSync(path.join(tmpdir(), "this-month-prefetch-"));
    try {
      expect(loadingFilesIn(dir)).toEqual([]);
      expect(loadingFilesIn(path.join(dir, "missing"))).toEqual([]);
      for (const ext of LOADING_EXTENSIONS) {
        const sub = path.join(dir, ext);
        mkdirSync(sub);
        writeFileSync(path.join(sub, `loading.${ext}`), "export default function Loading() { return null; }");
        expect(loadingFilesIn(sub)).toEqual([`loading.${ext}`]);
      }
      // loading 以外の名前は数えない。
      const other = path.join(dir, "other");
      mkdirSync(other);
      writeFileSync(path.join(other, "page.tsx"), "");
      expect(loadingFilesIn(other)).toEqual([]);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("(3) enablesPrefetchingFeatures: true を検出し、false・不在・コメントは数えない", () => {
    expect(enablesPrefetchingFeatures(`const c = { cacheComponents: true };`)).toEqual(["cacheComponents"]);
    expect(enablesPrefetchingFeatures(`const c = { experimental: { partialPrefetching: true } };`)).toEqual([
      "partialPrefetching",
    ]);
    expect(
      enablesPrefetchingFeatures(`const c = { cacheComponents: true, experimental: { partialPrefetching: true } };`),
    ).toEqual(["cacheComponents", "partialPrefetching"]);
    expect(enablesPrefetchingFeatures(`const c = { "cacheComponents": true };`)).toEqual(["cacheComponents"]);
    for (const ok of [
      `const c = { cacheComponents: false, partialPrefetching: false };`,
      `const c = {};`,
      `// cacheComponents: true\nconst c = {};`,
      `const c = { reactStrictMode: true };`,
    ]) {
      expect(enablesPrefetchingFeatures(ok), ok).toEqual([]);
    }
  });

  it("(4) mentionsPrefetch: prefetch を含む字面を検出する", () => {
    expect(mentionsPrefetch(`<Link href="/x" prefetch={true}>x</Link>`)).toBe(true);
    expect(mentionsPrefetch(`<Link href="/x" prefetch={false}>x</Link>`)).toBe(true);
    expect(mentionsPrefetch(`// prefetch は渡さない`)).toBe(true);
    expect(mentionsPrefetch(`<Link href="/x">x</Link>`)).toBe(false);
  });
});

describe("/this-month を先読みされない動的なページに保つ（W-7「先読み」）", () => {
  it("(1) app/this-month/page.tsx が export const dynamic = \"force-dynamic\" を持つ", () => {
    expect(existsSync(PAGE_PATH), `${PAGE_PATH} が無い`).toBe(true);
    expect(exportsForceDynamic(readFileSync(PAGE_PATH, "utf8"))).toBe(true);
  });

  it("(2) src/app/ と src/app/this-month/ に loading.* が無い", () => {
    expect(loadingFilesIn(appDir)).toEqual([]);
    expect(loadingFilesIn(path.join(appDir, "this-month"))).toEqual([]);
  });

  it("(3) next.config.ts が cacheComponents・partialPrefetching を true にしていない", () => {
    expect(existsSync(NEXT_CONFIG_PATH), `${NEXT_CONFIG_PATH} が無い`).toBe(true);
    expect(enablesPrefetchingFeatures(readFileSync(NEXT_CONFIG_PATH, "utf8"))).toEqual([]);
  });

  it.each([
    ["components/side-nav.tsx", SIDE_NAV_PATH],
    ["app/_components/initial-entry-action.tsx", INITIAL_ENTRY_PATH],
  ])("(4) %s の字面に prefetch が現れない（next/link に prefetch を渡さない）", (_name, file) => {
    expect(existsSync(file), `${file} が無い`).toBe(true);
    expect(mentionsPrefetch(readFileSync(file, "utf8"))).toBe(false);
  });
});
