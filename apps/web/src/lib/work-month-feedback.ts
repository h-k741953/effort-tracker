// docs/specs/web-screens-implementation-design.md M-9・M-11。純粋な関数と文言。
// 文言の字面は AI 起案（人間の承認前）。

import type { WorkMonthResult } from "./work-month-client";

export type OperationFailureKind = "invalid-input" | "not-allowed" | "state-changed" | "communication";
export type LoadFailureKind = "missing" | "failed";
type NotOk = Extract<WorkMonthResult, { kind: "rejected" } | { kind: "failed" }>;

export const OPERATION_FAILURE_MESSAGES: Readonly<Record<OperationFailureKind, string>> = {
  "invalid-input": "入力した内容を受け付けられませんでした。内容を見直して、もう一度保存してください。",
  "not-allowed": "この勤務月では、この操作はできません。",
  "state-changed": "勤務月の状態が変わっていたため、最新の内容に更新しました。",
  communication: "通信に失敗しました。時間をおいて、もう一度お試しください。",
};

export function operationFailureKind(result: NotOk): OperationFailureKind {
  if (result.kind === "failed") return "communication";
  if (result.status === 400) return "invalid-input";
  if (result.status === 409) return "state-changed";
  return "not-allowed";
}

export function loadFailureKind(result: NotOk): LoadFailureKind {
  if (result.kind === "rejected" && (result.status === 404 || result.status === 400)) return "missing";
  return "failed";
}

export const WORK_MONTH_LOAD_ERROR_MESSAGE =
  "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";

export const WORK_MONTH_MISSING: Readonly<{ title: string; description: string; linkLabel: string }> = {
  title: "勤務月を表示できません",
  description: "契約が見つからないか、URL の形式が正しくありません。",
  linkLabel: "勤務月の一覧を開く",
};
