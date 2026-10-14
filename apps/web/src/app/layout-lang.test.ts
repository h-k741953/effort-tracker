import { readFileSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";

// docs/specs/web-app-scaffold.md AC-13（13-2 / 13-3）。
//
// layout.tsx を TypeScript の構文木で読み、default export の RootLayout が返す <html> 要素が
// lang="ja" で画面の言語を示すことを読む。layout-theme.test.ts（同じ <html> の data-theme を読む）に倣う。
// 判定は述語として切り出し、テキストを受け取って単体で呼べる形にする（13-2-b）。合格側 3-a〜3-d と
// 不合格側 3-e〜3-u を fixture として通し、実ファイルへ適用し（3-v）、構文として読めない入力は投げる（3-w）。
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

/** 13-1 の 1-a〜1-f を構文から読む。構文として読めないテキストは投げる（2-d）。 */
function htmlLangComplies(text: string): boolean {
  const source = parseSource(text);
  const tags: HtmlTag[] = [];
  const collect = (node: ts.Node): void => {
    if (isHtmlTag(node)) tags.push(node);
    ts.forEachChild(node, collect);
  };
  collect(source);
  // 1-a: <html> はファイル内にちょうど1つ、かつ default export の RootLayout の本体の中。
  if (tags.length !== 1) return false;
  const root = findRootLayout(source);
  if (root?.body === undefined) return false;
  const html = tags[0];
  if (html.pos < root.body.pos || html.end > root.body.end) return false;

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
  ])("fixture %s", (_name, text, expected) => {
    expect(htmlLangComplies(text)).toBe(expected);
  });

  it("3-w: 構文として読めないテキストは、合格にせず投げる", () => {
    expect(() => htmlLangComplies('const f = <T>(x: T) => x;\n<html lang="ja" />')).toThrow(/構文として読めない/);
  });
});
