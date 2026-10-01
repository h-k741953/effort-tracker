// docs/specs/design-system.md AC-10-3-a / AC-10-3-g / AC-10-3-i / AC-10-4 の検査が
// 共有する補助。アプリの実装からは import しない（検査専用）。
//
// 同じ判定の材料を検査ごとに書き写すと、片方だけを直す変更で検査どうしの判定が
// ずれたまま緑になる（AC-10-3-a「単位の列は1か所に置く」）。そのため1か所に置く。

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
