// docs/specs/web-screens-implementation-design.md W-1・T-0-2。承認待ち一覧の本体。

import { redirect } from "next/navigation";
import { decideScreen } from "@/lib/screen-access";
import type { ScreenDeps } from "../_lib/screen-deps";
import { AppShell } from "./app-shell";
import { PendingApprovalsView } from "./pending-approvals-view";

export async function pendingApprovalsScreen(deps: ScreenDeps) {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    throw new Error("認証の構成が欠けています。");
  }
  const decision = decideScreen("pending-approvals", viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }
  const theme = await deps.getTheme();
  return (
    <AppShell viewer={viewer} theme={theme} current="pending-approvals">
      <PendingApprovalsView />
    </AppShell>
  );
}
