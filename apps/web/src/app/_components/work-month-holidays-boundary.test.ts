import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// docs/specs/web-screens-implementation-design.md M-1・M-12・T-3（段の境界）。
// lib/work-month-calendar.ts・WorkMonthView・workMonthScreen と、段1 の表のテストは lib/holidays/ を輸入しない。
// 輸入するのは work-month-holiday-view.tsx だけ（M-12 の段で置く）。ファイル走査で字面を読む。

const SRC_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const SELF = "app/_components/work-month-holidays-boundary.test.ts";

/** 内容に holidays の輸入（from "…holidays…" の形）があるか。 */
function importsHolidays(content: string): boolean {
  return /from\s+["'][^"']*holidays[^"']*["']/.test(content);
}

const IMPLEMENTATIONS = [
  "lib/work-month-calendar.ts",
  "app/_components/work-month-view.tsx",
  "app/_components/work-month-screen.tsx",
];

// 段1 の表（T-3）のテストファイル。本テスト自身を除く。
const STAGE1_TESTS = [
  "lib/work-month-actions.test.ts",
  "lib/work-month.test.ts",
  "lib/work-month-client.test.ts",
  "lib/work-month-feedback.test.ts",
  "lib/work-month-calendar.test.ts",
  "lib/jst-month.test.ts",
  "lib/time-entry.test.ts",
  "lib/side-nav-items.test.ts",
  "app/_components/app-side-nav.test.tsx",
  "app/_components/work-month-screen.test.tsx",
  "app/_components/work-month-summary.test.tsx",
  "app/_components/work-month-calendar.test.tsx",
  "app/_components/daily-record-form.test.tsx",
  "app/_components/work-month-operations.test.tsx",
  "app/_components/work-month-view.test.tsx",
];

const read = (relative: string) => readFileSync(path.join(SRC_DIR, relative), "utf-8");

describe("述語の判別力（陽性対照）", () => {
  it.each([
    'import { isHoliday } from "@/lib/holidays";',
    "import { isHoliday } from '../../lib/holidays/index';",
    'import { HOLIDAY_ROWS } from "@/lib/holidays/data";',
  ])("輸入の字面を含む内容に当たる: %s", (content) => {
    expect(importsHolidays(content)).toBe(true);
  });

  it.each([
    "// holidays は work-month-holiday-view だけが輸入する",
    'const label = "holidays";',
    'import { calendarCells } from "@/lib/work-month-calendar";',
  ])("輸入でない内容には当たらない: %s", (content) => {
    expect(importsHolidays(content)).toBe(false);
  });
});

describe("走査の母集団", () => {
  it("走査するファイルは0件でない（実装3件と段1 のテスト15件。本テスト自身を含まない）", () => {
    const targets = [...IMPLEMENTATIONS, ...STAGE1_TESTS];
    expect(targets).toHaveLength(18);
    expect(targets).not.toContain(SELF);
  });
});

describe("段の境界（M-1・M-12）", () => {
  it.each(IMPLEMENTATIONS)("%s は holidays を輸入しない（ファイルが存在する）", (relative) => {
    const content = read(relative);
    expect(content.length, `${relative} が空`).toBeGreaterThan(0);
    expect(importsHolidays(content)).toBe(false);
  });

  it.each(STAGE1_TESTS)("段1 のテスト %s は holidays を輸入しない", (relative) => {
    const content = read(relative);
    expect(content.length, `${relative} が空`).toBeGreaterThan(0);
    expect(importsHolidays(content)).toBe(false);
  });
});

// docs/specs/web-screens-implementation-design.md M-1・M-12・T-3（段2。lib/holidays/ を輸入するファイルの集合）。

interface SourceFile {
  path: string;
  content: string;
}

/** 内容から、輸入の指定子（from "…"・副作用の import "…"・動的な import("…")）を取り出す。 */
function importSpecifiers(content: string): string[] {
  const patterns = [
    /\bfrom\s*["']([^"']+)["']/g,
    /^\s*import\s*["']([^"']+)["']/gm,
    /\bimport\s*\(\s*["']([^"']+)["']\s*\)/g,
  ];
  return patterns.flatMap((re) => [...content.matchAll(re)].map((m) => m[1]));
}

/** 指定子が、/ で分けた要素に holidays をちょうど持つか。 */
function isHolidaysSpecifier(specifier: string): boolean {
  return specifier.split("/").includes("holidays");
}

/** パスと内容の組の列を受けて、lib/holidays/ を輸入するパスの集合（昇順）を返す。 */
function holidaysImporters(files: readonly SourceFile[]): string[] {
  return files
    .filter((f) => importSpecifiers(f.content).some(isHolidaysSpecifier))
    .map((f) => f.path)
    .sort();
}

const ALLOWED_IMPORTERS = [
  "app/_components/work-month-holiday-view.test.tsx",
  "app/_components/work-month-holiday-view.tsx",
];

function walk(dir: string, base = ""): string[] {
  return readdirSync(path.join(dir, base), { withFileTypes: true }).flatMap((e) => {
    const rel = base === "" ? e.name : `${base}/${e.name}`;
    if (e.isDirectory()) return walk(dir, rel);
    return /\.tsx?$/.test(e.name) ? [rel] : [];
  });
}

/** src 配下の *.ts・*.tsx から、lib/holidays/ の中と本テスト自身を除いた列。 */
function scanSources(): SourceFile[] {
  return walk(SRC_DIR)
    .filter((p) => !p.startsWith("lib/holidays/") && p !== SELF)
    .map((p) => ({ path: p, content: read(p) }));
}

describe("輸入の指定子の読み（述語の判別力）", () => {
  it.each([
    ['from（import）', 'import { isHoliday } from "@/lib/holidays";'],
    ["from（export）", "export { isHoliday } from '../../lib/holidays/index';"],
    ["副作用の import", 'import "@/lib/holidays/data";'],
    ["動的な import", 'const m = await import("./holidays");'],
  ])("%s の輸入を持つ組は、許す集合と一致しなくなる", (_name, content) => {
    const bad: SourceFile = { path: "app/_components/x.tsx", content };
    expect(holidaysImporters([bad])).toEqual(["app/_components/x.tsx"]);
    const real = scanSources();
    expect(holidaysImporters([...real, bad])).not.toEqual(ALLOWED_IMPORTERS);
    expect(holidaysImporters([...real, bad])).toContain("app/_components/x.tsx");
  });

  it.each([
    'const label = "holidays";',
    "// holidays の注記",
    'import { x } from "./work-month-holidays-boundary";',
    'import { y } from "@/lib/holidays-table";',
  ])("輸入でない字面・要素が holidays でない指定子には当たらない: %s", (content) => {
    expect(holidaysImporters([{ path: "app/_components/x.tsx", content }])).toEqual([]);
  });
});

describe("lib/holidays/ を輸入するファイルの集合", () => {
  it("走査が0件でなく、許す集合の2つが走査に含まれる", () => {
    const paths = scanSources().map((f) => f.path);
    expect(paths.length).toBeGreaterThan(0);
    for (const allowed of ALLOWED_IMPORTERS) expect(paths, `${allowed} が走査から漏れている`).toContain(allowed);
  });

  it("許す集合とちょうど一致する", () => {
    expect(holidaysImporters(scanSources())).toEqual(ALLOWED_IMPORTERS);
  });
});
