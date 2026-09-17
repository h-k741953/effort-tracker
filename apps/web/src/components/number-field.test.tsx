/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { NumberField } from "./number-field";

// docs/specs/design-system.md AC-6-5（検証手段は AC-10-6）。
//
// min / max / step の既定値の内蔵有無は業務ルール側（AC-7-1）であり、
// AC-11-3「AC-7 は大部分が機械検査されない」の対象。ここでは AC-6 の表が
// 名指しした入出力（type="number"・label との結び付き・aria-invalid・
// aria-describedby）のみを検証する。

afterEach(() => {
  cleanup();
});

describe("NumberField - AC-6-5", () => {
  it("<input type='number'> を出力し、label と結びつく", () => {
    render(
      <NumberField id="hours" label="時間" value={5} onChange={vi.fn()} />,
    );
    const input = screen.getByLabelText("時間");
    expect(input.tagName).toBe("INPUT");
    expect(input.getAttribute("type")).toBe("number");
    expect(input.getAttribute("id")).toBe("hours");
  });

  it("invalid かつ errorId があるとき aria-invalid='true' と aria-describedby を付ける", () => {
    render(
      <NumberField
        id="minutes"
        label="分"
        value={30}
        onChange={vi.fn()}
        invalid
        errorId="minutes-error"
      />,
    );
    const input = screen.getByLabelText("分");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(input.getAttribute("aria-describedby")).toBe("minutes-error");
  });

  it("invalid が無いとき aria-invalid='true' を付けない", () => {
    render(
      <NumberField id="hours2" label="時間2" value={5} onChange={vi.fn()} />,
    );
    const input = screen.getByLabelText("時間2");
    expect(input.getAttribute("aria-invalid")).not.toBe("true");
  });

  it("6-5-i: 値が変わる操作で onChange がちょうど1回、空でない文字列は数値・空文字は空文字を引数に呼ばれる", () => {
    // min / max を与えない描画で行う（AC-7-1。値域を期待値に持ち込まない）。
    const onChange = vi.fn();
    render(
      <NumberField id="hours3" label="時間3" value={5} onChange={onChange} />,
    );
    const input = screen.getByLabelText("時間3");

    fireEvent.change(input, { target: { value: "7" } });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange.mock.calls[0][0]).toBe(7);

    fireEvent.change(input, { target: { value: "" } });
    expect(onChange).toHaveBeenCalledTimes(2);
    expect(onChange.mock.calls[1][0]).toBe("");
  });
});
