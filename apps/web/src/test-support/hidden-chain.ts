// docs/specs/readonly-screens-implementation-design.md SP-3「図と同じ内容の表」・決定2（表を画面に見える形で置く）。
// 稼働の推移の表・図・本体のテストが共有する。要素の自身か祖先（document の根まで）が、視覚的にも支援技術にも
// 隠されているかを、隠している形の名前で返す。空配列なら隠されていない。
//
// 母集団の根拠: apps/web/src で実装が実際に使う非表示系のクラスは sr-only だけである
// （`command grep -rnE "sr-only|invisible|opacity-0|hidden" --include=*.tsx` で実測。overflow-hidden は隠さないので数えない）。
// ただし図の側が表を包むときに使いうる Tailwind の非表示系として、hidden・invisible・collapse・opacity-0 と、
// 大きさを 0 にする h-0・w-0・size-0・max-h-0 も（次の条件のときだけ）数える。クラスは `md:hidden` のような接頭辞を外した名前で比べる。
// 大きさを 0 にするクラスは、同じ要素に overflow-hidden か overflow-clip があるときだけ数える（`w-0 flex-1 overflow-x-auto` は
// 隠さない）。`[&_svg]:hidden` のような `[` で始まる任意セレクタの変種（子孫への当て方）は数えない。
// 限界: 画面外への配置（-left-[9999px] 等）や clip など、jsdom では見え方を判定できない形は検出しない（列挙しきれない）。
// style は display:none・visibility:hidden・opacity:0 を数える。

export const HIDING_CLASSES = ["sr-only", "hidden", "invisible", "collapse", "opacity-0"];
/** 単体では隠さず、overflow-hidden / overflow-clip と一緒のときだけ隠す形。 */
export const ZERO_SIZE_CLASSES = ["h-0", "w-0", "size-0", "max-h-0"];
const CLIPPING = ["overflow-hidden", "overflow-clip"];

export function hiddenChain(el: Element): string[] {
  const found: string[] = [];
  for (let cur: Element | null = el; cur !== null; cur = cur.parentElement) {
    const names = [...cur.classList]
      .filter((token) => !token.startsWith("[")) // 任意セレクタの変種は、その要素自身を隠さない
      .map((token) => token.slice(token.lastIndexOf(":") + 1));
    const clips = names.some((n) => CLIPPING.includes(n));
    for (const name of names) {
      if (HIDING_CLASSES.includes(name) || (clips && ZERO_SIZE_CLASSES.includes(name))) found.push(name);
    }
    if (cur.hasAttribute("hidden")) found.push("hidden-attr");
    if (cur.getAttribute("aria-hidden") === "true") found.push("aria-hidden");
    const style = (cur as HTMLElement).style;
    if (style !== undefined) {
      if (style.display === "none") found.push("display:none");
      if (style.visibility === "hidden") found.push("visibility:hidden");
      if (style.opacity === "0") found.push("opacity:0");
    }
  }
  return found;
}
