import { readFileSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";

// docs/specs/web-app-scaffold.md AC-13（13-2 / 13-3）。
//
// layout.tsx を TypeScript の構文木で読み、default export の RootLayout が返す <html> 要素が
// lang="ja" で画面の言語を示すことを読む。layout-theme.test.ts（同じ <html> の data-theme を読む）に倣う。
// 判定は述語として切り出し、テキストを受け取って単体で呼べる形にする（13-2-b）。合格側 3-a〜3-d・3-ag・3-ah と
// 不合格側 3-e〜3-u・3-x〜3-af を fixture として通し、実ファイルへ適用し（3-v）、構文として読めない入力は投げる（3-w）。
// 描画された HTML・実ブラウザは読まない（13-L3）。

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

type HtmlTag = ts.JsxOpeningElement | ts.JsxSelfClosingElement;

function isHtmlTag(node: ts.Node): node is HtmlTag {
  return (
    (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) &&
    ts.isIdentifier(node.tagName) &&
    node.tagName.text === "html"
  );
}

/** default export の関数宣言 RootLayout。 */
function findRootLayout(source: ts.SourceFile): ts.FunctionDeclaration | undefined {
  return source.statements.find(
    (s): s is ts.FunctionDeclaration =>
      ts.isFunctionDeclaration(s) &&
      s.name?.text === "RootLayout" &&
      (ts.getModifiers(s) ?? []).some((m) => m.kind === ts.SyntaxKind.ExportKeyword) &&
      (ts.getModifiers(s) ?? []).some((m) => m.kind === ts.SyntaxKind.DefaultKeyword),
  );
}

/** lang の属性値が、合格とする2つの形（文字列リテラル / 式コンテナ内の文字列リテラル）で、ちょうど "ja" か。 */
function isStaticJa(attribute: ts.JsxAttribute): boolean {
  const init = attribute.initializer;
  if (init === undefined) return false;
  if (ts.isStringLiteral(init)) return init.text === "ja";
  if (ts.isJsxExpression(init) && init.expression !== undefined && ts.isStringLiteral(init.expression)) {
    return init.expression.text === "ja";
  }
  return false;
}

/**
 * 1-a (i)(ii): <html> 要素が return 文の式（括弧を外したもの）そのものであり、
 * その return 文を囲む最も内側の関数が root であること。
 */
function isReturnedBy(tag: HtmlTag, root: ts.FunctionDeclaration): boolean {
  let node: ts.Node = ts.isJsxOpeningElement(tag) ? tag.parent : tag;
  while (ts.isParenthesizedExpression(node.parent)) node = node.parent;
  if (!ts.isReturnStatement(node.parent)) return false;
  let fn: ts.Node | undefined = node.parent.parent;
  while (fn !== undefined && !ts.isFunctionLike(fn)) fn = fn.parent;
  return fn === root;
}

/** 13-1 の 1-a〜1-f を構文から読む。構文として読めないテキストは投げる（2-d）。 */
function htmlLangComplies(text: string): boolean {
  const source = parseSource(text);
  const tags: HtmlTag[] = [];
  const collect = (node: ts.Node): void => {
    if (isHtmlTag(node)) tags.push(node);
    ts.forEachChild(node, collect);
  };
  collect(source);
  // 1-a: <html> はファイル内にちょうど1つ、かつ default export の RootLayout が「返す」。
  if (tags.length !== 1) return false;
  const root = findRootLayout(source);
  if (root === undefined) return false;
  const html = tags[0];
  if (!isReturnedBy(html, root)) return false;

  const props = html.attributes.properties;
  // 1-e: spread 属性を置かない。
  if (props.some((p) => ts.isJsxSpreadAttribute(p))) return false;
  // 1-b: lang をちょうど1回（data-lang 等は数えない）。
  const langs = props.filter(
    (p): p is ts.JsxAttribute => ts.isJsxAttribute(p) && ts.isIdentifier(p.name) && p.name.text === "lang",
  );
  if (langs.length !== 1) return false;
  // 1-c / 1-d: 値は静的な文字列リテラルでちょうど "ja"。
  return isStaticJa(langs[0]);
}

const wrap = (html: string): string => `
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    ${html}
  );
}
`;

// 宣言の形（default / export / 名前）だけを変える fixture 用。実ファイルに揃えて async とし、
// 戻り値の形は合格側の fixture（3-a）と同じにする（3-x〜3-z）。
const declare = (head: string, html: string): string => `
${head}({ children }: { children: React.ReactNode }) {
  const cookieStore = await cookies();
  return (
    ${html}
  );
}
`;

// RootLayout の本体と return 文を差し替える fixture 用（3-ab〜3-ah）。1行に並べた形は別の fixture にする（13-2-b）。
const layoutBody = (body: string): string => `
export default function RootLayout({ children }: { children: React.ReactNode }) {
${body}
}
`;
const H = `<html lang="ja"><body>{children}</body></html>`;

describe("layout.tsx の <html lang> - AC-13", () => {
  it("3-v: 実ファイルの layout.tsx の <html> は lang=\"ja\" である", () => {
    const text = readFileSync(path.join(process.cwd(), "src", "app", "layout.tsx"), "utf8");
    expect(text.length).toBeGreaterThan(0);
    expect(htmlLangComplies(text)).toBe(true);
  });

  it.each([
    ["3-a: lang=\"ja\"", wrap(`<html lang="ja"><body>{children}</body></html>`), true],
    ["3-b: lang='ja'（一重引用符）", wrap(`<html lang='ja'><body>{children}</body></html>`), true],
    ["3-c: lang={\"ja\"}（式コンテナ内の文字列リテラル）", wrap(`<html lang={"ja"}><body>{children}</body></html>`), true],
    [
      "3-d: 他の属性と並ぶ（lang が先頭）",
      wrap(`<html lang="ja" data-theme={theme} className="h-full"><body>{children}</body></html>`),
      true,
    ],
    [
      "3-d: 他の属性と並ぶ（lang が先頭でない）",
      wrap(`<html data-theme={theme} className="h-full" lang="ja"><body>{children}</body></html>`),
      true,
    ],
    ["3-e: lang=\"en\"", wrap(`<html lang="en"><body>{children}</body></html>`), false],
    ["3-f: lang の欠落", wrap(`<html><body>{children}</body></html>`), false],
    ["3-g: lang=\"\"", wrap(`<html lang=""><body>{children}</body></html>`), false],
    ["3-h: 値の無い属性 <html lang>", wrap(`<html lang><body>{children}</body></html>`), false],
    ["3-i: lang=\"JA\"", wrap(`<html lang="JA"><body>{children}</body></html>`), false],
    ["3-j: lang=\"ja-JP\"", wrap(`<html lang="ja-JP"><body>{children}</body></html>`), false],
    ["3-k: 先頭の空白 lang=\" ja\"", wrap(`<html lang=" ja"><body>{children}</body></html>`), false],
    ["3-k: 末尾の空白 lang=\"ja \"", wrap(`<html lang="ja "><body>{children}</body></html>`), false],
    ["3-l: lang の2回の記述（ja, en）", wrap(`<html lang="ja" lang="en"><body>{children}</body></html>`), false],
    ["3-l: lang の2回の記述（ja, ja）", wrap(`<html lang="ja" lang="ja"><body>{children}</body></html>`), false],
    ["3-m: 似た名前の属性だけ data-lang=\"ja\"", wrap(`<html data-lang="ja"><body>{children}</body></html>`), false],
    ["3-n: 変数 lang={locale}", wrap(`<html lang={locale}><body>{children}</body></html>`), false],
    ["3-o: 関数呼び出し lang={resolveLang()}", wrap(`<html lang={resolveLang()}><body>{children}</body></html>`), false],
    [
      "3-p: 置換を含まないテンプレートリテラル",
      wrap("<html lang={`ja`}><body>{children}</body></html>"),
      false,
    ],
    ["3-q: spread が lang の後", wrap(`<html lang="ja" {...rest}><body>{children}</body></html>`), false],
    ["3-q: spread が lang の前", wrap(`<html {...rest} lang="ja"><body>{children}</body></html>`), false],
    ["3-r: lang が <body> にだけある", wrap(`<html><body lang="ja">{children}</body></html>`), false],
    [
      "3-s: <html> 要素が無い",
      `export default function RootLayout({ children }: { children: React.ReactNode }) {\n  return <body lang="ja">{children}</body>;\n}\n`,
      false,
    ],
    [
      "3-t: <html lang=\"ja\"> が2つ（条件で出し分ける形）",
      wrap(
        `cond ? <html lang="ja"><body>{children}</body></html> : <html lang="ja"><body>{children}</body></html>`,
      ),
      false,
    ],
    [
      "3-u: <html lang=\"ja\"> が RootLayout 以外の関数にだけある",
      `function Other({ children }: { children: React.ReactNode }) {
  return <html lang="ja"><body>{children}</body></html>;
}
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return <body>{children}</body>;
}
`,
      false,
    ],
    [
      "3-x: named export の RootLayout（default export でない）",
      declare("export async function RootLayout", `<html lang="ja"><body>{children}</body></html>`),
      false,
    ],
    [
      "3-y: export していない RootLayout",
      declare("async function RootLayout", `<html lang="ja"><body>{children}</body></html>`),
      false,
    ],
    [
      "3-z: 名前が RootLayout でない default export",
      declare("export default async function Layout", `<html lang="ja"><body>{children}</body></html>`),
      false,
    ],
    [
      "3-aa: lang={c ? \"ja\" : \"ja\"}（条件式。両方の枝が ja）",
      wrap(`<html lang={c ? "ja" : "ja"}><body>{children}</body></html>`),
      false,
    ],
    [
      "3-ab: 入れ子: 式本体のアロー関数",
      layoutBody(`  const inner = () => ${H};
  return <body>{children}</body>;`),
      false,
    ],
    [
      "3-ab: 入れ子: ブロック本体のアロー関数",
      layoutBody(`  const inner = () => { return ${H}; };
  return <body>{children}</body>;`),
      false,
    ],
    [
      "3-ab: 入れ子: 関数式",
      layoutBody(`  const inner = function () { return ${H}; };
  return <body>{children}</body>;`),
      false,
    ],
    [
      "3-ab: 入れ子: 関数宣言",
      layoutBody(`  function Inner() { return ${H}; }
  return <body>{children}</body>;`),
      false,
    ],
    [
      "3-ab: 入れ子: オブジェクトリテラルのメソッド",
      layoutBody(`  const o = { render() { return ${H}; } };
  return <body>{children}</body>;`),
      false,
    ],
    [
      "3-ab: 入れ子: クラスのメソッド",
      layoutBody(`  class C { render() { return ${H}; } }
  return <body>{children}</body>;`),
      false,
    ],
    [
      "3-ac: 入れ子の関数を呼び出して返す return inner()",
      layoutBody(`  const inner = () => ${H};
  return inner();`),
      false,
    ],
    [
      "3-ac: 入れ子の関数を JSX として描く return <Inner />",
      layoutBody(`  function Inner() { return ${H}; }
  return <Inner />;`),
      false,
    ],
    [
      "3-ad: 変数を経て返す const el = ...; return el",
      layoutBody(`  const el = ${H};
  return el;`),
      false,
    ],
    [
      "3-ad: 変数に入れるだけで返さない",
      layoutBody(`  const el = ${H};
  return <body>{children}</body>;`),
      false,
    ],
    [
      "3-ae: 他の要素の子 <Providers>",
      layoutBody(`  return <Providers>${H}</Providers>;`),
      false,
    ],
    [
      "3-ae: フラグメントの子",
      layoutBody(`  return <>${H}</>;`),
      false,
    ],
    [
      "3-ae: 条件式 c ? <html> : null",
      layoutBody(`  return c ? ${H} : null;`),
      false,
    ],
    [
      "3-ae: 論理式 c && <html>",
      layoutBody(`  return c && ${H};`),
      false,
    ],
    [
      "3-af: 入れ子が2段（最も内側の inner だけが <html> を返す）",
      layoutBody(`  const outer = () => { const inner = () => ${H}; return inner(); };
  return outer();`),
      false,
    ],
    [
      "3-ag: 括弧なし",
      layoutBody(`  return ${H};`),
      true,
    ],
    [
      "3-ag: 括弧あり",
      layoutBody(`  return (${H});`),
      true,
    ],
    [
      "3-ag: 括弧の重ね",
      layoutBody(`  return ((${H}));`),
      true,
    ],
    [
      "3-ah: <html> を含まない入れ子の関数がある",
      layoutBody(`  const pick = (v: string) => v.trim();
  return (${H});`),
      true,
    ],
    [
      "3-ah: 早期 if (c) return null; がある",
      layoutBody(`  if (c) return null;
  return (${H});`),
      true,
    ],
    [
      "3-ah: 入れ子の関数と早期 return null の両方がある",
      layoutBody(`  const pick = (v: string) => v.trim();
  if (c) return null;
  return (${H});`),
      true,
    ],
  ])("fixture %s", (_name, text, expected) => {
    expect(() => parseSource(text)).not.toThrow();
    expect(htmlLangComplies(text)).toBe(expected);
  });

  it("3-w: 構文として読めないテキストは、合格にせず投げる", () => {
    expect(() => htmlLangComplies('const f = <T>(x: T) => x;\n<html lang="ja" />')).toThrow(/構文として読めない/);
  });
});
