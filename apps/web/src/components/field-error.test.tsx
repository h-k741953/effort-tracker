/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { FieldError } from "./field-error";

// docs/specs/design-system.md AC-6-6（検証手段は AC-10-6）。

afterEach(() => {
  cleanup();
});

describe("FieldError - AC-6-6", () => {
  it("id を持ち role='alert' で children を出力する", () => {
    render(<FieldError id="hours-error">必須です</FieldError>);
    const alert = screen.getByRole("alert");
    expect(alert.getAttribute("id")).toBe("hours-error");
    expect(alert.textContent).toBe("必須です");
  });
});
