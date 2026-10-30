import { describe, expect, it } from "vitest";
import { TIME_ENTRY_MESSAGES } from "./time-entry";
import {
  OPERATION_FAILURE_MESSAGES,
  WORK_MONTH_LOAD_ERROR_MESSAGE,
  WORK_MONTH_MISSING,
  loadFailureKind,
  operationFailureKind,
  type OperationFailureKind,
} from "./work-month-feedback";

// docs/specs/work-month-screen-ui.md AC-6、daily-record-entry-ui.md 4-1-a・4-3・4-9、monthly-closing-ui.md 6-1、
// approval-ui.md 10-8。設計 M-9・M-11・T-3。M-9 の2つの表を入出力にする。
// 文言は M-9（AI 起案・未承認）の字面をテストに書く。人間が改めたら、実装とここの字面を改める。

const rejected = (status: number, code: string) => ({ kind: "rejected", status, code }) as const;
const FAILED = { kind: "failed" } as const;

describe("operationFailureKind（M-9 の操作の表）", () => {
  it.each([
    [400, "TIME_OF_DAY_OUT_OF_RANGE"],
    [400, "INVALID_TIME_COMBINATION"],
    [400, "FUTURE_DATE_NOT_ALLOWED"],
    [400, "DATE_OUT_OF_WORK_MONTH"],
    [400, "INVALID_REQUEST"],
    [400, "ANY_OTHER_CODE"],
  ])("rejected %i（%s）→ invalid-input（code を問わない）", (status, code) => {
    expect(operationFailureKind(rejected(status, code))).toBe("invalid-input");
  });

  it.each([
    [403, "FORBIDDEN_NOT_OWNER"],
    [403, "ANY_OTHER_CODE"],
    [404, "CONTRACT_NOT_FOUND"],
    [404, "ANY_OTHER_CODE"],
  ])("rejected %i（%s）→ not-allowed（code を問わない）", (status, code) => {
    expect(operationFailureKind(rejected(status, code))).toBe("not-allowed");
  });

  it.each([
    [409, "STATE_CONFLICT"],
    [409, "ANY_OTHER_CODE"],
  ])("rejected %i（%s）→ state-changed（code を問わない）", (status, code) => {
    expect(operationFailureKind(rejected(status, code))).toBe("state-changed");
  });

  it("failed → communication", () => {
    expect(operationFailureKind(FAILED)).toBe("communication");
  });
});

describe("loadFailureKind（M-9 の取得の表）", () => {
  it.each([
    [404, "CONTRACT_NOT_FOUND"],
    [404, "ANY_OTHER_CODE"],
    [400, "INVALID_REQUEST"],
    [400, "ANY_OTHER_CODE"],
  ])("rejected %i（%s）→ missing", (status, code) => {
    expect(loadFailureKind(rejected(status, code))).toBe("missing");
  });

  it.each([
    [403, "FORBIDDEN_NOT_OWNER"],
    [409, "STATE_CONFLICT"],
  ])("rejected %i（%s）→ failed", (status, code) => {
    expect(loadFailureKind(rejected(status, code))).toBe("failed");
  });

  it("failed → failed", () => {
    expect(loadFailureKind(FAILED)).toBe("failed");
  });
});

describe("文言（M-9）", () => {
  const EXPECTED: Record<OperationFailureKind, string> = {
    "invalid-input": "入力した内容を受け付けられませんでした。内容を見直して、もう一度保存してください。",
    "not-allowed": "この勤務月では、この操作はできません。",
    "state-changed": "勤務月の状態が変わっていたため、最新の内容に更新しました。",
    communication: "通信に失敗しました。時間をおいて、もう一度お試しください。",
  };

  it("OPERATION_FAILURE_MESSAGES の4つの文が M-9 の字面と一致する", () => {
    expect(OPERATION_FAILURE_MESSAGES).toEqual(EXPECTED);
  });

  it("4つの文は互いに異なる", () => {
    expect(new Set(Object.values(OPERATION_FAILURE_MESSAGES)).size).toBe(4);
  });

  it("4つの文は TIME_ENTRY_MESSAGES（M-7）の7つの文のどれとも異なる（入力 UI 4-9）", () => {
    const timeEntry = Object.values(TIME_ENTRY_MESSAGES);
    expect(timeEntry).toHaveLength(7);
    for (const message of Object.values(OPERATION_FAILURE_MESSAGES)) {
      expect(timeEntry).not.toContain(message);
    }
  });

  it("WORK_MONTH_LOAD_ERROR_MESSAGE（W-4 の取得エラーの文言と同じ字面）", () => {
    expect(WORK_MONTH_LOAD_ERROR_MESSAGE).toBe(
      "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。",
    );
  });

  it("WORK_MONTH_MISSING の title・description・linkLabel", () => {
    expect(WORK_MONTH_MISSING).toEqual({
      title: "勤務月を表示できません",
      description: "契約が見つからないか、URL の形式が正しくありません。",
      linkLabel: "勤務月の一覧を開く",
    });
  });
});
