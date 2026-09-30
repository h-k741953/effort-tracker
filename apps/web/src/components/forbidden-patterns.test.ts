import { existsSync, readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import ts from "typescript";
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

// AC-4-6: outline-none を書くなら、同じファイル内に代替のリング指定
// （focus-visible のリングの幅を与えるユーティリティ）を伴うこと。判定を述語
// として切り出すのは、条件つきの検査に陽性対照を置くため（AC-10-3-g）。
// 幅は正の整数か `[<長さ>]` に限る（AC-10-3-a）。`focus-visible:ring-focus-ring`
// は色を設定するだけでリングを描かないため、代替に数えない。長さの単位は
// Tailwind v4 が任意値を長さとして推論するもの（大文字は色として扱われる）。
// 語の前後に境界を置き、`group-focus-visible:ring-2` のようにバリアントを前置した
// 語（要素自身のフォーカスでは描かれない）や後ろへ続く語を数えない（AC-10-3-g）。
const CSS_LENGTH_UNITS =
  "cm|mm|Q|in|pc|pt|px|em|ex|ch|rem|lh|rlh|vw|vh|vmin|vmax|vb|vi|svw|svh|lvw|lvh|dvw|dvh|cqw|cqh|cqi|cqb|cqmin|cqmax";
const focusVisibleRingPattern = new RegExp(
  `(?<![\\w:-])focus-visible:ring-(?:[1-9]\\d*|\\[\\d*\\.?\\d+(?:${CSS_LENGTH_UNITS})\\])(?=[\\s"'\`]|$)`,
);
function violatesOutlineNoneRule(text: string): boolean {
  return text.includes("outline-none") && !focusVisibleRingPattern.test(text);
}

// AC-10-3-f: 不在を主張する5本を1つの表に集め、実ファイルの走査と陽性対照が
// 同じ経路（findForbiddenExpressions）を通るようにする。経路を分けると、
// 対照だけが通る形へパターンを差し替えられてしまい対照の意味が無くなる。
// ラベルの文字列は定数に持つ。表・陽性対照の一覧・担い手の期待値のいずれも
// この定数を参照することで、文字列がずれない（AC-10-3-f）。
const HEX_COLOR_LABEL = "16進の色";
const RGB_OR_HSL_LABEL = "rgb() / hsl() の関数記法";
const PALETTE_UTILITY_LABEL = "パレットユーティリティ";
const NON_MD_ROUNDED_LABEL = "rounded-md 以外の角丸";
const FETCH_CALL_LABEL = "fetch の呼び出し";

const FORBIDDEN_EXPRESSION_PATTERNS = [
  { label: HEX_COLOR_LABEL, pattern: hexColorPattern },
  { label: RGB_OR_HSL_LABEL, pattern: rgbOrHslFunctionPattern },
  { label: PALETTE_UTILITY_LABEL, pattern: paletteUtilityPattern },
  { label: NON_MD_ROUNDED_LABEL, pattern: nonMdRoundedPattern },
  { label: FETCH_CALL_LABEL, pattern: fetchCallPattern },
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

    // AC-10-3-f (iii): 読んだテキストが対象ファイルの全体であることを、読み取りとは
    // 別の出所（同じファイルを別に読んだバイト列）で読む。0文字ガードと両端の対照は、
    // 読み取り位置そのものを加工する変異（readFileSync(...) の結果を切る・置き換える
    // 形）を通してしまう —— 対照は content の下流で組み立てるため両端とも無傷に残り、
    // 実ファイル由来の違反だけが不可視になる（この形を述べているのは 10-3-f (iii)
    // 自身であり、AC-11-29 の (a)〜(d) ではない）。
    //
    // 長さの一致では足りない。1文字を同じ長さの別の文字へ置き換える形
    // （replaceAll("#", " ") など）はバイト数の比較を通り抜けるため、内容そのものの
    // 同一性を読む。
    expect(
      Buffer.compare(Buffer.from(content, "utf8"), readFileSync(filePath)),
      `${fileName} から読み取ったテキストが対象ファイルの全体と一致しない。読み取り` +
        `位置で切り落とすか置き換えると、不在の検査が空振りしたまま緑になる` +
        `（AC-10-3-f (iii)）。`,
    ).toBe(0);

    // AC-10-3-f: 陽性対照の担い手を「実ファイルから読んだテキストそのもの」に
    // する。読んだテキストの前後の両端へ、互いに異なる2つの形の違反を1つずつ
    // 足したものを、不在の主張と同じ1回の走査へ通し、いずれも検出されることを
    // 読む。こうすると、走査対象を切り落とす変異（引数を丸ごと切る形・先頭側
    // だけを残す形・末尾側だけを残す形）はどの向きでも片方の対照を消すため
    // 落ちる。片端だけに置くと、その端が残る向きの切り落とし（末尾に置いたなら
    // slice(-N)）が対照を消さないまま、実ファイル由来の違反を不可視にする
    // （限界は AC-11-29）。
    //
    // 担い手は AC-2-4 の2つの形（16進 / rgb() hsl() の関数記法）から選ぶ
    // （AC-10-3-f）。担い手に選んだ形の不在は「自身の引数を切る変異では落ちない」側
    // （AC-11-29 (a)）へ移るため、その形が当該 AC の唯一の機械的な足場である場合、
    // AC がまるごと無検査になる —— fetch（AC-5-5）と rounded-md 以外の角丸（AC-4-4）
    // はこのファイルが唯一の足場なので担い手にしない。AC-2-4 は3つの形を持つため、
    // 2つを担い手にしても残る1つ（パレットユーティリティ）が統合した主張の側に残る。
    const controlled =
      `// const __headControl = "#ff0000";\n` +
      `${content}\n// const __tailControl = "rgb(0 0 0)";\n`;

    // AC-10-3-f (i): 担い手が実ファイルから読んだテキストそのものであること
    // 自体を読む。これを読まないと、担い手を検査自身のテキストへ差し替える変異
    // （実ファイルのテキストを連結しない形）が落ちず、10-3-f が新設した要求が
    // 機械検査されないまま残る。両端へ足すため「始まる」ではなく「部分として
    // 含む」で読む。content の長さは上で0文字でないことを読んでいる。
    expect(
      controlled.includes(content) && controlled.length > content.length,
      `${fileName} の陽性対照が実ファイルのテキストを担い手にしていない（AC-10-3-f）。`,
    ).toBe(true);

    // AC-10-3-f (ii): 不在の主張と陽性対照を1本の主張へまとめ、走査の結果と
    // 期待値の間に可変の絞り込み（filter 等）を挟まない。絞り込みを挟むと、
    // それを空にする変異で不在の主張だけが空虚に真になり、対照は別の主張として
    // 通り続ける。期待値をちょうど2要素（両端の対照が出した2つの形）にすること
    // で、走査結果が欠ける変異は陽性対照として落ち、実ファイル由来の違反は余剰
    // として落ちる。両側を sort して集合として読む —— 表の並び順は読まない
    // （10-3-d が順序を要求するのは接頭辞リストだけであり、並びを期待値に持つと
    // 表を並べ替えただけで Red になる。I8-1 と同型の偽 Red）。sort は要素を
    // 減らさないため、走査結果が欠ける変異はこの形でも落ちる。
    expect(
      [...findForbiddenExpressions(controlled)].sort(),
      `${fileName} の走査結果が「${HEX_COLOR_LABEL}と${RGB_OR_HSL_LABEL}だけ」に` +
        `ならない。どちらかが出ていないなら走査対象が切り落とされており、不在の主張が` +
        `空虚に真になっている。他の種類が出ているならそのファイルが禁じた表現を含む` +
        `（AC-2-4 / AC-4-4 / AC-5-5 / AC-10-3-f）。`,
    ).toEqual([HEX_COLOR_LABEL, RGB_OR_HSL_LABEL].sort());

    // AC-10-3-f (iv): 実ファイルのテキストだけを走査する不在の主張を、上の統合した
    // 主張と対にして別に1本置く。単独ではこの主張は走査経路が壊れると空虚に真になる
    // （それが (ii) で統合した理由である）。いっぽう統合した主張は、両端の対照を
    // いずれも残したまま結果を絞り込む変異（`.sort().slice(0, 2)` —— 対照の2形より
    // 後ろに並ぶラベルが落ちる）や中間だけを落とす変異で空虚になる。両者の弱点は
    // 重ならないため、対にすると単一の変異ではどちらかが必ず落ちる。統合した主張を
    // 置き換えるのではなく、相互に守らせるために足す。
    expect(
      [...findForbiddenExpressions(content)],
      `${fileName} が禁止表現を含む（AC-2-4 / AC-4-4 / AC-5-5）。`,
    ).toEqual([]);

    // 対照そのものに用いた2つの形の不在は、上の統合した主張では実ファイル由来のそれを
    // 読めない（対照が必ず検出させてしまう）ため、実ファイルのテキストで別に読む。
    // この2本と (iv) は担い手の2形について互いの足場になるので、どちらか一方の引数を
    // 切る変異はもう一方が落とす。担い手でない3形は (iv) と統合した主張が対になる。
    // どちらも AC-2-4 の形であり、同 AC には残る1つ（パレットユーティリティ）が
    // 統合した主張の側に足場として残る。rounded-md 以外の角丸（AC-4-4）と fetch
    // （AC-5-5）はこちらではなく上の2本が読む。
    expect(
      hexColorPattern.test(content),
      `${fileName} が${HEX_COLOR_LABEL}を含む（AC-2-4）。`,
    ).toBe(false);
    expect(
      rgbOrHslFunctionPattern.test(content),
      `${fileName} が${RGB_OR_HSL_LABEL}を含む（AC-2-4）。`,
    ).toBe(false);

    // AC-4-6。判定は述語に持たせ、陽性対照は下の一覧が別に読む（AC-10-3-g）。
    expect(
      violatesOutlineNoneRule(content),
      `${fileName} が outline-none を代替のリング指定なしで用いている（AC-4-6）。`,
    ).toBe(false);
  });

  // AC-10-3-h: 走査する母集団が components/ 直下の実装 *.tsx の集合と一致する
  // ことを、この検査の中で読む。一覧から1行落とす変異は、そのファイルの不在検査を
  // 丸ごと消すが、件数が1件減るだけで誰も落とさない（AC-10-4 は別の検査が持つ
  // 別の一覧に対する検査であり、こちらの一覧を縛らない）。
  it("走査する母集団が components/ 直下の実装 *.tsx の集合と一致する", () => {
    const actual = readdirSync(componentsDir, { withFileTypes: true })
      .filter((entry) => entry.isFile())
      .map((entry) => entry.name)
      .filter((name) => name.endsWith(".tsx") && !name.endsWith(".test.tsx"))
      .sort();
    expect(
      actual,
      "走査する母集団が components/ 直下の実装 *.tsx の集合と一致しない。一覧から" +
        "落ちたファイルは不在の検査を丸ごと免れる（AC-10-3-h）。",
    ).toEqual([...EXPECTED_FILES].sort());
  });

  // AC-10-3-g: 4-6 の条件つき検査の判別力。判定パターンを恒真へ緩めるだけで
  // 実ファイル由来の違反（代替のリング指定を伴わない outline-none）が通るため、
  // 適合・不適合の両ケースを実ファイルと同じ述語で読む。10-3-f の陽性対照は
  // 10-3 の5つの形だけを覆い、4-6 を覆わない。
  it.each([
    ["outline-none に focus-visible のリング指定を伴う", 'className="outline-none focus-visible:ring-2"', false],
    ["outline-none をリング指定なしで用いる", 'className="outline-none px-2"', true],
    ["outline-none を用いない", 'className="px-2 focus-visible:ring-2"', false],
    ["リング指定が focus-visible を伴わない", 'className="outline-none ring-2"', true],
    ["リング指定が色だけ", 'className="outline-none focus-visible:ring-focus-ring"', true],
    [
      "リング指定がオフセットと色だけ",
      'className="outline-none focus-visible:ring-offset-2 focus-visible:ring-focus-ring"',
      true,
    ],
    [
      "長さの任意値で幅を与える",
      'className="outline-none focus-visible:ring-[3px] focus-visible:ring-focus-ring"',
      false,
    ],
    [
      "px 以外の長さの単位で幅を与える",
      'className="outline-none focus-visible:ring-[0.15vw] focus-visible:ring-focus-ring"',
      false,
    ],
    ["リング指定がグループのバリアントだけ", 'className="outline-none group-focus-visible:ring-2"', true],
    ["リング指定の語が後ろへ続く", 'className="outline-none focus-visible:ring-2x"', true],
  ])("4-6 の判定: %s", (_name, text, expected) => {
    expect(
      violatesOutlineNoneRule(text as string),
      `4-6 の判定が期待と異なる（AC-10-3-g）。対象: ${text}`,
    ).toBe(expected);
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
    label: HEX_COLOR_LABEL,
    text: 'export function Sample() {\n  const color = "#ff0000";\n  return color;\n}\n',
  },
  {
    label: RGB_OR_HSL_LABEL,
    text: 'export const style = {\n  color: "rgb(1, 2, 3)",\n};\n',
  },
  {
    label: PALETTE_UTILITY_LABEL,
    text: 'export function Sample() {\n  return <div className="bg-red-50 p-4" />;\n}\n',
  },
  {
    label: NON_MD_ROUNDED_LABEL,
    text: 'export function Sample() {\n  return <div className="rounded-full p-4" />;\n}\n',
  },
  {
    label: FETCH_CALL_LABEL,
    text: 'export async function load() {\n  const res = await fetch("/api/x");\n  return res;\n}\n',
  },
];

describe("AC-10-3-f: 走査経路の陽性対照（不在の主張が空虚に真になっていないこと）", () => {
  it("対照の一覧は、不在を主張するパターンを1つずつ覆う", () => {
    // 読むのは集合としての一致であり、表と対照の並び順は読まない（10-3-d が
    // 「順序を含めて」を要求するのは接頭辞リストだけである）。並びを期待値に
    // 持つと、表の要素を入れ替えただけで Red になる（偽 Red）。
    expect(
      POSITIVE_CONTROL_TEXTS.map(({ label }) => label).sort(),
      "対照とパターンの対応が崩れている。パターンを増減させたなら対照も同時に増減させること（順序は読まない。AC-10-3-f）。",
    ).toEqual(FORBIDDEN_EXPRESSION_PATTERNS.map(({ label }) => label).sort());
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

// docs/specs/design-system.md AC-10-3-i。
//
// AC-6 の表の 6-3 は「型 Role は apps/web/src/lib/role-cookie.ts の既存の型を
// 輸入して使う（リテラル union を再定義しない）」と定める。この要求は型検査でも
// DOM の検査でも落ちない —— ローカルに同じリテラル union を書いた実装は tsc /
// eslint を通り、出力も変わらないため 10-6 の検査にも掛からない。したがって
// 実装のテキストで読む（10-3 と同じ手段）。
//
// 輸入元のファイルと対象コンポーネントのファイル名は条文の字面から取る
// （テスト側に書いた写しを期待値にすると条文と乖離したまま緑になる。10-3-d /
// 10-2-a と同じ理由）。0件のときは失敗とする。
const ROLE_TYPE_NAME = "Role";

// 宣言と輸入はテキストの字面ではなく構文木で読む。コメントを字面の置換で除くと、
// 文字列の中の `//` や `/*` から後ろまで消えて再定義を読み落とし、逆にコメントの
// 中にだけ書いた輸入を輸入として数えてしまう（AC-10-3-i）。typescript は既存の
// devDependency であり、新たな依存は加えない。
function parseSource(text: string): ts.SourceFile {
  return ts.createSourceFile("source.tsx", text, ts.ScriptTarget.Latest, false, ts.ScriptKind.TSX);
}

/**
 * テキストが型 Role をローカルに宣言しているか（リテラル union の再定義）。
 * 宣言として数えるのは `type Role =` / `interface Role` / `enum Role` の文だけで、
 * 輸入・再輸出の中の `type Role`（`import { type Role }`）とコメント中の語は
 * 数えない —— 数えると条文どおりの輸入が落ちる（AC-10-3-i）。
 */
function declaresLocalRoleType(text: string): boolean {
  let found = false;
  const visit = (node: ts.Node): void => {
    if (
      (ts.isTypeAliasDeclaration(node) || ts.isInterfaceDeclaration(node) || ts.isEnumDeclaration(node)) &&
      node.name.text === ROLE_TYPE_NAME
    ) {
      found = true;
    }
    ts.forEachChild(node, visit);
  };
  visit(parseSource(text));
  return found;
}

/** テキストが型 Role を、末尾が moduleBase のモジュールから輸入しているか。 */
function importsRoleTypeFrom(text: string, moduleBase: string): boolean {
  return parseSource(text).statements.some((statement) => {
    if (!ts.isImportDeclaration(statement) || !ts.isStringLiteral(statement.moduleSpecifier)) return false;
    if (!statement.moduleSpecifier.text.endsWith(`/${moduleBase}`)) return false;
    const bindings = statement.importClause?.namedBindings;
    return (
      bindings !== undefined &&
      ts.isNamedImports(bindings) &&
      bindings.elements.some((element) => (element.propertyName ?? element.name).text === ROLE_TYPE_NAME)
    );
  });
}

describe("AC-10-3-i: 型 Role は条文が名指す既存の型を輸入する（6-3）", () => {
  const specPath = path.join(componentsDir, "..", "..", "..", "..", "docs", "specs", "design-system.md");
  const hint =
    `仕様書 ${specPath} の字面から 6-3 / AC-5-1 の記述を読めなかった。条文の体裁を` +
    `変えたのなら、要求の変更ではないのでこの抽出側を追随させること（10-3-i は抽出` +
    `手段を仕様で固定していない）。`;

  function readSpec(): string {
    expect(existsSync(specPath), `仕様書が見つからない: ${specPath}`).toBe(true);
    return readFileSync(specPath, "utf8");
  }

  /** AC-6 の表の 6-3 行から、輸入元のモジュール名（拡張子なしのファイル名）を読む。 */
  function readRoleModuleBase(spec: string): string {
    const row = spec.match(/^\|\s*6-3\s*\|.*$/m);
    expect(row, hint).not.toBeNull();
    const pathMatch = row![0].match(/`(apps\/web\/src\/lib\/[\w./-]+)\.ts`/);
    expect(pathMatch, `${hint} 6-3 行から lib 配下のファイルパスを切り出せなかった。`).not.toBeNull();
    const base = pathMatch![1].split("/").pop() as string;
    expect(base.length, `${hint} 読み取ったモジュール名が空である。`).toBeGreaterThan(0);
    return base;
  }

  /** AC-5-1 の台帳から RoleSwitcher の実装ファイル名を読む。 */
  function readRoleSwitcherFile(spec: string): string {
    const row = spec.match(/^\|\s*5-1-[a-z]+\s*\|\s*`RoleSwitcher`\s*\|\s*`([^`]+)`/m);
    expect(row, `${hint} AC-5-1 の台帳から RoleSwitcher の行を切り出せなかった。`).not.toBeNull();
    const fileName = row![1];
    // 台帳から読んだファイル名が、この検査が走査する母集団（10-3-h が実ディレクトリ
    // との一致を読む）に在ることを確かめる。空振りしたまま緑にならないようにする。
    expect(EXPECTED_FILES, `${hint} 台帳の ${fileName} が走査する母集団に無い。`).toContain(
      fileName,
    );
    return fileName;
  }

  it("条文から輸入元のモジュール名と対象ファイル名を読める（0件なら失敗）", () => {
    const spec = readSpec();
    expect(readRoleModuleBase(spec).length).toBeGreaterThan(0);
    expect(readRoleSwitcherFile(spec).length).toBeGreaterThan(0);
  });

  it("RoleSwitcher の実装は Role を条文が名指すモジュールから輸入する", () => {
    const spec = readSpec();
    const moduleBase = readRoleModuleBase(spec);
    const fileName = readRoleSwitcherFile(spec);
    const source = readFileSync(path.join(componentsDir, fileName), "utf8");
    expect(source.length, `${fileName} から読み取ったテキストが0文字である。`).toBeGreaterThan(0);
    expect(
      importsRoleTypeFrom(source, moduleBase),
      `${fileName} が型 ${ROLE_TYPE_NAME} を ${moduleBase} から輸入していない（AC-6-3 / AC-10-3-i）。`,
    ).toBe(true);
  });

  // 「ローカルに再定義しない」は母集団全体で読む。1ファイルだけを名指しすると、
  // 他のコンポーネントが同じ union を書き始めても誰も読まない（10-3-h が是正した
  // のと同型の穴）。母集団は 10-3-h が実ディレクトリとの一致を読む列である。
  it.each(EXPECTED_FILES)("%s は型 Role をローカルに宣言しない（リテラル union の再定義）", (fileName) => {
    const source = readFileSync(path.join(componentsDir, fileName), "utf8");
    expect(source.length, `${fileName} から読み取ったテキストが0文字である。`).toBeGreaterThan(0);
    expect(
      declaresLocalRoleType(source),
      `${fileName} が型 ${ROLE_TYPE_NAME} をローカルに宣言している（AC-6-3 は既存の型の輸入を求める / AC-10-3-i）。`,
    ).toBe(false);
  });

  // 上の2本のうち「宣言しない」側は不在の主張であり、判定が恒偽へ潰れると空虚に
  // 真になる。輸入の側も、判定が恒真へ潰れると実装を読まずに緑になる。どちらも
  // 実ファイルと同じ述語へテキストを与えて両側を読む（10-3-g と同じ形）。
  it.each([
    ["リテラル union を再定義する", 'type Role = "Engineer" | "Approver";', true],
    ["interface で再定義する", "interface Role { name: string }", true],
    ["輸入だけを行う", 'import type { Role } from "@/lib/role-cookie";', false],
    ["型として参照するだけ", 'const role: Role = "Engineer";', false],
    ["export つきで再定義する", 'export type Role = "Engineer";', true],
    ["enum で再定義する", "enum Role { Engineer }", true],
    ["type 節つきで輸入する", 'import { type Role } from "@/lib/role-cookie";', false],
    ["type 節つきで複数行に分けて輸入する", 'import {\n  type Role,\n} from "@/lib/role-cookie";', false],
    ["type 節つきで再輸出する", 'export { type Role } from "@/lib/role-cookie";', false],
    ["コメントで型の名に触れる", "// type Role = を再定義しない\n/* interface Role */", false],
    [
      "URL を含む文字列の後ろで再定義する",
      'const docs = "https://example.com"; type Role = "Engineer" | "Approver";',
      true,
    ],
    [
      "/* と */ を含む文字列に挟んで再定義する",
      'const a = "src/*"; type Role = "Engineer" | "Approver"; const b = "*/";',
      true,
    ],
  ])("ローカル宣言の判定: %s", (_name, text, expected) => {
    expect(
      declaresLocalRoleType(text as string),
      `ローカル宣言の判定が期待と異なる（AC-10-3-i）。対象: ${text}`,
    ).toBe(expected);
  });

  it.each([
    ["名指すモジュールから輸入する", 'import type { Role } from "@/lib/role-cookie";', true],
    ["型として輸入する（type 節つき）", 'import { type Role } from "@/lib/role-cookie";', true],
    ["別のモジュールから輸入する", 'import type { Role } from "./local-role";', false],
    ["別の型だけを輸入する", 'import type { RoleName } from "@/lib/role-cookie";', false],
    ["輸入せずに宣言する", 'type Role = "Engineer";', false],
    ["コメントの中にだけ輸入を書く", '// import type { Role } from "@/lib/role-cookie";', false],
    ["文字列の中にだけ輸入を書く", 'const s = \'import type { Role } from "@/lib/role-cookie";\';', false],
  ])("輸入の判定: %s", (_name, text, expected) => {
    expect(
      importsRoleTypeFrom(text as string, "role-cookie"),
      `輸入の判定が期待と異なる（AC-10-3-i）。対象: ${text}`,
    ).toBe(expected);
  });
});
