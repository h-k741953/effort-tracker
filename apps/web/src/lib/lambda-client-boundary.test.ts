import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { describe, expect, it } from "vitest";
// docs/specs/bff-wiring.md AC-1（一方通行と集約）の検査 1-a〜1-d。
//
// 「不在の主張には陽性対照を添える」「母集団（走査の対象ファイル）は手書きで
// 列挙せず、ファイルの走査で得る」（AC-1 検査の前文・AC-11-4）。
//
// 母集団: apps/web/src/ 配下の全 .ts / .tsx。**テスト（*.test.*）と検査専用の
// 補助（src/test-support/）は除く**（test-support は実装から import されない
// ことを test-support-boundary.test.ts が別に固定している）。

const srcDir = path.join(fileURLToPath(new URL(".", import.meta.url)), "..");
const LAMBDA_CLIENT_PATH = path.join("lib", "lambda-client.ts");

function listImplementationFiles(): string[] {
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (full === path.join(srcDir, "test-support")) continue;
        walk(full);
      } else if (
        (entry.name.endsWith(".ts") || entry.name.endsWith(".tsx")) &&
        !/\.test\.tsx?$/.test(entry.name)
      ) {
        files.push(path.relative(srcDir, full));
      }
    }
  };
  walk(srcDir);
  return files.sort();
}

/** import / 再輸出 / 動的 import() / import 型のモジュール指定子を構文として読む。 */
function moduleSpecifiers(text: string): string[] {
  const source = ts.createSourceFile("source.tsx", text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const specifiers: string[] = [];
  const visit = (node: ts.Node): void => {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier !== undefined &&
      ts.isStringLiteral(node.moduleSpecifier)
    ) {
      specifiers.push(node.moduleSpecifier.text);
    } else if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      node.arguments.length > 0 &&
      ts.isStringLiteralLike(node.arguments[0])
    ) {
      specifiers.push(node.arguments[0].text);
    } else if (
      ts.isImportTypeNode(node) &&
      ts.isLiteralTypeNode(node.argument) &&
      ts.isStringLiteral(node.argument.literal)
    ) {
      specifiers.push(node.argument.literal.text);
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return specifiers;
}

function importsLambdaClient(text: string): boolean {
  return moduleSpecifiers(text).some((specifier) =>
    /(^|\/)lambda-client$/.test(specifier),
  );
}

/** ファイル先頭のディレクティブに "use client" があるか。 */
function hasUseClientDirective(text: string): boolean {
  const source = ts.createSourceFile("source.tsx", text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  for (const statement of source.statements) {
    if (
      ts.isExpressionStatement(statement) &&
      ts.isStringLiteral(statement.expression)
    ) {
      if (statement.expression.text === "use client") return true;
      continue;
    }
    break;
  }
  return false;
}

/**
 * AC-1-2: lambda-client を import してよいのは src/app/api/ 配下の Route Handler
 * と、それが呼ぶ src/lib/ 側の処理に限る。src/app/ のページ・レイアウト、
 * src/components/ は許可しない。
 */
function isAllowedImporter(relativePath: string): boolean {
  const normalized = relativePath.split(path.sep).join("/");
  return normalized.startsWith("app/api/") || normalized.startsWith("lib/");
}

function collectImporters(): string[] {
  return listImplementationFiles().filter((file) =>
    importsLambdaClient(readFileSync(path.join(srcDir, file), "utf8")),
  );
}

describe("AC-1-2: lambda-client を import するファイルの場所（検査 1-a・1-b）", () => {
  it("1-a: lambda-client を import するファイルがすべて許可された場所にあり、1件以上ある（0件を一致とみなさない）", () => {
    const importers = collectImporters();

    expect(importers.length).toBeGreaterThanOrEqual(1);
    // 業務の Route Handler が実際に lambda-client を使っていること。
    expect(importers.some((file) => file.split(path.sep).join("/").startsWith("app/api/"))).toBe(true);
    const violations = importers.filter((file) => !isAllowedImporter(file));
    expect(violations).toEqual([]);
  });

  it("1-b（陽性対照）: 許可外の場所（ページ・components）に import を持つ合成入力は違反として検出される", () => {
    const synthetic = [
      { file: path.join("app", "page.tsx"), text: 'import { createLambdaClient } from "@/lib/lambda-client";' },
      { file: path.join("components", "card-list.tsx"), text: 'import { createLambdaClient } from "../lib/lambda-client";' },
    ];

    const violations = synthetic
      .filter(({ text }) => importsLambdaClient(text))
      .map(({ file }) => file)
      .filter((file) => !isAllowedImporter(file));

    expect(violations).toEqual([
      path.join("app", "page.tsx"),
      path.join("components", "card-list.tsx"),
    ]);
  });

  it("1-b（対）: 許可された場所の合成入力は違反にならず、無関係な import は拾わない", () => {
    expect(isAllowedImporter(path.join("app", "api", "work-months", "route.ts"))).toBe(true);
    expect(isAllowedImporter(path.join("lib", "work-month-handlers.ts"))).toBe(true);
    expect(importsLambdaClient('import { x } from "./not-lambda-client-like";')).toBe(false);
  });
});

describe("AC-1-3: クライアントへ載せない（検査 1-c）", () => {
  it("1-c: lambda-client.ts と、それを import する全ファイルが 'use client' を含まない", () => {
    const files = [LAMBDA_CLIENT_PATH, ...collectImporters()];

    expect(files.length).toBeGreaterThanOrEqual(2);
    for (const file of files) {
      const text = readFileSync(path.join(srcDir, file), "utf8");
      expect(`${file}: ${hasUseClientDirective(text)}`).toBe(`${file}: false`);
    }
  });

  it("1-c（陽性対照）: 'use client' を持つ合成入力は違反として検出される", () => {
    expect(hasUseClientDirective('"use client";\nimport "./lambda-client";')).toBe(true);
    expect(hasUseClientDirective("'use client'\nexport const x = 1;")).toBe(true);
    expect(hasUseClientDirective('import "./lambda-client";\n')).toBe(false);
  });
});

describe("AC-1-4: 呼び先をブラウザへ露出させない（検査 1-d）", () => {
  it("1-d: lambda-client.ts が NEXT_PUBLIC_ で始まる環境変数名を読まない（陽性対照つき）", () => {
    const text = readFileSync(path.join(srcDir, LAMBDA_CLIENT_PATH), "utf8");

    expect(text).not.toMatch(/NEXT_PUBLIC_/);
    // 陽性対照: 同じ判定が、接頭辞を含む合成入力を検出する。
    expect('const url = process.env.NEXT_PUBLIC_DOMAIN_API_URL;').toMatch(/NEXT_PUBLIC_/);
  });
});
