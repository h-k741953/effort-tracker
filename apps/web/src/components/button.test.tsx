/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { Button } from "./button";

// docs/specs/design-system.md AC-6-2（検証手段は AC-10-5・AC-10-6）。

afterEach(() => {
  cleanup();
});

describe("Button - AC-6-2", () => {
  it("<button> を出力し、type の既定は 'button' である", () => {
    render(<Button>送信</Button>);
    const button = screen.getByRole("button", { name: "送信" });
    expect(button.tagName).toBe("BUTTON");
    expect(button.getAttribute("type")).toBe("button");
  });

  it("children をそのまま表示する", () => {
    render(<Button>保存する</Button>);
    expect(screen.getByText("保存する")).toBeTruthy();
  });

  it("button の標準属性（onClick 等）を透過する", () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>押す</Button>);
    screen.getByRole("button", { name: "押す" }).click();
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  // AC-10-5: variant は primary / secondary / danger の3値のみ。
  it("型: variant は primary / secondary / danger の3値のみである（AC-10-5）", () => {
    // @ts-expect-error variant は3値のリテラルユニオンのみ許可される
    const invalid = <Button variant="tertiary">x</Button>;
    void invalid;
  });
});
