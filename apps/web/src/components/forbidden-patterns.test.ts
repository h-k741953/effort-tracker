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

// AC-10-3-f: 不在を主張する5本を1つの表に集め、実ファイルの走査と陽性対照が
// 同じ経路（findForbiddenExpressions）を通るようにする。経路を分けると、
// 対照だけが通る形へパターンを差し替えられてしまい対照の意味が無くなる。
// 陽性対照の担い手に用いる形（AC-10-3-f）。表と対照の双方がこの定数を
// 参照することで、ラベルの文字列がずれない。
const HEX_COLOR_LABEL = "16進の色";

const FORBIDDEN_EXPRESSION_PATTERNS = [
  { label: HEX_COLOR_LABEL, pattern: hexColorPattern },
  { label: "rgb() / hsl() の関数記法", pattern: rgbOrHslFunctionPattern },
  { label: "パレットユーティリティ", pattern: paletteUtilityPattern },
  { label: "rounded-md 以外の角丸", pattern: nonMdRoundedPattern },
  { label: "fetch の呼び出し", pattern: fetchCallPattern },
];

/** テキストに現れた禁止表現のラベルを、表の順序で返す。 */
function findForbiddenExpressions(text: string): string[] {
  return FORBIDDEN_EXPRESSION_PATTERNS.filter(({ pattern }) => pattern.test(text)).map(
    ({ label }) => label,
  );
}

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

    // AC-10-3-f: 読んだテキストが0文字なら失敗とする。不在の主張は走査対象が
    // 空になると空虚に真になるため（10-3 の「1件も読めなかったときは失敗と
    // する」と同型。あちらが数えるのはファイルの件数、ここは文字数である）。
    expect(
      content.length,
      `${fileName} から読み取ったテキストが0文字である。不在の検査が空振りしたまま` +
        `緑になるため失敗とする（AC-10-3-f）。`,
    ).toBeGreaterThan(0);

    // AC-10-3-f: 陽性対照の担い手を「実ファイルから読んだテキストそのもの」に
    // する。読んだテキストの末尾へ16進の色を1つだけ足したものを、不在の主張と
    // 同じ1回の走査へ通し、16進の色が検出されることを読む。こうすると、走査
    // 対象を切り落とす変異（引数を丸ごと切る形・先頭の一定文字数だけを残す形）
    // が対照を消すため落ちる。対照を検査自身のテキストだけで持つと、実ファイル
    // 側の引数を切る変異が対照に触れないまま生き残る（限界は AC-11-29）。
    const controlled = `${content}\n// const __positiveControl = "#ff0000";\n`;
    const labels = findForbiddenExpressions(controlled);
    expect(
      labels,
      `${fileName} のテキストを担い手にした陽性対照（${HEX_COLOR_LABEL}）が検出されない。` +
        `走査対象が切り落とされており、不在の主張が空虚に真になっている（AC-10-3-f）。`,
    ).toContain(HEX_COLOR_LABEL);
    expect(
      labels.filter((label) => label !== HEX_COLOR_LABEL),
      `${fileName} が禁じた表現を含む（AC-2-4 / AC-4-4 / AC-5-5。検出された種類を配列で示す）。`,
    ).toEqual([]);

    // 16進の色そのものの不在は、上の対照が必ず検出させてしまうため、実ファイル
    // のテキストで別に読む（この1本だけは引数を切る変異が残る。AC-11-29）。
    expect(
      hexColorPattern.test(content),
      `${fileName} が${HEX_COLOR_LABEL}を含む（AC-2-4）。`,
    ).toBe(false);

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

/**
 * design-system.md の 10-3-c の行（表のセル1本）を返す。
 * 行頭一致は表のセル区切りと ID だけを読む（ID 前後の余白を許す）。
 * ちょうど1本に絞れなければ、以降の抽出の前提が崩れているため失敗させる。
 */
function readClause10_3_cLine(hint: string): string {
  const specPath = path.join(componentsDir, "..", "..", "..", "..", "docs", "specs", "design-system.md");
  expect(existsSync(specPath), `仕様書が見つからない: ${specPath}`).toBe(true);

  const specContent = readFileSync(specPath, "utf8");
  const targetLines = specContent
    .split("\n")
    .filter((line) => /^\|\s*10-3-c\s*\|/.test(line));

  expect(
    targetLines,
    `${hint} 表の行 \`| 10-3-c |\` がちょうど1本に絞れなかった（${targetLines.length}本）。`,
  ).toHaveLength(1);
  return targetLines[0];
}

describe("AC-10-3-d: 10-3-c の接頭辞リスト（条文とテストの逐語対応）", () => {
  it("design-system.md の 10-3-c から抽出した接頭辞の列は、テストが読む接頭辞の列と順序を含めて一致する", () => {
    const targetLine = readClause10_3_cLine(SPEC_DRIFT_HINT);

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

// docs/specs/design-system.md AC-10-3-e。
//
// 10-3-c を実装したパターン（paletteUtilityPattern）の判別力を、一致する側と
// 一致しない側の両方で読む。**読む相手はパターンそのものであり、実装ファイルの
// 内容ではない** —— 実装が偶然そのトークンを含むかどうかで判別力が変わる状態を
// 残さない（10-3-d と 10-3-c の逐語対応を無傷のまま、交替の組み立てだけを
// より広い形・より狭い形へ差し替える変更を落とすのが目的である）。
//
// 一致しない側に並べるのは、11-24（方向・軸つき）と 11-25（任意値記法）が
// 「落ちない」と宣言している形である。したがってこれは要求の追加ではなく、
// 宣言済みの穴を機械で固定するものである。穴が塞がれる（＝検出されるように
// なる）ときは、11-24 / 11-25 の字面の変更を伴うため、ここも同時に変わる。

// 色名・シェードの軸だけを見るため、接頭辞は1つに固定する（接頭辞の側は
// 10-3-c / 10-3-d と上記 AC-10-3-c の34ケースが持つ）。
const PALETTE_SAMPLE_PREFIX = "bg";

const NON_MATCHING_SAMPLES = [
  // AC-2 のトークンを参照する形（AC-1-3 のマッピング）。適合しているので落ちない。
  "bg-primary",
  "border-border",
  "ring-focus-ring",
  "bg-surface",
  "text-surface-foreground",
  // AC-2 のトークン名に数値シェードを足した形。色名の側を「語 + `-` + 2〜3桁の
  // 数値」という *形* だけで判定する（色名の交替を [a-z]+ 等へ広げる）変異は、
  // 上の5本では判別できない —— いずれもこの形を持たないためである。
  "bg-primary-500",
  // 11-24: 接頭辞と色名の間に方向・軸のセグメントが挟まる形。
  "border-t-red-500",
  "divide-x-red-500",
  // 11-25: 16進でも rgb() / hsl() でもない任意値記法。
  "bg-[oklch(70%_0.1_20)]",
  // シェードの桁の境界（10-3-c が読むのは2桁・3桁である）。
  "bg-red-5",
  "bg-red-5000",
];

describe("AC-10-3-e: paletteUtilityPattern の判別力（色名・シェードと、一致すべきでない形）", () => {
  it.each(TAILWIND_SHADED_PALETTE_COLORS)(
    "bg-%s-500（シェードを持つ色）を検出できる",
    (color) => {
      expect(paletteUtilityPattern.test(`${PALETTE_SAMPLE_PREFIX}-${color}-500`)).toBe(true);
    },
  );

  it.each(TAILWIND_SHADELESS_PALETTE_COLORS)(
    "bg-%s（シェードを持たない色）を検出できる",
    (color) => {
      expect(paletteUtilityPattern.test(`${PALETTE_SAMPLE_PREFIX}-${color}`)).toBe(true);
    },
  );

  it.each(["bg-red-50", "bg-red-500"])("%s（シェード2桁・3桁）を検出できる", (sample) => {
    expect(
      paletteUtilityPattern.test(sample),
      `${sample} を検出できない。10-3-c が読むシェードは2桁・3桁の両方である。`,
    ).toBe(true);
  });

  it.each(NON_MATCHING_SAMPLES)("%s は検出しない", (sample) => {
    expect(
      paletteUtilityPattern.test(sample),
      `${sample} を誤検出した。AC-2 のトークン参照は AC-2-4 に適合しており、` +
        `方向・軸つき（11-24）と任意値記法（11-25）は 10-3-c が読む形の外にある。` +
        `検出するよう強めるなら 10-3-c の字面（読む形の定義）と 11-24 / 11-25 を同時に変えること。`,
    ).toBe(false);
  });

  it("シェードを持つ色の件数は 10-3-c の字面が述べる件数と一致し、シェードを持たない色は字面の語と一致する", () => {
    const hint =
      "10-3-c の字面から色名の側の記述を抽出できなかった。条文の体裁（括弧・区切り・強調）を変えたのなら、" +
      "色の増減ではないので条文ではなくこの抽出側を追随させること（10-3-e は抽出手段を仕様で固定していない）。";
    const targetLine = readClause10_3_cLine(hint);

    // 「色名の側（…）」の括弧の中だけを読む。10-3-c の行には17種の接頭辞も
    // バッククォート付きで並ぶため、行全体を相手にすると混ざる。
    // 括弧の種類（全角・半角）と前後の余白は体裁であり契約ではないため、
    // どちらも許す（W5-3 と同じ理由。偽 Red を作らない）。
    const colorNoteMatch = targetLine.match(/色名の側\s*[（(]\s*([^）)]*)[）)]/);
    expect(colorNoteMatch, `${hint} 「色名の側（…）」を切り出せなかった。`).not.toBeNull();
    const colorNote = colorNoteMatch![1];

    const shadedCountMatch = colorNote.match(/(\d+)\s*色/);
    expect(shadedCountMatch, `${hint} 「N色」の件数を読めなかった: ${colorNote}`).not.toBeNull();
    const shadedCount = Number(shadedCountMatch![1]);
    // 0件は突き合わせにならない（10-3-d と同じ理由）。
    expect(shadedCount, `${hint} 読み取った件数が 0 である。`).toBeGreaterThan(0);

    expect(
      TAILWIND_SHADED_PALETTE_COLORS.length,
      "シェードを持つ色の件数が 10-3-c の字面と一致しない。色を増減させるなら条文の件数も同時に変えること" +
        "（要素そのものの固定は 10-3-c が色名を逐語で列挙しないため行えない。限界は 11-26）。",
    ).toBe(shadedCount);

    // シェードを持たない色は条文が字面に挙げているため、要素まで突き合わせる。
    const extractedShadeless = [...colorNote.matchAll(/`([a-z-]+)`/g)].map((match) => match[1]);
    expect(
      extractedShadeless.length,
      `${hint} 括弧の中にバッククォート付きの色名が1つも無かった。`,
    ).toBeGreaterThan(0);
    // 順序は読まない。10-3-e が要求するのは「字面に挙がっている語」であり、
    // 「順序を含めて」を明文で要求しているのは 10-3-d（接頭辞の側）だけである。
    // 条文の語順だけを入れ替える編集で落ちると偽 Red になるため、両側を
    // 並べ替えて比べる。語の増減は並べ替えても落ちる（W5-3 と同じ理由）。
    expect(
      [...extractedShadeless].sort(),
      "シェードを持たない色の集合が 10-3-c の字面と一致しない（順序は読まない）。",
    ).toEqual([...TAILWIND_SHADELESS_PALETTE_COLORS].sort());
  });
});

// docs/specs/design-system.md AC-10-3-f。
//
// 10-3 / 10-3-c の検査は「禁じた表現が現れないこと」という *不在* の主張で
// あり、走査経路（パターンをテキストへ当てる部分）が壊れると空虚に真になる。
// 陽性対照を欠いた緑は何も意味しない、という条文の趣旨を機械検査へ落とす。
//
// 対照のテキストは、違反を1つだけ含む複数行のテキストとし、違反をテキストの
// 先頭・末尾のいずれでもない位置に置く。こうしておくと、パターンをテキスト
// 全体に係るアンカー（^…$）へ差し替える変異が落ちる。
//
// なお、これが示すのは「パターンを当てる経路」までである。正しいファイルを
// 読めていることは示さない（限界は AC-11-28。そちらは 10-3 の0件失敗と 10-4
// が担保する）。
const POSITIVE_CONTROL_TEXTS = [
  {
    label: "16進の色",
    text: 'export function Sample() {\n  const color = "#ff0000";\n  return color;\n}\n',
  },
  {
    label: "rgb() / hsl() の関数記法",
    text: 'export const style = {\n  color: "rgb(1, 2, 3)",\n};\n',
  },
  {
    label: "パレットユーティリティ",
    text: 'export function Sample() {\n  return <div className="bg-red-50 p-4" />;\n}\n',
  },
  {
    label: "rounded-md 以外の角丸",
    text: 'export function Sample() {\n  return <div className="rounded-full p-4" />;\n}\n',
  },
  {
    label: "fetch の呼び出し",
    text: 'export async function load() {\n  const res = await fetch("/api/x");\n  return res;\n}\n',
  },
];

describe("AC-10-3-f: 走査経路の陽性対照（不在の主張が空虚に真になっていないこと）", () => {
  it("対照の一覧は、不在を主張するパターンを1つずつ覆う", () => {
    // 読むのは集合としての一致であり、表と対照の並び順は読まない（10-3-d が
    // 「順序を含めて」を要求するのは接頭辞リストだけである）。並びを期待値に
    // 持つと、表の要素を入れ替えただけで Red になる（偽 Red）。
    expect(
      [...POSITIVE_CONTROL_TEXTS].map(({ label }) => label).sort(),
      "対照とパターンの対応が崩れている。パターンを増減させたなら対照も同時に増減させること（順序は読まない。AC-10-3-f）。",
    ).toEqual([...FORBIDDEN_EXPRESSION_PATTERNS].map(({ label }) => label).sort());
  });

  it.each(POSITIVE_CONTROL_TEXTS)(
    "$label を含む複数行テキストを、実ファイルと同じ経路で検出できる",
    ({ label, text }) => {
      expect(
        text.split("\n").length,
        `${label} の対照が複数行でない。行をまたぐ走査であることを示せない（AC-10-3-f）。`,
      ).toBeGreaterThan(1);

      // 実ファイルの走査と同じ関数を通す。
      expect(
        findForbiddenExpressions(text),
        `${label} を含むテキストを検出できていない。走査経路が空洞化している` +
          `（パターンをテキスト全体に係るアンカーへ差し替えた／決して一致しない形へ` +
          `差し替えた等）可能性がある。不在の主張はこの経路が壊れると空虚に真になる（AC-10-3-f）。`,
      ).toContain(label);

      // 違反がテキストの先頭・末尾のいずれでもない位置にあること（AC-10-3-f）。
      // ここを満たさない対照は、アンカーを付ける変異を判別できない。
      const entry = FORBIDDEN_EXPRESSION_PATTERNS.find((p) => p.label === label);
      expect(entry, `${label} に対応するパターンが表に無い。`).not.toBeUndefined();
      const matched = entry!.pattern.exec(text);
      expect(matched, `${label} の対照から一致位置を取れなかった。`).not.toBeNull();
      expect(
        matched!.index,
        `${label} の対照が先頭で一致している。先頭アンカーを付ける変異を判別できない（AC-10-3-f）。`,
      ).toBeGreaterThan(0);
      expect(
        matched!.index + matched![0].length,
        `${label} の対照が末尾で一致している。末尾アンカーを付ける変異を判別できない（AC-10-3-f）。`,
      ).toBeLessThan(text.length);
    },
  );
});
