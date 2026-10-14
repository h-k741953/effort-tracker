/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, cleanup, fireEvent, within } from "@testing-library/react";
import type { ComponentProps } from "react";
import { SelectField } from "./select-field";

// docs/specs/design-system.md AC-6-12（6-12-i〜6-12-v。検証手段は AC-10-6・10-6-o・10-6-p）。
//
// 6-12-i は options を2つ以上与えた描画、placeholder を与えた描画と与えない描画、options を空に
// して placeholder を与えない描画の4つで読む。6-12-ii は fireEvent.change で選択を変え、onChange が
// 1回だけ呼ばれ、引数が選んだ選択肢の value の文字列と型まで一致することを読む（数値に見える値
// "9" を与え、引数が文字列 "9" であること）。6-12-iii は 10-6-o の読み方（選択の要素の labels を読む。
// aria-label だけでの結び付きを通さない）で、hideLabel の真・偽の両方で読む。期待値（選択肢の
// 値と表示）はテストが与えた options から取り、時・分の値域を期待値に持たない（AC-7-1）。

afterEach(() => {
  cleanup();
});

const OPTIONS = [
  { value: "9", label: "9" },
  { value: "10", label: "10" },
  { value: "11", label: "11" },
] as const;

function renderField(props: Partial<ComponentProps<typeof SelectField>> = {}) {
  const onChange = vi.fn();
  render(
    <SelectField id="start-hour" label="開始（時）" value="10" options={OPTIONS} onChange={onChange} {...props} />,
  );
  return { onChange, select: screen.getByRole("combobox") as HTMLSelectElement };
}

describe("SelectField - AC-6-12", () => {
  it("<select> を出力し、id を持つ", () => {
    const { select } = renderField();
    expect(select.tagName).toBe("SELECT");
    expect(select.id).toBe("start-hour");
  });

  describe("6-12-i: 選択肢", () => {
    it("options と同じ順で同じ表示（と値）の option が並ぶ（placeholder なし）", () => {
      const { select } = renderField();
      const rendered = within(select).getAllByRole("option");
      expect(rendered.map((o) => [(o as HTMLOptionElement).value, o.textContent])).toEqual(
        OPTIONS.map((o) => [o.value, o.label]),
      );
    });

    it("options の値と表示が異なっても、与えたとおりに出る（部品が値域を内蔵しない）", () => {
      const { select } = renderField({
        value: "a",
        options: [
          { value: "a", label: "あ" },
          { value: "b", label: "い" },
        ],
      });
      expect(
        within(select)
          .getAllByRole("option")
          .map((o) => [(o as HTMLOptionElement).value, o.textContent]),
      ).toEqual([
        ["a", "あ"],
        ["b", "い"],
      ]);
    });

    it("placeholder を与えると、先頭に値が空文字で表示が placeholder の選択肢がちょうど1つ加わる", () => {
      const { select } = renderField({ placeholder: "選択してください", value: "" });
      const rendered = within(select).getAllByRole("option") as HTMLOptionElement[];
      expect(rendered).toHaveLength(OPTIONS.length + 1);
      expect([rendered[0].value, rendered[0].textContent]).toEqual(["", "選択してください"]);
      expect(rendered.slice(1).map((o) => [o.value, o.textContent])).toEqual(OPTIONS.map((o) => [o.value, o.label]));
      expect(rendered.filter((o) => o.value === "")).toHaveLength(1);
    });

    it("placeholder を与えないと、値が空文字の選択肢は加わらない", () => {
      const { select } = renderField();
      expect(within(select).getAllByRole("option")).toHaveLength(OPTIONS.length);
      expect((within(select).getAllByRole("option") as HTMLOptionElement[]).some((o) => o.value === "")).toBe(false);
    });

    it("options を空にして placeholder を与えないとき、選択肢が0個である（部品が選択肢を内蔵しない）", () => {
      const { select } = renderField({ options: [], value: "" });
      expect(within(select).queryAllByRole("option")).toHaveLength(0);
    });

    it("options を空にして placeholder だけを与えると、選択肢は placeholder の1つだけである", () => {
      const { select } = renderField({ options: [], value: "", placeholder: "なし" });
      const rendered = within(select).getAllByRole("option") as HTMLOptionElement[];
      expect(rendered.map((o) => [o.value, o.textContent])).toEqual([["", "なし"]]);
    });
  });

  describe("6-12-ii: onChange", () => {
    it("選択の変化を発火させると onChange がちょうど1回、選んだ選択肢の value の文字列そのものを引数に呼ばれる", () => {
      const { onChange, select } = renderField({ value: "10" });
      fireEvent.change(select, { target: { value: "9" } });
      expect(onChange).toHaveBeenCalledTimes(1);
      expect(onChange.mock.calls[0]).toEqual(["9"]);
      // 数値へ変換しない（型まで一致する）。
      expect(typeof onChange.mock.calls[0][0]).toBe("string");
    });

    it("別の選択肢を選んでも、その value の文字列を渡す", () => {
      const { onChange, select } = renderField({ value: "9" });
      fireEvent.change(select, { target: { value: "11" } });
      expect(onChange).toHaveBeenCalledTimes(1);
      expect(onChange.mock.calls[0]).toEqual(["11"]);
    });

    it("placeholder の選択肢を選んだときの引数は空文字である", () => {
      const { onChange, select } = renderField({ value: "10", placeholder: "選択してください" });
      fireEvent.change(select, { target: { value: "" } });
      expect(onChange).toHaveBeenCalledTimes(1);
      expect(onChange.mock.calls[0]).toEqual([""]);
    });
  });

  describe("6-12-iii: label と選択の要素の結び付き（10-6-o。<label> 要素との結び付きとして読む）", () => {
    it.each([
      { hideLabel: false, name: "hideLabel が偽" },
      { hideLabel: true, name: "hideLabel が真（視覚的に隠すが DOM には残す）" },
    ])("$name のとき、select の labels のテキストが label と一致する", ({ hideLabel }) => {
      const { select } = renderField({ hideLabel });
      const labels = Array.from(select.labels ?? []);
      expect(labels.map((l) => l.tagName)).toEqual(["LABEL"]);
      expect(labels.map((l) => l.textContent)).toEqual(["開始（時）"]);
    });

    it("hideLabel を省略したときも labels のテキストが label と一致する", () => {
      const { select } = renderField();
      expect(Array.from(select.labels ?? []).map((l) => l.textContent)).toEqual(["開始（時）"]);
    });

    it("hideLabel が真でも、label がアクセシブル名として読まれる", () => {
      renderField({ hideLabel: true });
      expect(screen.getByRole("combobox", { name: "開始（時）" })).toBeTruthy();
    });

    it("label の for が select の id を指す", () => {
      const { select } = renderField();
      const [label] = Array.from(select.labels ?? []);
      expect(label.getAttribute("for")).toBe("start-hour");
    });
  });

  describe("6-12-iv: 現在値", () => {
    it.each(["9", "10", "11"])("value=%s のとき、選択の要素の値が value と一致する", (value) => {
      const { select } = renderField({ value });
      expect(select.value).toBe(value);
    });

    it("placeholder を与えたとき、value が空文字なら選択の要素の値が空文字である", () => {
      const { select } = renderField({ value: "", placeholder: "選択してください" });
      expect(select.value).toBe("");
    });
  });

  describe("6-12-v: 不正表示", () => {
    it("invalid が真のとき aria-invalid=\"true\" を付ける", () => {
      const { select } = renderField({ invalid: true });
      expect(select.getAttribute("aria-invalid")).toBe("true");
    });

    it("invalid が偽・省略のとき aria-invalid を付けない", () => {
      cleanup();
      expect(renderField({ invalid: false }).select.hasAttribute("aria-invalid")).toBe(false);
      cleanup();
      expect(renderField().select.hasAttribute("aria-invalid")).toBe(false);
    });

    it("errorId を与えたとき aria-describedby がその値を含む", () => {
      const { select } = renderField({ invalid: true, errorId: "start-hour-error" });
      expect((select.getAttribute("aria-describedby") ?? "").split(/\s+/)).toContain("start-hour-error");
    });
  });

  // AC-10-5 / AC-7-2: 表に無い props（role・勤務月の状態を受けて可否を判定する props）を受け取らない。
  it("型: 表に無い props を受け取らない（AC-6 の網羅）", () => {
    // @ts-expect-error role は公開 props に無い（AC-7-2）
    const withRole = <SelectField id="a" label="a" value="" options={[]} onChange={vi.fn()} role="Engineer" />;
    // @ts-expect-error value は文字列（数値を受け取らない。値を数値へ変換しない）
    const withNumber = <SelectField id="a" label="a" value={9} options={[]} onChange={vi.fn()} />;
    void withRole;
    void withNumber;
  });
});
