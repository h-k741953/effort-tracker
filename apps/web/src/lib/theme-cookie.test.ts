import { readFileSync } from "node:fs";
import { builtinModules } from "node:module";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";
import { readAcSection, readDesignSystemSpec } from "@/test-support/design-system";
import { THEME_COOKIE_NAME, resolveTheme, serializeThemeCookie } from "./theme-cookie";

// docs/specs/design-system.md AC-12-3 / AC-12-4 / AC-12-6（検証手段は AC-10-7-a の (i)(ii)）。
//
// (i) 初期値と復元 —— resolveTheme に、Cookie の値として undefined → light、"light" → light、
//     "dark" → dark、"" / "DARK" / "dark " / "system" → light を与えた結果を表で読む（保存が無い
//     ときと読めない値のときに明色であることの観測可能な形）。
// (ii) 保存した値の往復 —— serializeThemeCookie が light / dark のそれぞれについて返す文字列が、
//     名前 THEME_COOKIE_NAME・値がその選択の文字列であること、属性 Path=/・SameSite=Lax・Secure・
//     正の整数の Max-Age を持ち HttpOnly を持たないことを読み、その名前と値の組を resolveTheme に
//     戻すと元の選択になることを読む。
// 期待する Cookie の名前は AC-12-6 の字面から取る。

const hint =
  "仕様書の AC-12-6 の行から Cookie の名前を読めなかった。条文の体裁を変えたのなら、名前の変更では" +
  "ないのでこの抽出側を追随させること（10-7-a は抽出手段を仕様で固定していない）。";

function readSpecCookieName(spec: string): string {
  const row = readAcSection(spec, 12, hint).match(/^\|\s*12-6\s*\|.*$/m);
  if (!row) throw new Error(`${hint} 12-6 の行が見つからない。`);
  const m = row[0].match(/`THEME_COOKIE_NAME`（値は `([^`]+)`/);
  if (!m) throw new Error(`${hint} 「THEME_COOKIE_NAME（値は …）」を切り出せなかった。`);
  return m[1];
}

const SPEC_COOKIE_NAME = readSpecCookieName(readDesignSystemSpec());

/** Set-Cookie 形式の文字列を、最初の `名前=値` と属性の列に分ける。 */
function parseSetCookie(cookie: string): { name: string; value: string; attributes: Map<string, string | true> } {
  const [first, ...rest] = cookie.split(";").map((part) => part.trim());
  const eq = first.indexOf("=");
  const attributes = new Map<string, string | true>();
  for (const attribute of rest) {
    const index = attribute.indexOf("=");
    if (index === -1) attributes.set(attribute.toLowerCase(), true);
    else attributes.set(attribute.slice(0, index).toLowerCase(), attribute.slice(index + 1));
  }
  return { name: first.slice(0, eq), value: first.slice(eq + 1), attributes };
}

describe("theme-cookie - AC-12-6 / AC-10-7-a", () => {
  it("条文から読んだ Cookie の名前は空でなく、THEME_COOKIE_NAME と一致する（12-6）", () => {
    expect(SPEC_COOKIE_NAME.length).toBeGreaterThan(0);
    expect(THEME_COOKIE_NAME).toBe(SPEC_COOKIE_NAME);
  });

  // (i) 初期値と復元（12-4）。
  it.each([
    [undefined, "light"],
    ["light", "light"],
    ["dark", "dark"],
    ["", "light"],
    ["DARK", "light"],
    ["Light", "light"],
    ["dark ", "light"],
    [" dark", "light"],
    ["system", "light"],
    ["auto", "light"],
  ])("(i) resolveTheme(%j) は %s を返す（保存が無いとき・読めない値のときは常に明色）", (cookieValue, expected) => {
    expect(resolveTheme(cookieValue)).toBe(expected);
  });

  // (ii) 保存した値の往復（12-3・12-6）。
  describe.each(["light", "dark"] as const)("(ii) serializeThemeCookie(%s)", (theme) => {
    const cookie = serializeThemeCookie(theme);
    const parsed = parseSetCookie(cookie);

    it("名前は THEME_COOKIE_NAME、値はその選択の文字列である（署名しない・他の情報を結び付けない）", () => {
      expect(parsed.name).toBe(THEME_COOKIE_NAME);
      expect(parsed.value).toBe(theme);
    });

    it("属性 Path=/・SameSite=Lax・Secure・正の整数の Max-Age を持つ", () => {
      expect(parsed.attributes.get("path")).toBe("/");
      expect(parsed.attributes.get("samesite")).toBe("Lax");
      expect(parsed.attributes.get("secure")).toBe(true);
      const maxAge = parsed.attributes.get("max-age");
      expect(typeof maxAge).toBe("string");
      expect(maxAge as string).toMatch(/^[1-9]\d*$/);
    });

    it("HttpOnly を持たない（スクリプトから書くため）", () => {
      expect(parsed.attributes.has("httponly")).toBe(false);
    });

    it("その名前と値の組を resolveTheme に戻すと元の選択になる", () => {
      expect(resolveTheme(parsed.value)).toBe(theme);
    });
  });

  it("light と dark で値だけが異なる", () => {
    const light = parseSetCookie(serializeThemeCookie("light"));
    const dark = parseSetCookie(serializeThemeCookie("dark"));
    expect(light.name).toBe(dark.name);
    expect(light.value).not.toBe(dark.value);
  });

  // 12-6: ブラウザからも使うため、Node 専用のモジュール（node:crypto 等）を輸入しない。
  it("Node 専用のモジュールを輸入しない（12-6）", () => {
    const text = readFileSync(path.join(process.cwd(), "src", "lib", "theme-cookie.ts"), "utf8");
    expect(text.length).toBeGreaterThan(0);
    const source = ts.createSourceFile("theme-cookie.ts", text, ts.ScriptTarget.Latest, true);
    const specifiers: string[] = [];
    const visit = (node: ts.Node): void => {
      if (
        (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
        node.moduleSpecifier !== undefined &&
        ts.isStringLiteral(node.moduleSpecifier)
      ) {
        specifiers.push(node.moduleSpecifier.text);
      }
      ts.forEachChild(node, visit);
    };
    visit(source);
    const nodeOnly = specifiers.filter(
      (specifier) => specifier.startsWith("node:") || builtinModules.includes(specifier),
    );
    expect(nodeOnly).toEqual([]);
  });
});
