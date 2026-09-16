/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { Pagination } from "./pagination";

// docs/specs/design-system.md AC-6-10（検証手段は AC-10-6）。
//
// 「押せない形にする」は AC-7-3 が disabled の使用を明示的に許す唯一の
// 業務外の抑止（入力範囲外の抑止）であるため、disabled 属性で検証する。
// nav のアクセシブル名の具体的な文字列は仕様が指定しないため検証しない
// （AC-11-7）。

afterEach(() => {
  cleanup();
});

describe("Pagination - AC-6-10", () => {
  it("<nav> を出力する", () => {
    render(<Pagination page={2} pageCount={5} onPageChange={vi.fn()} />);
    expect(screen.getByRole("navigation")).toBeTruthy();
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
});
