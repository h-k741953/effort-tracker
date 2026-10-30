// docs/specs/web-screens-implementation-design.md M-1・T-0-2。勤務月画面の本体。
// 祝日のデータは view が持つ（lib/holidays/ を輸入しない）。fetch を呼ばない。

import type { ComponentType } from "react";
import { redirect } from "next/navigation";
import { decideScreen } from "@/lib/screen-access";
import type { ScreenDeps } from "../_lib/screen-deps";
import { AppShell } from "./app-shell";
import type { WorkMonthViewBaseProps } from "./work-month-view";

export async function workMonthScreen(
  deps: ScreenDeps,
  params: { contractId: string; yearMonth: string },
  View: ComponentType<WorkMonthViewBaseProps>,
) {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    throw new Error("認証の構成が欠けています。");
  }
  const decision = decideScreen("work-month", viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }
  const theme = await deps.getTheme();
  const { contractId, yearMonth } = params;
  return (
    <AppShell viewer={viewer} theme={theme} current={{ kind: "work-month", contractId, yearMonth }}>
      <View viewer={viewer} contractId={contractId} yearMonth={yearMonth} />
    </AppShell>
  );
}
