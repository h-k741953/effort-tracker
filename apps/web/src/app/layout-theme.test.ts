import { readFileSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";
import { readAcSection, readDesignSystemSpec } from "../test-support/design-system";

// docs/specs/design-system.md AC-12-3 / AC-12-6（検証手段は AC-10-7-a の (iii)）。
//
// (iii) 次の描画での反映 —— apps/web/src/app/layout.tsx を TypeScript の構文木で読み、出力する
// <html> 要素の data-theme 属性の式が、theme-cookie.ts から輸入した resolveTheme の呼び出しであり、
// その引数が THEME_COOKIE_NAME の名前で読んだ Cookie の値であることを読む。(i)(ii)（theme-cookie.test.ts）と
// 合わせて「保存した値が次の描画で <html> の data-theme に反映される」と「保存が無いとき light である」の
// 観測可能な形とする。判定は述語として切り出し、条文どおりの形 → 適合、data-theme を定数 "light" に
// した形・別の名前の Cookie を読む形・resolveTheme を経ずに Cookie の値をそのまま渡す形 → 不適合、を
// 実ケースと同じ手段で読む。期待する Cookie の名前は AC-12-6 の字面から取る。
// 実ブラウザの保存・送出は読まない（11-43）。

const hint =
  "仕様書の AC-12-6 の行から Cookie の名前・モジュールを読めなかった。条文の体裁を変えたのなら、要求の" +
  "変更ではないのでこの抽出側を追随させること（10-7-a は抽出手段を仕様で固定していない）。";

function readSpecRow(): string {
  const row = readAcSection(readDesignSystemSpec(), 12, hint).match(/^\|\s*12-6\s*\|.*$/m);
  if (!row) throw new Error(`${hint} 12-6 の行が見つからない。`);
  return row[0];
}

const ROW_12_6 = readSpecRow();
const SPEC_COOKIE_NAME = (ROW_12_6.match(/`THEME_COOKIE_NAME`（値は `([^`]+)`/) ?? [])[1];
const SPEC_MODULE = (ROW_12_6.match(/`apps\/web\/src\/lib\/([\w-]+)\.ts`/) ?? [])[1];

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

/** theme-cookie から輸入した名前（別名を含む）の、ローカルの名前。 */
function importedLocalNames(source: ts.SourceFile, moduleBase: string): Map<string, string> {
  const names = new Map<string, string>();
  for (const statement of source.statements) {
    if (
      ts.isImportDeclaration(statement) &&
      ts.isStringLiteral(statement.moduleSpecifier) &&
      statement.moduleSpecifier.text.endsWith(`/${moduleBase}`)
    ) {
      const bindings = statement.importClause?.namedBindings;
      if (bindings !== undefined && ts.isNamedImports(bindings)) {
        for (const element of bindings.elements) names.set((element.propertyName ?? element.name).text, element.name.text);
      }
    }
  }
  return names;
}

/** 括弧・await・型アサーション・非 null を外し、const の識別子はその初期化式へ辿る。 */
function unwrap(expression: ts.Expression, source: ts.SourceFile, depth = 0): ts.Expression {
  let current = expression;
  for (;;) {
    if (
      ts.isParenthesizedExpression(current) ||
      ts.isAwaitExpression(current) ||
      ts.isAsExpression(current) ||
      ts.isNonNullExpression(current)
    ) {
      current = current.expression;
    } else break;
  }
  if (ts.isIdentifier(current) && depth < 4) {
    const name = current.text;
    let init: ts.Expression | undefined;
    const visit = (node: ts.Node): void => {
      if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.name.text === name && node.initializer) {
        init = node.initializer;
      }
      ts.forEachChild(node, visit);
    };
    visit(source);
    if (init !== undefined) return unwrap(init, source, depth + 1);
  }
  return current;
}

/** 式の部分木のどこかに `<…>.get(<名前>)` があり、その名前が THEME_COOKIE_NAME（または条文の値の文字列）か。 */
function readsThemeCookie(expression: ts.Expression, source: ts.SourceFile, cookieNameLocal: string | undefined): boolean {
  let found = false;
  const visit = (node: ts.Node): void => {
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      node.expression.name.text === "get" &&
      node.arguments.length === 1
    ) {
      const arg = node.arguments[0];
      if (
        (cookieNameLocal !== undefined && ts.isIdentifier(arg) && arg.text === cookieNameLocal) ||
        (ts.isStringLiteralLike(arg) && arg.text === SPEC_COOKIE_NAME)
      ) {
        found = true;
      }
    }
    ts.forEachChild(node, visit);
  };
  // 識別子で渡した引数（const raw = cookieStore.get(NAME)?.value）は初期化式まで辿る。
  visit(unwrap(expression, source));
  return found;
}

/** <html> の data-theme 属性の式が、条文どおりの resolveTheme(<THEME_COOKIE_NAME の Cookie の値>) か。 */
function htmlDataThemeComplies(text: string, moduleBase: string): boolean {
  const source = parseSource(text);
  const imported = importedLocalNames(source, moduleBase);
  const resolveLocal = imported.get("resolveTheme");
  const nameLocal = imported.get("THEME_COOKIE_NAME");
  if (resolveLocal === undefined) return false;
  let complies = false;
  const visit = (node: ts.Node): void => {
    if (
      (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) &&
      ts.isIdentifier(node.tagName) &&
      node.tagName.text === "html"
    ) {
      for (const attribute of node.attributes.properties) {
        if (
          ts.isJsxAttribute(attribute) &&
          ts.isStringLiteral(attribute.name as unknown as ts.Node) === false &&
          attribute.name.getText(source) === "data-theme" &&
          attribute.initializer !== undefined &&
          ts.isJsxExpression(attribute.initializer) &&
          attribute.initializer.expression !== undefined
        ) {
          const expression = unwrap(attribute.initializer.expression, source);
          if (
            ts.isCallExpression(expression) &&
            ts.isIdentifier(expression.expression) &&
            expression.expression.text === resolveLocal &&
            expression.arguments.length === 1 &&
            readsThemeCookie(expression.arguments[0], source, nameLocal)
          ) {
            complies = true;
          }
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return complies;
}

const COMPLIANT = `
import { cookies } from "next/headers";
import { THEME_COOKIE_NAME, resolveTheme } from "@/lib/theme-cookie";
export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const cookieStore = await cookies();
  return (
    <html lang="ja" data-theme={resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)}>
      <body>{children}</body>
    </html>
  );
}
`;

describe("layout.tsx の <html data-theme> - AC-12-3 / AC-10-7-a (iii)", () => {
  it("期待値（Cookie の名前・モジュール）を条文の字面から読める", () => {
    expect(SPEC_COOKIE_NAME).toBeTruthy();
    expect(SPEC_MODULE).toBe("theme-cookie");
  });

  it("layout.tsx の <html> の data-theme は、theme-cookie の resolveTheme(THEME_COOKIE_NAME の Cookie の値) である", () => {
    const text = readFileSync(path.join(process.cwd(), "src", "app", "layout.tsx"), "utf8");
    expect(text.length).toBeGreaterThan(0);
    expect(htmlDataThemeComplies(text, SPEC_MODULE)).toBe(true);
  });

  it.each([
    ["条文どおりの形", COMPLIANT, true],
    [
      "値を const に取ってから渡す形",
      COMPLIANT.replace(
        `<html lang="ja" data-theme={resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)}>`,
        `<html lang="ja" data-theme={theme}>`,
      ).replace(`return (`, `const theme = resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value);\n  return (`),
      true,
    ],
    [
      "Cookie の値を const に取ってから resolveTheme へ渡す形",
      COMPLIANT.replace(`resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)`, `resolveTheme(raw)`).replace(
        `return (`,
        `const raw = cookieStore.get(THEME_COOKIE_NAME)?.value;\n  return (`,
      ),
      true,
    ],
    ["data-theme を定数 \"light\" にした形", COMPLIANT.replace(`{resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)}`, `"light"`), false],
    ["data-theme を式の定数にした形", COMPLIANT.replace(`resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)`, `"dark"`), false],
    ["別の名前の Cookie を読む形", COMPLIANT.replace(`THEME_COOKIE_NAME)?.value`, `"et_other")?.value`), false],
    [
      "resolveTheme を経ずに Cookie の値をそのまま渡す形",
      COMPLIANT.replace(`resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)`, `cookieStore.get(THEME_COOKIE_NAME)?.value`),
      false,
    ],
    [
      "resolveTheme を theme-cookie 以外から輸入した形",
      COMPLIANT.replace(`import { THEME_COOKIE_NAME, resolveTheme } from "@/lib/theme-cookie";`, `import { THEME_COOKIE_NAME } from "@/lib/theme-cookie";\nimport { resolveTheme } from "@/lib/other";`),
      false,
    ],
    ["data-theme 属性が無い形", COMPLIANT.replace(` data-theme={resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)}`, ""), false],
    ["<html> 以外の要素に付けた形", COMPLIANT.replace(`<html lang="ja" data-theme={resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)}>`, `<html lang="ja">`).replace(`<body>`, `<body data-theme={resolveTheme(cookieStore.get(THEME_COOKIE_NAME)?.value)}>`), false],
    ["resolveTheme の引数が Cookie の値でない形", COMPLIANT.replace(`cookieStore.get(THEME_COOKIE_NAME)?.value`, `"dark"`), false],
  ])("data-theme の式の判定: %s", (_name, text, expected) => {
    expect(htmlDataThemeComplies(text as string, "theme-cookie")).toBe(expected);
  });

  it("構文として読めないテキストは、読み進めずに投げる", () => {
    expect(() => htmlDataThemeComplies('const f = <T>(x: T) => x;\n<html data-theme="light" />', "theme-cookie")).toThrow(
      /構文として読めない/,
    );
  });
});
