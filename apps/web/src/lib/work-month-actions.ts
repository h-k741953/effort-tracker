// docs/specs/web-screens-implementation-design.md M-4・M-5・M-11。純粋な関数。
// 出し分けは画面の表示であり認可ではない（弾く判定はドメイン API が持つ）。

import type { Role } from "./role-cookie";
import type { Ownership } from "./work-month";
import type { WorkMonthState } from "./work-month-list";

export type Operator = { kind: "guest" } | { kind: "signed-in"; role: Role; ownership: Ownership };
export interface WorkMonthActions {
  edit: boolean;
  close: boolean;
  approve: boolean;
  reject: boolean;
}

const NONE: WorkMonthActions = { edit: false, close: false, approve: false, reject: false };

export function workMonthActions(params: {
  state: WorkMonthState;
  generated: boolean;
  operator: Operator;
}): WorkMonthActions {
  const { state, generated, operator } = params;
  if (operator.kind === "guest" || operator.ownership === "unknown") return NONE;
  const own = operator.ownership === "own";
  if (state === "Draft" && own) return { edit: true, close: generated, approve: false, reject: false };
  if (state === "PendingApproval" && operator.role === "Approver" && !own) {
    return { edit: false, close: false, approve: true, reject: true };
  }
  return NONE;
}

export function workMonthNote(params: {
  state: WorkMonthState;
  operator: Operator;
  actions: WorkMonthActions;
  justClosed: boolean;
}): string {
  const { state, operator, actions, justClosed } = params;
  if (actions.edit) return "下書きの勤務月です。カレンダーの日を選んで、開始・終了・休憩を入力してください。";
  if (justClosed && state === "PendingApproval") return "締めました。承認されるまで稼働実績は編集できません。";
  if (operator.kind === "guest") return "ゲストとして閲覧しています。";
  if (state === "Approved") return "承認済の勤務月です。";
  if (actions.approve) return "技術者が締めた勤務月です。";
  if (operator.role === "Approver" && operator.ownership === "own") {
    return "ご自身の勤務月のため、承認・差戻しはできません。";
  }
  if (state === "PendingApproval") return "締め済のため編集できません。";
  return "下書きの勤務月です。";
}
