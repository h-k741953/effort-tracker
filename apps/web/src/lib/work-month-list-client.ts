// docs/specs/web-screens-implementation-design.md W-2・W-3・W-8。ブラウザで動く。
// 一覧の取得は BFF の Route Handler への GET だけで、本文を持たない。

import { currentJstYearMonth } from "./jst-month";
import type { ContractSummary, WorkMonthRow } from "./work-month-list";

export type FetchResult<T> = { kind: "ok"; value: T } | { kind: "refresh" } | { kind: "failed" };

type Page = { items: unknown[]; total: number };

const OWN_PATH = "/api/work-months/own";
const PENDING_PATH = "/api/work-months/pending-approval";
const DEMO_PATH = "/api/work-months/demo";
const CONTRACTS_PATH = "/api/contracts";

async function getJson(url: string, fetchImpl: typeof fetch): Promise<FetchResult<Record<string, unknown>>> {
  try {
    const response = await fetchImpl(url, { method: "GET" });
    if (response.status === 401 || response.status === 403) return { kind: "refresh" };
    if (response.status !== 200) return { kind: "failed" };
    const body: unknown = await response.json();
    if (typeof body !== "object" || body === null || !Array.isArray((body as { items?: unknown }).items)) {
      return { kind: "failed" };
    }
    return { kind: "ok", value: body as Record<string, unknown> };
  } catch {
    return { kind: "failed" };
  }
}

async function getPage(url: string, fetchImpl: typeof fetch): Promise<FetchResult<Page>> {
  const result = await getJson(url, fetchImpl);
  if (result.kind !== "ok") return result;
  const total = result.value.total;
  return {
    kind: "ok",
    value: { items: result.value.items as unknown[], total: typeof total === "number" ? total : 0 },
  };
}

export async function fetchAllWorkMonths(
  path: string,
  fetchImpl: typeof fetch = fetch,
): Promise<FetchResult<WorkMonthRow[]>> {
  const rows: WorkMonthRow[] = [];
  const seen = new Set<string>();
  let offset = 0;
  for (;;) {
    const page = await getPage(`${path}?offset=${offset}`, fetchImpl);
    if (page.kind !== "ok") return page;
    for (const item of page.value.items as WorkMonthRow[]) {
      const key = JSON.stringify([item.contractId, item.yearMonth]);
      if (seen.has(key)) continue;
      seen.add(key);
      rows.push(item);
    }
    offset += page.value.items.length;
    if (page.value.items.length === 0 || offset >= page.value.total) {
      return { kind: "ok", value: rows };
    }
  }
}

export function fetchOwnWorkMonths(fetchImpl: typeof fetch = fetch): Promise<FetchResult<WorkMonthRow[]>> {
  return fetchAllWorkMonths(OWN_PATH, fetchImpl);
}

export function fetchPendingApprovalWorkMonths(
  fetchImpl: typeof fetch = fetch,
): Promise<FetchResult<WorkMonthRow[]>> {
  return fetchAllWorkMonths(PENDING_PATH, fetchImpl);
}

export async function fetchDemoWorkMonths(
  fetchImpl: typeof fetch = fetch,
): Promise<FetchResult<WorkMonthRow[]>> {
  const result = await getJson(DEMO_PATH, fetchImpl);
  if (result.kind !== "ok") return result;
  return { kind: "ok", value: result.value.items as WorkMonthRow[] };
}

export async function fetchOwnContracts(
  now: Date,
  fetchImpl: typeof fetch = fetch,
): Promise<FetchResult<ContractSummary[]>> {
  const query = new URLSearchParams({ yearMonth: currentJstYearMonth(now) });
  const result = await getJson(`${CONTRACTS_PATH}?${query.toString()}`, fetchImpl);
  if (result.kind !== "ok") return result;
  return { kind: "ok", value: result.value.items as ContractSummary[] };
}
