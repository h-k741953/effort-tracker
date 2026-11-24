import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  OPERATOR_OWNERSHIP_KEY,
  formatHours,
  formatSettlementRange,
  formatTimeOfDay,
  formatTimeRange,
  readOwnership,
} from "./work-month";

// docs/specs/work-month-screen-ui.md 1-3・1-4・AC-3、daily-record-entry-ui.md 1-1・1-6。
// 設計 M-3・M-5・M-11・T-3。本人かの真偽のキー名の字面はテストに書かず、定数を輸入して本文を組む。

describe("formatHours（M-5。小数第2位までに丸め、末尾が 0 なら1字だけ落とす）", () => {
  it.each([
    [{ hours: 9, minutes: 30 }, "9.5"],
    [{ hours: 10, minutes: 0 }, "10.0"],
    [{ hours: 7, minutes: 45 }, "7.75"],
    [{ hours: 0, minutes: 15 }, "0.25"],
    [{ hours: 0, minutes: 0 }, "0.0"],
    [{ hours: 180, minutes: 15 }, "180.25"],
  ])("%j → %s", (value, expected) => {
    expect(formatHours(value)).toBe(expected);
  });
});

describe("formatTimeOfDay（M-5。時は 0 で埋めず、分は2桁）", () => {
  it.each([
    [{ hour: 9, minute: 0 }, "9:00"],
    [{ hour: 17, minute: 45 }, "17:45"],
    [{ hour: 0, minute: 15 }, "0:15"],
  ])("%j → %s", (value, expected) => {
    expect(formatTimeOfDay(value)).toBe(expected);
  });
});

describe("formatTimeRange（M-11。「〜」で結ぶ）", () => {
  it("9:00〜18:00", () => {
    expect(formatTimeRange({ hour: 9, minute: 0 }, { hour: 18, minute: 0 })).toBe("9:00〜18:00");
  });
  it("0:15〜23:45", () => {
    expect(formatTimeRange({ hour: 0, minute: 15 }, { hour: 23, minute: 45 })).toBe("0:15〜23:45");
  });
});

describe("readOwnership（M-3）", () => {
  it.each([
    ["キーの値が true", { [OPERATOR_OWNERSHIP_KEY]: true }, "own"],
    ["キーの値が false", { [OPERATOR_OWNERSHIP_KEY]: false }, "other"],
    ["キーが無い", {}, "unknown"],
    ["値が文字列 \"true\"", { [OPERATOR_OWNERSHIP_KEY]: "true" }, "unknown"],
    ["値が 1", { [OPERATOR_OWNERSHIP_KEY]: 1 }, "unknown"],
    ["値が null", { [OPERATOR_OWNERSHIP_KEY]: null }, "unknown"],
    ["本文が null", null, "unknown"],
    ["本文が配列", [], "unknown"],
    ["本文が文字列", "text", "unknown"],
  ])("%s → %s", (_name, body, expected) => {
    expect(readOwnership(body)).toBe(expected);
  });
});

describe("キー名の持ち主が1つであること（M-3）", () => {
  const SRC_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");

  interface SourceFile {
    path: string;
    content: string;
  }

  /** 「パスと内容の組の列」から、内容に needle の字面（引用符を問わない）が現れるパスを返す。 */
  function pathsContaining(files: readonly SourceFile[], needle: string): string[] {
    return files.filter((f) => f.content.includes(needle)).map((f) => f.path);
  }

  function readSourceFiles(): SourceFile[] {
    const out: SourceFile[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          walk(full);
        } else if (entry.name.endsWith(".ts") || entry.name.endsWith(".tsx")) {
          out.push({
            path: path.relative(SRC_DIR, full).split(path.sep).join("/"),
            content: readFileSync(full, "utf-8"),
          });
        }
      }
    };
    walk(SRC_DIR);
    return out;
  }

  it("走査した *.ts・*.tsx が0件でない（テストを含む）", () => {
    const files = readSourceFiles();
    expect(files.length).toBeGreaterThan(0);
    expect(files.some((f) => f.path.endsWith(".test.ts") || f.path.endsWith(".test.tsx"))).toBe(true);
  });

  it("キー名の値の字面が現れるファイルは lib/work-month.ts のちょうど1つ", () => {
    expect(pathsContaining(readSourceFiles(), OPERATOR_OWNERSHIP_KEY)).toEqual(["lib/work-month.ts"]);
  });

  it("陽性対照: 値の字面を含む第2の組を足して与えると2つを返す（述語が空虚に偽でない）", () => {
    const extra: SourceFile = { path: "lib/extra.ts", content: `const k = "${OPERATOR_OWNERSHIP_KEY}";` };
    expect(pathsContaining([...readSourceFiles(), extra], OPERATOR_OWNERSHIP_KEY).sort()).toEqual([
      "lib/extra.ts",
      "lib/work-month.ts",
    ]);
  });

  it("陽性対照: 引用符を問わない（単引用符・バッククォート・引用符なしでも当たる）", () => {
    for (const content of [
      `const k = '${OPERATOR_OWNERSHIP_KEY}';`,
      `const k = \`${OPERATOR_OWNERSHIP_KEY}\`;`,
      `x.${OPERATOR_OWNERSHIP_KEY}`,
    ]) {
      expect(pathsContaining([{ path: "a.ts", content }], OPERATOR_OWNERSHIP_KEY)).toEqual(["a.ts"]);
    }
  });
});

describe("キー名が契約 AC-10-1 の表と一致すること（M-3。字面をテストに書かず、契約の表から読む）", () => {
  const CONTRACT_PATH = path.join(
    path.dirname(fileURLToPath(import.meta.url)),
    "..", "..", "..", "..", "docs", "specs", "domain-api-http-contract.md",
  );

  /** AC-10-1 の節（見出しから次の #### まで）の行を返す。 */
  function readAc101Section(markdown: string): string[] {
    const lines = markdown.split("\n");
    const start = lines.findIndex((l) => l.startsWith("#### AC-10-1."));
    if (start < 0) return [];
    const rest = lines.slice(start + 1);
    const end = rest.findIndex((l) => l.startsWith("####"));
    return end < 0 ? rest : rest.slice(0, end);
  }

  /** 1列目が `key`（バッククォート囲み）で、2列目が boolean で始まる行。 */
  function booleanRowsFor(sectionLines: readonly string[], key: string): string[] {
    return sectionLines.filter((l) => {
      const cells = l.split("|").map((c) => c.trim());
      return cells.length >= 3 && cells[1] === `\`${key}\`` && cells[2].startsWith("boolean");
    });
  }

  /** 1列目が `key` の行（2列目は問わない）。 */
  function rowsFor(sectionLines: readonly string[], key: string): string[] {
    return sectionLines.filter((l) => l.split("|").map((c) => c.trim())[1] === `\`${key}\``);
  }

  const section = () => readAc101Section(readFileSync(CONTRACT_PATH, "utf-8"));

  it("契約 AC-10-1 の表に、定数の値を1列目・boolean を2列目に持つ行がちょうど1つある", () => {
    expect(booleanRowsFor(section(), OPERATOR_OWNERSHIP_KEY)).toHaveLength(1);
  });

  it("陽性対照: 同じ表で generated の行がちょうど1つ見つかる（走査が空振りしていない）", () => {
    expect(rowsFor(section(), "generated")).toHaveLength(1);
  });

  it("陽性対照: 定数の値に接尾辞を足した値では0行（どの値でも当たる述語でない）", () => {
    expect(booleanRowsFor(section(), `${OPERATOR_OWNERSHIP_KEY}Suffix`)).toHaveLength(0);
  });
});

describe("formatSettlementRange（K-1・2-2-a。formatHours の丸めで下限〜上限。単位を含まない）", () => {
  it.each([
    [{ lowerBound: { hours: 140, minutes: 0 }, upperBound: { hours: 180, minutes: 0 } }, "140.0〜180.0"],
    [{ lowerBound: { hours: 140, minutes: 15 }, upperBound: { hours: 180, minutes: 30 } }, "140.25〜180.5"],
  ])("%j → %s", (range, expected) => {
    expect(formatSettlementRange(range)).toBe(expected);
  });

  it("単位「時間」を含まない（呼ぶ側が添える）", () => {
    const text = formatSettlementRange({
      lowerBound: { hours: 140, minutes: 0 },
      upperBound: { hours: 180, minutes: 0 },
    });
    expect(text.includes("時間")).toBe(false);
    // 陽性対照: 単位を含む字面を、この判定は検出する。
    expect("140.0〜180.0 時間".includes("時間")).toBe(true);
  });
});
