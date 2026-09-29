import { readFileSync, readdirSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { describe, expect, it } from "vitest";

// docs/specs/design-system.md AC-1 / AC-2 / AC-4-1〜4-3（検証手段は AC-10-1）。
//
// 正解は globals.css の1箇所（AC-1-1）。このテストは globals.css を読み、
// トークン名・値・定義ブロック（:root / dark / @theme inline）の有無を
// AC-2 の表と突き合わせる。実装（コンポーネント）に依存しないため、
// このファイル自体が Red を踏むかどうかは globals.css の現在の値そのものに
// よって決まる（意味のある Red になる）。

const appDir = fileURLToPath(new URL(".", import.meta.url));
const cssPath = path.join(appDir, "globals.css");
const css = readFileSync(cssPath, "utf8");

// AC-2 の17トークン（表 2-1-a 〜 2-1-q）。値は仕様の表そのもの。
const TOKENS: ReadonlyArray<{
  name: string;
  light: string;
  dark: string;
}> = [
  { name: "background", light: "#ffffff", dark: "#0a0a0a" },
  { name: "foreground", light: "#171717", dark: "#ededed" },
  { name: "surface", light: "#f8fafc", dark: "#171717" },
  { name: "surface-foreground", light: "#171717", dark: "#ededed" },
  { name: "muted-foreground", light: "#52525b", dark: "#a1a1aa" },
  { name: "border", light: "#71717a", dark: "#a1a1aa" },
  { name: "primary", light: "#1d4ed8", dark: "#93c5fd" },
  { name: "primary-foreground", light: "#ffffff", dark: "#0a0a0a" },
  { name: "danger", light: "#b91c1c", dark: "#fca5a5" },
  { name: "danger-foreground", light: "#ffffff", dark: "#0a0a0a" },
  { name: "focus-ring", light: "#1d4ed8", dark: "#93c5fd" },
  { name: "state-draft", light: "#e4e4e7", dark: "#3f3f46" },
  { name: "state-draft-foreground", light: "#27272a", dark: "#f4f4f5" },
  { name: "state-pending-approval", light: "#fef3c7", dark: "#78350f" },
  {
    name: "state-pending-approval-foreground",
    light: "#78350f",
    dark: "#fef3c7",
  },
  { name: "state-approved", light: "#dcfce7", dark: "#14532d" },
  { name: "state-approved-foreground", light: "#14532d", dark: "#dcfce7" },
];

/** 単純な `--name: value;` の列挙のみを想定したブロック抽出（入れ子なし）。 */
function extractBlockBody(source: string, headerRegex: RegExp): string | undefined {
  const match = headerRegex.exec(source);
  if (!match) return undefined;
  const openIndex = source.indexOf("{", match.index + match[0].length - 1);
  if (openIndex === -1) return undefined;
  const closeIndex = source.indexOf("}", openIndex);
  if (closeIndex === -1) return undefined;
  return source.slice(openIndex + 1, closeIndex);
}

// AC-10-1-a: 同じヘッダに一致するブロックを「最初の1つ」ではなく**すべて**取る。
// 最初の1つだけを読むと、2つ目以降のブロックへ書いたトークンが 1-4（表に無い
// トークンを増やさない）の母集団から外れ、素通りする。値については後に書かれた
// ものが CSS の詰め込み順で勝つため、出現順に連結して読む。
function extractBlockBodies(source: string, headerRegex: RegExp): string[] {
  const re = new RegExp(headerRegex.source, "g");
  const bodies: string[] = [];
  let match: RegExpExecArray | null;
  while ((match = re.exec(source))) {
    const openIndex = source.indexOf("{", match.index + match[0].length - 1);
    if (openIndex === -1) break;
    const closeIndex = source.indexOf("}", openIndex);
    if (closeIndex === -1) break;
    bodies.push(source.slice(openIndex + 1, closeIndex));
    re.lastIndex = closeIndex + 1;
  }
  return bodies;
}

function parseCustomProps(block: string | undefined): Map<string, string> {
  const map = new Map<string, string>();
  if (!block) return map;
  const re = /--([\w-]+)\s*:\s*([^;]+);/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(block))) {
    map.set(m[1].trim(), m[2].trim());
  }
  return map;
}

// @media (prefers-color-scheme: dark) { :root { ... } } を先に取り出し、
// 残りのテキストから素の :root ブロック（明色）を取る。
const darkMediaHeader = /@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)\s*\{\s*:root\s*/;
const darkBlockBodies = extractBlockBodies(css, darkMediaHeader);
const darkTokens = parseCustomProps(darkBlockBodies.join("\n"));

const cssWithoutDarkBlock = css.replace(
  new RegExp(darkMediaHeader.source + "\\{[^}]*\\}\\s*\\}", "g"),
  "",
);

const lightBlockBodies = extractBlockBodies(cssWithoutDarkBlock, /:root\s*/);
const lightTokens = parseCustomProps(lightBlockBodies.join("\n"));

const themeInlineBodies = extractBlockBodies(cssWithoutDarkBlock, /@theme\s+inline\s*/);
const themeInlineTokens = parseCustomProps(themeInlineBodies.join("\n"));

describe("globals.css - AC-1: トークンの定義場所と形式", () => {
  it("1-1: globals.css 以外に CSS ファイルを持たない（src 配下）", () => {
    const srcDir = path.join(appDir, "..");
    const cssFiles: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name === "node_modules") continue;
          walk(full);
        } else if (entry.name.endsWith(".css")) {
          cssFiles.push(path.relative(srcDir, full));
        }
      }
    };
    walk(srcDir);
    expect(cssFiles).toEqual(["app/globals.css"]);
  });

  it("1-1: tailwind.config.* を持たない（Tailwind v4・config-less）", () => {
    const webDir = path.join(appDir, "..", "..");
    const hasConfig = readdirSync(webDir).some((f) => f.startsWith("tailwind.config."));
    expect(hasConfig).toBe(false);
  });

  it("1-2: :root（明色）と @media (prefers-color-scheme: dark) の :root（暗色）の両方が定義されている", () => {
    expect(lightBlockBodies.length).toBeGreaterThan(0);
    expect(darkBlockBodies.length).toBeGreaterThan(0);
  });

  it("1-3: AC-2 の各トークンに対応する --color-X: var(--X) が @theme inline にある", () => {
    for (const token of TOKENS) {
      expect(themeInlineTokens.get(`color-${token.name}`)).toBe(`var(--${token.name})`);
    }
  });

  it("1-4: :root（明色）のカスタムプロパティは AC-2 の17トークンのみである", () => {
    const names = [...lightTokens.keys()].sort();
    const expected = TOKENS.map((t) => t.name).sort();
    expect(names).toEqual(expected);
  });

  // AC-10-1-a: 暗色側も同じ母集団で読む。1-4 は「表に無いトークンを増やさない」
  // であり、明色だけを読むと暗色ブロックへ足したトークンが素通りする。
  it("1-4: 暗色（prefers-color-scheme: dark）の :root のカスタムプロパティも AC-2 の17トークンのみである", () => {
    const names = [...darkTokens.keys()].sort();
    const expected = TOKENS.map((t) => t.name).sort();
    expect(names).toEqual(expected);
  });

  // AC-10-1-a: ブロックの抽出が「最初の1つ」で止まっていないことを、字面に
  // 現れるブロックの個数と突き合わせて読む。0件のときは失敗させる（0件どうしの
  // 一致は一致ではない）。
  it("1-4: :root / @theme inline / 暗色ブロックの抽出が取りこぼしていない", () => {
    const rootOccurrences = cssWithoutDarkBlock.match(/:root\s*\{/g) ?? [];
    const themeOccurrences = cssWithoutDarkBlock.match(/@theme\s+inline\s*\{/g) ?? [];
    const darkOccurrences =
      css.match(/@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)\s*\{\s*:root\s*\{/g) ?? [];
    expect(rootOccurrences.length).toBeGreaterThan(0);
    expect(themeOccurrences.length).toBeGreaterThan(0);
    expect(darkOccurrences.length).toBeGreaterThan(0);
    expect(lightBlockBodies.length).toBe(rootOccurrences.length);
    expect(themeInlineBodies.length).toBe(themeOccurrences.length);
    expect(darkBlockBodies.length).toBe(darkOccurrences.length);
  });

  it("1-4: @theme inline の --color-* は AC-2 の17トークンのみである", () => {
    const names = [...themeInlineTokens.keys()].sort();
    const expected = TOKENS.map((t) => `color-${t.name}`).sort();
    expect(names).toEqual(expected);
  });

  it("1-5: すべての値が6桁の16進小文字表記（#rrggbb）である", () => {
    const hexPattern = /^#[0-9a-f]{6}$/;
    for (const [name, value] of lightTokens) {
      expect({ name, value, matches: hexPattern.test(value) }).toEqual({
        name,
        value,
        matches: true,
      });
    }
    for (const [name, value] of darkTokens) {
      expect({ name, value, matches: hexPattern.test(value) }).toEqual({
        name,
        value,
        matches: true,
      });
    }
  });
});

describe("globals.css - AC-2: セマンティック色トークンの名称と値", () => {
  it.each(TOKENS)("2-2: --$name は明色 $light / 暗色 $dark を持つ", ({ name, light, dark }) => {
    expect(lightTokens.get(name)).toBe(light);
    expect(darkTokens.get(name)).toBe(dark);
  });

  it("2-3: Excess / Shortfall 専用の色トークンを定義しない", () => {
    const allNames = [...lightTokens.keys(), ...darkTokens.keys()];
    const offending = allNames.filter((n) => /excess|shortfall/i.test(n));
    expect(offending).toEqual([]);
  });

  it("2-5: 状態バッジ3トークン組（地色）は明色・暗色ともに互いに異なる値である", () => {
    for (const tokens of [lightTokens, darkTokens]) {
      const values = [
        tokens.get("state-draft"),
        tokens.get("state-pending-approval"),
        tokens.get("state-approved"),
      ];
      expect(new Set(values).size).toBe(values.length);
    }
  });

  it("2-5: 状態バッジ3トークン組（文字色）は明色・暗色ともに互いに異なる値である", () => {
    for (const tokens of [lightTokens, darkTokens]) {
      const values = [
        tokens.get("state-draft-foreground"),
        tokens.get("state-pending-approval-foreground"),
        tokens.get("state-approved-foreground"),
      ];
      expect(new Set(values).size).toBe(values.length);
    }
  });
});

describe("globals.css - AC-4-1〜4-3", () => {
  it("4-1: --spacing- / --text- / --radius- を再定義しない", () => {
    const offending = css.match(/--(spacing|text|radius)-[\w-]*\s*:/g) ?? [];
    expect(offending).toEqual([]);
  });

  it("4-2: body に font-family を直接書かない", () => {
    const bodyBlock = extractBlockBody(css, /\bbody\s*/);
    expect(bodyBlock).toBeDefined();
    expect(bodyBlock).not.toMatch(/font-family/);
  });

  it("4-3: @import url( を書かない（外部フォント配信に依存しない）", () => {
    expect(css).not.toMatch(/@import\s+url\(/);
  });

  // AC-10-1-b: 4-3 は使用箇所を限定していないため、母集団は `src` 配下の
  // TypeScript / TSX の全体とする。layout.tsx だけを読むと、他のファイル
  // （page.tsx 等）での使用が素通りする。読めたファイルが0件のときは失敗させる。
  it("4-3: next/font/google を使わない（src 配下の全ファイル）", () => {
    const srcDir = path.join(appDir, "..");
    const sources: Array<{ file: string; text: string }> = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name === "node_modules") continue;
          walk(full);
        } else if (
          (entry.name.endsWith(".ts") || entry.name.endsWith(".tsx")) &&
          !entry.name.endsWith(".test.ts") &&
          !entry.name.endsWith(".test.tsx")
        ) {
          // 検査ファイル自身は母集団から外す（アプリの出力に入らないうえ、
          // 禁じた文字列を期待値として持つため自己言及で落ちる）。10-3 が
          // *.test.tsx を除くのと同じ扱い。
          sources.push({ file: path.relative(srcDir, full), text: readFileSync(full, "utf8") });
        }
      }
    };
    walk(srcDir);
    expect(sources.length).toBeGreaterThan(0);
    // layout.tsx が母集団に入っていることを明示的に読む（走査が空振りして
    // いないことの陽性側の足場）。
    expect(existsSync(path.join(appDir, "layout.tsx"))).toBe(true);
    expect(sources.map((s) => s.file)).toContain(path.join("app", "layout.tsx"));
    const offending = sources
      .filter((s) => /next\/font\/google/.test(s.text))
      .map((s) => s.file);
    expect(offending).toEqual([]);
  });
});
