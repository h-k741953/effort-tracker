/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { Pagination } from "./pagination";

// docs/specs/design-system.md AC-6-10（検証手段は AC-10-6・10-6-f）。
//
// 「押せない形にする」は AC-7-3 が disabled の使用を明示的に許す唯一の
// 業務外の抑止（入力範囲外の抑止）であるため、disabled 属性で検証する。
// 10-6-f・AC-11-16 により、<nav> は空でないアクセシブル名を持つことを
// 読む。仕様が固定しないのは名の具体的な文字列であって、名を与えること
// 自体ではないため、name オプションで空でないことを検証する。

afterEach(() => {
  cleanup();
});

describe("Pagination - AC-6-10", () => {
  it("<nav> が空でないアクセシブル名を持つ", () => {
    render(<Pagination page={2} pageCount={5} onPageChange={vi.fn()} />);
    expect(screen.getByRole("navigation", { name: /\S/ })).toBeTruthy();
  });

  it("先頭ページ（page=1）で「前へ」を押せない形にする", () => {
    render(<Pagination page={1} pageCount={5} onPageChange={vi.fn()} />);
    const prev = screen.getByRole("button", { name: "前へ" });
    expect((prev as HTMLButtonElement).disabled).toBe(true);
  });

  it("末尾ページで「次へ」を押せない形にする", () => {
    render(<Pagination page={5} pageCount={5} onPageChange={vi.fn()} />);
    const next = screen.getByRole("button", { name: "次へ" });
    expect((next as HTMLButtonElement).disabled).toBe(true);
  });

  it("中間ページでは「前へ」「次へ」ともに押せる", () => {
    render(<Pagination page={3} pageCount={5} onPageChange={vi.fn()} />);
    const prev = screen.getByRole("button", { name: "前へ" });
    const next = screen.getByRole("button", { name: "次へ" });
    expect((prev as HTMLButtonElement).disabled).toBe(false);
    expect((next as HTMLButtonElement).disabled).toBe(false);
  });

  it("6-10-i: 「前へ」「次へ」は onPageChange をちょうど1回、page との大小関係を満たす引数で呼ぶ", () => {
    // 先頭でも末尾でもない page で描画する（10-6-j）。具体的なページ番号を
    // 期待値に持たず、描画に与えた page との大小関係だけを読む。
    const page = 3;
    const onPageChange = vi.fn();
    render(<Pagination page={page} pageCount={5} onPageChange={onPageChange} />);
    const prev = screen.getByRole("button", { name: "前へ" });
    const next = screen.getByRole("button", { name: "次へ" });

    fireEvent.click(prev);
    expect(onPageChange).toHaveBeenCalledTimes(1);
    expect(onPageChange.mock.calls[0][0]).toBeLessThan(page);

    fireEvent.click(next);
    expect(onPageChange).toHaveBeenCalledTimes(2);
    expect(onPageChange.mock.calls[1][0]).toBeGreaterThan(page);
  });
});
