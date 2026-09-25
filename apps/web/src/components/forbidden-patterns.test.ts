import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { describe, expect, it } from "vitest";

// docs/specs/design-system.md AC-2-4 / AC-4-4 / AC-4-6 / AC-5-5（検証手段は AC-10-3）。
//
// 「apps/web/src/components/ 配下の実装ファイル（*.test.tsx を除く）を読み、
// 禁じた表現（生の色・rounded-md 以外の角丸・代替なしの outline-none・fetch）
// の不在を検査する」を満たす。対象は AC-5-1 の表が挙げるファイルであり、
// ファイルが無い現時点では「存在しない」ことそのものが Red になる
// （コンポーネント未実装のため）。

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
];

// シェードを持つ色（bg-red-500 のように末尾に数値シェードを伴う）。
const TAILWIND_SHADED_PALETTE_COLORS = [
  "red",
  "orange",
  "amber",
  "yellow",
  "lime",
  "green",
  "emerald",
  "teal",
  "cyan",
  "sky",
  "blue",
  "indigo",
  "violet",
  "purple",
  "fuchsia",
  "pink",
  "rose",
  "slate",
  "gray",
  "zinc",
  "neutral",
  "stone",
];

// シェードを持たない色（bg-white / text-black のように数値シェードを伴わない）。
const TAILWIND_SHADELESS_PALETTE_COLORS = ["black", "white"];

// AC-10-3-c: 検査対象とする接頭辞の17種。条文が字面に逐語で列挙したものを、
// 順序も含めてそのまま写す（条文を書き換えずに増減させない）。
const TAILWIND_COLOR_UTILITY_PREFIXES = [
  "bg",
  "text",
  "border",
  "ring",
  "ring-offset",
  "outline",
  "accent",
  "caret",
  "decoration",
  "divide",
  "fill",
  "stroke",
  "from",
  "via",
  "to",
  "shadow",
  "placeholder",
];

// 正規表現の交替（|）は最左位置で最初に一致した候補を採るが、その後ろが
// パターン全体（-色名 部分）に一致しなければバックトラックして次の候補を
// 試す。したがって ring ⊂ ring-offset のように一方が他方の接頭辞になって
// いても、条文の字面の順序（bg / text / … / ring / ring-offset / … ）の
// ままで交替を組んでよい —— ring-offset-red-500 は ring を先に試みても
// 後ろの -red-500 に一致しないため ring-offset へ後退して一致する
// （17種すべてで判別力が残ることは、下記の
// 「AC-10-3-c: 接頭辞ごとに paletteUtilityPattern が検出できる」で
// 接頭辞ごとに機械検査する）。
const paletteUtilityPrefixAlternation = TAILWIND_COLOR_UTILITY_PREFIXES.join("|");

const paletteUtilityPattern = new RegExp(
  `\\b(?:${paletteUtilityPrefixAlternation})-(?:(?:${TAILWIND_SHADED_PALETTE_COLORS.join("|")})-\\d{2,3}|(?:${TAILWIND_SHADELESS_PALETTE_COLORS.join("|")}))\\b`,
);
const hexColorPattern = /#[0-9a-fA-F]{3,8}\b/;
const rgbOrHslFunctionPattern = /\b(?:rgb|rgba|hsl|hsla)\(/;
// AC-4-4: 角丸ユーティリティは rounded-md のみ。rounded-md 以外の rounded* を拾う。
const nonMdRoundedPattern = /\brounded(?!-md\b)(?:-[\w-]+)?\b/;
const fetchCallPattern = /\bfetch\s*\(/;

// AC-10-3-c: 「この17種のリストは、本条文の字面とテストの実装が逐語で
// 対応すること」を支える判別力の検査。定数 TAILWIND_COLOR_UTILITY_PREFIXES
// と条文（10-3-d が突き合わせる）の側は無傷のまま、交替の組み立て
//（paletteUtilityPrefixAlternation / paletteUtilityPattern）だけを弱める
// 変更（例: 特定の接頭辞だけを交替から外す）があれば、対応する接頭辞の
// ケースだけが個別に Red になる。17種を1つの it に潰すと、どの接頭辞が
// 落ちたかが読めなくなるため、it.each で1接頭辞ずつ検査する。
describe("AC-10-3-c: 接頭辞ごとに paletteUtilityPattern が検出できる", () => {
  it.each(TAILWIND_COLOR_UTILITY_PREFIXES)(
    "%s-red-500（シェードあり）を検出できる",
    (prefix) => {
      expect(paletteUtilityPattern.test(`${prefix}-red-500`)).toBe(true);
    },
  );

  it.each(TAILWIND_COLOR_UTILITY_PREFIXES)(
    "%s-white（シェードなし）を検出できる",
    (prefix) => {
      expect(paletteUtilityPattern.test(`${prefix}-white`)).toBe(true);
    },
  );
});

describe("components 実装 - 禁止表現の不在（AC-2-4 / AC-4-4 / AC-4-6 / AC-5-5）", () => {
  it.each(EXPECTED_FILES)("%s が存在し、禁止表現を含まない", (fileName) => {
    const filePath = path.join(componentsDir, fileName);
    expect(existsSync(filePath)).toBe(true);

    const content = readFileSync(filePath, "utf8");

    expect(hexColorPattern.test(content)).toBe(false);
    expect(rgbOrHslFunctionPattern.test(content)).toBe(false);
    expect(paletteUtilityPattern.test(content)).toBe(false);
    expect(nonMdRoundedPattern.test(content)).toBe(false);
    expect(fetchCallPattern.test(content)).toBe(false);

    // AC-4-6: outline-none を書くなら、同じファイル内に代替のリング指定
    // （focus-visible:ring 系）を伴うこと。
    if (content.includes("outline-none")) {
      expect(content).toMatch(/focus-visible:[^\s"'`]*ring/);
    }
  });
});

// docs/specs/design-system.md AC-10-3-d。
//
// 10-3-c 末尾「この17種のリストは、本条文の字面とテストの実装が逐語で
// 対応すること」の検査。抽出の元は条文の字面であり、テスト側の定数の
// 写しではない（写しどうしを比べると自明に一致してしまうため）。
// 「順序を含めて突き合わせること」「どちらかが0件なら失敗とすること」の
// 2点は 10-3-d が要求として固定している。
//
// 抽出の手段は 10-3-d が仕様で固定していない。したがって条文側の体裁
//（区切り記号・強調の閉じ位置・セル内の余白・括弧書きの挿入）は契約では
// なく、体裁を変えただけで落ちるのは偽 Red である。アンカーは体裁ではなく
// 内容語（表の ID と「17種」）へ寄せ、それでも抽出に失敗したときは
// 「条文の体裁を変えたなら抽出側を追随させる」ことが読める失敗メッセージ
// を出す（原因が診断に出ない偽 Red を残さない）。
const SPEC_DRIFT_HINT =
  "10-3-c の字面から接頭辞の列を抽出できなかった。条文の体裁（区切り・強調・余白）を変えたのなら、" +
  "接頭辞の増減ではないので条文ではなくこの抽出側を追随させること（10-3-d は抽出手段を仕様で固定していない）。";

describe("AC-10-3-d: 10-3-c の接頭辞リスト（条文とテストの逐語対応）", () => {
  it("design-system.md の 10-3-c から抽出した接頭辞の列は、テストが読む接頭辞の列と順序を含めて一致する", () => {
    const specPath = path.join(componentsDir, "..", "..", "..", "..", "docs", "specs", "design-system.md");
    expect(existsSync(specPath), `仕様書が見つからない: ${specPath}`).toBe(true);

    const specContent = readFileSync(specPath, "utf8");
    // 行頭一致は表のセル区切りと ID だけを読む（ID 前後の余白を許す）。
    const targetLines = specContent
      .split("\n")
      .filter((line) => /^\|\s*10-3-c\s*\|/.test(line));

    // 10-3-c の行がちょうど1本に絞れなければ、以降の抽出の前提が崩れて
    // いるため、ここで失敗させる。
    expect(
      targetLines,
      `${SPEC_DRIFT_HINT} 表の行 \`| 10-3-c |\` がちょうど1本に絞れなかった（${targetLines.length}本）。`,
    ).toHaveLength(1);
    const targetLine = targetLines[0];

    // 「17種」という語から、続く最初の句点までを列挙部として切り出す。
    // 件数は 10-3-c の要求そのものなので体裁ではなく内容であり、
    // 区切り記号（——・コロン）や強調の閉じ位置には依存しない。
    const enumerationMatch = targetLine.match(/17種([^。]*)。/);
    expect(
      enumerationMatch,
      `${SPEC_DRIFT_HINT} 「17種」から次の句点までを切り出せなかった。`,
    ).not.toBeNull();

    const extractedPrefixes = [...enumerationMatch![1].matchAll(/`([a-z-]+)`/g)].map(
      (match) => match[1],
    );

    // 0件ヒットどうしの一致は一致ではない（10-3-d が明示する既知の偽 Green）。
    expect(
      extractedPrefixes.length,
      `${SPEC_DRIFT_HINT} 切り出した列挙部にバッククォート付きの接頭辞が1つも無かった。`,
    ).toBeGreaterThan(0);
    expect(
      TAILWIND_COLOR_UTILITY_PREFIXES.length,
      "テスト側の接頭辞の定数が空である。条文が要求する17種を読めていない。",
    ).toBeGreaterThan(0);

    expect(
      extractedPrefixes,
      "条文の字面から抽出した接頭辞の列と、テストが読む定数が、順序を含めて一致しない。" +
        "接頭辞を増減させるなら 10-3-c の字面とこの定数の両方を同時に変えること。",
    ).toEqual(TAILWIND_COLOR_UTILITY_PREFIXES);
  });
});
