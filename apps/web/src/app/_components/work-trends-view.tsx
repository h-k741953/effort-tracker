"use client";

// docs/specs/readonly-screens-implementation-design.md SP-2・SP-3。稼働の推移の中身。
// 取得はブラウザで行い、契約・期間は取得済みの全件をブラウザで絞る。状態を変える操作を持たない。

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { EmptyState } from "@/components/empty-state";
import { ErrorBanner } from "@/components/error-banner";
import type { ContractListItem } from "@/lib/contract-list";
import { contractOptionsFromContracts } from "@/lib/work-month-list";
import { fetchContractList, fetchWorkTrends } from "@/lib/work-month-list-client";
import {
  defaultPeriod,
  periodOptions,
  rowsInPeriod,
  rowsOfContract,
  type TrendPeriod,
  type WorkTrendRow,
} from "@/lib/work-trends";
import { WorkTrendsChart } from "./work-trends-chart";
import { WorkTrendsFilters } from "./work-trends-filters";
import { WorkTrendsSummary } from "./work-trends-summary";

const ERROR_TEXT = "稼働の推移を読み込めませんでした。通信状態を確認して、もう一度お試しください。";

type Loaded = { contracts: ContractListItem[]; rows: WorkTrendRow[] };
type LoadState = { kind: "loading" } | { kind: "failed" } | { kind: "loaded"; value: Loaded };

export function WorkTrendsView() {
  const router = useRouter();
  const routerRef = useRef(router);
  useEffect(() => {
    routerRef.current = router;
  });

  const [attempt, setAttempt] = useState(0);
  const [load, setLoad] = useState<LoadState>({ kind: "loading" });
  const [contractId, setContractId] = useState("");
  const [period, setPeriod] = useState<TrendPeriod | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const [contracts, trends] = await Promise.all([fetchContractList(new Date()), fetchWorkTrends()]);
      if (cancelled) return;
      if (contracts.kind === "refresh" || trends.kind === "refresh") {
        routerRef.current.refresh();
        return;
      }
      if (contracts.kind === "failed" || trends.kind === "failed") {
        setLoad({ kind: "failed" });
        return;
      }
      const first = contracts.value[0]?.contractId ?? "";
      setContractId(first);
      setPeriod(defaultPeriod(rowsOfContract(trends.value, first)));
      setLoad({ kind: "loaded", value: { contracts: contracts.value, rows: trends.value } });
    })();
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  const retry = () => {
    setLoad({ kind: "loading" });
    setAttempt((n) => n + 1);
  };

  const loaded = load.kind === "loaded" ? load.value : undefined;
  const contractRows = useMemo(
    () => (loaded === undefined ? [] : rowsOfContract(loaded.rows, contractId)),
    [loaded, contractId],
  );

  const changeContract = (next: string) => {
    setContractId(next);
    setPeriod(defaultPeriod(rowsOfContract(loaded?.rows ?? [], next)));
  };

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 className="text-xl font-semibold">稼働の推移</h1>
        <p className="text-sm text-muted-foreground">月ごとの総稼働時間と、契約の精算幅との関係です。</p>
      </div>
      <div aria-busy={load.kind === "loading" ? "true" : undefined} className="flex flex-col gap-4">
        {load.kind === "failed" ? <ErrorBanner onRetry={retry}>{ERROR_TEXT}</ErrorBanner> : null}
        {loaded !== undefined ? renderLoaded(loaded) : null}
      </div>
    </div>
  );

  function renderLoaded({ contracts }: Loaded) {
    if (contracts.length === 0) {
      return <EmptyState title="担当している契約はありません" />;
    }
    const hasRows = contractRows.length > 0 && period !== undefined;
    return (
      <>
        <WorkTrendsFilters
          contractId={contractId}
          contractOptions={contractOptionsFromContracts(contracts)}
          period={hasRows ? period : undefined}
          periodOptions={periodOptions(contractRows)}
          onContractChange={changeContract}
          onPeriodChange={setPeriod}
        />
        {hasRows ? (
          <>
            <WorkTrendsSummary rows={rowsInPeriod(contractRows, period)} />
            <WorkTrendsChart rows={rowsInPeriod(contractRows, period)} />
          </>
        ) : (
          <EmptyState title="この契約の勤務月はまだありません" />
        )}
      </>
    );
  }
}
