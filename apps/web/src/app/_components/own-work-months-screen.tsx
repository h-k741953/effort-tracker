// docs/specs/web-screens-implementation-design.md W-1・T-0-2。本人の勤務月一覧の本体。

import { redirect } from "next/navigation";
import { decideScreen } from "@/lib/screen-access";
import type { ScreenDeps } from "../_lib/screen-deps";
import { AppShell } from "./app-shell";
import { OwnWorkMonthsView } from "./own-work-months-view";

export async function ownWorkMonthsScreen(
  deps: ScreenDeps,
  searchParams: { [key: string]: string | string[] | undefined },
) {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    throw new Error("認証の構成が欠けています。");
  }
  const decision = decideScreen("own-work-months", viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }
  const theme = await deps.getTheme();
  const contract = searchParams.contract;
  return (
    <AppShell viewer={viewer} theme={theme} current="own-work-months">
      <OwnWorkMonthsView initialContractId={typeof contract === "string" ? contract : undefined} />
    </AppShell>
  );
}
