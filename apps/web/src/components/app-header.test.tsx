/** @vitest-environment jsdom */
import { readFileSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup, within } from "@testing-library/react";
import { AppHeader } from "./app-header";

// docs/specs/design-system.md AC-6-11（検証手段は AC-10-6）。
//
// 「アプリ名」の具体的な DOM 構造（見出しレベル等）は AC-6 の表が名指し
// しておらず AC-11-7 により検査対象外。ただしアプリ名の文字列が
// `apps/web/src/app/layout.tsx` の metadata.title と同一であることは
// 6-11-i の DOM 契約が定めるため、ここで検証する（AC-10-6-c）。
//
// 突き合わせは相互比較（見出しのテキストと metadata.title を互いに比べる
// 形）では行わない。相互比較だと、両方を同時に書き換える一括改名を
// 素通ししてしまう（AC-11-13 が指摘する穴）。下記の定数は正解の写しでは
// なく、6-11-i 本条文が定めた値（effort-tracker）をテストへピン留めした
// ものである。正解は本条文が持ち、実装（見出し・metadata.title）は
// それに従う関係であって、docs の外に正解が増えるわけではない。

// 10-6-c は metadata.title の読み取りを「輸入・ファイル読み取りのいずれの手段でもよい」とする。
// layout.tsx は 4-3-a により next/font/local の `localFont` を呼ぶ。その呼び出しは Next のビルド
// （SWC の変換）が静的に解決するもので、vitest が素のまま import すると `localFont()` が実行時に
// 投げる。そのため layout.tsx は import せず、ファイルを構文木で読んで metadata.title の
// 文字列リテラルを取り出す。
const layoutPath = path.join(process.cwd(), "src", "app", "layout.tsx");

/** `export const metadata = { title: "…" }` の title の文字列リテラル。読めなければ undefined。 */
function readMetadataTitle(text: string): string | undefined {
  const source = ts.createSourceFile("layout.tsx", text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  let title: string | undefined;
  const visit = (node: ts.Node): void => {
    if (
      ts.isVariableDeclaration(node) &&
      ts.isIdentifier(node.name) &&
      node.name.text === "metadata" &&
      node.initializer !== undefined &&
      ts.isObjectLiteralExpression(node.initializer)
    ) {
      for (const property of node.initializer.properties) {
        if (
          ts.isPropertyAssignment(property) &&
          ts.isIdentifier(property.name) &&
          property.name.text === "title" &&
          (ts.isStringLiteral(property.initializer) || ts.isNoSubstitutionTemplateLiteral(property.initializer))
        ) {
          title = property.initializer.text;
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return title;
}

afterEach(() => {
  cleanup();
});

describe("AppHeader - AC-6-11", () => {
  it("<header> を出力し、アプリ名を見出し要素として含む", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    expect(within(header).getByRole("heading")).toBeTruthy();
  });

  // 6-11-i の定数（正解の写しではなくテスト側のピン留め。上記コメント参照）。
  const EXPECTED_APP_NAME = "effort-tracker";

  it("6-11-i: header 内の見出しのテキストが定数と一致する", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    const heading = within(header).getByRole("heading");
    expect(heading.textContent).toBe(EXPECTED_APP_NAME);
  });

  it("6-11-i: layout.tsx の metadata.title が定数と一致する", () => {
    expect(readMetadataTitle(readFileSync(layoutPath, "utf8"))).toBe(EXPECTED_APP_NAME);
  });

  // 読み取りの判別力（10-3-g と同じ形）。読めない形は undefined（= 定数と一致しない）になり、
  // 適合の形は文字列を返す。
  it.each([
    ['export const metadata: Metadata = { title: "effort-tracker", description: "x" };', "effort-tracker"],
    ["export const metadata = { title: `effort-tracker` };", "effort-tracker"],
    ['export const metadata = { title: "other" };', "other"],
    ['export const metadata = { description: "x" };', undefined],
    ["export const metadata = { title: name };", undefined],
    ['const other = { title: "effort-tracker" };', undefined],
  ])("metadata.title の読み取り: %s", (text, expected) => {
    expect(readMetadataTitle(text)).toBe(expected);
  });

  // 6-11: アプリ名の前に置く装飾の印（時計の図案）は aria-hidden="true" とし、テキストを持たない
  // （<title> を含めない）。見出しのテキストが `effort-tracker` と一致する 6-11-i を変えないため。
  // 印を置くかどうかは条文が「置いてよい」とするだけなので、置いた場合の形を読む。
  it("6-11: 装飾の印（svg）は aria-hidden=\"true\" で、<title> を持たない", () => {
    render(<AppHeader />);
    const header = screen.getByRole("banner");
    for (const svg of Array.from(header.querySelectorAll("svg"))) {
      expect(svg.getAttribute("aria-hidden")).toBe("true");
      expect(svg.querySelector("title")).toBeNull();
      expect(svg.textContent).toBe("");
    }
  });

  it("right に渡した内容を差し込む", () => {
    render(<AppHeader right={<span>差込内容</span>} />);
    expect(screen.getByText("差込内容")).toBeTruthy();
  });
});
