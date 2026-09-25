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
const darkBlockBody = extractBlockBody(css, darkMediaHeader);
const darkTokens = parseCustomProps(darkBlockBody);

const cssWithoutDarkBlock = darkBlockBody
  ? css.replace(
      new RegExp(
        darkMediaHeader.source + "\\{[^}]*\\}\\s*\\}",
      ),
      "",
    )
  : css;

const lightBlockBody = extractBlockBody(cssWithoutDarkBlock, /:root\s*/);
const lightTokens = parseCustomProps(lightBlockBody);

const themeInlineBody = extractBlockBody(cssWithoutDarkBlock, /@theme\s+inline\s*/);
const themeInlineTokens = parseCustomProps(themeInlineBody);

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
    expect(lightBlockBody).toBeDefined();
    expect(darkBlockBody).toBeDefined();
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

  it("4-3: next/font/google を使わない", () => {
    const layoutPath = path.join(appDir, "layout.tsx");
    const layout = existsSync(layoutPath) ? readFileSync(layoutPath, "utf8") : "";
    expect(layout).not.toMatch(/next\/font\/google/);
  });
});
