"use client";

// docs/specs/web-screens-implementation-design.md M-2〜M-10。勤務月画面の本体（クライアント）。
// 勤務月の取得（R-1）と操作（R-5〜R-9）はブラウザから BFF の Route Handler へ送る。
// 祝日の判定は props の isHoliday で受ける（lib/holidays/ を輸入しない）。

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/button";
import { Card } from "@/components/card";
import { EmptyState } from "@/components/empty-state";
import { ErrorBanner } from "@/components/error-banner";
import { StatusBadge } from "@/components/status-badge";
import { currentJstDate } from "@/lib/jst-month";
import { defaultScreenPath } from "@/lib/screen-access";
import type { Viewer } from "@/lib/viewer";
import { formatYearMonth } from "@/lib/work-month-list";
import { type Ownership, type WorkMonthDetail } from "@/lib/work-month";
import { workMonthActions, workMonthNote, type Operator } from "@/lib/work-month-actions";
import { calendarCells, firstInputDay } from "@/lib/work-month-calendar";
import {
  approveWorkMonth,
  closeWorkMonth,
  deleteDailyRecord,
  fetchWorkMonth,
  rejectWorkMonth,
  saveDailyRecord,
  type DailyRecordInput,
  type WorkMonthResult,
} from "@/lib/work-month-client";
import {
  OPERATION_FAILURE_MESSAGES,
  WORK_MONTH_LOAD_ERROR_MESSAGE,
  WORK_MONTH_MISSING,
  loadFailureKind,
  operationFailureKind,
  type OperationFailureKind,
} from "@/lib/work-month-feedback";
import { DailyRecordForm } from "./daily-record-form";
import { WorkMonthCalendar } from "./work-month-calendar";
import { CloseAction, ReviewActions } from "./work-month-operations";
import { WorkMonthSummary } from "./work-month-summary";

export interface WorkMonthViewBaseProps {
  viewer: Viewer;
  contractId: string;
  yearMonth: string;
}

type LoadState =
  | { kind: "failed"; forKey: string }
  | { kind: "missing"; forKey: string }
  | { kind: "loaded"; forKey: string; value: WorkMonthDetail; ownership: Ownership };

function viewerKey(viewer: Viewer): string {
  return viewer.kind === "guest" ? "guest" : `signed-in:${viewer.role}`;
}

export function WorkMonthView({
  viewer,
  contractId,
  yearMonth,
  isHoliday,
}: WorkMonthViewBaseProps & { isHoliday: (date: string) => boolean }) {
  const router = useRouter();
  const routerRef = useRef(router);
  useEffect(() => {
    routerRef.current = router;
  });

  const key = viewerKey(viewer);
  const [load, setLoad] = useState<LoadState | undefined>(undefined);
  const [attempt, setAttempt] = useState(0);
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<OperationFailureKind | null>(null);
  // 締めた直後の注記は、締めた時点の取得のキーと一緒に持つ。キーが変われば偽になる。
  const fetchKey = `${contractId}|${yearMonth}|${key}`;
  const [closedFor, setClosedFor] = useState<string | undefined>(undefined);
  const justClosed = closedFor === fetchKey;
  const busyRef = useRef(false);
  const seq = useRef(0);

  // 取得の結果を反映する。置き換えは最新の要求のものだけ（古い応答は捨てる）。
  const applyLoad = (result: WorkMonthResult, mine: number, forKey: string): boolean => {
    if (mine !== seq.current) return false;
    if (result.kind === "ok") {
      setLoad({ kind: "loaded", forKey, value: result.value, ownership: result.ownership });
    } else if (result.kind === "refresh") {
      routerRef.current.refresh();
    } else {
      setFailure(null);
      setLoad({ kind: loadFailureKind(result), forKey });
    }
    return true;
  };

  useEffect(() => {
    const mine = ++seq.current;
    void fetchWorkMonth(contractId, yearMonth).then((result) => applyLoad(result, mine, key));
    return () => {
      seq.current += 1;
    };
  }, [contractId, yearMonth, key, attempt]);

  const retry = () => {
    setLoad(undefined);
    setAttempt((n) => n + 1);
  };

  const run = async (operation: () => Promise<WorkMonthResult>, afterOk?: () => void) => {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setFailure(null);
    const mine = seq.current;
    const result = await operation();
    busyRef.current = false;
    setBusy(false);
    if (mine !== seq.current) return;
    if (result.kind === "ok") {
      setLoad({ kind: "loaded", forKey: key, value: result.value, ownership: result.ownership });
      afterOk?.();
    } else if (result.kind === "refresh") {
      routerRef.current.refresh();
    } else {
      const kind = operationFailureKind(result);
      setFailure(kind);
      if (kind === "state-changed") {
        // 409: 最新の表現を取り直す（取り直す間は操作要素を出さない）。
        const reload = ++seq.current;
        setLoad(undefined);
        setClosedFor(undefined);
        void fetchWorkMonth(contractId, yearMonth).then((again) => applyLoad(again, reload, key));
      }
    }
  };

  const state = load !== undefined && load.forKey === key ? load : undefined;

  return (
    <div className="flex flex-col gap-4" aria-busy={state === undefined ? "true" : undefined}>
      {state?.kind === "failed" ? (
        <ErrorBanner onRetry={retry}>{WORK_MONTH_LOAD_ERROR_MESSAGE}</ErrorBanner>
      ) : null}
      {state?.kind === "missing" ? (
        <EmptyState
          title={WORK_MONTH_MISSING.title}
          description={WORK_MONTH_MISSING.description}
          action={
            <Link
              href={defaultScreenPath(viewer)}
              className="text-primary underline focus-visible:ring-2 focus-visible:ring-focus-ring"
            >
              {WORK_MONTH_MISSING.linkLabel}
            </Link>
          }
        />
      ) : null}
      {state?.kind === "loaded" ? renderLoaded(state.value, state.ownership) : null}
    </div>
  );

  function renderLoaded(workMonth: WorkMonthDetail, ownership: Ownership) {
    const operator: Operator =
      viewer.kind === "guest" ? { kind: "guest" } : { kind: "signed-in", role: viewer.role, ownership };
    const actions = workMonthActions({ state: workMonth.state, generated: workMonth.generated, operator });
    const cells = calendarCells({
      yearMonth: workMonth.yearMonth,
      records: workMonth.dailyRecords,
      isHoliday,
      today: currentJstDate(new Date()),
    });
    const selectedCell = cells.find(
      (cell): cell is Extract<typeof cell, { kind: "day" }> => cell.kind === "day" && cell.date === selectedDate,
    );

    const save = (date: string) => (input: DailyRecordInput) =>
      run(() => saveDailyRecord(contractId, yearMonth, date, input));

    return (
      <>
        {viewer.kind === "guest" ? (
          <p role="note" className="text-sm text-muted-foreground">
            デモ用のサンプルです（閲覧のみ・毎日リセットされます）
          </p>
        ) : null}
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-xl font-semibold">
            {`${workMonth.contractDisplayName} × ${formatYearMonth(workMonth.yearMonth)}`}
          </h1>
          <StatusBadge state={workMonth.state} />
        </div>
        <p className="text-sm text-muted-foreground">
          {workMonthNote({ state: workMonth.state, operator, actions, justClosed })}
        </p>
        <WorkMonthSummary workMonth={workMonth} />
        {failure !== null ? <ErrorBanner>{OPERATION_FAILURE_MESSAGES[failure]}</ErrorBanner> : null}
        <Card title="日次の稼働実績">
          <WorkMonthCalendar
            cells={cells}
            editable={actions.edit}
            selectedDate={selectedDate}
            onSelect={setSelectedDate}
          />
        </Card>
        {actions.edit ? (
          selectedCell !== undefined ? (
            <Card>
              <DailyRecordForm
                key={`${selectedCell.date}:${selectedCell.record === undefined ? "new" : "saved"}`}
                cell={selectedCell}
                busy={busy}
                onSave={save(selectedCell.date)}
                onDelete={() => void run(() => deleteDailyRecord(contractId, yearMonth, selectedCell.date))}
                onClose={() => setSelectedDate(null)}
              />
            </Card>
          ) : workMonth.dailyRecords.length === 0 ? (
            <EmptyState
              title="まだ稼働実績がありません"
              description="カレンダーの日を選んで、稼働時間を入力してください。最初の入力でこの勤務月が作られます。"
              action={
                <Button variant="primary" onClick={() => setSelectedDate(firstInputDay(cells))}>
                  稼働実績を入力する
                </Button>
              }
            />
          ) : (
            <Card title="稼働実績の入力">
              <p className="text-sm">カレンダーの日を選ぶと、ここで稼働時間を入力・編集・削除できます。</p>
            </Card>
          )
        ) : null}
        {actions.close ? (
          <CloseAction
            yearMonth={workMonth.yearMonth}
            busy={busy}
            onConfirm={() => void run(() => closeWorkMonth(contractId, yearMonth), () => setClosedFor(fetchKey))}
          />
        ) : null}
        {actions.approve || actions.reject ? (
          <ReviewActions
            busy={busy}
            onApprove={() => void run(() => approveWorkMonth(contractId, yearMonth))}
            onReject={() => void run(() => rejectWorkMonth(contractId, yearMonth))}
          />
        ) : null}
      </>
    );
  }
}
