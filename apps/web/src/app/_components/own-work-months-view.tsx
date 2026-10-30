"use client";

// docs/specs/web-screens-implementation-design.md W-2〜W-4。本人一覧。
// 取得・絞り込み・ページはブラウザで行い、URL へ書き戻さない。

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/button";
import { Card } from "@/components/card";
import { EmptyState } from "@/components/empty-state";
import { ErrorBanner } from "@/components/error-banner";
import { Pagination } from "@/components/pagination";
import { StatusBadge } from "@/components/status-badge";
import { thisMonthEntry } from "@/lib/this-month-entry";
import { workMonthPath } from "@/lib/screen-routes";
import {
  EMPTY_WORK_MONTH_FILTER,
  contractOptionsFromContracts,
  filterWorkMonths,
  formatYearMonth,
  latestRow,
  pageCount,
  pageRows,
  stateOptions,
  yearMonthOptions,
  type ContractSummary,
  type WorkMonthFilter,
  type WorkMonthRow,
} from "@/lib/work-month-list";
import { fetchOwnContracts, fetchOwnWorkMonths } from "@/lib/work-month-list-client";
import { InitialEntryAction } from "./initial-entry-action";
import { WorkMonthFilters } from "./work-month-filters";
import { WorkMonthTable } from "./work-month-table";

const ERROR_TEXT = "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";

type Loaded = { rows: WorkMonthRow[]; contracts: ContractSummary[] };
type LoadState = { kind: "loading" } | { kind: "failed" } | { kind: "loaded"; value: Loaded };

export function OwnWorkMonthsView({ initialContractId }: { initialContractId?: string }) {
  const router = useRouter();
  const routerRef = useRef(router);
  useEffect(() => {
    routerRef.current = router;
  });

  const [attempt, setAttempt] = useState(0);
  const [load, setLoad] = useState<LoadState>({ kind: "loading" });
  const [filter, setFilter] = useState<WorkMonthFilter>(EMPTY_WORK_MONTH_FILTER);
  const [page, setPage] = useState(1);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const [rows, contracts] = await Promise.all([
        fetchOwnWorkMonths(),
        fetchOwnContracts(new Date()),
      ]);
      if (cancelled) return;
      if (rows.kind === "refresh" || contracts.kind === "refresh") {
        routerRef.current.refresh();
        return;
      }
      if (rows.kind === "failed" || contracts.kind === "failed") {
        setLoad({ kind: "failed" });
        return;
      }
      const known = contracts.value.some((c) => c.contractId === initialContractId);
      setFilter({ ...EMPTY_WORK_MONTH_FILTER, contractId: known ? (initialContractId ?? "") : "" });
      setPage(1);
      setLoad({ kind: "loaded", value: { rows: rows.value, contracts: contracts.value } });
    })();
    return () => {
      cancelled = true;
    };
  }, [attempt, initialContractId]);

  const retry = () => {
    setLoad({ kind: "loading" });
    setAttempt((n) => n + 1);
  };
  const clear = () => {
    setFilter(EMPTY_WORK_MONTH_FILTER);
    setPage(1);
  };

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 className="text-xl font-semibold">自分の勤務月</h1>
        <p className="text-sm text-muted-foreground">担当しているすべての契約の勤務月です。</p>
      </div>
      <div aria-busy={load.kind === "loading" ? "true" : undefined} className="flex flex-col gap-4">
        {load.kind === "failed" ? <ErrorBanner onRetry={retry}>{ERROR_TEXT}</ErrorBanner> : null}
        {load.kind === "loaded" ? renderLoaded(load.value) : null}
      </div>
    </div>
  );

  function renderLoaded({ rows, contracts }: Loaded) {
    if (rows.length === 0) {
      return (
        <EmptyState
          title="勤務月がまだありません"
          description="稼働実績を入力すると、その月の勤務月がここに表示されます。"
          action={<InitialEntryAction entry={thisMonthEntry(contracts)} />}
        />
      );
    }
    const latest = latestRow(rows);
    const filtered = filterWorkMonths(rows, filter);
    return (
      <>
        {latest !== undefined ? (
          <Card title="直近の勤務月">
            <div className="flex flex-wrap items-center gap-3">
              <span>{formatYearMonth(latest.yearMonth)}</span>
              <span>{latest.contractDisplayName}</span>
              <StatusBadge state={latest.state} />
              <Link
                href={workMonthPath(latest.contractId, latest.yearMonth)}
                className="text-primary underline focus-visible:ring-2 focus-visible:ring-focus-ring"
              >
                直近の勤務月を開く
              </Link>
            </div>
          </Card>
        ) : null}
        <WorkMonthFilters
          filter={filter}
          contractOptions={contractOptionsFromContracts(contracts)}
          yearMonthOptions={yearMonthOptions(rows)}
          stateOptions={stateOptions()}
          onChange={(next) => {
            setFilter(next);
            setPage(1);
          }}
          onClear={clear}
        />
        {filtered.length === 0 ? (
          <EmptyState
            title="条件に合う勤務月はありません"
            description="絞り込みの条件を変えるか、クリアしてください。"
            action={<Button onClick={clear}>条件をクリア</Button>}
          />
        ) : (
          <>
            <Card>
              <WorkMonthTable label="すべての勤務月" rows={pageRows(filtered, page)} />
            </Card>
            <Pagination page={page} pageCount={pageCount(filtered.length)} onPageChange={setPage} />
          </>
        )}
      </>
    );
  }
}
