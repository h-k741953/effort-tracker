// docs/specs/web-screens-implementation-design.md M-2・M-11。ブラウザで動く。
// R-1・R-5〜R-9 を BFF の Route Handler へ送り、結果を分類する。

import type { Break, TimeOfDay } from "./working-hours";
import { readOwnership, type Ownership, type WorkMonthDetail } from "./work-month";

export type WorkMonthResult =
  | { kind: "ok"; value: WorkMonthDetail; ownership: Ownership }
  | { kind: "refresh" }
  | { kind: "rejected"; status: number; code: string }
  | { kind: "failed" };

export interface DailyRecordInput {
  startTime: TimeOfDay;
  endTime: TimeOfDay;
  break: Break | null;
}

const REJECTED_STATUSES = [400, 403, 404, 409];

export function workMonthApiPath(contractId: string, yearMonth: string): string {
  return `/api/work-months/${encodeURIComponent(contractId)}/${encodeURIComponent(yearMonth)}`;
}

function toDetail(body: Record<string, unknown>): WorkMonthDetail {
  return {
    contractId: body.contractId as string,
    contractDisplayName: body.contractDisplayName as string,
    yearMonth: body.yearMonth as string,
    state: body.state as WorkMonthDetail["state"],
    generated: body.generated as boolean,
    settlementRange: body.settlementRange as WorkMonthDetail["settlementRange"],
    totalHours: body.totalHours as WorkMonthDetail["totalHours"],
    excess: body.excess as WorkMonthDetail["excess"],
    shortfall: body.shortfall as WorkMonthDetail["shortfall"],
    dailyRecords: body.dailyRecords as WorkMonthDetail["dailyRecords"],
  };
}

function errorCode(body: unknown): string | undefined {
  if (typeof body !== "object" || body === null) return undefined;
  const error = (body as { error?: unknown }).error;
  if (typeof error !== "object" || error === null) return undefined;
  const code = (error as { code?: unknown }).code;
  return typeof code === "string" ? code : undefined;
}

async function send(
  url: string,
  init: RequestInit,
  fetchImpl: typeof fetch | undefined,
): Promise<WorkMonthResult> {
  try {
    const impl = fetchImpl ?? globalThis.fetch;
    const response = await impl(url, init);
    if (response.status === 401) return { kind: "refresh" };
    if (response.status === 200) {
      const body: unknown = await response.json();
      if (
        typeof body !== "object" ||
        body === null ||
        typeof (body as { contractId?: unknown }).contractId !== "string" ||
        !Array.isArray((body as { dailyRecords?: unknown }).dailyRecords)
      ) {
        return { kind: "failed" };
      }
      return {
        kind: "ok",
        value: toDetail(body as Record<string, unknown>),
        ownership: readOwnership(body),
      };
    }
    if (REJECTED_STATUSES.includes(response.status)) {
      const code = errorCode(await response.json());
      if (code !== undefined) return { kind: "rejected", status: response.status, code };
    }
    return { kind: "failed" };
  } catch {
    return { kind: "failed" };
  }
}

function dayPath(contractId: string, yearMonth: string, date: string): string {
  return `${workMonthApiPath(contractId, yearMonth)}/daily-records/${encodeURIComponent(date)}`;
}

export function fetchWorkMonth(contractId: string, yearMonth: string, fetchImpl?: typeof fetch) {
  return send(workMonthApiPath(contractId, yearMonth), { method: "GET" }, fetchImpl);
}

export function saveDailyRecord(
  contractId: string,
  yearMonth: string,
  date: string,
  input: DailyRecordInput,
  fetchImpl?: typeof fetch,
) {
  return send(
    dayPath(contractId, yearMonth, date),
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ startTime: input.startTime, endTime: input.endTime, break: input.break }),
    },
    fetchImpl,
  );
}

export function deleteDailyRecord(contractId: string, yearMonth: string, date: string, fetchImpl?: typeof fetch) {
  return send(dayPath(contractId, yearMonth, date), { method: "DELETE" }, fetchImpl);
}

function post(action: string) {
  return (contractId: string, yearMonth: string, fetchImpl?: typeof fetch) =>
    send(`${workMonthApiPath(contractId, yearMonth)}/${action}`, { method: "POST" }, fetchImpl);
}

export const closeWorkMonth = post("close");
export const approveWorkMonth = post("approve");
export const rejectWorkMonth = post("reject");
