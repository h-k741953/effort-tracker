"use client";

// docs/specs/web-screens-implementation-design.md W-2・W-5。デモ勤務月群。絞り込み・ページ・直近を置かない（9-3）。

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Card } from "@/components/card";
import { EmptyState } from "@/components/empty-state";
import { ErrorBanner } from "@/components/error-banner";
import type { WorkMonthRow } from "@/lib/work-month-list";
import { fetchDemoWorkMonths } from "@/lib/work-month-list-client";
import { WorkMonthTable } from "./work-month-table";

const ERROR_TEXT = "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";
const TITLE = "サンプル勤務月";

type LoadState =
  | { kind: "loading" }
  | { kind: "failed" }
  | { kind: "loaded"; rows: WorkMonthRow[] };

export function DemoWorkMonthsView() {
  const router = useRouter();
  const routerRef = useRef(router);
  useEffect(() => {
    routerRef.current = router;
  });

  const [attempt, setAttempt] = useState(0);
  const [load, setLoad] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const result = await fetchDemoWorkMonths();
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

  return (
    <div className="flex flex-col gap-4">
      <div role="note" className="rounded-(--et-radius-card) border border-border p-3 text-sm">
        <p className="font-medium">デモ用のサンプルです（閲覧のみ・毎日リセットされます）</p>
        <p>ゲストとして閲覧しています。入力・締め・承認はできません。</p>
      </div>
      <h1 className="text-xl font-semibold">{TITLE}</h1>
      <div aria-busy={load.kind === "loading" ? "true" : undefined} className="flex flex-col gap-4">
        {load.kind === "failed" ? <ErrorBanner onRetry={retry}>{ERROR_TEXT}</ErrorBanner> : null}
        {load.kind === "loaded" && load.rows.length === 0 ? (
          <EmptyState title="サンプル勤務月はありません" />
        ) : null}
        {load.kind === "loaded" && load.rows.length > 0 ? (
          <Card>
            <WorkMonthTable label={TITLE} rows={load.rows} />
          </Card>
        ) : null}
      </div>
    </div>
  );
}
