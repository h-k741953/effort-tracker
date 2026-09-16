import { readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// docs/specs/design-system.md AC-5-1 / AC-5-2（検証手段は AC-10-4）。
//
// 「apps/web/src/components/ 直下の *.tsx（*.test.tsx を除く）の集合が表と
// 過不足なく一致することを検査する（表に無い *.tsx の存在も違反。テスト
// ファイルと .gitkeep 等の非 *.tsx は対象外）。あわせて index.ts / index.tsx
// （バレル）が存在しないことを検査する」を満たす。

const componentsDir = fileURLToPath(new URL(".", import.meta.url));

// AC-5-1 の表（ファイル名列）。
const EXPECTED_FILES = [
  "app-header.tsx",
  "role-switcher.tsx",
  "status-badge.tsx",
  "button.tsx",
  "card.tsx",
  "number-field.tsx",
  "field-error.tsx",
  "error-banner.tsx",
  "empty-state.tsx",
  "confirm-dialog.tsx",
  "pagination.tsx",
].sort();

function listEntries(): string[] {
  return readdirSync(componentsDir, { withFileTypes: true })
    .filter((e) => e.isFile())
    .map((e) => e.name);
}

describe("components ディレクトリ構成 - AC-5-1 / AC-5-2", () => {
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
