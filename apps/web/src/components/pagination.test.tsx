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

afterEach(() => {
  cleanup();
});

/**
 * 直前の描画から、ちょうど1つだけ disabled になっているボタンの
 * インデックスを返す（6-10-i: 識別手段は表示文言に依存させない）。
 * ちょうど1つに定まらない場合は、境界での抑止（AC-6-10）が満たされて
 * いないことを示すため、ここで失敗させる。
 */
function findSoleDisabledButtonIndex(): number {
  const buttons = screen.getAllByRole("button");
  const disabledIndexes = buttons
    .map((button, index) => ({ index, disabled: (button as HTMLButtonElement).disabled }))
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

  it("先頭ページ（page=1）では、戻る方向がちょうど1つに定まる形で押せない", () => {
    render(<Pagination page={1} pageCount={5} onPageChange={vi.fn()} />);
    expect(() => findSoleDisabledButtonIndex()).not.toThrow();
  });

  it("末尾ページでは、進む方向がちょうど1つに定まる形で押せない", () => {
    render(<Pagination page={5} pageCount={5} onPageChange={vi.fn()} />);
    expect(() => findSoleDisabledButtonIndex()).not.toThrow();
  });

  it("先頭で押せない側と末尾で押せない側は互いに異なる", () => {
    render(<Pagination page={1} pageCount={5} onPageChange={vi.fn()} />);
    const backIndex = findSoleDisabledButtonIndex();
    cleanup();

    render(<Pagination page={5} pageCount={5} onPageChange={vi.fn()} />);
    const advanceIndex = findSoleDisabledButtonIndex();

    expect(advanceIndex).not.toBe(backIndex);
  });

  it("中間ページでは戻る方向・進む方向のいずれも押せる", () => {
    render(<Pagination page={3} pageCount={5} onPageChange={vi.fn()} />);
    const buttons = screen.getAllByRole("button");
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
    const buttons = screen.getAllByRole("button");

    fireEvent.click(buttons[backIndex]);
    expect(onPageChange).toHaveBeenCalledTimes(1);
    expect(onPageChange.mock.calls[0][0]).toBeLessThan(page);

    fireEvent.click(buttons[advanceIndex]);
    expect(onPageChange).toHaveBeenCalledTimes(2);
    expect(onPageChange.mock.calls[1][0]).toBeGreaterThan(page);
  });
});
