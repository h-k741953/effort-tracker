import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";
import { readAcSection, readDesignSystemSpec } from "../test-support/design-system";

// docs/specs/design-system.md AC-4-2 / AC-4-3 / 4-3-a〜4-3-d（検証手段は AC-10-1-c・10-1-d）。
//
// (ii) 10-1-b と同じ母集団（apps/web/src 配下の実装 *.ts / *.tsx）を読み、next/font/local を輸入する
//      ファイルが apps/web/src/app/layout.tsx だけであることを読む。
// (iii) layout.tsx を TypeScript の構文木で読み、next/font/local の既定の輸入（名前は問わない）を呼ぶ式を
//      すべて取る。0件のときは失敗とする。
// (iv) 各呼び出しの引数のオブジェクトリテラルから、src の各 path が文字列リテラルでスキーム・`//` で
//      始まらず、layout.tsx から解決した先が apps/web/src/app/fonts/ の直下の書体のディレクトリの中の
//      実在する .woff2 であること、1つの呼び出しの path がすべて同じ書体のディレクトリを指すこと、
//      weight の集合が 4-3-b の字面と一致すること（各ウェイトちょうど1つ）、display が無いか "swap"、
//      preload が false、declarations が unicode-range と font-family をちょうど1つずつ持ち、
//      font-family の値（引用符を除いたもの）が書体のディレクトリに対応する名前と一致することを読む。
// (v) 2つの書体のそれぞれについて、呼び出しが1つ以上あること。
// 10-1-d: (iv) が path から得た書体のディレクトリのそれぞれに、.woff2 でないファイル（ライセンス文）が
//      1つ以上在る。ディレクトリが0件のときは失敗とする。
// unicode-range の値の中身（範囲の数・境界・重なり）は読まない（4-3-c が固定しない）。
// 期待するウェイト・ディレクトリ・名前は 4-3-a〜4-3-c と 4-2・10-1-c の字面から取る。

const srcDir = path.join(process.cwd(), "src");
const layoutPath = path.join(srcDir, "app", "layout.tsx");
const spec = readDesignSystemSpec();
const hint =
  "仕様書の 4-3-a〜4-3-c / 10-1-c の字面から期待値を読めなかった。条文の体裁を変えたのなら、要求の変更では" +
  "ないのでこの抽出側を追随させること（10-1-c は抽出手段を仕様で固定していない）。";

interface FontExpectation {
  directory: string;
  family: string;
  weights: string[];
}

function readExpectations(specText: string): FontExpectation[] {
  const ac4 = readAcSection(specText, 4, hint);
  const ac10 = readAcSection(specText, 10, hint);
  const rowOf = (section: string, id: string) => {
    const row = section.match(new RegExp(`^\\|\\s*${id}\\s*\\|.*$`, "m"));
    if (!row) throw new Error(`${hint} ${id} の行が見つからない。`);
    return row[0];
  };
  // 10-1-c: `zen-kaku-gothic-new/` → `Zen Kaku Gothic New`、`space-grotesk/` → `Space Grotesk`
  const mapping = [...rowOf(ac10, "10-1-c").matchAll(/`([a-z-]+)\/`\s*→\s*`([^`]+)`/g)].map((m) => ({
    directory: m[1],
    family: m[2],
  }));
  if (mapping.length !== 2) throw new Error(`${hint} 10-1-c のディレクトリと名前の対応を2つ読めなかった（${mapping.length} 件）。`);
  // 4-3-b: Zen Kaku Gothic New は 400 / 500 / 700、Space Grotesk は 500 / 600 / 700
  const weightsRow = rowOf(ac4, "4-3-b");
  return mapping.map(({ directory, family }) => {
    const m = weightsRow.match(new RegExp(`${family} は ((?:\\d+(?: / )?)+)`));
    if (!m) throw new Error(`${hint} 4-3-b から ${family} のウェイトを読めなかった。`);
    const weights = m[1].split("/").map((w) => w.trim());
    if (weights.length === 0 || weights.some((w) => !/^\d+$/.test(w))) throw new Error(`${hint} ウェイトの形が合わない。`);
    return { directory, family, weights: weights.sort() };
  });
}

/** 4-3-a: フォントの置き場所（apps/web/src/app/fonts/）。 */
function readFontsBase(specText: string): string {
  const row = readAcSection(specText, 4, hint).match(/^\|\s*4-3-a\s*\|.*$/m);
  const m = row?.[0].match(/`apps\/web\/src\/app\/(fonts)\/`/);
  if (!m) throw new Error(`${hint} 4-3-a から置き場所を読めなかった。`);
  return path.join(srcDir, "app", m[1]);
}

const EXPECTATIONS = readExpectations(spec);
const FONTS_BASE = readFontsBase(spec);

function parseSource(text: string): ts.SourceFile {
  const { diagnostics = [] } = ts.transpileModule(text, {
    fileName: "source.tsx",
    reportDiagnostics: true,
    compilerOptions: { jsx: ts.JsxEmit.Preserve },
  });
  if (diagnostics.length > 0) {
    const messages = diagnostics.map((d) => ts.flattenDiagnosticMessageText(d.messageText, "\n"));
    throw new Error(`構文として読めないテキストである: ${messages.join(" / ")}`);
  }
  return ts.createSourceFile("source.tsx", text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
}

const FONT_LOCAL = "next/font/local";

/** テキストが next/font/local を輸入しているか（import 文・再輸出・動的 import()）。コメント・文字列の中は数えない。 */
function importsNextFontLocal(text: string): boolean {
  let found = false;
  const visit = (node: ts.Node): void => {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier !== undefined &&
      ts.isStringLiteral(node.moduleSpecifier) &&
      node.moduleSpecifier.text === FONT_LOCAL
    ) {
      found = true;
    } else if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      node.arguments.length > 0 &&
      ts.isStringLiteralLike(node.arguments[0]) &&
      node.arguments[0].text === FONT_LOCAL
    ) {
      found = true;
    }
    ts.forEachChild(node, visit);
  };
  visit(parseSource(text));
  return found;
}

interface LocalFontCall {
  paths: string[];
  weights: string[];
  display: string | undefined;
  preload: boolean | undefined;
  declarations: { prop: string; value: string }[];
}

type ReadCall = { ok: true; call: LocalFontCall } | { ok: false; reason: string };

const propertyName = (p: ts.ObjectLiteralElementLike): string | undefined =>
  ts.isPropertyAssignment(p) && (ts.isIdentifier(p.name) || ts.isStringLiteral(p.name)) ? p.name.text : undefined;

function findProperty(object: ts.ObjectLiteralExpression, name: string): ts.Expression | undefined {
  for (const p of object.properties) {
    if (propertyName(p) === name && ts.isPropertyAssignment(p)) return p.initializer;
  }
  return undefined;
}

const literalText = (e: ts.Expression | undefined): string | undefined =>
  e !== undefined && (ts.isStringLiteral(e) || ts.isNoSubstitutionTemplateLiteral(e) || ts.isNumericLiteral(e))
    ? e.text
    : undefined;

/** `localFont({ … })` の呼び出し1つの引数を読む。オブジェクトリテラルで直に書かれていない形は読めない（11-44）。 */
function readLocalFontCall(call: ts.CallExpression): ReadCall {
  const arg = call.arguments[0];
  if (arg === undefined || !ts.isObjectLiteralExpression(arg)) {
    return { ok: false, reason: "引数をオブジェクトリテラルで直に書いていない" };
  }
  const src = findProperty(arg, "src");
  if (src === undefined || !ts.isArrayLiteralExpression(src)) return { ok: false, reason: "src を配列リテラルで直に書いていない" };
  const paths: string[] = [];
  const weights: string[] = [];
  for (const entry of src.elements) {
    if (!ts.isObjectLiteralExpression(entry)) return { ok: false, reason: "src の要素がオブジェクトリテラルでない" };
    const p = findProperty(entry, "path");
    const w = findProperty(entry, "weight");
    if (p === undefined || !ts.isStringLiteralLike(p)) return { ok: false, reason: "path が文字列リテラルでない" };
    const weight = literalText(w);
    if (weight === undefined) return { ok: false, reason: "weight が文字列リテラルでない" };
    paths.push(p.text);
    weights.push(weight);
  }
  const display = literalText(findProperty(arg, "display"));
  const preloadNode = findProperty(arg, "preload");
  const preload =
    preloadNode === undefined
      ? undefined
      : preloadNode.kind === ts.SyntaxKind.FalseKeyword
        ? false
        : preloadNode.kind === ts.SyntaxKind.TrueKeyword
          ? true
          : undefined;
  const declarationsNode = findProperty(arg, "declarations");
  const declarations: { prop: string; value: string }[] = [];
  if (declarationsNode !== undefined) {
    if (!ts.isArrayLiteralExpression(declarationsNode)) return { ok: false, reason: "declarations を配列リテラルで直に書いていない" };
    for (const entry of declarationsNode.elements) {
      if (!ts.isObjectLiteralExpression(entry)) return { ok: false, reason: "declarations の要素がオブジェクトリテラルでない" };
      const prop = literalText(findProperty(entry, "prop"));
      const value = literalText(findProperty(entry, "value"));
      if (prop === undefined || value === undefined) return { ok: false, reason: "declarations の prop / value が文字列リテラルでない" };
      declarations.push({ prop, value });
    }
  }
  return { ok: true, call: { paths, weights, display, preload, declarations } };
}

/** layout.tsx の next/font/local の既定の輸入（名前は問わない）を呼ぶ式をすべて取る。 */
function readLocalFontCalls(text: string): ReadCall[] {
  const source = parseSource(text);
  const localNames = new Set<string>();
  for (const statement of source.statements) {
    if (
      ts.isImportDeclaration(statement) &&
      ts.isStringLiteral(statement.moduleSpecifier) &&
      statement.moduleSpecifier.text === FONT_LOCAL &&
      statement.importClause?.name !== undefined
    ) {
      localNames.add(statement.importClause.name.text);
    }
  }
  const calls: ReadCall[] = [];
  const visit = (node: ts.Node): void => {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && localNames.has(node.expression.text)) {
      calls.push(readLocalFontCall(node));
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return calls;
}

/** スキーム（`<英字>:` の形）または `//` で始まる URL か（4-3）。 */
function isExternalUrl(value: string): boolean {
  return /^[A-Za-z][A-Za-z0-9+.-]*:/.test(value) || value.startsWith("//");
}

/**
 * 呼び出し1つが 4-3-a〜4-3-c を満たさない理由の一覧（空なら適合）。layout.tsx の位置と実在の判定は
 * 引数で受ける（述語として切り出し、条文どおりの形と崩れた形の両側を同じ手段で読むため）。
 */
function callViolations(
  call: LocalFontCall,
  expectations: readonly FontExpectation[],
  fontsBase: string,
  layoutDir: string,
  exists: (absolutePath: string) => boolean,
): string[] {
  const reasons: string[] = [];
  if (call.paths.length === 0) reasons.push("src が空");
  const directories = new Set<string>();
  for (const p of call.paths) {
    if (isExternalUrl(p)) {
      reasons.push(`path が URL: ${p}`);
      continue;
    }
    const resolved = path.resolve(layoutDir, p);
    const relative = path.relative(fontsBase, resolved);
    const segments = relative.split(path.sep);
    const known = expectations.find((e) => e.directory === segments[0]);
    if (relative.startsWith("..") || segments.length !== 2 || known === undefined || !segments[1].endsWith(".woff2")) {
      reasons.push(`path が fonts/<書体のディレクトリ>/*.woff2 を指さない: ${p}`);
      continue;
    }
    directories.add(segments[0]);
    if (!exists(resolved)) reasons.push(`path の先に .woff2 が実在しない: ${p}`);
  }
  if (directories.size > 1) reasons.push(`1つの呼び出しが複数の書体のディレクトリを指す: ${[...directories].join(", ")}`);
  const expectation = directories.size === 1 ? expectations.find((e) => e.directory === [...directories][0]) : undefined;
  if (expectation !== undefined) {
    if (JSON.stringify([...call.weights].sort()) !== JSON.stringify(expectation.weights)) {
      reasons.push(`weight の集合が ${expectation.weights.join("/")} と一致しない: ${call.weights.join("/")}`);
    }
    const families = call.declarations.filter((d) => d.prop === "font-family");
    if (families.length !== 1) reasons.push(`declarations の font-family が ${families.length} 個`);
    else if (families[0].value.replace(/^['"]|['"]$/g, "") !== expectation.family) {
      reasons.push(`font-family が ${expectation.family} でない: ${families[0].value}`);
    }
  }
  if (call.declarations.filter((d) => d.prop === "unicode-range").length !== 1) {
    reasons.push(`declarations の unicode-range が ${call.declarations.filter((d) => d.prop === "unicode-range").length} 個`);
  }
  if (call.display !== undefined && call.display !== "swap") reasons.push(`display が swap でない: ${call.display}`);
  if (call.preload !== false) reasons.push("preload が false でない");
  return reasons;
}

/** 書体のディレクトリに、.woff2 でないファイル（ライセンス文）が1つ以上在るか（10-1-d）。 */
function hasLicenseFile(directory: string): boolean {
  return readdirSync(directory, { withFileTypes: true }).some((e) => e.isFile() && !e.name.endsWith(".woff2"));
}

const layoutText = readFileSync(layoutPath, "utf8");
const layoutDir = path.dirname(layoutPath);

describe("layout.tsx のフォント読み込み - AC-4-3 / 4-3-a〜4-3-c（AC-10-1-c）", () => {
  it("期待値（書体のディレクトリ・名前・ウェイト）を条文の字面から読める", () => {
    expect(EXPECTATIONS).toEqual([
      { directory: "zen-kaku-gothic-new", family: "Zen Kaku Gothic New", weights: ["400", "500", "700"] },
      { directory: "space-grotesk", family: "Space Grotesk", weights: ["500", "600", "700"] },
    ]);
  });

  // (ii) next/font/local を輸入するファイルは layout.tsx だけ。
  it("(ii) next/font/local を輸入するのは app/layout.tsx だけである（母集団は src 配下の実装 *.ts / *.tsx）", () => {
    const population = ts.sys
      .readDirectory(srcDir, [".ts", ".tsx"])
      .filter((file) => !/\.test\.tsx?$/.test(file))
      .map((file) => path.relative(srcDir, file))
      .sort();
    expect(population.length).toBeGreaterThan(0);
    expect(population).toContain(path.join("app", "layout.tsx"));
    // 10-1-b: 母集団は走査とは別の手段（再帰の readdirSync）で数えた集合と過不足なく一致する。
    const walked: string[] = [];
    const walk = (dir: string): void => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== "node_modules") walk(full);
        } else if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
          walked.push(path.relative(srcDir, full));
        }
      }
    };
    walk(srcDir);
    expect(walked.length).toBeGreaterThan(0);
    expect(population).toEqual(walked.sort());
    const importers = population.filter((file) => importsNextFontLocal(readFileSync(path.join(srcDir, file), "utf8")));
    expect(importers).toEqual([path.join("app", "layout.tsx")]);
  });

  it("(iii) layout.tsx に next/font/local の呼び出しが1つ以上ある（0件は読み取りの空振りか同梱の欠落）", () => {
    expect(readLocalFontCalls(layoutText).length).toBeGreaterThan(0);
  });

  it("(iv) すべての呼び出しが 4-3-a〜4-3-c を満たす", () => {
    const calls = readLocalFontCalls(layoutText);
    expect(calls.length).toBeGreaterThan(0);
    const problems = calls.flatMap((read, index) =>
      read.ok
        ? callViolations(read.call, EXPECTATIONS, FONTS_BASE, layoutDir, existsSync).map((r) => `#${index}: ${r}`)
        : [`#${index}: 読めない形 — ${read.reason}`],
    );
    expect(problems).toEqual([]);
  });

  it("(v) 2つの書体のそれぞれについて、呼び出しが1つ以上ある", () => {
    const calls = readLocalFontCalls(layoutText).flatMap((read) => (read.ok ? [read.call] : []));
    for (const expectation of EXPECTATIONS) {
      const covering = calls.filter((call) =>
        call.declarations.some((d) => d.prop === "font-family" && d.value.replace(/^['"]|['"]$/g, "") === expectation.family),
      );
      expect(covering.length, `${expectation.family} の呼び出しが無い`).toBeGreaterThan(0);
    }
  });

  // 10-1-d
  it("10-1-d: 書体のディレクトリのそれぞれに、.woff2 でないファイル（ライセンス文）が1つ以上在る", () => {
    const calls = readLocalFontCalls(layoutText).flatMap((read) => (read.ok ? [read.call] : []));
    const directories = new Set(
      calls.flatMap((call) => call.paths.filter((p) => !isExternalUrl(p)).map((p) => path.dirname(path.resolve(layoutDir, p)))),
    );
    // ディレクトリが0件のときは失敗とする。
    expect(directories.size).toBeGreaterThan(0);
    const missing = [...directories].filter((dir) => !existsSync(dir) || !hasLicenseFile(dir));
    expect(missing.map((dir) => path.relative(srcDir, dir))).toEqual([]);
  });

  // ---- 述語の両側（10-3-g と同じ形）。実ケースと同じ関数で読む。 ----

  it.each([
    ["https://fonts.example/x.woff2", true],
    ["http://fonts.example/x.woff2", true],
    ["//fonts.example/x.woff2", true],
    ["data:font/woff2;base64,AAAA", true],
    ["./fonts/zen-kaku-gothic-new/x.woff2", false],
    ["fonts/zen-kaku-gothic-new/x.woff2", false],
    ["../fonts/space-grotesk/x.woff2", false],
  ])("URL の判定: %s は違反=%s", (value, expected) => {
    expect(isExternalUrl(value)).toBe(expected);
  });

  describe("呼び出しの形の判定", () => {
    const base = path.join("/virtual", "src", "app", "fonts");
    const dir = path.join("/virtual", "src", "app");
    const zen = (file: string) => `./fonts/zen-kaku-gothic-new/${file}`;
    const compliant: LocalFontCall = {
      paths: [zen("a-400.woff2"), zen("a-500.woff2"), zen("a-700.woff2")],
      weights: ["400", "500", "700"],
      display: "swap",
      preload: false,
      declarations: [
        { prop: "unicode-range", value: "U+0020-007F" },
        { prop: "font-family", value: "Zen Kaku Gothic New" },
      ],
    };
    const check = (call: LocalFontCall) => callViolations(call, EXPECTATIONS, base, dir, () => true);

    it("条文どおりの呼び出しは適合", () => {
      expect(check(compliant)).toEqual([]);
    });
    it("display を省いた形・font-family に引用符を含む形は適合", () => {
      expect(check({ ...compliant, display: undefined })).toEqual([]);
      expect(
        check({
          ...compliant,
          declarations: [compliant.declarations[0], { prop: "font-family", value: "'Zen Kaku Gothic New'" }],
        }),
      ).toEqual([]);
    });
    it("Space Grotesk の呼び出しも条文のウェイトで適合", () => {
      expect(
        check({
          ...compliant,
          paths: ["./fonts/space-grotesk/a.woff2", "./fonts/space-grotesk/b.woff2", "./fonts/space-grotesk/c.woff2"],
          weights: ["500", "600", "700"],
          declarations: [compliant.declarations[0], { prop: "font-family", value: "Space Grotesk" }],
        }),
      ).toEqual([]);
    });
    it.each([
      ["preload を欠く形", { ...compliant, preload: undefined }],
      ["preload が true の形", { ...compliant, preload: true }],
      ["font-family を欠く形", { ...compliant, declarations: [compliant.declarations[0]] }],
      ["unicode-range を欠く形", { ...compliant, declarations: [compliant.declarations[1]] }],
      ["font-family が別の書体の名前の形", { ...compliant, declarations: [compliant.declarations[0], { prop: "font-family", value: "Space Grotesk" }] }],
      ["ウェイトが1つ足りない形", { ...compliant, paths: compliant.paths.slice(0, 2), weights: ["400", "500"] }],
      ["ウェイトが重複する形", { ...compliant, weights: ["400", "400", "700"] }],
      ["2つの書体のディレクトリを混ぜた形", { ...compliant, paths: [zen("a.woff2"), "./fonts/space-grotesk/b.woff2", zen("c.woff2")] }],
      ["display が swap 以外の形", { ...compliant, display: "block" }],
      ["path が URL の形", { ...compliant, paths: ["https://fonts.example/a.woff2", zen("b.woff2"), zen("c.woff2")] }],
      ["path が fonts/ の外を指す形", { ...compliant, paths: ["./other/a.woff2", zen("b.woff2"), zen("c.woff2")] }],
      ["path が woff2 でない形", { ...compliant, paths: [zen("a.ttf"), zen("b.woff2"), zen("c.woff2")] }],
      ["path が書体のディレクトリの入れ子を指す形", { ...compliant, paths: [zen("sub/a.woff2"), zen("b.woff2"), zen("c.woff2")] }],
    ])("%s は不適合", (_name, call) => {
      expect(check(call as LocalFontCall).length).toBeGreaterThan(0);
    });
    it("path の先が実在しない形は不適合", () => {
      expect(callViolations(compliant, EXPECTATIONS, base, dir, () => false).length).toBeGreaterThan(0);
    });
  });

  describe("構文木からの読み取り", () => {
    const sample = (body: string) =>
      `import localFont from "next/font/local";\nconst a = localFont(${body});\nexport default function L() { return null; }`;
    it("オブジェクトリテラルで直に書いた呼び出しを読む（輸入の名前は問わない）", () => {
      const calls = readLocalFontCalls(
        `import anyName from "next/font/local";\nconst a = anyName({ src: [{ path: "./a.woff2", weight: "400" }], display: "swap", preload: false, declarations: [{ prop: "font-family", value: "X" }] });`,
      );
      expect(calls).toEqual([
        {
          ok: true,
          call: {
            paths: ["./a.woff2"],
            weights: ["400"],
            display: "swap",
            preload: false,
            declarations: [{ prop: "font-family", value: "X" }],
          },
        },
      ]);
    });
    it("引数を変数で渡した形は読めない形として失敗側に倒れる（11-44）", () => {
      expect(readLocalFontCalls(`import localFont from "next/font/local";\nconst o = {};\nlocalFont(o);`)).toEqual([
        { ok: false, reason: "引数をオブジェクトリテラルで直に書いていない" },
      ]);
    });
    it("next/font/local 以外から輸入した関数の呼び出しは数えない", () => {
      expect(readLocalFontCalls(`import localFont from "./other";\nlocalFont({ src: [] });`)).toEqual([]);
    });
    it("src が空配列の呼び出しは不適合になる", () => {
      const [read] = readLocalFontCalls(sample("{ src: [], preload: false, declarations: [] }"));
      expect(read.ok).toBe(true);
      if (read.ok) {
        expect(callViolations(read.call, EXPECTATIONS, FONTS_BASE, layoutDir, () => true).length).toBeGreaterThan(0);
      }
    });
    it("構文として読めないテキストは、読み進めずに投げる", () => {
      expect(() => readLocalFontCalls('const f = <T>(x: T) => x;\nlocalFont({ src: [] });')).toThrow(/構文として読めない/);
    });
    it.each([
      ['import localFont from "next/font/local";', true],
      ['export { default } from "next/font/local";', true],
      ['const m = await import("next/font/local");', true],
      ['// import localFont from "next/font/local";', false],
      ['const s = \'import localFont from "next/font/local"\';', false],
      ['import localFont from "next/font/google";', false],
    ])("next/font/local の輸入の判定: %s は %s", (text, expected) => {
      expect(importsNextFontLocal(text as string)).toBe(expected);
    });
  });

  describe("10-1-d のライセンス文の判定", () => {
    it("woff2 だけのディレクトリは偽、woff2 以外のファイルがあれば真", () => {
      const root = mkdtempSync(path.join(os.tmpdir(), "et-fonts-"));
      try {
        const onlyFonts = path.join(root, "only-fonts");
        const withLicense = path.join(root, "with-license");
        mkdirSync(onlyFonts);
        mkdirSync(withLicense);
        writeFileSync(path.join(onlyFonts, "a.woff2"), "");
        writeFileSync(path.join(withLicense, "a.woff2"), "");
        writeFileSync(path.join(withLicense, "OFL.txt"), "license");
        expect(hasLicenseFile(onlyFonts)).toBe(false);
        expect(hasLicenseFile(withLicense)).toBe(true);
      } finally {
        rmSync(root, { recursive: true, force: true });
      }
    });
  });
});
