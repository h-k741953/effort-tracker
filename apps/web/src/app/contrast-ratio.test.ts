import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { describe, expect, it } from "vitest";

// docs/specs/design-system.md AC-3（検証手段は AC-10-2）。
//
// 「AC-2 の表の16進値から相対輝度とコントラスト比を計算し、下限と比較する
// （計算は外部ライブラリを使わずテスト内で行う）」を満たす。値は globals.css
// から読む（AC-1-1 の「正は globals.css の1箇所」に沿う。globals.css.test.ts
// の値検査 (AC-2) が別に落ちるため、ここでは重複を避け AC-3 の計算のみを扱う）。

const appDir = fileURLToPath(new URL(".", import.meta.url));
const cssPath = path.join(appDir, "globals.css");
const css = readFileSync(cssPath, "utf8");

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

const darkMediaHeader = /@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)\s*\{\s*:root\s*/;
const darkBlockBody = extractBlockBody(css, darkMediaHeader);
const darkTokens = parseCustomProps(darkBlockBody);

const cssWithoutDarkBlock = darkBlockBody
  ? css.replace(new RegExp(darkMediaHeader.source + "\\{[^}]*\\}\\s*\\}"), "")
  : css;

const lightBlockBody = extractBlockBody(cssWithoutDarkBlock, /:root\s*/);
const lightTokens = parseCustomProps(lightBlockBody);

// WCAG 2.1 の相対輝度・コントラスト比の定義。
function srgbChannelToLinear(c: number): number {
  const s = c / 255;
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function relativeLuminance(hex: string): number {
  const m = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex);
  if (!m) throw new Error(`not a 6-digit hex color: ${hex}`);
  const r = srgbChannelToLinear(parseInt(m[1], 16));
  const g = srgbChannelToLinear(parseInt(m[2], 16));
  const b = srgbChannelToLinear(parseInt(m[3], 16));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrastRatio(a: string, b: string): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const lighter = Math.max(la, lb);
  const darker = Math.min(la, lb);
  return (lighter + 0.05) / (darker + 0.05);
}

// AC-3 の9行（3-7 は状態バッジ3組ぶん展開して11ペア）。
const PAIRS: ReadonlyArray<{ id: string; fg: string; bg: string; min: number }> = [
  { id: "3-1", fg: "foreground", bg: "background", min: 4.5 },
  { id: "3-2", fg: "surface-foreground", bg: "surface", min: 4.5 },
  { id: "3-3", fg: "muted-foreground", bg: "background", min: 4.5 },
  { id: "3-4", fg: "primary-foreground", bg: "primary", min: 4.5 },
  { id: "3-5", fg: "danger-foreground", bg: "danger", min: 4.5 },
  { id: "3-6", fg: "danger", bg: "background", min: 4.5 },
  { id: "3-7 (draft)", fg: "state-draft-foreground", bg: "state-draft", min: 4.5 },
  {
    id: "3-7 (pending-approval)",
    fg: "state-pending-approval-foreground",
    bg: "state-pending-approval",
    min: 4.5,
  },
  { id: "3-7 (approved)", fg: "state-approved-foreground", bg: "state-approved", min: 4.5 },
  { id: "3-8", fg: "border", bg: "background", min: 3 },
  { id: "3-9", fg: "focus-ring", bg: "background", min: 3 },
];

describe("contrast-ratio - AC-3: コントラスト比の下限", () => {
  describe("明色（:root）", () => {
    it.each(PAIRS)("$id: --$fg / --$bg は $min:1 以上", ({ fg, bg, min }) => {
      const fgValue = lightTokens.get(fg);
      const bgValue = lightTokens.get(bg);
      expect(fgValue).toBeDefined();
      expect(bgValue).toBeDefined();
      const ratio = contrastRatio(fgValue as string, bgValue as string);
      expect(ratio).toBeGreaterThanOrEqual(min);
    });
  });

  describe("暗色（prefers-color-scheme: dark）", () => {
    it.each(PAIRS)("$id: --$fg / --$bg は $min:1 以上", ({ fg, bg, min }) => {
      const fgValue = darkTokens.get(fg);
      const bgValue = darkTokens.get(bg);
      expect(fgValue).toBeDefined();
      expect(bgValue).toBeDefined();
      const ratio = contrastRatio(fgValue as string, bgValue as string);
      expect(ratio).toBeGreaterThanOrEqual(min);
    });
  });
});
