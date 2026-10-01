// docs/specs/design-system.md AC-10-3-k
//
// 検査が共有する補助（src/test-support/）を、アプリの実装は import しない。
// 検査専用の補助がアプリの出力に混ざると、検査の都合で実装のふるまいが変わる。
// import はモジュール指定子を構文として読む（コメントや文字列の中の語は数えない）。

import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { describe, expect, it } from "vitest";

const testSupportDir = fileURLToPath(new URL(".", import.meta.url));
const srcDir = path.join(testSupportDir, "..");

/** モジュール指定子が test-support のパス要素を含むか。 */
function pointsToTestSupport(specifier: string): boolean {
  return /(^|\/)test-support(\/|$)/.test(specifier);
}

/** テキストが test-support を import しているか（import 文・再輸出・動的 import()・import 型）。 */
function importsTestSupport(text: string): boolean {
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
  return specifiers.some(pointsToTestSupport);
}

/** アプリの実装（src 配下の *.ts / *.tsx。*.test.* と test-support/ 自身を除く）。 */
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
        !entry.name.endsWith(".test.ts") &&
        !entry.name.endsWith(".test.tsx")
      ) {
        files.push(path.relative(srcDir, full));
      }
    }
  };
  walk(srcDir);
  return files.sort();
}

describe("AC-10-3-k: アプリの実装は test-support を import しない", () => {
  it("母集団（アプリの実装）を読める（0件なら失敗）", () => {
    const files = listImplementationFiles();
    expect(files.length).toBeGreaterThan(0);
    // 走査が空振りしていないことの陽性側の足場。
    expect(files).toContain(path.join("components", "button.tsx"));
    expect(files.some((file) => file.startsWith(`test-support${path.sep}`))).toBe(false);
    // 1件の足場だけでは、走査の範囲を狭める変更（拡張子・階層を落とす形）が通る。
    // 走査とは別の手段で数えた集合と過不足なく一致することを読む（AC-10-3-k）。
    const independent = ts.sys
      .readDirectory(srcDir, [".ts", ".tsx"])
      .map((full) => path.relative(srcDir, full))
      .filter((file) => !/\.test\.tsx?$/.test(file) && !file.startsWith(`test-support${path.sep}`))
      .sort();
    expect(independent.length).toBeGreaterThan(0);
    expect(files).toEqual(independent);
  });

  it.each(listImplementationFiles())("%s は test-support を import しない", (file) => {
    const text = readFileSync(path.join(srcDir, file), "utf8");
    expect(text.length, `${file} から読み取ったテキストが0文字である。`).toBeGreaterThan(0);
    expect(
      importsTestSupport(text),
      `${file} が検査専用の test-support を import している（AC-10-3-k）。`,
    ).toBe(false);
  });

  // 判定の判別力: 恒偽へ潰すと不在の主張が空虚に真になるため、両側を読む。
  it.each([
    ["相対パスで輸入する", 'import { a } from "../test-support/design-system";', true],
    ["パスの別名で輸入する", 'import { a } from "@/test-support/design-system";', true],
    ["再輸出する", 'export * from "../test-support/design-system";', true],
    ["名前を挙げて再輸出する", 'export { a } from "../test-support/design-system";', true],
    ["副作用だけのために import する", 'import "../test-support/design-system";', true],
    ["import type で輸入する", 'import type { a } from "../test-support/design-system";', true],
    ["動的に import する", 'const m = await import("../test-support/design-system");', true],
    ["テンプレートリテラルで動的に import する", "const m = await import(`../test-support/design-system`);", true],
    ["import 型で参照する", 'type T = import("../test-support/design-system").T;', true],
    ["別のモジュールを輸入する", 'import { a } from "@/lib/role-cookie";', false],
    ["コメントで触れる", '// import { a } from "../test-support/design-system";', false],
    ["文字列で触れる", 'const s = "../test-support/design-system";', false],
    ["似た名のディレクトリを輸入する", 'import { a } from "../test-supports/x";', false],
    ["前に語が付いた名のディレクトリを輸入する", 'import { a } from "../my-test-support/x";', false],
    ["ディレクトリそのものを輸入する", 'import { a } from "@/test-support";', true],
  ])("test-support の import の判定: %s", (_name, text, expected) => {
    expect(
      importsTestSupport(text as string),
      `test-support の import の判定が期待と異なる（AC-10-3-k）。対象: ${text}`,
    ).toBe(expected);
  });
});
