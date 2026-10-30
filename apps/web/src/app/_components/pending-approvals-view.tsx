"use client";

// docs/specs/web-screens-implementation-design.md W-2・W-3・W-5。承認待ち一覧。

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/button";
import { Card } from "@/components/card";
import { EmptyState } from "@/components/empty-state";
import { ErrorBanner } from "@/components/error-banner";
import { Pagination } from "@/components/pagination";
import {
  EMPTY_WORK_MONTH_FILTER,
  contractOptionsFromRows,
  filterWorkMonths,
  pageCount,
  pageRows,
  yearMonthOptions,
  type WorkMonthFilter,
  type WorkMonthRow,
} from "@/lib/work-month-list";
import { fetchPendingApprovalWorkMonths } from "@/lib/work-month-list-client";
import { WorkMonthFilters } from "./work-month-filters";
import { WorkMonthTable } from "./work-month-table";

const ERROR_TEXT = "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";
const TABLE_LABEL = "承認待ちの勤務月";

type LoadState =
  | { kind: "loading" }
  | { kind: "failed" }
  | { kind: "loaded"; rows: WorkMonthRow[] };

export function PendingApprovalsView() {
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
      const result = await fetchPendingApprovalWorkMonths();
      if (cancelled) return;
      if (result.kind === "refresh") {
        routerRef.current.refresh();
        return;
      }
      setLoad(result.kind === "ok" ? { kind: "loaded", rows: result.value } : { kind: "failed" });
    })();
    return () => {
      cancelled = true;
    };
  }, [attempt]);

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
        <h1 className="text-xl font-semibold">承認待ちの勤務月</h1>
        <p className="text-sm text-muted-foreground">
          勤務月を開いて内容を確認し、承認または差し戻しをしてください。
        </p>
      </div>
      <div aria-busy={load.kind === "loading" ? "true" : undefined} className="flex flex-col gap-4">
        {load.kind === "failed" ? <ErrorBanner onRetry={retry}>{ERROR_TEXT}</ErrorBanner> : null}
        {load.kind === "loaded" ? renderLoaded(load.rows) : null}
      </div>
    </div>
  );

  function renderLoaded(rows: WorkMonthRow[]) {
    if (rows.length === 0) {
      return (
        <EmptyState
          title="承認待ちの勤務月はありません"
          description="技術者が勤務月を締めると、ここに表示されます。"
        />
      );
    }
    const filtered = filterWorkMonths(rows, filter);
    return (
      <>
        <WorkMonthFilters
          filter={filter}
          contractOptions={contractOptionsFromRows(rows)}
          yearMonthOptions={yearMonthOptions(rows)}
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
              <WorkMonthTable label={TABLE_LABEL} rows={pageRows(filtered, page)} />
            </Card>
            <Pagination page={page} pageCount={pageCount(filtered.length)} onPageChange={setPage} />
          </>
        )}
      </>
    );
  }
}
