import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { computeWorkingHours, type Break, type TimeOfDay } from "./working-hours";

// docs/specs/daily-record-entry-ui.md AC-3-6-d・3-6-e（算出の写しの突き合わせ）。
//
// 画面の算出（computeWorkingHours）はドメインの算出（daily-record-entry.md AC-2-1-a）の写しである。
// 両側を **同じ期待値の表で検査する**。表の持ち主は daily-record-entry.md AC-3 の 3-B と 3-C で、
// そのうち入力の列に時刻の組（開始〜終了／休憩）を示す行を使う。行をテストコードへ手で写さず、
// この表から入力と期待を読む（ドメイン側は services/api/internal/domain/workmonth の
// daily_record_test.go が同じ表を読む。AC-12-18 ①）。
//
// 期待の読み方（3-6-d）:
//   - 「許可」の行 —— 戻り値がその行の稼働時間と一致する（minutes は 0〜59）。
//   - 「弾く」の行 —— 画面は値を示さない。関数の側では null を返す（0 や負の値を返さない。例外を投げない。3-6-e）。
//
// 表の解析の形（3-6-d が「テスト工程が決める」とした点）:
//   - 行の入力の列から「`h:m`〜`h:m`」の組を取り出す。
//   - 入力に「／休憩なし」があれば休憩なし、「／休憩 」があれば2つ目の組を休憩とする。
//   - 「／」を持たない入力（3-5-a・3-5-e 等）は、見つかった組のそれぞれを休憩なしの別々の入力として読む。
//   - 期待の列に「弾く」があれば弾く行、無ければ許可の行として「N時間M分」を稼働時間とする。
//   - 時刻の組を持たない行（3-1〜3-4-a の時刻1つの行、3-10、3-12）は読み飛ばす。
//     3-A の「弾く」値（粒度・値域の外）を渡したときの戻り値は固定しない（UI 限界 8-9）。

const SPEC_PATH = path.join(process.cwd(), "..", "..", "docs", "specs", "daily-record-entry.md");

interface SpecRow {
  id: string;
  start: TimeOfDay;
  end: TimeOfDay;
  brk: Break | null;
  reject: boolean;
  /** 許可の行の稼働時間（分）。弾く行では null。 */
  wantMinutes: number | null;
}

/** 仕様書から「### AC-N.」の節の表の行を（行番号・入力の列・期待の列）で読む。 */
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

function parseRows(rows: string[][]): SpecRow[] {
  const out: SpecRow[] = [];
  for (const [id, input, expect] of rows) {
    const pairs = [...input.matchAll(/`(\d+):(\d+)`〜`(\d+):(\d+)`/g)];
    if (pairs.length === 0) continue;
    const toRange = (m: RegExpMatchArray): { start: TimeOfDay; end: TimeOfDay } => ({
      start: { hour: Number(m[1]), minute: Number(m[2]) },
      end: { hour: Number(m[3]), minute: Number(m[4]) },
    });
    const reject = expect.includes("弾く");
    const duration = expect.match(/(\d+)時間(\d+)分/);
    const wantMinutes = !reject && duration ? Number(duration[1]) * 60 + Number(duration[2]) : null;
    if (!reject && wantMinutes === null) {
      throw new Error(`行 ${id}: 許可の行なのに稼働時間（N時間M分）を読めない: ${expect}`);
    }
    if (input.includes("／休憩なし")) {
      const r = toRange(pairs[0]);
      out.push({ id, ...r, brk: null, reject, wantMinutes });
    } else if (input.includes("／休憩 ")) {
      if (pairs.length < 2) throw new Error(`行 ${id}: 休憩の時刻の組を読めない: ${input}`);
      const r = toRange(pairs[0]);
      const b = toRange(pairs[1]);
      out.push({ id, ...r, brk: { start: b.start, end: b.end }, reject, wantMinutes });
    } else {
      for (const p of pairs) {
        const r = toRange(p);
        out.push({ id, ...r, brk: null, reject, wantMinutes });
      }
    }
  }
  return out;
}

const SPEC_ROWS = parseRows(readSection(readFileSync(SPEC_PATH, "utf-8"), "### AC-3.", "### AC-4."));

const label = (r: SpecRow) =>
  `${r.id}: ${r.start.hour}:${r.start.minute}〜${r.end.hour}:${r.end.minute}` +
  (r.brk ? ` 休憩 ${r.brk.start.hour}:${r.brk.start.minute}〜${r.brk.end.hour}:${r.brk.end.minute}` : " 休憩なし");

const ALLOWED_IDS = ["3-5", "3-5-d", "3-6", "3-7", "3-7-a", "3-9-b", "3-11", "3-11-a", "3-11-b", "3-11-c"];
const REJECTED_IDS = ["3-5-a", "3-5-b", "3-5-c", "3-5-e", "3-6-a", "3-6-b", "3-8", "3-8-a", "3-8-b", "3-9", "3-9-a"];

describe("仕様書の表の読み取り（解析の陽性対照）", () => {
  // 0件どうしの一致は一致ではない。解析が空振りして表駆動のテストが 0 件で緑になるのを防ぐ。
  it("AC-12-18 ① が参照する許可の行・弾く行がすべて読めている", () => {
    const seen = new Set(SPEC_ROWS.map((r) => r.id));
    for (const id of [...ALLOWED_IDS, ...REJECTED_IDS]) {
      expect(seen.has(id), `仕様書の表から行 ${id} を読めなかった（表の形が変わった、または解析の不具合）`).toBe(true);
    }
  });

  it("3-5-e は例示が2つ（23:45〜23:45 と 23:45〜0:00）とも読めている", () => {
    expect(SPEC_ROWS.filter((r) => r.id === "3-5-e")).toHaveLength(2);
  });

  // 行を足しても落ちない（表に足した行は下の表駆動のテストに自動で現れる）。ここで見るのは、
  // AC-12-18 ① が名指しした行が、許可／弾くの側を取り違えて読まれていないことだけ。
  it("名指しされた行は、許可の側・弾く側を取り違えて読まれていない", () => {
    for (const id of ALLOWED_IDS) {
      for (const r of SPEC_ROWS.filter((x) => x.id === id)) expect(r.reject, `行 ${id}`).toBe(false);
    }
    for (const id of REJECTED_IDS) {
      for (const r of SPEC_ROWS.filter((x) => x.id === id)) expect(r.reject, `行 ${id}`).toBe(true);
    }
  });
});

describe("computeWorkingHours — 仕様書 AC-3 の 3-B・3-C の表（3-6-d・3-6-e）", () => {
  const allowedRows = SPEC_ROWS.filter((r) => !r.reject);
  const rejectedRows = SPEC_ROWS.filter((r) => r.reject);

  it.each(allowedRows.map((r) => [label(r), r] as const))("許可: %s は表の稼働時間を返す", (_name, row) => {
    const got = computeWorkingHours(row.start, row.end, row.brk);

    expect(got).toEqual({ hours: Math.floor(row.wantMinutes! / 60), minutes: row.wantMinutes! % 60 });
    // minutes は 0〜59（時と分の組。契約 D-3）
    expect(got!.minutes).toBeGreaterThanOrEqual(0);
    expect(got!.minutes).toBeLessThanOrEqual(59);
  });

  it.each(rejectedRows.map((r) => [label(r), r] as const))("弾く: %s は null を返す（値を示さない）", (_name, row) => {
    // 例外を投げない（3-6-e）。0 や負の値を返さない（3-6-b）。
    const got = computeWorkingHours(row.start, row.end, row.brk);

    expect(got).toBeNull();
  });
});

describe("computeWorkingHours — 許可の行と弾く行の境界の対（AC-12-18 ① (ii) と同じ対を画面側にも置く）", () => {
  const byId = (id: string): SpecRow => {
    const r = SPEC_ROWS.find((x) => x.id === id);
    if (!r) throw new Error(`仕様書の表から行 ${id} を読めなかった`);
    return r;
  };
  const pairs: Array<[string, string]> = [
    ["3-5", "3-5-a"],
    ["3-6", "3-6-a"],
    ["3-7", "3-8"],
    ["3-7-a", "3-8-a"],
    ["3-9-b", "3-9"],
  ];

  it.each(pairs)("許可 %s と弾く %s は同じ関数で分かれる", (allowedId, rejectedId) => {
    const a = byId(allowedId);
    const r = byId(rejectedId);

    expect(computeWorkingHours(a.start, a.end, a.brk)).not.toBeNull();
    expect(computeWorkingHours(r.start, r.end, r.brk)).toBeNull();
  });
});

describe("computeWorkingHours — 休憩なしと最小の時刻を持つ休憩を取り違えない（daily-record-entry.md AC-2-6-b・2-6-c）", () => {
  it("休憩なし（null）は差し引かない", () => {
    const got = computeWorkingHours({ hour: 9, minute: 0 }, { hour: 18, minute: 0 }, null);

    expect(got).toEqual({ hours: 9, minutes: 0 });
  });

  it("0:00〜0:15 の休憩は休憩ありとして差し引く（ゼロ値を休憩なしと取り違えない）", () => {
    const got = computeWorkingHours(
      { hour: 0, minute: 0 },
      { hour: 23, minute: 45 },
      { start: { hour: 0, minute: 0 }, end: { hour: 0, minute: 15 } },
    );

    expect(got).toEqual({ hours: 23, minutes: 30 });
  });
});

describe("computeWorkingHours — 純粋関数（3-6-e）", () => {
  it("同じ引数には常に同じ戻り値を返し、引数を書き換えない", () => {
    const start: TimeOfDay = Object.freeze({ hour: 9, minute: 15 });
    const end: TimeOfDay = Object.freeze({ hour: 17, minute: 45 });
    const brk: Break = Object.freeze({
      start: Object.freeze({ hour: 12, minute: 30 }),
      end: Object.freeze({ hour: 13, minute: 15 }),
    });

    const first = computeWorkingHours(start, end, brk);
    const second = computeWorkingHours(start, end, brk);

    expect(first).toEqual({ hours: 7, minutes: 45 });
    expect(second).toEqual(first);
    expect(start).toEqual({ hour: 9, minute: 15 });
    expect(brk).toEqual({ start: { hour: 12, minute: 30 }, end: { hour: 13, minute: 15 } });
  });

  it("弾く行を何度呼んでも例外を投げず null を返す", () => {
    const calls = () => [
      computeWorkingHours({ hour: 9, minute: 0 }, { hour: 9, minute: 0 }, null),
      computeWorkingHours({ hour: 18, minute: 0 }, { hour: 9, minute: 0 }, null),
    ];

    expect(calls).not.toThrow();
    expect(calls()).toEqual([null, null]);
  });
});
