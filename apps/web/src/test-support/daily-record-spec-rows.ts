// docs/specs/daily-record-entry-ui.md AC-3-6-d。daily-record-entry.md AC-3 の 3-B・3-C の表から、
// 時刻の組を持つ行を読む補助（working-hours.test.ts の読み方と同じ。行をテストへ手で写さない）。
//
// 読み方: 入力の列から「`h:m`〜`h:m`」の組を取り出す。「／休憩なし」があれば休憩なし、「／休憩 」があれば
// 2つ目の組を休憩とする。「／」を持たない入力は、見つかった組のそれぞれを休憩なしの別々の入力として読む。
// 期待の列に「弾く」があれば弾く行。時刻の組を持たない行は読み飛ばす。

import { readFileSync } from "node:fs";
import path from "node:path";
import type { Break, TimeOfDay } from "@/lib/working-hours";

export interface SpecTimeRow {
  id: string;
  start: TimeOfDay;
  end: TimeOfDay;
  brk: Break | null;
  reject: boolean;
}

const SPEC_PATH = path.join(process.cwd(), "..", "..", "docs", "specs", "daily-record-entry.md");

function readSection(spec: string, heading: string, nextHeading: string): string[][] {
  const start = spec.indexOf(heading);
  if (start < 0) throw new Error(`仕様書に ${heading} の節が見つからない。`);
  const end = spec.indexOf(nextHeading, start);
  if (end < 0) throw new Error(`仕様書に ${nextHeading} の節が見つからない。`);
  const rows: string[][] = [];
  for (const line of spec.slice(start, end).split("\n")) {
    if (!line.startsWith("| ")) continue;
    const cells = line.split(" | ");
    if (cells.length < 3) continue;
    const id = cells[0].replace(/^\|/, "").trim();
    if (id === "" || id === "#" || id.startsWith("---")) continue;
    rows.push([id, cells[1], cells.slice(2).join(" | ").replace(/ \|$/, "")]);
  }
  return rows;
}

export function readSpecTimeRows(): SpecTimeRow[] {
  const rows = readSection(readFileSync(SPEC_PATH, "utf-8"), "### AC-3.", "### AC-4.");
  const out: SpecTimeRow[] = [];
  for (const [id, input, expected] of rows) {
    const pairs = [...input.matchAll(/`(\d+):(\d+)`〜`(\d+):(\d+)`/g)];
    if (pairs.length === 0) continue;
    const toRange = (m: RegExpMatchArray): { start: TimeOfDay; end: TimeOfDay } => ({
      start: { hour: Number(m[1]), minute: Number(m[2]) },
      end: { hour: Number(m[3]), minute: Number(m[4]) },
    });
    const reject = expected.includes("弾く");
    if (input.includes("／休憩なし")) {
      out.push({ id, ...toRange(pairs[0]), brk: null, reject });
    } else if (input.includes("／休憩 ")) {
      if (pairs.length < 2) throw new Error(`行 ${id}: 休憩の時刻の組を読めない: ${input}`);
      const r = toRange(pairs[0]);
      const b = toRange(pairs[1]);
      out.push({ id, ...r, brk: { start: b.start, end: b.end }, reject });
    } else {
      for (const p of pairs) out.push({ id, ...toRange(p), brk: null, reject });
    }
  }
  return out;
}
