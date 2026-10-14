import { readFileSync, readdirSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";
import { readAcSection } from "../test-support/design-system";

// docs/specs/design-system.md AC-1 / AC-2 / AC-4-1〜4-3 / AC-4-8 / AC-4-9
// （検証手段は AC-10-1・10-1-a・10-3-m）。
//
// 正解は globals.css の1箇所（AC-1-1）。このテストは globals.css を読み、
// トークン名・値・定義ブロック（:root / dark / @theme inline）の有無を
// AC-2 の表と突き合わせる。実装（コンポーネント）に依存しないため、
// このファイル自体が Red を踏むかどうかは globals.css の現在の値そのものに
// よって決まる（意味のある Red になる）。

const appDir = fileURLToPath(new URL(".", import.meta.url));
const cssPath = path.join(appDir, "globals.css");
const css = readFileSync(cssPath, "utf8");

// AC-2 の32トークン（表 2-1-a 〜 2-1-af）。名前と明色・暗色の値は AC-2 の表の字面から
// 読む（AC-10-1）。テスト側に書いた写しを期待値にすると、表を書き換えても写しと
// globals.css が一致したまま緑になり、表と突き合わせたことにならない（10-4 と同じ理由）。
// 表は AC-2 の節の中だけから読む。0件のとき、および形の合わない `2-1-N` 行を
// 読み飛ばしたときは失敗とする。
type TokenRow = { name: string; light: string; dark: string };

const specPath = path.join(appDir, "..", "..", "..", "..", "docs", "specs", "design-system.md");

function parseAc2Tokens(spec: string): TokenRow[] {
  const hint =
    `仕様書 ${specPath} の AC-2 の表を読めなかった。条文の体裁（見出し・表の列）を変えたのなら、` +
    `トークンの増減ではないのでこの抽出側を追随させること（10-1 は抽出手段を仕様で固定していない）。`;
  const start = spec.indexOf("### AC-2.");
  if (start < 0) throw new Error(`${hint} AC-2 の節が見つからない。`);
  const rest = spec.slice(start + 1);
  const end = rest.indexOf("\n### ");
  const section = end === -1 ? rest : rest.slice(0, end);
  const rowLines = section.match(/^\|\s*2-1-[a-z]+\s*\|.*$/gm) ?? [];
  const rows = rowLines.flatMap((line) => {
    const m = line.match(
      /^\|\s*2-1-[a-z]+\s*\|\s*`--([a-z-]+)`\s*\|\s*`(#[0-9a-f]{6})`\s*\|\s*`(#[0-9a-f]{6})`\s*\|[^|]*\|\s*$/,
    );
    return m ? [{ name: m[1], light: m[2], dark: m[3] }] : [];
  });
  if (rows.length === 0) throw new Error(`${hint} 表の行を1件も読めなかった。`);
  if (rows.length !== rowLines.length) {
    throw new Error(`${hint} 表に形の合わない行がある（${rowLines.length} 行中 ${rows.length} 行だけ読めた）。`);
  }
  return rows;
}

const spec = readFileSync(specPath, "utf8");
const TOKENS: ReadonlyArray<TokenRow> = parseAc2Tokens(spec);

// AC-4-9 の表（角丸・影・重ね幕・動きの専用トークン）。AC-4 の節の中だけから読む（AC-10-1）。
// 暗色の欄が `—` のトークンは dark を null とする（暗色のブロックに当該トークンが無いことを読む）。
type MotionRow = { id: string; name: string; light: string; dark: string | null };

function parseAc49Tokens(specText: string): MotionRow[] {
  const hint =
    `仕様書 ${specPath} の AC-4-9 の表を読めなかった。条文の体裁を変えたのなら、` +
    `トークンの増減ではないのでこの抽出側を追随させること（10-1 は抽出手段を仕様で固定していない）。`;
  const section = readAcSection(specText, 4, hint);
  const rowLines = section.match(/^\|\s*4-9-[a-z]+\s*\|.*$/gm) ?? [];
  const rows = rowLines.flatMap((line) => {
    const m = line.match(
      /^\|\s*(4-9-[a-z]+)\s*\|\s*`--([a-z-]+)`\s*\|\s*`([^`]+)`\s*\|\s*(?:`([^`]+)`|—)\s*\|[^|]*\|\s*$/,
    );
    return m ? [{ id: m[1], name: m[2], light: m[3], dark: m[4] ?? null }] : [];
  });
  if (rows.length === 0) throw new Error(`${hint} 表の行を1件も読めなかった。`);
  if (rows.length !== rowLines.length) {
    throw new Error(`${hint} 表に形の合わない行がある（${rowLines.length} 行中 ${rows.length} 行だけ読めた）。`);
  }
  return rows;
}

const MOTION_TOKENS: ReadonlyArray<MotionRow> = parseAc49Tokens(spec);

/** 値を比べるとき、`,` の前後の空白の有無と連続する空白を区別しない（AC-4-9）。 */
function normalizeValue(value: string): string {
  return value.replace(/\s*,\s*/g, ",").replace(/\s+/g, " ").trim();
}

// AC-4-2: `--font-sans` / `--font-numeric` の値は 4-2 の字面から取る。
function parseFontTokens(specText: string): Map<string, string> {
  const hint =
    `仕様書 ${specPath} の 4-2 の行から字面を読めなかった。条文の体裁を変えたのなら、` +
    `値の変更ではないのでこの抽出側を追随させること。`;
  const section = readAcSection(specText, 4, hint);
  const row = section.match(/^\|\s*4-2\s*\|.*$/m);
  if (!row) throw new Error(`${hint} 4-2 の行が見つからない。`);
  const map = new Map<string, string>();
  for (const m of row[0].matchAll(/`--(font-sans|font-numeric):\s*([^`]+)`/g)) {
    map.set(m[1], m[2].trim());
  }
  if (map.size !== 2) throw new Error(`${hint} --font-sans / --font-numeric の2つを読めなかった（${map.size} 件）。`);
  return map;
}

const FONT_TOKENS = parseFontTokens(spec);

// AC-1-5 の例外: 値の中の色を rgba() で書いてよい4トークン。名前は 1-5 の字面から取る
// （条文の字面から取り、0件のときは失敗とする。AC-10-1）。
function parseAlphaExceptionNames(specText: string): string[] {
  const hint =
    `仕様書 ${specPath} の 1-5 の行から例外の4トークンを読めなかった。条文の体裁を変えたのなら、` +
    `トークンの増減ではないのでこの抽出側を追随させること。`;
  const section = readAcSection(specText, 1, hint);
  const row = section.match(/^\|\s*1-5\s*\|.*$/m);
  if (!row) throw new Error(`${hint} 1-5 の行が見つからない。`);
  const segment = row[0].match(/例外は[^。]*?に限る/);
  if (!segment) throw new Error(`${hint} 「例外は…に限る」を切り出せなかった。`);
  const names = [...segment[0].matchAll(/`--([a-z-]+)`/g)].map((m) => m[1]);
  if (names.length === 0) throw new Error(`${hint} 例外のトークンを1件も読めなかった。`);
  return names;
}

const ALPHA_EXCEPTION_NAMES = parseAlphaExceptionNames(spec);

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

// AC-1-2: 明色の素の値は `:root`、暗色は `:root[data-theme="dark"]` の1ブロックで同名のトークンを
// 上書きする。AC-10-1-a: 同じヘッダに一致するブロックを「最初の1つ」ではなく**すべて**取り、
// 出現順に連結して読む（後に書かれたものが勝つ）。`@media (prefers-color-scheme: dark)` による
// 上書きは置かない。
const darkHeader = /:root\[data-theme="dark"\]\s*\{/;
const rootHeader = /:root\s*\{/;
const themeInlineHeader = /@theme\s+inline\s*\{/;
const colorSchemeMediaHeader = /@media\s*\(\s*prefers-color-scheme/;

const darkBlockBodies = extractBlockBodies(css, darkHeader);
const darkTokens = parseCustomProps(darkBlockBodies.join("\n"));
const lightBlockBodies = extractBlockBodies(css, rootHeader);
const lightTokens = parseCustomProps(lightBlockBodies.join("\n"));
const themeInlineBodies = extractBlockBodies(css, themeInlineHeader);
const themeInlineTokens = parseCustomProps(themeInlineBodies.join("\n"));

function countOccurrences(source: string, pattern: RegExp): number {
  return (source.match(new RegExp(pattern.source, "g")) ?? []).length;
}

/** AC-1-5: 値に rgba( / rgb( / hsl( / 8桁の16進が現れるトークンのうち、例外の名前の外にあるもの。 */
function alphaFormViolations(tokens: Map<string, string>, allowedNames: readonly string[]): string[] {
  return [...tokens]
    .filter(([name, value]) => /\b(?:rgba|rgb|hsl|hsla)\(|#[0-9a-fA-F]{8}\b/.test(value) && !allowedNames.includes(name))
    .map(([name]) => name)
    .sort();
}

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
    // AC-10-1: 走査の範囲を狭める変更（階層を落とす形）が、範囲の外に置いた CSS を
    // 見逃したまま通らないよう、走査とは別の手段で数えた集合と突き合わせる。
    const independent = ts.sys
      .readDirectory(srcDir, [".css"])
      .map((full) => path.relative(srcDir, full))
      .sort();
    expect(independent.length).toBeGreaterThan(0);
    expect([...cssFiles].sort()).toEqual(independent);
    expect(cssFiles).toEqual(["app/globals.css"]);
  });

  it("1-1: tailwind.config.* を持たない（Tailwind v4・config-less）", () => {
    const webDir = path.join(appDir, "..", "..");
    const hasConfig = readdirSync(webDir).some((f) => f.startsWith("tailwind.config."));
    expect(hasConfig).toBe(false);
  });

  it("1-2: :root（明色）と :root[data-theme=\"dark\"]（暗色）の両方が定義されている", () => {
    expect(lightBlockBodies.length).toBeGreaterThan(0);
    expect(darkBlockBodies.length).toBeGreaterThan(0);
  });

  // AC-1-2: OS の設定によるトークンの上書きを置かない（切替の正を属性1つに揃える）。
  // 0件の主張は走査が空振りしても真になるため、陽性側（暗色のブロックが在る）を同じ
  // テキストで併せて読む。
  it("1-2: @media (prefers-color-scheme: ...) のヘッダが無い", () => {
    expect(css.length).toBeGreaterThan(0);
    expect(countOccurrences(css, darkHeader)).toBeGreaterThan(0);
    expect(countOccurrences(css, colorSchemeMediaHeader)).toBe(0);
  });

  // AC-1-2: ネイティブの選択肢の一覧・スクロールバーを配色に合わせる。
  it("1-2: :root に color-scheme: light、暗色のブロックに color-scheme: dark を書く", () => {
    expect(lightBlockBodies.join("\n")).toMatch(/(^|[;{\s])color-scheme\s*:\s*light\s*;/);
    expect(darkBlockBodies.join("\n")).toMatch(/(^|[;{\s])color-scheme\s*:\s*dark\s*;/);
    expect(lightBlockBodies.join("\n")).not.toMatch(/color-scheme\s*:\s*dark/);
    expect(darkBlockBodies.join("\n")).not.toMatch(/color-scheme\s*:\s*light/);
  });

  it("1-3: AC-2 の各トークンに対応する --color-X: var(--X) が @theme inline にある", () => {
    expect(TOKENS.length).toBeGreaterThan(0);
    for (const token of TOKENS) {
      expect(themeInlineTokens.get(`color-${token.name}`)).toBe(`var(--${token.name})`);
    }
  });

  // AC-1-3 / AC-4-9: 形のトークンにはマッピングを置かない。1-4 の集合の一致が
  // 同じことを読むが、理由が違うため名前を挙げて読む。
  it("1-3: AC-4-9 の形のトークンには --color-* のマッピングを置かない", () => {
    expect(MOTION_TOKENS.length).toBeGreaterThan(0);
    for (const token of MOTION_TOKENS) {
      expect(themeInlineTokens.has(`color-${token.name}`)).toBe(false);
    }
  });

  // AC-1-4: 許す名前は AC-2 の表のトークン・その --color-* のマッピング・
  // --font-sans / --font-numeric・AC-4-9 の表のトークンだけ（10-1-a）。
  it("1-4: :root（明色）のカスタムプロパティは AC-2 の32トークンと AC-4-9 の表のトークンのみである", () => {
    const names = [...lightTokens.keys()].sort();
    const expected = [...TOKENS.map((t) => t.name), ...MOTION_TOKENS.map((t) => t.name)].sort();
    expect(names).toEqual(expected);
  });

  // AC-10-1-a: 暗色側も同じ母集団で読む。1-4 は「表に無いトークンを増やさない」
  // であり、明色だけを読むと暗色ブロックへ足したトークンが素通りする。
  it("1-4: 暗色（:root[data-theme=dark]）のカスタムプロパティは AC-2 の32トークンと AC-4-9 の暗色の欄に値があるトークンのみである", () => {
    const names = [...darkTokens.keys()].sort();
    const expected = [
      ...TOKENS.map((t) => t.name),
      ...MOTION_TOKENS.filter((t) => t.dark !== null).map((t) => t.name),
    ].sort();
    expect(names).toEqual(expected);
  });

  // AC-10-1-a: ブロックの抽出が「最初の1つ」で止まっていないことを、字面に
  // 現れるブロックの個数と突き合わせて読む。3種それぞれ、出現回数が0のときは失敗
  // させる（0件どうしの一致は一致ではない）。
  it("1-4: :root / @theme inline / 暗色ブロックの抽出が取りこぼしていない", () => {
    const rootOccurrences = countOccurrences(css, rootHeader);
    const themeOccurrences = countOccurrences(css, themeInlineHeader);
    const darkOccurrences = countOccurrences(css, darkHeader);
    expect(rootOccurrences).toBeGreaterThan(0);
    expect(themeOccurrences).toBeGreaterThan(0);
    expect(darkOccurrences).toBeGreaterThan(0);
    expect(lightBlockBodies.length).toBe(rootOccurrences);
    expect(themeInlineBodies.length).toBe(themeOccurrences);
    expect(darkBlockBodies.length).toBe(darkOccurrences);
  });

  it("1-4: @theme inline の名前は AC-2 の --color-* と --font-sans / --font-numeric のみである", () => {
    const names = [...themeInlineTokens.keys()].sort();
    const expected = [...TOKENS.map((t) => `color-${t.name}`), ...FONT_TOKENS.keys()].sort();
    expect(names).toEqual(expected);
  });

  it("1-5: AC-2 の色トークンの値は6桁の16進小文字表記（#rrggbb）である", () => {
    const hexPattern = /^#[0-9a-f]{6}$/;
    const colorNames = TOKENS.map((t) => t.name);
    for (const tokens of [lightTokens, darkTokens]) {
      for (const name of colorNames) {
        const value = tokens.get(name);
        expect({ name, value, matches: hexPattern.test(value ?? "") }).toEqual({
          name,
          value,
          matches: true,
        });
      }
    }
  });

  // AC-1-5 / AC-10-1: rgba() 等が現れるトークンの名前の集合が、1-5 の字面が名指す4トークンの
  // 部分集合であること。名指しの4つは条文の字面から取る。0件のときは失敗とする。
  it("1-5: rgba( / rgb( / hsl( / 8桁の16進を値に持つトークンは、条文が名指す4トークンの外に無い", () => {
    expect(ALPHA_EXCEPTION_NAMES).toHaveLength(4);
    expect(alphaFormViolations(lightTokens, ALPHA_EXCEPTION_NAMES)).toEqual([]);
    expect(alphaFormViolations(darkTokens, ALPHA_EXCEPTION_NAMES)).toEqual([]);
  });

  // 述語の両側（10-3-g と同じ形）。実ケースと同じ関数で読む。
  it.each([
    ["例外の --et-overlay の rgba(…) は違反でない", "et-overlay", "rgba(15,23,42,.45)", []],
    ["例外の --et-shadow の rgba(…) は違反でない", "et-shadow", "0 1px 3px rgba(15,23,42,.06)", []],
    ["--background の rgba(…) は違反", "background", "rgba(15,23,42,.45)", ["background"]],
    ["--primary の8桁の16進は違反", "primary", "#4338ca80", ["primary"]],
    ["--foreground の rgb(…) は違反", "foreground", "rgb(15, 23, 42)", ["foreground"]],
    ["--border の hsl(…) は違反", "border", "hsl(220 20% 90%)", ["border"]],
    ["--danger の6桁の16進は違反でない", "danger", "#be123c", []],
  ])("1-5 の判定: %s", (_name, token, value, expected) => {
    expect(alphaFormViolations(new Map([[token as string, value as string]]), ALPHA_EXCEPTION_NAMES)).toEqual(
      expected,
    );
  });
});

describe("globals.css - AC-2: セマンティック色トークンの名称と値", () => {
  it("10-1: 期待値を AC-2 の表の字面から読める（0件なら失敗）", () => {
    expect(TOKENS.length).toBeGreaterThan(0);
    expect(new Set(TOKENS.map((t) => t.name)).size).toBe(TOKENS.length);
  });

  // 形の合わない行を読み飛ばさない検査は、現行の表では一度も働かないため、実ケースと
  // 同じ関数へ合成した表を与えて両側を読む（10-2-a / 10-4 と同じ形）。
  it("10-1: 表の読み取りは形の合う行を読み、崩れた行・0件・節の外の行を読み飛ばさない", () => {
    const head = "### AC-2. セマンティック色トークン\n\n| # | トークン | 明色 | 暗色 | 用途 |\n|---|---|---|---|---|\n";
    const good = "| 2-1-a | `--a-b` | `#ffffff` | `#0a0a0a` | 用途 |\n";
    expect(parseAc2Tokens(`${head}${good}`)).toEqual([{ name: "a-b", light: "#ffffff", dark: "#0a0a0a" }]);
    // 列の間の空白の揺れは契約ではない。
    expect(parseAc2Tokens(`${head}|2-1-a|\`--a-b\`|  \`#ffffff\`  |\`#0a0a0a\`|用途|\n`)).toEqual([
      { name: "a-b", light: "#ffffff", dark: "#0a0a0a" },
    ]);
    // 2文字の英小文字の行番号（2-1-aa 以降）も読む。
    expect(parseAc2Tokens(`${head}| 2-1-aa | \`--a-b\` | \`#ffffff\` | \`#0a0a0a\` | 用途 |\n`)).toHaveLength(1);
    // 節の外に置いた同じ形の行は読まない。
    const stray = "| 2-1-z | `--z` | `#000000` | `#111111` | 用途 |\n";
    expect(parseAc2Tokens(`### AC-1. 前\n\n${stray}\n${head}${good}\n### AC-3. 後\n\n${stray}`)).toHaveLength(1);
    expect(() => parseAc2Tokens(`${head}`)).toThrow(/1件も読めなかった/);
    expect(() => parseAc2Tokens(`${good}`)).toThrow(/AC-2 の節が見つからない/);
    expect(() => parseAc2Tokens(`${head}${good}| 2-1-b | \`--c\` | #ffffff | \`#0a0a0a\` | 用途 |\n`)).toThrow(
      /形の合わない行/,
    );
    expect(() => parseAc2Tokens(`${head}${good}| 2-1-b | \`--c\` | \`#fff\` | \`#0a0a0a\` | 用途 |\n`)).toThrow(
      /形の合わない行/,
    );
  });

  it("2-2 の母集団: AC-2 の表は32トークンである（明色・暗色の両方に定義する対象の件数）", () => {
    expect(TOKENS).toHaveLength(32);
  });

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

// docs/specs/design-system.md AC-4-9（検証手段は AC-10-1）。表は AC-4 の節の中から読む。
// 明色の値は `:root`、暗色の欄に値がある行は暗色のブロックの値と突き合わせ、暗色の欄が
// `—` の行は暗色のブロックに当該トークンが無いことを読む。値は `,` の前後の空白の有無を
// 区別せずに比べる。
describe("globals.css - AC-4-9: 角丸・影・重ね幕・動きの専用トークン", () => {
  it("10-1: AC-4-9 の表の読み取りは形の合う行を読み、崩れた行・0件・節の外の行を読み飛ばさない", () => {
    const head = "### AC-4. タイポグラフィ\n\n| # | トークン | 明色 | 暗色 | 用途 |\n|---|---|---|---|---|\n";
    const good = "| 4-9-a | `--et-x` | `16px` | — | 用途 |\n";
    const goodDark = "| 4-9-b | `--et-y` | `0 1px 3px rgba(0,0,0,.06)` | `0 1px 3px rgba(0,0,0,.4)` | 用途 |\n";
    expect(parseAc49Tokens(`${head}${good}${goodDark}`)).toEqual([
      { id: "4-9-a", name: "et-x", light: "16px", dark: null },
      { id: "4-9-b", name: "et-y", light: "0 1px 3px rgba(0,0,0,.06)", dark: "0 1px 3px rgba(0,0,0,.4)" },
    ]);
    const stray = "| 4-9-z | `--et-z` | `1px` | — | 用途 |\n";
    expect(parseAc49Tokens(`### AC-3. 前\n\n${stray}\n${head}${good}\n### AC-5. 後\n\n${stray}`)).toHaveLength(1);
    expect(() => parseAc49Tokens(`${head}`)).toThrow(/1件も読めなかった/);
    expect(() => parseAc49Tokens(`${good}`)).toThrow(/AC-4 の節が見つからない/);
    expect(() => parseAc49Tokens(`${head}${good}| 4-9-b | \`--et-y\` | 16px | — | 用途 |\n`)).toThrow(
      /形の合わない行/,
    );
    expect(() => parseAc49Tokens(`${head}${good}| 4-9-b | \`--et-y\` | \`16px\` | 1px | 用途 |\n`)).toThrow(
      /形の合わない行/,
    );
  });

  it("4-9: 表は23行（4-9-a 〜 4-9-w）である", () => {
    expect(MOTION_TOKENS.map((t) => t.id)).toEqual("abcdefghijklmnopqrstuvw".split("").map((c) => `4-9-${c}`));
  });

  it.each(MOTION_TOKENS)("4-9: --$name の明色は $light", ({ name, light }) => {
    const actual = lightTokens.get(name);
    expect(actual).toBeDefined();
    expect(normalizeValue(actual as string)).toBe(normalizeValue(light));
  });

  it.each(MOTION_TOKENS)("4-9: --$name の暗色は $dark（null は暗色のブロックに書かない）", ({ name, dark }) => {
    if (dark === null) {
      expect(darkTokens.has(name)).toBe(false);
    } else {
      const actual = darkTokens.get(name);
      expect(actual).toBeDefined();
      expect(normalizeValue(actual as string)).toBe(normalizeValue(dark));
    }
  });

  it("4-9: 暗色の欄に値がある行は影と重ね幕の4つであり、AC-1-5 の例外と同じ集合である", () => {
    expect(MOTION_TOKENS.filter((t) => t.dark !== null).map((t) => t.name).sort()).toEqual(
      [...ALPHA_EXCEPTION_NAMES].sort(),
    );
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

  // AC-4-2 / AC-10-1: `--font-sans` / `--font-numeric` の値を @theme inline の中から読み、
  // 4-2 の字面と突き合わせる（字面は条文から取る）。
  it.each(["font-sans", "font-numeric"])("4-2: @theme inline の --%s は条文の字面と一致する", (name) => {
    const expected = FONT_TOKENS.get(name);
    expect(expected).toBeDefined();
    const actual = themeInlineTokens.get(name);
    expect(actual).toBeDefined();
    expect(normalizeValue(actual as string)).toBe(normalizeValue(expected as string));
  });

  // 4-2: 数字の書体の2番目に本文の書体を置く（数字の書体に無い文字を本文の書体で出す）。
  it("4-2: 条文から読んだ --font-numeric の値は2番目に本文の書体を置く", () => {
    expect(FONT_TOKENS.get("font-numeric")).toMatch(/^"Space Grotesk",\s*"Zen Kaku Gothic New",/);
    expect(FONT_TOKENS.get("font-sans")).toMatch(/^"Zen Kaku Gothic New",/);
  });

  it("4-3: @import url( を書かない（外部フォント配信に依存しない）", () => {
    expect(css.length).toBeGreaterThan(0);
    expect(css).not.toMatch(/@import\s+url\(/);
  });

  // 4-3 / 10-1-c (i): フォントの読み込みは next/font/local だけで行い、globals.css に
  // @font-face を書かない（読み込みの経路を1つにする）。
  it("4-3: globals.css に @font-face を書かない", () => {
    expect(css.length).toBeGreaterThan(0);
    expect(css).not.toMatch(/@font-face/);
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
    // AC-10-1-b: 1件の足場だけでは、走査の範囲を狭める変更（拡張子・階層を落とす形）が
    // layout.tsx を残したまま他のファイルを落として通る。走査とは別の手段で数えた
    // 集合と過不足なく一致することを読む（10-3-k と同じ理由）。
    const independent = ts.sys
      .readDirectory(srcDir, [".ts", ".tsx"])
      .map((full) => path.relative(srcDir, full))
      .filter((file) => !/\.test\.tsx?$/.test(file))
      .sort();
    expect(independent.length).toBeGreaterThan(0);
    expect(sources.map((s) => s.file).sort()).toEqual(independent);
    const offending = sources
      .filter((s) => /next\/font\/google/.test(s.text))
      .map((s) => s.file);
    expect(offending).toEqual([]);
  });
});

// docs/specs/design-system.md AC-4-8（検証手段は AC-10-3-m）。
//
// `@media (prefers-reduced-motion: reduce)` のヘッダの出現がちょうど1つであり、そのブロックの
// 中に、セレクタ `*` / `*::before` / `*::after` を含む規則があって、その規則が
// `animation: none !important` と `transition: none !important` の2つの宣言を持つこと。
// このブロックは内側に `{}` を持つため、最初の `}` で切る抽出（11-32）では読めない —— 入れ子を
// 数えて閉じ括弧を対応させる形で読む。判定は述語として切り出し、両側を実ケースと同じ手段で読む。
const reducedMotionHeader = /@media\s*\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)\s*\{/g;
const REQUIRED_SELECTORS = ["*", "*::before", "*::after"];
const REQUIRED_DECLARATIONS = ["animation:none!important", "transition:none!important"];

/** ヘッダの `{` の直後から、入れ子を数えて対応する `}` の手前までを返す。閉じられなければ undefined。 */
function readBalancedBody(source: string, bodyStart: number): string | undefined {
  let depth = 1;
  for (let i = bodyStart; i < source.length; i++) {
    if (source[i] === "{") depth++;
    else if (source[i] === "}") {
      depth--;
      if (depth === 0) return source.slice(bodyStart, i);
    }
  }
  return undefined;
}

/** prefers-reduced-motion のブロックの中身（出現順）。 */
function reducedMotionBodies(source: string): string[] {
  const bodies: string[] = [];
  for (const m of source.matchAll(reducedMotionHeader)) {
    const body = readBalancedBody(source, (m.index ?? 0) + m[0].length);
    if (body === undefined) throw new Error("prefers-reduced-motion のブロックが閉じられていない。");
    bodies.push(body);
  }
  return bodies;
}

/** 条文（4-8）どおりか。ブロックがちょうど1つで、中に必要な規則が在る。 */
function compliesWithReducedMotionRule(source: string): boolean {
  const bodies = reducedMotionBodies(source);
  if (bodies.length !== 1) return false;
  for (const rule of bodies[0].matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selectors = rule[1].split(",").map((selector) => selector.trim());
    const declarations = rule[2].split(";").map((declaration) => declaration.replace(/\s+/g, ""));
    if (
      REQUIRED_SELECTORS.every((selector) => selectors.includes(selector)) &&
      REQUIRED_DECLARATIONS.every((declaration) => declarations.includes(declaration))
    ) {
      return true;
    }
  }
  return false;
}

describe("globals.css - AC-4-8: prefers-reduced-motion（AC-10-3-m）", () => {
  const COMPLIANT = `@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation: none !important;
    transition: none !important;
  }
}`;

  it("globals.css に prefers-reduced-motion のブロックがちょうど1つあり、条文どおりの規則を持つ", () => {
    expect(css.length).toBeGreaterThan(0);
    // 0件のときは失敗とする（出現回数の読みと述語の読みの両方で）。
    expect(countOccurrences(css, new RegExp(reducedMotionHeader.source))).toBe(1);
    expect(compliesWithReducedMotionRule(css)).toBe(true);
  });

  it.each([
    ["条文どおりの形", COMPLIANT, true],
    ["別の規則を前後に伴う形", `a { color: red; }\n${COMPLIANT}\nb { color: blue; }`, true],
    ["宣言を1行にまとめた形", `@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation: none !important; transition: none !important; } }`, true],
    ["transition の宣言を欠く形", COMPLIANT.replace("    transition: none !important;\n", ""), false],
    ["animation の宣言を欠く形", COMPLIANT.replace("    animation: none !important;\n", ""), false],
    ["!important を欠く形", COMPLIANT.replace("transition: none !important", "transition: none"), false],
    ["*::after を欠く形", COMPLIANT.replace("  *::after {", "  {").replace("  *::before,\n", "  *::before"), false],
    ["ブロックが2つある形", `${COMPLIANT}\n${COMPLIANT}`, false],
    ["ブロックが無い形", "a { color: red; }", false],
    ["別の @media の中にだけ在る形", COMPLIANT.replace("prefers-reduced-motion: reduce", "prefers-reduced-motion: no-preference"), false],
  ])("4-8 の判定: %s", (_name, source, expected) => {
    expect(compliesWithReducedMotionRule(source as string)).toBe(expected);
  });

  it("閉じられていないブロックは、読み進めずに投げる", () => {
    expect(() => compliesWithReducedMotionRule("@media (prefers-reduced-motion: reduce) { * { animation: none")).toThrow(
      /閉じられていない/,
    );
  });
});
