import { describe, expect, it } from "vitest";
import type { WorkMonthState } from "./work-month-list";
import { workMonthActions, workMonthNote, type Operator, type WorkMonthActions } from "./work-month-actions";

// docs/specs/work-month-screen-ui.md AC-3（全行）・5-1・5-2、monthly-closing-ui.md 1-1〜1-3・4-1・5-1・5-2、
// approval-ui.md 1-1〜1-4・3-1・3-2、domain-api-http-contract.md AC-6 の申し送り。
// 設計 M-4・M-5・M-11・T-3。M-4 の表をそのまま入出力にする。

const NONE: WorkMonthActions = { edit: false, close: false, approve: false, reject: false };

const GUEST: Operator = { kind: "guest" };
const ENGINEER_OWN: Operator = { kind: "signed-in", role: "Engineer", ownership: "own" }; // ①
const ENGINEER_OTHER: Operator = { kind: "signed-in", role: "Engineer", ownership: "other" }; // ②
const APPROVER_OTHER: Operator = { kind: "signed-in", role: "Approver", ownership: "other" }; // ③
const APPROVER_OWN: Operator = { kind: "signed-in", role: "Approver", ownership: "own" }; // ④
const ENGINEER_UNKNOWN: Operator = { kind: "signed-in", role: "Engineer", ownership: "unknown" };
const APPROVER_UNKNOWN: Operator = { kind: "signed-in", role: "Approver", ownership: "unknown" };

const STATES: WorkMonthState[] = ["Draft", "PendingApproval", "Approved"];

// M-4 の表。edit・approve・reject はそのまま、close は「generated が真のときだけ」を closeNeedsGenerated で表す。
interface Expect {
  edit: boolean;
  closeNeedsGenerated: boolean;
  approve: boolean;
  reject: boolean;
}
const OFF: Expect = { edit: false, closeNeedsGenerated: false, approve: false, reject: false };
const EDIT_AND_CLOSE: Expect = { edit: true, closeNeedsGenerated: true, approve: false, reject: false };
const REVIEW: Expect = { edit: false, closeNeedsGenerated: false, approve: true, reject: true };

const MATRIX: Array<[string, Operator, Record<WorkMonthState, Expect>]> = [
  ["⑤ ゲスト", GUEST, { Draft: OFF, PendingApproval: OFF, Approved: OFF }],
  ["Engineer・本人か不明", ENGINEER_UNKNOWN, { Draft: OFF, PendingApproval: OFF, Approved: OFF }],
  ["Approver・本人か不明", APPROVER_UNKNOWN, { Draft: OFF, PendingApproval: OFF, Approved: OFF }],
  ["① 技術者本人", ENGINEER_OWN, { Draft: EDIT_AND_CLOSE, PendingApproval: OFF, Approved: OFF }],
  ["② 他の技術者", ENGINEER_OTHER, { Draft: OFF, PendingApproval: OFF, Approved: OFF }],
  ["③ 承認者（本人でない）", APPROVER_OTHER, { Draft: OFF, PendingApproval: REVIEW, Approved: OFF }],
  ["④ 承認者かつ本人", APPROVER_OWN, { Draft: EDIT_AND_CLOSE, PendingApproval: OFF, Approved: OFF }],
];

describe("workMonthActions — M-4 の表（AC-3 のマトリクス）", () => {
  const cases = MATRIX.flatMap(([name, operator, byState]) =>
    STATES.flatMap((state) =>
      [true, false].map((generated) => ({
        title: `${name} × ${state} × generated=${generated}`,
        operator,
        state,
        generated,
        want: {
          edit: byState[state].edit,
          close: byState[state].closeNeedsGenerated && generated,
          approve: byState[state].approve,
          reject: byState[state].reject,
        } satisfies WorkMonthActions,
      })),
    ),
  );

  it("母集団: 7種の操作者 × 3状態 × generated 2値 = 42組を読む", () => {
    expect(cases).toHaveLength(42);
  });

  it.each(cases.map((c) => [c.title, c] as const))("%s", (_title, c) => {
    expect(workMonthActions({ state: c.state, generated: c.generated, operator: c.operator })).toEqual(c.want);
  });
});

describe("workMonthActions — 締めの提供条件（契約 AC-6 の申し送り・締め UI 5-1・5-2）", () => {
  it("未生成（generated=false）の Draft: ①④とも close は偽で、edit は真（初回の入力で暗黙生成される）", () => {
    for (const operator of [ENGINEER_OWN, APPROVER_OWN]) {
      expect(workMonthActions({ state: "Draft", generated: false, operator })).toEqual({
        edit: true,
        close: false,
        approve: false,
        reject: false,
      });
    }
  });

  it("空月（generated=true）の Draft: ①④とも close は真（実績0件を理由に妨げない）", () => {
    for (const operator of [ENGINEER_OWN, APPROVER_OWN]) {
      expect(workMonthActions({ state: "Draft", generated: true, operator }).close).toBe(true);
    }
  });
});

describe("workMonthNote — M-5 の注記（この順に最初に当たったもの）", () => {
  const EDIT: WorkMonthActions = { ...NONE, edit: true, close: true };
  const REVIEW_ACTIONS: WorkMonthActions = { ...NONE, approve: true, reject: true };

  const cases: Array<[string, Parameters<typeof workMonthNote>[0], string]> = [
    [
      "編集できる",
      { state: "Draft", operator: ENGINEER_OWN, actions: EDIT, justClosed: false },
      "下書きの勤務月です。カレンダーの日を選んで、開始・終了・休憩を入力してください。",
    ],
    [
      "締めた直後で PendingApproval",
      { state: "PendingApproval", operator: ENGINEER_OWN, actions: NONE, justClosed: true },
      "締めました。承認されるまで稼働実績は編集できません。",
    ],
    [
      "ゲスト",
      { state: "Draft", operator: GUEST, actions: NONE, justClosed: false },
      "ゲストとして閲覧しています。",
    ],
    [
      "Approved",
      { state: "Approved", operator: ENGINEER_OWN, actions: NONE, justClosed: false },
      "承認済の勤務月です。",
    ],
    [
      "承認を出す",
      { state: "PendingApproval", operator: APPROVER_OTHER, actions: REVIEW_ACTIONS, justClosed: false },
      "技術者が締めた勤務月です。",
    ],
    [
      "Approver・本人（PendingApproval）",
      { state: "PendingApproval", operator: APPROVER_OWN, actions: NONE, justClosed: false },
      "ご自身の勤務月のため、承認・差戻しはできません。",
    ],
    [
      "PendingApproval（技術者本人）",
      { state: "PendingApproval", operator: ENGINEER_OWN, actions: NONE, justClosed: false },
      "締め済のため編集できません。",
    ],
    [
      "それ以外（Draft・他の技術者）",
      { state: "Draft", operator: ENGINEER_OTHER, actions: NONE, justClosed: false },
      "下書きの勤務月です。",
    ],
  ];

  it.each(cases)("%s", (_name, params, want) => {
    expect(workMonthNote(params)).toBe(want);
  });

  it("順の優先: ゲストで Approved → ゲストの文（Approved の文より先）", () => {
    expect(workMonthNote({ state: "Approved", operator: GUEST, actions: NONE, justClosed: false })).toBe(
      "ゲストとして閲覧しています。",
    );
  });

  it("順の優先: 締めた直後で PendingApproval → 「締めました。…」（「締め済のため編集できません。」より先）", () => {
    expect(
      workMonthNote({ state: "PendingApproval", operator: ENGINEER_OWN, actions: NONE, justClosed: true }),
    ).toBe("締めました。承認されるまで稼働実績は編集できません。");
  });

  it("締めた直後でも PendingApproval でなければ「締めました」を出さない", () => {
    expect(workMonthNote({ state: "Draft", operator: ENGINEER_OTHER, actions: NONE, justClosed: true })).toBe(
      "下書きの勤務月です。",
    );
  });

  it("8つの文は互いに異なる", () => {
    const sentences = cases.map(([, params]) => workMonthNote(params));
    expect(new Set(sentences).size).toBe(8);
  });
});
