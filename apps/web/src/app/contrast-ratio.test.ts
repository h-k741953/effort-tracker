import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { describe, expect, it } from "vitest";

// docs/specs/design-system.md AC-3（検証手段は AC-10-2）。
//
// 「AC-2 の表の16進値から相対輝度とコントラスト比を計算し、下限と比較する
// （計算は外部ライブラリを使わずテスト内で行う）」を満たす。値は globals.css
// から読む（AC-1-1 の「正は globals.css の1箇所」に沿う。globals.css.test.ts
// の値検査 (AC-2) が別に落ちるため、ここでは重複を避け AC-3 の計算のみを扱う）。

const appDir = fileURLToPath(new URL(".", import.meta.url));
const cssPath = path.join(appDir, "globals.css");
const css = readFileSync(cssPath, "utf8");

function extractBlockBody(source: string, headerRegex: RegExp): string | undefined {
  const match = headerRegex.exec(source);
  if (!match) return undefined;
  const openIndex = source.indexOf("{", match.index + match[0].length - 1);
  if (openIndex === -1) return undefined;
  const closeIndex = source.indexOf("}", openIndex);
  if (closeIndex === -1) return undefined;
  return source.slice(openIndex + 1, closeIndex);
}

function parseCustomProps(block: string | undefined): Map<string, string> {
  const map = new Map<string, string>();
  if (!block) return map;
  const re = /--([\w-]+)\s*:\s*([^;]+);/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(block))) {
    map.set(m[1].trim(), m[2].trim());
  }
  return map;
}

const darkMediaHeader = /@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)\s*\{\s*:root\s*/;
const darkBlockBody = extractBlockBody(css, darkMediaHeader);
const darkTokens = parseCustomProps(darkBlockBody);

const cssWithoutDarkBlock = darkBlockBody
  ? css.replace(new RegExp(darkMediaHeader.source + "\\{[^}]*\\}\\s*\\}"), "")
  : css;

const lightBlockBody = extractBlockBody(cssWithoutDarkBlock, /:root\s*/);
const lightTokens = parseCustomProps(lightBlockBody);

// WCAG 2.1 の相対輝度・コントラスト比の定義。
function srgbChannelToLinear(c: number): number {
  const s = c / 255;
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function relativeLuminance(hex: string): number {
  const m = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex);
  if (!m) throw new Error(`not a 6-digit hex color: ${hex}`);
  const r = srgbChannelToLinear(parseInt(m[1], 16));
  const g = srgbChannelToLinear(parseInt(m[2], 16));
  const b = srgbChannelToLinear(parseInt(m[3], 16));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrastRatio(a: string, b: string): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const lighter = Math.max(la, lb);
  const darker = Math.min(la, lb);
  return (lighter + 0.05) / (darker + 0.05);
}

// AC-3 の表の各行（3-7 は状態バッジ3組ぶん展開する）。この一覧が表と過不足なく
// 対応していることは下の AC-10-2-a の検査が条文の字面に対して読む —— 手書きの
// 一覧は行を落としても残りの行が緑のまま通るため、それだけでは AC-3 の母集団を
// 担保しない。
const PAIRS: ReadonlyArray<{ id: string; fg: string; bg: string; min: number }> = [
  { id: "3-1", fg: "foreground", bg: "background", min: 4.5 },
  { id: "3-2", fg: "surface-foreground", bg: "surface", min: 4.5 },
  { id: "3-3", fg: "muted-foreground", bg: "background", min: 4.5 },
  { id: "3-4", fg: "primary-foreground", bg: "primary", min: 4.5 },
  { id: "3-5", fg: "danger-foreground", bg: "danger", min: 4.5 },
  { id: "3-6", fg: "danger", bg: "background", min: 4.5 },
  { id: "3-7 (draft)", fg: "state-draft-foreground", bg: "state-draft", min: 4.5 },
  {
    id: "3-7 (pending-approval)",
    fg: "state-pending-approval-foreground",
    bg: "state-pending-approval",
    min: 4.5,
  },
  { id: "3-7 (approved)", fg: "state-approved-foreground", bg: "state-approved", min: 4.5 },
  { id: "3-8", fg: "border", bg: "background", min: 3 },
  { id: "3-9", fg: "focus-ring", bg: "background", min: 3 },
];

describe("contrast-ratio - AC-3: コントラスト比の下限", () => {
  describe("明色（:root）", () => {
    it.each(PAIRS)("$id: --$fg / --$bg は $min:1 以上", ({ fg, bg, min }) => {
      const fgValue = lightTokens.get(fg);
      const bgValue = lightTokens.get(bg);
      expect(fgValue).toBeDefined();
      expect(bgValue).toBeDefined();
      const ratio = contrastRatio(fgValue as string, bgValue as string);
      expect(ratio).toBeGreaterThanOrEqual(min);
    });
  });

  describe("暗色（prefers-color-scheme: dark）", () => {
    it.each(PAIRS)("$id: --$fg / --$bg は $min:1 以上", ({ fg, bg, min }) => {
      const fgValue = darkTokens.get(fg);
      const bgValue = darkTokens.get(bg);
      expect(fgValue).toBeDefined();
      expect(bgValue).toBeDefined();
      const ratio = contrastRatio(fgValue as string, bgValue as string);
      expect(ratio).toBeGreaterThanOrEqual(min);
    });
  });
});

// docs/specs/design-system.md AC-10-2-a。
//
// 上の PAIRS は手書きの一覧であり、それ自体は AC-3 の表と結ばれていない。行を
// 1つ落としても残りの行は緑のまま通るため、落とした組のコントラストは誰も
// 測らないまま緑になる（10-3-h / 10-3-a が母集団について是正したのと同型の穴）。
// そこで条文の字面から AC-3 の表を読み、#・前景 / 背景・下限の3つを突き合わせる。
// 抽出結果が0件のときは失敗とする（0件どうしの一致は一致ではない。10-3-d と
// 同じ理由）。
const specPath = path.join(appDir, "..", "..", "..", "..", "docs", "specs", "design-system.md");

type SpecPairRow = { id: string; fg: string; bg: string; min: number };

/** 条文の AC-3 の節から表の行を読む。体裁を読めないときは投げる（Red にする）。 */
function parseAc3Rows(spec: string): SpecPairRow[] {
  const hint =
    `仕様書 ${specPath} の AC-3 の表を読めなかった。条文の体裁（見出し・表の列・下限の` +
    `強調）を変えたのなら、組の増減ではないのでこの抽出側を追随させること（10-2-a は` +
    `抽出手段を仕様で固定していない）。`;
  const start = spec.indexOf("### AC-3.");
  if (start < 0) throw new Error(`${hint} AC-3 の節が見つからない。`);
  const rest = spec.slice(start + 1);
  const end = rest.indexOf("\n### ");
  const section = end === -1 ? rest : rest.slice(0, end);
  const rows: SpecPairRow[] = [];
  for (const match of section.matchAll(
    /^\|\s*(3-\d+)\s*\|([^|]*)\|\s*\*\*([\d.]+):1\*\*\s*\|\s*$/gm,
  )) {
    const tokens = [...match[2].matchAll(/`--([^`]+)`/g)].map((token) => token[1]);
    if (tokens.length !== 2) {
      throw new Error(`${hint} ${match[1]} 行から前景 / 背景の2トークンを読めない: ${match[2]}`);
    }
    rows.push({ id: match[1], fg: tokens[0], bg: tokens[1], min: Number(match[3]) });
  }
  if (rows.length === 0) throw new Error(`${hint} 表の行を1件も読めなかった。`);
  // 形の合わない `3-N` 行を黙って読み飛ばすと、その組のコントラストは誰も測らない
  // まま緑になる（上の一覧を条文へ結んだ理由そのもの）。節の中の `3-N` 行の数と、
  // 読めた行の数が一致することを読む（AC-10-2-a）。
  const rowLines = section.match(/^\|\s*3-\d+\s*\|.*$/gm) ?? [];
  if (rowLines.length !== rows.length) {
    const readIds = new Set(rows.map((row) => row.id));
    const unread = rowLines.filter((line) => !readIds.has(line.split("|")[1].trim()));
    throw new Error(
      `${hint} AC-3 の表に形の合わない行がある（下限の欄が **<値>:1** でない等）: ${unread.join(" / ")}`,
    );
  }
  return rows;
}

const AC3_ROWS = parseAc3Rows(readFileSync(specPath, "utf8"));

/** 「3-7 (draft)」のような展開後の # から、条文の行の # を取る。 */
function baseId(id: string): string {
  return id.split(" ")[0];
}

function escapeForRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** `state-<状態>-foreground` のような雛形を、`<…>` を1つの捕捉群に替えた正規表現へ。 */
function templateToRegExp(template: string, placeholder: string): RegExp {
  const [head, ...tailParts] = template.split(placeholder);
  return new RegExp(`^${escapeForRegExp(head)}(.+)${escapeForRegExp(tailParts.join(placeholder))}$`);
}

describe("contrast-ratio - AC-10-2-a: 検査ペアが AC-3 の表の字面と一致する", () => {
  it("条文から読んだ AC-3 の行は0件ではない", () => {
    expect(AC3_ROWS.length).toBeGreaterThan(0);
  });

  // 形の合わない行を読み飛ばさない検査は、現行の条文では一度も働かない（全行が
  // 形に合う）ため、検査を外しても緑のままになる。実ケースと同じ関数へ合成した
  // 節を与えて、形の合う行だけの節は読め、崩れた行を含む節は投げることを読む。
  const SECTION_HEAD = "### AC-3. コントラスト\n\n| # | 組 | 下限 |\n|---|---|---|\n";
  it("AC-3 の表の読み取り: 形の合う行だけなら読める", () => {
    expect(
      parseAc3Rows(`${SECTION_HEAD}| 3-1 | \`--a\` / \`--b\` | **4.5:1** |\n`).map((row) => row.id),
    ).toEqual(["3-1"]);
  });
  it("AC-3 の表の読み取り: 下限が強調されていない行を読み飛ばさずに投げる", () => {
    expect(() =>
      parseAc3Rows(
        `${SECTION_HEAD}| 3-1 | \`--a\` / \`--b\` | **4.5:1** |\n| 3-2 | \`--c\` / \`--d\` | 4.5:1 |\n`,
      ),
    ).toThrow(/形の合わない行/);
  });

  it("検査ペアの # の集合が AC-3 の表の行と一致する", () => {
    const fromPairs = [...new Set(PAIRS.map((pair) => baseId(pair.id)))].sort();
    expect(
      fromPairs,
      "検査ペアの # の集合が AC-3 の表と一致しない。表から落ちた行のコントラストは" +
        "誰も測らないまま緑になる（AC-10-2-a）。組を増減させるなら条文の表も同時に" +
        "変えること（下限そのものを下げる判断は人間の承認事項。AC-3-10）。",
    ).toEqual(AC3_ROWS.map((row) => row.id).sort());
  });

  it.each(AC3_ROWS)("$id: 検査ペアが条文の前景 / 背景 / 下限と一致する", (row) => {
    const entries = PAIRS.filter((pair) => baseId(pair.id) === row.id);
    expect(entries.length, `${row.id} に対応する検査ペアが無い（AC-10-2-a）。`).toBeGreaterThan(0);

    for (const entry of entries) {
      expect(
        entry.min,
        `${entry.id} の下限が条文と一致しない（AC-3 / AC-10-2-a）。下限を下げる判断は` +
          `人間の承認事項である（AC-3-10）。`,
      ).toBe(row.min);
    }

    const placeholderMatch = `${row.fg} ${row.bg}`.match(/<[^>]+>/);
    if (placeholderMatch === null) {
      // 具体のトークン名を持つ行は、1行が1ペアに対応する。
      expect(
        entries.map((entry) => `${entry.fg} / ${entry.bg}`),
        `${row.id} の検査ペアが条文のトークン名と一致しない（AC-10-2-a）。`,
      ).toEqual([`${row.fg} / ${row.bg}`]);
      return;
    }

    // 3-7 のように `<状態>` を含む行は、条文が「3組すべて」を求めている。展開の
    // 期待値は globals.css に定義されている状態トークンから取る —— 展開後の一覧を
    // 自分で数えるだけでは、1組落としても残りが緑のまま通る。
    const placeholder = placeholderMatch[0];
    const fgPattern = templateToRegExp(row.fg, placeholder);
    const expanded = entries.map((entry) => {
      const matched = fgPattern.exec(entry.fg);
      expect(
        matched,
        `${entry.id} の前景 ${entry.fg} が条文の雛形 ${row.fg} に合わない（AC-10-2-a）。`,
      ).not.toBeNull();
      const substitution = matched![1];
      expect(
        entry.bg,
        `${entry.id} の背景が条文の雛形 ${row.bg} の展開と一致しない（AC-10-2-a）。`,
      ).toBe(row.bg.replace(placeholder, substitution));
      return substitution;
    });

    const fromCss = [...lightTokens.keys()]
      .map((name) => fgPattern.exec(name))
      .filter((matched): matched is RegExpExecArray => matched !== null)
      .map((matched) => matched[1]);
    expect(
      fromCss.length,
      `${row.id} の雛形に合うトークンが globals.css に1件も無い（AC-10-2-a）。`,
    ).toBeGreaterThan(0);
    expect(
      [...expanded].sort(),
      `${row.id} の展開が globals.css の状態トークンと一致しない。1組でも欠けると` +
        `その組のコントラストは誰も測らないまま緑になる（AC-3「3組すべて」/ AC-10-2-a）。`,
    ).toEqual([...fromCss].sort());
  });
});
