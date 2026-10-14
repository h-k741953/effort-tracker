// docs/specs/design-system.md AC-10-3-a / AC-10-3-g / AC-10-3-i / AC-10-4 の検査が
// 共有する補助。アプリの実装からは import しない（検査専用）。
//
// 同じ判定の材料を検査ごとに書き写すと、片方だけを直す変更で検査どうしの判定が
// ずれたまま緑になる（AC-10-3-a「単位の列は1か所に置く」）。そのため1か所に置く。

import { readFileSync } from "node:fs";
import path from "node:path";
import { compile } from "tailwindcss";

// AC-10-3-a (i): `focus-visible:ring-[<長さ>]` の長さの単位。Tailwind v4 が任意値を
// 長さとして推論するもの（大文字の綴りは色として扱われ、リングを描かない）。
// この列の正しさは tailwindTreatsAsRingWidth と突き合わせて読む。
export const CSS_LENGTH_UNITS =
  "cm|mm|Q|in|pc|pt|px|em|ex|ch|rem|lh|rlh|vw|vh|vmin|vmax|vb|vi|svw|svh|lvw|lvh|dvw|dvh|cqw|cqh|cqi|cqb|cqmin|cqmax";

// 単位の判定を Tailwind と突き合わせるときの候補。CSS_LENGTH_UNITS から作らない
// （作ると、列を狭める変更が候補も一緒に狭めて通る）。長さでない単位・Tailwind が
// 推論しない長さの単位・大文字の綴りを負の側として含める。推論しない長さの単位は、
// 推論する単位と同じ系統の綴り（`svw` に対する `svi` 等）まで置く —— 置かないと、
// 列へその綴りを足す変更が候補のどこにも当たらずに通る（AC-10-3-a）。
export const RING_WIDTH_UNIT_CANDIDATES = [
  "cm", "mm", "Q", "in", "pc", "pt", "px", "em", "ex", "ch", "rem", "lh", "rlh",
  "vw", "vh", "vmin", "vmax", "vb", "vi", "svw", "svh", "lvw", "lvh", "dvw", "dvh",
  "cqw", "cqh", "cqi", "cqb", "cqmin", "cqmax",
  "rex", "rch", "cap", "rcap", "ic", "ric",
  "svi", "svb", "lvi", "lvb", "dvi", "dvb", "svmin", "svmax", "lvmin", "lvmax", "dvmin", "dvmax",
  "q", "x", "%", "deg", "fr", "s", "PX", "Rem", "Vw",
];

/** Tailwind v4 が `ring-[<value>]` からリングの幅を生成するか（AC-10-3-a の正解）。 */
export async function tailwindTreatsAsRingWidth(value: string): Promise<boolean> {
  // build は呼ぶたびに候補を蓄積するため、ケースごとに作り直す。
  const compiler = await compile("@tailwind utilities;");
  const css = compiler.build([`ring-[${value}]`]);
  return css.includes(`calc(${value} + var(--tw-ring-offset-width))`);
}

/**
 * 仕様書から AC-5 の節（`### AC-5.` から次の `### ` の手前まで）を切り出す。
 * AC-5-1 の台帳はこの節の中から読む（AC-10-4 / AC-10-3-i）。節が無ければ投げる。
 */
export function readAc5Section(spec: string, hint: string): string {
  const start = spec.indexOf("### AC-5.");
  if (start < 0) throw new Error(`${hint} AC-5 の節が見つからない。`);
  const rest = spec.slice(start + 1);
  const end = rest.indexOf("\n### ");
  return end === -1 ? rest : rest.slice(0, end);
}

// ---------------------------------------------------------------------------
// Theme B（#107）で加わった検査が共有する補助。いずれも仕様書の字面から期待値を
// 読むための抽出と、クラスの「語」の分解である。抽出に失敗したときは、期待値の
// 写しへ退避せず投げる（0件どうしの一致は一致ではない。AC-10-3-d）。
// ---------------------------------------------------------------------------

/** 仕様書 docs/specs/design-system.md の絶対パス。vitest を apps/web から実行する前提。 */
export function designSystemSpecPath(): string {
  return path.join(process.cwd(), "..", "..", "docs", "specs", "design-system.md");
}

export function readDesignSystemSpec(): string {
  return readFileSync(designSystemSpecPath(), "utf8");
}

/** 仕様書から `### AC-<n>.` の節（次の `### ` の手前まで）を切り出す。節が無ければ投げる。 */
export function readAcSection(spec: string, acNumber: number, hint: string): string {
  const start = spec.indexOf(`### AC-${acNumber}.`);
  if (start < 0) throw new Error(`${hint} AC-${acNumber} の節が見つからない。`);
  const rest = spec.slice(start + 1);
  const end = rest.indexOf("\n### ");
  return end === -1 ? rest : rest.slice(0, end);
}

export interface LedgerEntry {
  id: string;
  component: string;
  file: string;
}

/** AC-5-1 の台帳（AC-5 の節の中だけ）を読む。0件・形の合わない行は投げる。 */
export function parseLedgerEntries(spec: string, hint: string): LedgerEntry[] {
  const section = readAc5Section(spec, hint);
  const rowLines = section.match(/^\|\s*5-1-[a-z]+\s*\|.*$/gm) ?? [];
  const entries = rowLines.flatMap((line) => {
    const m = line.match(/^\|\s*(5-1-[a-z]+)\s*\|\s*`([A-Za-z]+)`\s*\|\s*`([a-z-]+\.tsx)`\s*\|/);
    return m ? [{ id: m[1], component: m[2], file: m[3] }] : [];
  });
  if (entries.length === 0) throw new Error(`${hint} 台帳の行を1件も読めなかった。`);
  if (entries.length !== rowLines.length) {
    throw new Error(`${hint} 台帳に形の合わない行がある（${rowLines.length} 行中 ${entries.length} 行だけ読めた）。`);
  }
  return entries;
}

export interface RoundedRule {
  id: string;
  component: string;
  /** その部品に許す角丸ユーティリティ（字面）。空なら角丸を書かない部品。 */
  allowed: string[];
}

/** AC-4-4 の表（AC-4 の節の中だけ）を読む。0件・形の合わない行は投げる。 */
export function parseRoundedRules(spec: string, hint: string): RoundedRule[] {
  const section = readAcSection(spec, 4, hint);
  const rowLines = section.match(/^\|\s*4-4-[a-z]+\s*\|.*$/gm) ?? [];
  const rules = rowLines.flatMap((line) => {
    const m = line.match(/^\|\s*(4-4-[a-z]+)\s*\|\s*`([A-Za-z]+)`\s*\|([^|]*)\|\s*$/);
    if (!m) return [];
    const allowed = [...m[3].matchAll(/`(rounded[^`]*)`/g)].map((token) => token[1]);
    return [{ id: m[1], component: m[2], allowed }];
  });
  if (rules.length === 0) throw new Error(`${hint} 角丸の表の行を1件も読めなかった。`);
  if (rules.length !== rowLines.length) {
    throw new Error(`${hint} 角丸の表に形の合わない行がある（${rowLines.length} 行中 ${rules.length} 行だけ読めた）。`);
  }
  return rules;
}

export interface ClassWord {
  word: string;
  start: number;
  end: number;
}

/**
 * クラス文字列を含むテキストを「語」に分ける（位置つき）。`rounded-(--et-radius-card)` のように
 * 丸括弧を含む語を壊さないよう、空白・引用符・波括弧・カンマ・セミコロン・山括弧・等号で区切る。
 */
export function classWords(text: string): ClassWord[] {
  return [...text.matchAll(/[^\s"'`{},;<>=]+/g)].map((m) => ({
    word: m[0],
    start: m.index ?? 0,
    end: (m.index ?? 0) + m[0].length,
  }));
}

export function splitClassWords(text: string): string[] {
  return classWords(text).map((entry) => entry.word);
}

/** 語を「バリアントの連なり」と「本体」に分ける（`:` で区切った最後の部分が本体）。 */
export function splitVariants(word: string): { variants: string[]; body: string } {
  const parts = word.split(":");
  const body = parts[parts.length - 1];
  return { variants: parts.slice(0, -1), body };
}
