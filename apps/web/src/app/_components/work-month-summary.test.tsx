/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { hm, workMonthBody, type TestWorkMonthState } from "@/test-support/work-month";
import type { WorkMonthDetail } from "@/lib/work-month";
import { WorkMonthSummary } from "./work-month-summary";

// docs/specs/work-month-screen-ui.md 1-3〜1-5・4-1〜4-3・4-5・4-6。設計 M-5・M-11・T-3。
// 表現の例は test-support の workMonthBody（総稼働時間 16時間45分 = 16.75、精算幅 140.0〜180.0）。
// danger の印は text-danger のクラスで読む（design-system.md 2-3 が持つトークンの当て方）。

afterEach(cleanup);

function summary(options: { state: TestWorkMonthState; excess?: { hours: number; minutes: number } | null; shortfall?: { hours: number; minutes: number } | null; total?: { hours: number; minutes: number } }) {
  const body = workMonthBody({
    state: options.state,
    totalHours: options.total ?? hm(9, 30),
    excess: options.excess,
    shortfall: options.shortfall,
  });
  return render(<WorkMonthSummary workMonth={body as unknown as WorkMonthDetail} />);
}

/** 要素自身かその祖先に text-danger があるか。 */
const inDanger = (el: Element) => el.closest(".text-danger") !== null;

describe("Draft（4-1・4-5）", () => {
  it("総稼働時間と精算幅を出し、超過／不足は数値を出さず「締め時に確定します」を出す", () => {
    summary({ state: "Draft", total: hm(9, 30) });
    expect(screen.getByRole("heading", { name: "総稼働時間" })).toBeTruthy();
    expect(screen.getByText(/^\s*9\.5\s*(時間)?\s*$/)).toBeTruthy();
    expect(screen.getByRole("heading", { name: "精算幅" })).toBeTruthy();
    expect(screen.getByText(/140\.0〜180\.0/)).toBeTruthy();
    expect(screen.getByRole("heading", { name: "超過／不足" })).toBeTruthy();
    expect(screen.getByText("締め時に確定します")).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "超過" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "不足" })).toBeNull();
  });

  it("「締め時に確定します」は text-danger を持たない（4-6）", () => {
    const { container } = summary({ state: "Draft" });
    expect(inDanger(screen.getByText("締め時に確定します"))).toBe(false);
    expect(container.querySelector(".text-danger")).toBeNull();
  });
});

describe.each<TestWorkMonthState>(["PendingApproval", "Approved"])("%s（4-2・4-3・4-6）", (state) => {
  it("超過・不足の値が formatHours で出る。超過 0.25・不足 0.0 のとき、超過の数値と見出しだけが text-danger", () => {
    summary({ state, excess: hm(0, 15), shortfall: hm(0, 0), total: hm(9, 30) });

    const excessHeading = screen.getByRole("heading", { name: "超過" });
    const shortfallHeading = screen.getByRole("heading", { name: "不足" });
    const excessValue = screen.getByText(/^\s*0\.25\s*(時間)?\s*$/);
    const shortfallValue = screen.getByText(/^\s*0\.0\s*(時間)?\s*$/);

    expect(inDanger(excessHeading) || excessHeading.querySelector(".text-danger") !== null).toBe(true);
    expect(inDanger(excessValue)).toBe(true);
    expect(inDanger(shortfallHeading) || shortfallHeading.querySelector(".text-danger") !== null).toBe(false);
    expect(inDanger(shortfallValue)).toBe(false);
    // 総稼働時間・精算幅は赤にしない
    expect(inDanger(screen.getByRole("heading", { name: "総稼働時間" }))).toBe(false);
    expect(inDanger(screen.getByText(/^\s*9\.5\s*(時間)?\s*$/))).toBe(false);
    expect(inDanger(screen.getByRole("heading", { name: "精算幅" }))).toBe(false);
    expect(inDanger(screen.getByText(/140\.0〜180\.0/))).toBe(false);
  });

  it("超過 0.0・不足 0.5 のとき、不足の数値と見出しだけが text-danger", () => {
    summary({ state, excess: hm(0, 0), shortfall: hm(0, 30), total: hm(9, 30) });
    const shortfallHeading = screen.getByRole("heading", { name: "不足" });
    const excessHeading = screen.getByRole("heading", { name: "超過" });
    expect(inDanger(shortfallHeading) || shortfallHeading.querySelector(".text-danger") !== null).toBe(true);
    expect(inDanger(screen.getByText(/^\s*0\.5\s*(時間)?\s*$/))).toBe(true);
    expect(inDanger(excessHeading) || excessHeading.querySelector(".text-danger") !== null).toBe(false);
    expect(inDanger(screen.getByText(/^\s*0\.0\s*(時間)?\s*$/))).toBe(false);
  });

  it("超過も不足も 0 なら text-danger を持つ要素が無い（精算幅に収まった月）", () => {
    const { container } = summary({ state, excess: hm(0, 0), shortfall: hm(0, 0) });
    expect(screen.getByRole("heading", { name: "超過" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "不足" })).toBeTruthy();
    expect(container.querySelector(".text-danger")).toBeNull();
  });

  it("「締め時に確定します」は出さない", () => {
    summary({ state, excess: hm(0, 0), shortfall: hm(0, 0) });
    expect(screen.queryByText("締め時に確定します")).toBeNull();
  });
});
