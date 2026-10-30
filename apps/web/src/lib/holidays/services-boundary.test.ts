import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// docs/specs/daily-record-entry-ui.md 10-2-a、設計 M-12・T-3（段2）。
// services/ 配下に祝日データを置かず、services/ のコードから apps/web の lib/holidays を参照しない。ファイル走査で読む。

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..", "..", "..");
const SERVICES_DIR = path.join(REPO_ROOT, "services");
const SKIP_DIRS = new Set(["node_modules", ".git"]);

/** 名前（パスの各要素）に holiday・syukujitsu を含むか。 */
function hasHolidayName(relativePath: string): boolean {
  return relativePath.split(/[\\/]/).some((segment) => /holiday|syukujitsu/i.test(segment));
}

/** 内容に apps/web の lib/holidays への参照の字面があるか。 */
function referencesWebHolidays(content: string): boolean {
  return /lib\/holidays/.test(content);
}

function listFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (SKIP_DIRS.has(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) files.push(...listFiles(full));
    else files.push(full);
  }
  return files;
}

describe("述語の判別力（陽性対照）", () => {
  it.each(["holidays/data.ts", "internal/holiday.go", "x/syukujitsu.csv", "Syukujitsu/a.txt"])(
    "名前に含む場合に当たる: %s",
    (p) => {
      expect(hasHolidayName(p)).toBe(true);
    },
  );
  it.each(["internal/domain/work_month.go", "cmd/api/main.go"])("含まない名前に当たらない: %s", (p) => {
    expect(hasHolidayName(p)).toBe(false);
  });
  it.each(['import "../../apps/web/src/lib/holidays"', "// see apps/web/src/lib/holidays/index.ts"])(
    "参照の字面を含む内容に当たる: %s",
    (c) => {
      expect(referencesWebHolidays(c)).toBe(true);
    },
  );
  it("参照でない内容に当たらない", () => {
    expect(referencesWebHolidays("package domain\n")).toBe(false);
  });
});

describe("services/ と祝日データの分離（10-2-a）", () => {
  const files = listFiles(SERVICES_DIR);

  it("走査したファイルは0件でない", () => {
    expect(files.length).toBeGreaterThan(0);
  });

  it("名前に holiday・syukujitsu を含むファイル・ディレクトリが無い", () => {
    const hits = files.map((f) => path.relative(SERVICES_DIR, f)).filter(hasHolidayName);
    expect(hits).toEqual([]);
  });

  it("ファイルの字面に apps/web の lib/holidays への参照が無い", () => {
    const hits = files
      .filter((f) => referencesWebHolidays(readFileSync(f, "utf-8")))
      .map((f) => path.relative(SERVICES_DIR, f));
    expect(hits).toEqual([]);
  });
});
