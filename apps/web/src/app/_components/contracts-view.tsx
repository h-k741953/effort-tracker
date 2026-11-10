"use client";

// docs/specs/readonly-screens-implementation-design.md KP-1・KP-2。契約一覧の中身。
// 取得はブラウザで行う。状態を変える操作を持たない。

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Card } from "@/components/card";
import { EmptyState } from "@/components/empty-state";
import { ErrorBanner } from "@/components/error-banner";
import type { ContractListItem } from "@/lib/contract-list";
import { fetchContractList } from "@/lib/work-month-list-client";
import { ContractsTable } from "./contracts-table";

const ERROR_TEXT = "契約を読み込めませんでした。通信状態を確認して、もう一度お試しください。";

type LoadState = { kind: "loading" } | { kind: "failed" } | { kind: "loaded"; value: ContractListItem[] };

export function ContractsView() {
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
      const result = await fetchContractList(new Date());
      if (cancelled) return;
      if (result.kind === "refresh") {
        routerRef.current.refresh();
        return;
      }
      if (result.kind === "failed") {
        setLoad({ kind: "failed" });
        return;
      }
      setLoad({ kind: "loaded", value: result.value });
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
      <div>
        <h1 className="text-xl font-semibold">契約一覧</h1>
        <p className="text-sm text-muted-foreground">担当している契約の精算幅と契約期間です。</p>
      </div>
      <div aria-busy={load.kind === "loading" ? "true" : undefined} className="flex flex-col gap-4">
        {load.kind === "failed" ? <ErrorBanner onRetry={retry}>{ERROR_TEXT}</ErrorBanner> : null}
        {load.kind === "loaded" ? (
          load.value.length === 0 ? (
            <EmptyState title="担当している契約はありません" />
          ) : (
            <Card>
              <ContractsTable contracts={load.value} />
            </Card>
          )
        ) : null}
      </div>
    </div>
  );
}
