/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { Pagination } from "./pagination";

// docs/specs/design-system.md AC-6-10（検証手段は AC-10-6・10-6-f）。
//
// 6-10-i は「前へ」「次へ」の識別手段を本仕様で固定せず、末尾で
// 「識別を表示文言に依存させないことで、ラベルの表記が検査の期待値に
// ならない」と述べる（限界は AC-11-20）。したがって本テストは表示文言を
// 一切、識別にも期待値にも使わない —— 押せない状態になる側から識別する
// （page が先頭のとき押せない側＝戻る方向、page が末尾のとき押せない側＝
// 進む方向。6-10-i）。
//
// 「押せない形にする」は AC-7-3 が disabled の使用を明示的に許す唯一の
// 業務外の抑止（入力範囲外の抑止）であるため、disabled 属性で検証する。
// 10-6-f・AC-11-16 により、<nav> は空でないアクセシブル名を持つことを
// 読む。仕様が固定しないのは名の具体的な文字列であって、名を与えること
// 自体ではないため、name オプションで空でないことを検証する。
//
// 方向ボタンを name: /\S/ で引くのは 6-10-ii の要求である（検査手段は
// 10-6-k、限界は AC-11-27）。文言を識別に使わないこと（6-10-i）と、名を
// 与えること自体（6-10-ii）は別の事柄であり、6-10-ii はアイコンのみの
// 実装を禁じていない —— aria-label 等で名を与えれば足りる。名の具体的な
// 文字列は期待値に持たず、空でないことだけを読む。

afterEach(() => {
  cleanup();
});

/**
 * 空でないアクセシブル名を持つボタンを、描画順に返す。
 * 名の具体的な文字列は読まない（6-10-i / 6-10-ii）。
 *
 * 1本も引けない場合は getAllByRole がその場で throw する（testing-library
 * の英文メッセージ）。名の不在そのものを日本語で診断するのは 6-10-ii の
 * 専用の検査であり、そこでは queryAllByRole を使って0本の経路も読む。
 */
function getNamedButtons(): HTMLElement[] {
  return screen.getAllByRole("button", { name: /\S/ });
}

/**
 * 直前の描画から、名の有無で絞り込まずに button のロールを持つ要素の
 * 全体を返す（10-6-l）。無名のボタンも含めて数えるために getNamedButtons
 * とは別に持つ。
 */
function getAllButtons(): HTMLElement[] {
  return screen.getAllByRole("button");
}

/**
 * 非活性（押せない形）であるかを読む（10-6-l）。
 *
 * AC-7-3 は disabled と aria-disabled の両方を非活性の表現として名指すため、
 * どちらの形でも成り立つものとして読む。一方の形だけを読むと、他方で表した
 * 条文適合の実装が落ちる（偽 Red）。属性だけではクリックを止められないため、
 * 「押せない形」の実質は onPageChange が呼ばれないことで併せて読む。
 */
function isInactive(element: HTMLElement): boolean {
  return (
    (element as HTMLButtonElement).disabled === true ||
    element.getAttribute("aria-disabled") === "true"
  );
}

/**
 * 直前の描画から、ちょうど1つだけ disabled になっているボタンの
 * インデックスを返す（6-10-i: 識別手段は表示文言に依存させない）。
 * ちょうど1つに定まらない場合は、境界での抑止（AC-6-10）が満たされて
 * いないことを示すため、ここで失敗させる。
 */
function findSoleDisabledButtonIndex(): number {
  const buttons = getNamedButtons();
  // 名を持つボタンが2つ未満だと、戻る方向・進む方向の2つを別々に
  // 識別できない（10-6-k。6-10-i は両方向を名指している）。1本だけ名を
  // 持つ場合はここで落ち、0本の場合は getNamedButtons が先に throw する。
  expect(
    buttons.length,
    "空でないアクセシブル名を持つボタンが2つ未満である（6-10-ii / 10-6-k）",
  ).toBeGreaterThanOrEqual(2);
  const disabledIndexes = buttons
    .map((button, index) => ({ index, disabled: isInactive(button) }))
    .filter((entry) => entry.disabled)
    .map((entry) => entry.index);
  expect(disabledIndexes).toHaveLength(1);
  return disabledIndexes[0];
}

describe("Pagination - AC-6-10", () => {
  it("<nav> が空でないアクセシブル名を持つ", () => {
    render(<Pagination page={2} pageCount={5} onPageChange={vi.fn()} />);
    expect(screen.getByRole("navigation", { name: /\S/ })).toBeTruthy();
  });

  it("6-10-ii: 方向ボタンは空でないアクセシブル名で2つ以上問い合わせられる", () => {
    render(<Pagination page={2} pageCount={5} onPageChange={vi.fn()} />);
    // 「ちょうど2つ」を要求しない（10-6-k）。ページ番号のボタンを出す
    // 実装を本仕様は禁じていないため、2つ以上であることだけを読む。
    // 0本の経路もここで診断できるよう queryAllByRole で引く。
    const named = screen.queryAllByRole("button", { name: /\S/ });
    expect(
      named.length,
      "空でないアクセシブル名を持つボタンが2つ以上ない（6-10-ii / 10-6-k）。" +
        "アイコンのみのボタンは aria-label 等で名を与えること。名の文字列は仕様で固定しない（AC-11-27）。",
    ).toBeGreaterThanOrEqual(2);
  });

  it("先頭ページ（page=1）では、押せないボタンがちょうど1つに定まる", () => {
    render(<Pagination page={1} pageCount={5} onPageChange={vi.fn()} />);
    findSoleDisabledButtonIndex();
  });

  it("末尾ページでは、押せないボタンがちょうど1つに定まる", () => {
    render(<Pagination page={5} pageCount={5} onPageChange={vi.fn()} />);
    findSoleDisabledButtonIndex();
  });

  it("先頭で押せない側と末尾で押せない側は互いに異なる", () => {
    render(<Pagination page={1} pageCount={5} onPageChange={vi.fn()} />);
    const backIndex = findSoleDisabledButtonIndex();
    cleanup();

    render(<Pagination page={5} pageCount={5} onPageChange={vi.fn()} />);
    const advanceIndex = findSoleDisabledButtonIndex();

    expect(advanceIndex).not.toBe(backIndex);
  });

  // 10-6-l: 境界で押せなくなるボタンを、名の有無で絞り込まずに同定する。
  //
  // findSoleDisabledButtonIndex は「名を持つボタン」の中で数えるため、方向
  // ボタンを無名にしたうえで境界で別の名を持つボタンを disabled にする実装を
  // 落とせない（10-6-k / 10-6-f / 10-6-j も落とせない。11-27）。そこで button
  // のロールを持つ要素の全体で数え、ちょうど1つであること（7-3 が disabled の
  // 使用を 6-2 と 6-10 に限り、6-10 が名指す範囲外の入力が各境界で1方向だけ
  // であることから従う）と、その1つが空でない名を持つこと（6-10-ii）を読む。
  //
  // 新たな要求ではない。名の文言そのものは読まない（残る穴は 11-30）。
  it.each([
    { label: "先頭", page: 1 },
    { label: "末尾", page: 5 },
  ])(
    "10-6-l: $label ページでは、押せないボタンが全体でちょうど1つに定まり、空でない名を持ち、押しても呼ばれない",
    ({ page }) => {
      const onPageChange = vi.fn();
      render(<Pagination page={page} pageCount={5} onPageChange={onPageChange} />);
      const inactive = getAllButtons().filter(isInactive);
      expect(
        inactive.length,
        "境界で非活性なボタンが全体でちょうど1つに定まらない（AC-6-10 / AC-7-3 / 10-6-l）。" +
          "7-3 は disabled / aria-disabled の使用を多重送信の抑止（6-2）と入力範囲外の抑止（6-10）に限る。",
      ).toBe(1);
      // 名を持つボタンの集合に含まれることで、空でない名を持つことを読む
      //（6-10-ii。名の文字列は期待値に持たない）。
      expect(
        getNamedButtons().includes(inactive[0]),
        "境界で押せなくなるボタンが空でないアクセシブル名を持たない（6-10-ii / 10-6-l）。" +
          "アイコンのみのボタンは aria-label 等で名を与えること。",
      ).toBe(true);
      // 「押せない形にする」の実質。属性だけでは発火を止められないため併せて読む。
      fireEvent.click(inactive[0]);
      expect(
        onPageChange,
        "境界で非活性なボタンを押すと onPageChange が呼ばれる。属性だけでは" +
          "「押せない形」にならない（AC-6-10 / 10-6-l）。",
      ).not.toHaveBeenCalled();
    },
  );

  // 10-6-m: 単ページ（page = pageCount = 1）は 10-6-l の対象外（1 < pageCount を
  // 要求する）でありながら、6-10 の2条件が同時に効く唯一の組である。ここを
  // 覆わないと、単ページのとき戻る方向が押せる実装が全緑になる。
  //
  // 「全体がすべて非活性」を要求してはならない —— 単ページのページ番号は範囲内
  // であり AC-7-3 が非活性化を禁じるため、ページ番号ボタンを出す条文適合の実装が
  // 落ちる（偽 Red）。ちょうど2つで読む（限界は 11-31）。
  it("10-6-m: 単ページ（page=1 / pageCount=1）では、押せないボタンが全体でちょうど2つになり、いずれも空でない名を持ち、押しても呼ばれない", () => {
    const onPageChange = vi.fn();
    render(<Pagination page={1} pageCount={1} onPageChange={onPageChange} />);
    const inactive = getAllButtons().filter(isInactive);
    expect(
      inactive.length,
      "単ページで非活性なボタンが全体でちょうど2つに定まらない（AC-6-10 / AC-7-3 / 10-6-m）。" +
        "page=1 は先頭かつ末尾であり、両方向が同時に範囲外となる。",
    ).toBe(2);
    const named = getNamedButtons();
    expect(
      inactive.every((button) => named.includes(button)),
      "単ページで押せなくなるボタンに、空でないアクセシブル名を持たないものがある（6-10-ii / 10-6-m）。",
    ).toBe(true);
    inactive.forEach((button) => fireEvent.click(button));
    expect(
      onPageChange,
      "単ページで非活性なボタンを押すと onPageChange が呼ばれる（AC-6-10 / 10-6-m）。",
    ).not.toHaveBeenCalled();
  });

  it("中間ページでは戻る方向・進む方向のいずれも押せる", () => {
    render(<Pagination page={3} pageCount={5} onPageChange={vi.fn()} />);
    const buttons = getNamedButtons();
    expect(buttons.length).toBeGreaterThanOrEqual(2);
    expect(buttons.every((button) => !(button as HTMLButtonElement).disabled)).toBe(true);
  });

  it("6-10-i: 押せない側から識別した戻る方向・進む方向は onPageChange をちょうど1回、page との大小関係を満たす引数で呼ぶ", () => {
    // 境界での描画から、押せない側のインデックス（戻る方向＝backIndex、
    // 進む方向＝advanceIndex）を、表示文言を経由せずに求める。
    render(<Pagination page={1} pageCount={5} onPageChange={vi.fn()} />);
    const backIndex = findSoleDisabledButtonIndex();
    cleanup();

    render(<Pagination page={5} pageCount={5} onPageChange={vi.fn()} />);
    const advanceIndex = findSoleDisabledButtonIndex();
    cleanup();

    // 先頭でも末尾でもない page で描画する（10-6-j）。具体的なページ番号を
    // 期待値に持たず、描画に与えた page との大小関係だけを読む。
    const page = 3;
    const onPageChange = vi.fn();
    render(<Pagination page={page} pageCount={5} onPageChange={onPageChange} />);
    const buttons = getNamedButtons();

    fireEvent.click(buttons[backIndex]);
    expect(onPageChange).toHaveBeenCalledTimes(1);
    expect(onPageChange.mock.calls[0][0]).toBeLessThan(page);

    fireEvent.click(buttons[advanceIndex]);
    expect(onPageChange).toHaveBeenCalledTimes(2);
    expect(onPageChange.mock.calls[1][0]).toBeGreaterThan(page);
  });
});
