import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { readAc5Section } from "../test-support/design-system";

// docs/specs/design-system.md AC-5-1 / AC-5-2（検証手段は AC-10-4）。
//
// 「apps/web/src/components/ 直下の *.tsx（*.test.tsx を除く）の集合が表と
// 過不足なく一致することを検査する（表に無い *.tsx の存在も違反。テスト
// ファイルと .gitkeep 等の非 *.tsx は対象外）。あわせて index.ts / index.tsx
// （バレル）が存在しないことを検査する」を満たす。

const componentsDir = fileURLToPath(new URL(".", import.meta.url));

// AC-5-1 の表（ファイル名列）は条文の字面から読む。テスト側に書いた写しを
// 期待値にすると、台帳を書き換えても写しと実ディレクトリが一致したまま緑になる
// （AC-10-4。10-2-a / 10-3-i と同じ理由）。0件のとき、および形の合わない
// `5-1-x` 行を読み飛ばしたときは失敗とする。
const specPath = path.join(componentsDir, "..", "..", "..", "..", "docs", "specs", "design-system.md");

function parseLedgerFiles(spec: string): string[] {
  const hint =
    `仕様書 ${specPath} の AC-5-1 の台帳を読めなかった。条文の体裁を変えたのなら、` +
    `要求の変更ではないのでこの抽出側を追随させること（10-4 は抽出手段を仕様で固定していない）。`;
  // 台帳は AC-5 の節の中だけから読む。仕様書の全体から拾うと、他の節に同じ形の
  // 行を置いたときに台帳でない行を期待値に混ぜる（10-4）。
  const section = readAc5Section(spec, hint);
  const rowLines = section.match(/^\|\s*5-1-[a-z]+\s*\|.*$/gm) ?? [];
  const files = rowLines.flatMap((line) => {
    const match = line.match(/^\|\s*5-1-[a-z]+\s*\|\s*`[A-Za-z]+`\s*\|\s*`([a-z-]+\.tsx)`\s*\|/);
    return match ? [match[1]] : [];
  });
  if (files.length === 0) throw new Error(`${hint} 台帳の行を1件も読めなかった。`);
  if (files.length !== rowLines.length) {
    throw new Error(`${hint} 台帳に形の合わない行がある（${rowLines.length} 行中 ${files.length} 行だけ読めた）。`);
  }
  return files.sort();
}

const EXPECTED_FILES = parseLedgerFiles(readFileSync(specPath, "utf8"));

function listEntries(): string[] {
  return readdirSync(componentsDir, { withFileTypes: true })
    .filter((e) => e.isFile())
    .map((e) => e.name);
}

describe("components ディレクトリ構成 - AC-5-1 / AC-5-2", () => {
  it("10-4: 期待値を AC-5-1 の台帳の字面から読める（0件なら失敗）", () => {
    expect(EXPECTED_FILES.length).toBeGreaterThan(0);
    expect(new Set(EXPECTED_FILES).size).toBe(EXPECTED_FILES.length);
  });

  // 形の合わない行を読み飛ばさない検査は、現行の台帳では一度も働かないため、
  // 実ケースと同じ関数へ合成した台帳を与えて両側を読む。
  it("10-4: 台帳の読み取りは形の合う行を読み、崩れた行を含むと投げる", () => {
    const head = "### AC-5. 共通コンポーネントの一覧と境界\n\n";
    const good = `${head}| 5-1-a | \`Button\` | \`button.tsx\` | 各画面 | 出典 |\n`;
    expect(parseLedgerFiles(good)).toEqual(["button.tsx"]);
    // 節の外に置いた同じ形の行は、節の前・後ろのどちらでも台帳として読まない。
    const stray = "| 5-1-z | `Panel` | `panel.tsx` | 各画面 | 出典 |\n";
    expect(parseLedgerFiles(`${good}\n### AC-6. 別の節\n\n${stray}`)).toEqual(["button.tsx"]);
    // 節の前の行は、後ろに別の節が続く形で読む（後ろに節が無いと、節の終わりを探す
    // 分岐を通らず、読み始めを文書の先頭へ広げる変更が素通りする）。
    expect(parseLedgerFiles(`### AC-4. 前の節\n\n${stray}\n${good}\n### AC-6. 後の節\n`)).toEqual([
      "button.tsx",
    ]);
    expect(() => parseLedgerFiles("| 5-1-a | `Button` | `button.tsx` | 各画面 | 出典 |\n")).toThrow(
      /AC-5 の節が見つからない/,
    );
    expect(() => parseLedgerFiles(`${good}| 5-1-b | Card | card.tsx | 各画面 | 出典 |\n`)).toThrow(
      /形の合わない行/,
    );
  });

  it("5-1/5-2: *.tsx（*.test.tsx を除く）の集合が表と過不足なく一致する", () => {
    const tsxFiles = listEntries()
      .filter((name) => name.endsWith(".tsx") && !name.endsWith(".test.tsx"))
      .sort();
    expect(tsxFiles).toEqual(EXPECTED_FILES);
  });

  it("5-2: index.ts / index.tsx（バレル）が存在しない", () => {
    const entries = listEntries();
    expect(entries.includes("index.ts")).toBe(false);
    expect(entries.includes("index.tsx")).toBe(false);
  });
});
