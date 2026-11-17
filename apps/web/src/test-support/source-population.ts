// 検査の母集団（src 配下のファイルの集合）を、走査と、それとは別の手段での数え直しの2通りで読む補助。
// 10-1-b / 10-3-k / AC-1-1 の検査が共有する（置き場所は 10-3-k）。10-1-b の母集団の定義と fixture は
// 10-1-b と 10-1-c (ii) が同じものを読むため、ここに1つだけ置く（#170）。
//
// 2通りの突き合わせ（過不足なく一致すること）は、片方だけを狭める変異を落とす。両方を同じように狭める変異
// （拡張子・階層を落とす形）は一致したまま通るため、各検査は同じ定義を fixture にも当て、期待値を fixture 側に
// 持たせて落とす（#170）。fixture の深さが実データより深いことも各検査が読む（deepestDirectoryDepth）。

import { mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import ts from "typescript";

export interface PopulationDefinition {
  /** 母集団に含める拡張子（例: [".ts", ".tsx"]）。 */
  extensions: readonly string[];
  /** 母集団から除くファイルか（root からの相対パス）。走査と数え直しの両方に掛ける。 */
  excludesFile: (relative: string) => boolean;
  /** 数え直しで降りないディレクトリか（root からの相対パスと名前）。各検査の既存の扱いをそのまま渡す。 */
  skipsDirectory: (relative: string, name: string) => boolean;
}

export interface Population {
  /** 走査（ts.sys.readDirectory）。root からの相対パスで、整列済み。 */
  scan(root: string): string[];
  /** 走査とは別の手段（再帰の readdirSync）での数え直し。root からの相対パスで、整列済み。 */
  walk(root: string): string[];
}

export function definePopulation(definition: PopulationDefinition): Population {
  const { extensions, excludesFile, skipsDirectory } = definition;
  return {
    scan: (root) =>
      ts.sys
        .readDirectory(root, [...extensions])
        .map((file) => path.relative(root, file))
        .filter((file) => !excludesFile(file))
        .sort(),
    walk: (root) => {
      const walked: string[] = [];
      const walk = (dir: string): void => {
        for (const entry of readdirSync(dir, { withFileTypes: true })) {
          const full = path.join(dir, entry.name);
          const relative = path.relative(root, full);
          if (entry.isDirectory()) {
            if (!skipsDirectory(relative, entry.name)) walk(full);
          } else if (extensions.some((e) => entry.name.endsWith(e)) && !excludesFile(relative)) {
            walked.push(relative);
          }
        }
      };
      walk(root);
      return walked.sort();
    },
  };
}

/** fixture の深い階層（9段）。src の実データより深く置き、両側の階層を落とす変異を fixture で落とす。 */
export const DEEP_FIXTURE_DIR = path.join("a", "b", "c", "d", "e", "f", "g", "h", "i");

/** 相対パスの並びのうち、最も深いファイルが何段のディレクトリの中にあるか（直下は 0）。 */
export function deepestDirectoryDepth(relativePaths: readonly string[]): number {
  return Math.max(0, ...relativePaths.map((p) => p.split(path.sep).length - 1));
}

/** files（相対パス → 中身）を一時ディレクトリへ書き、body に渡す。終われば消す。 */
export function withFixture(files: Record<string, string>, body: (root: string) => void): void {
  const root = mkdtempSync(path.join(os.tmpdir(), "et-population-"));
  try {
    for (const [relative, content] of Object.entries(files)) {
      const full = path.join(root, relative);
      mkdirSync(path.dirname(full), { recursive: true });
      writeFileSync(full, content);
    }
    body(root);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

/** 10-1-b の母集団（src 配下の実装 *.ts / *.tsx。検査ファイルを除く）。10-1-b と 10-1-c (ii) が共有する。 */
export const IMPLEMENTATION_POPULATION_10_1_B = definePopulation({
  extensions: [".ts", ".tsx"],
  excludesFile: (file) => /\.test\.tsx?$/.test(file),
  skipsDirectory: (_relative, name) => name === "node_modules",
});

/** 10-1-b の母集団の定義を当てる fixture（相対パス → 中身）。実装 .ts / .tsx を深い階層まで置き、検査ファイルと他の拡張子を混ぜる。 */
export const IMPLEMENTATION_POPULATION_FIXTURE: Record<string, string> = {
  [path.join("app", "layout.tsx")]: "export default function L() { return null; }",
  "top.ts": "export const a = 1;",
  [path.join(DEEP_FIXTURE_DIR, "deep.ts")]: "export const b = 1;",
  [path.join(DEEP_FIXTURE_DIR, "deep.tsx")]: "export const c = 1;",
  [path.join("app", "page.test.tsx")]: "export {};",
  [path.join(DEEP_FIXTURE_DIR, "x.test.ts")]: "export {};",
  "styles.css": "",
  "script.js": "",
  "types.d.mts": "",
};

/** IMPLEMENTATION_POPULATION_FIXTURE から条文どおりに得られる母集団（整列済み）。 */
export const IMPLEMENTATION_POPULATION_EXPECTED: readonly string[] = [
  path.join("app", "layout.tsx"),
  path.join(DEEP_FIXTURE_DIR, "deep.ts"),
  path.join(DEEP_FIXTURE_DIR, "deep.tsx"),
  "top.ts",
].sort();
